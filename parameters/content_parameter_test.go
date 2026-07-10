// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	validatorcache "github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi-validator/config"
	validatorErrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/internal/requeststate"
	"github.com/pb33f/libopenapi-validator/router"
)

const contentParameterSpec = `openapi: 3.1.0
info: {title: params, version: 1.0.0}
paths:
  /items/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          content: {application/json: {schema: {type: integer, minimum: 1}}}
        - name: filter
          in: query
          required: true
          content:
            application/json:
              schema:
                type: object
                required: [active]
                properties: {active: {type: boolean}}
        - name: X-Data
          in: header
          required: true
          content: {application/json: {schema: {type: string, minLength: 2}}}
        - name: preference
          in: cookie
          required: true
          content: {application/json: {schema: {type: boolean}}}
      responses: {"204": {description: ok}}
`

func contentParameterModel(t *testing.T, specification string) *v3.Document {
	t.Helper()
	document, err := libopenapi.NewDocument([]byte(specification))
	require.NoError(t, err)
	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	return &model.Model
}

func TestContentParametersAllLocations(t *testing.T) {
	model := contentParameterModel(t, contentParameterSpec)
	routeFinder := router.NewRouter(model, router.WithPathOnlyMatching())
	opts := config.NewValidationOptions(config.WithContentParameterValidation())
	opts.Router = routeFinder
	validator := NewParameterValidator(model, config.WithExistingOpts(opts))
	t.Cleanup(func() {
		validator.Release()
		routeFinder.Release()
	})
	request, err := http.NewRequest(http.MethodGet, "http://example.com/items/12?filter="+url.QueryEscape(`{"active":true}`), nil)
	require.NoError(t, err)
	request.Header.Set("X-Data", `"ok"`)
	request.AddCookie(&http.Cookie{Name: "preference", Value: "true"})

	for _, validate := range []func(*http.Request) (bool, []*ValidationErrorAlias){
		validator.ValidatePathParams, validator.ValidateQueryParams, validator.ValidateHeaderParams, validator.ValidateCookieParams,
	} {
		valid, validationErrors := validate(request)
		require.True(t, valid, validationErrors)
		require.Empty(t, validationErrors)
	}
}

type ValidationErrorAlias = validatorErrors.ValidationError

func TestContentParameterMalformedMissingAndSchemaViolation(t *testing.T) {
	model := contentParameterModel(t, contentParameterSpec)
	validator := NewParameterValidator(model, config.WithContentParameterValidation())
	t.Cleanup(validator.Release)

	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/0?filter=not-json", nil)
	request.Header.Set("X-Data", `"x"`)
	request.AddCookie(&http.Cookie{Name: "preference", Value: "not-json"})
	for _, validate := range []func(*http.Request) (bool, []*ValidationErrorAlias){
		validator.ValidatePathParams, validator.ValidateQueryParams, validator.ValidateHeaderParams, validator.ValidateCookieParams,
	} {
		valid, validationErrors := validate(request)
		assert.False(t, valid)
		assert.NotEmpty(t, validationErrors)
	}

	missing, _ := http.NewRequest(http.MethodGet, "http://example.com/items/2", nil)
	valid, validationErrors := validator.ValidateQueryParams(missing)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "required")
	valid, validationErrors = validator.ValidateHeaderParams(missing)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "required")
}

func TestCustomContentParameterDecoder(t *testing.T) {
	model := contentParameterModel(t, contentParameterSpec)
	var inputs []*config.ContentParameterInput
	decoder := func(ctx context.Context, input *config.ContentParameterInput) (any, *base.Schema, error) {
		require.NotNil(t, ctx)
		inputs = append(inputs, input)
		if input.Parameter.Name == "filter" {
			return map[string]any{"active": true}, input.DefaultSchema, nil
		}
		return nil, input.DefaultSchema, errors.New("custom failure")
	}
	validator := NewParameterValidator(model, config.WithContentParameterDecoder(decoder))
	t.Cleanup(validator.Release)
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/2?filter=one&filter=two", nil)
	valid, validationErrors := validator.ValidateQueryParams(request)
	require.True(t, valid, validationErrors)
	require.Len(t, inputs, 1)
	assert.Equal(t, []string{"one", "two"}, inputs[0].RawValues)
	assert.Equal(t, "application/json", inputs[0].MediaType)
	assert.Same(t, request, inputs[0].Request)

	request.Header.Set("X-Data", `"ok"`)
	valid, validationErrors = validator.ValidateHeaderParams(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "custom failure")
}

func TestCustomContentParameterSchemaIdentityAndUnsupportedMedia(t *testing.T) {
	model := contentParameterModel(t, contentParameterSpec)
	decoder := func(context.Context, *config.ContentParameterInput) (any, *base.Schema, error) {
		return map[string]any{"active": true}, &base.Schema{Type: []string{"object"}}, nil
	}
	validator := NewParameterValidator(model, config.WithContentParameterDecoder(decoder))
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/2?filter=value", nil)
	valid, validationErrors := validator.ValidateQueryParams(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "low-level")
	validator.Release()

	unsupportedSpec := `openapi: 3.1.0
info: {title: unsupported, version: 1.0.0}
paths:
  /items:
    get:
      parameters:
        - name: data
          in: header
          content: {text/plain: {schema: {type: string}}}
      responses: {"204": {description: ok}}`
	model = contentParameterModel(t, unsupportedSpec)
	validator = NewParameterValidator(model)
	request, _ = http.NewRequest(http.MethodGet, "http://example.com/items", nil)
	valid, validationErrors = validator.ValidateHeaderParams(request)
	require.True(t, valid, validationErrors)
	request.Header.Set("data", "value")
	valid, validationErrors = validator.ValidateHeaderParams(request)
	require.True(t, valid, validationErrors)
	validator.Release()

	validator = NewParameterValidator(model, config.WithContentParameterValidation())
	valid, validationErrors = validator.ValidateHeaderParams(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "no content-parameter decoder")
	validator.Release()
}

func TestLegacyBuiltInQueryContentPreservesRepeatedValues(t *testing.T) {
	model := contentParameterModel(t, contentParameterSpec)
	validator := NewParameterValidator(model, config.WithContentParameterValidation())
	t.Cleanup(validator.Release)
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/2?filter=%7B%22active%22%3Atrue%7D&filter=%7B%22active%22%3Afalse%7D", nil)
	valid, validationErrors := validator.ValidateQueryParams(request)
	require.True(t, valid, validationErrors)
	assert.Empty(t, validationErrors)
}

func TestContentParameterHelpers(t *testing.T) {
	assert.Equal(t, []string{"a/b"}, pathParameterValues("/items/a%2Fb", "/items/{id}", "id"))
	assert.Nil(t, pathParameterValues("/items/x", "/items/{", "id"))
	assert.Nil(t, pathParameterValues("/other/x", "/items/{id}", "id"))
	assert.Nil(t, pathParameterValues("/items/x", "/items/{id}", "other"))
	assert.Equal(t, []string{"%zz"}, pathParameterValues("/items/%zz", "/items/{id}", "id"))
	assert.Empty(t, parameterLocationLabel(""))
	assert.Equal(t, "Query", parameterLocationLabel("query"))
	assert.Equal(t, helpers.ParameterValidation, parameterSubtype("unknown"))

	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/1?a=one&a=two", nil)
	request.Header.Add("X", "one")
	request.Header.Add("X", "two")
	request.AddCookie(&http.Cookie{Name: "cookie", Value: "value"})
	assert.Equal(t, []string{"one", "two"}, rawParameterValues(request, &v3.Parameter{Name: "a", In: helpers.Query}, "", nil))
	assert.Equal(t, []string{"one", "two"}, rawParameterValues(request, &v3.Parameter{Name: "X", In: helpers.Header}, "", nil))
	assert.Equal(t, []string{"value"}, rawParameterValues(request, &v3.Parameter{Name: "cookie", In: helpers.Cookie}, "", nil))
	assert.Nil(t, rawParameterValues(request, &v3.Parameter{Name: "missing", In: helpers.Cookie}, "", nil))
	assert.Equal(t, []string{"route"}, rawParameterValues(request, &v3.Parameter{Name: "id", In: helpers.Path}, "/items/{id}", map[string]string{"id": "route"}))
	assert.Nil(t, rawParameterValues(request, &v3.Parameter{Name: "x", In: "unknown"}, "", nil))

	errorValue := contentParameterError(&v3.Parameter{Name: "x"}, helpers.Header, "bad")
	assert.Equal(t, "x", errorValue.ParameterName)
}

func TestCustomContentParameterReceivesServerVariables(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: route context, version: 1.0.0}
servers:
  - url: /api/{version}
    variables: {version: {default: v1}}
paths:
  /items:
    get:
      parameters:
        - name: data
          in: query
          content: {application/json: {schema: {type: string}}}
      responses: {"204": {description: ok}}`
	model := contentParameterModel(t, spec)
	routeFinder := router.NewRouter(model)
	called := false
	decoder := func(_ context.Context, input *config.ContentParameterInput) (any, *base.Schema, error) {
		called = true
		assert.Equal(t, "v2", input.ServerVariables["version"])
		return "ok", input.DefaultSchema, nil
	}
	counter := &countingParameterRouter{Router: routeFinder}
	opts := config.NewValidationOptions(config.WithContentParameterDecoder(decoder))
	opts.Router = counter
	validator := NewParameterValidator(model, config.WithExistingOpts(opts))
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/api/v2/items?data=value", nil)
	resolved, resolveErr := routeFinder.FindRoute(request)
	require.NoError(t, resolveErr)
	valid, validationErrors := validator.ValidateQueryParamsWithPathItem(request, resolved.PathItem, resolved.Path)
	require.True(t, valid, validationErrors)
	assert.Equal(t, int64(1), counter.calls.Load())
	counter.calls.Store(0)
	restoreRoute := requeststate.AttachRoute(request, resolved)
	defer restoreRoute()
	valid, validationErrors = validator.ValidateQueryParamsWithPathItem(request, resolved.PathItem, resolved.Path)
	require.True(t, valid, validationErrors)
	assert.True(t, called)
	assert.Zero(t, counter.calls.Load())
	validator.Release()
	routeFinder.Release()
}

type countingSchemaCache struct {
	inner  validatorcache.SchemaCache
	loads  int
	stores int
}

func (c *countingSchemaCache) Load(key uint64) (*validatorcache.SchemaCacheEntry, bool) {
	c.loads++
	return c.inner.Load(key)
}

func (c *countingSchemaCache) Store(key uint64, value *validatorcache.SchemaCacheEntry) {
	c.stores++
	c.inner.Store(key, value)
}

func (c *countingSchemaCache) Range(fn func(uint64, *validatorcache.SchemaCacheEntry) bool) {
	c.inner.Range(fn)
}

func TestCustomContentParameterSchemaCacheReuse(t *testing.T) {
	model := contentParameterModel(t, contentParameterSpec)
	cache := &countingSchemaCache{inner: validatorcache.NewDefaultCache()}
	decoder := func(_ context.Context, input *config.ContentParameterInput) (any, *base.Schema, error) {
		return map[string]any{"active": true}, input.DefaultSchema, nil
	}
	validator := NewParameterValidator(model, config.WithSchemaCache(cache), config.WithContentParameterDecoder(decoder))
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/2?filter=value", nil)
	for range 2 {
		valid, validationErrors := validator.ValidateQueryParams(request)
		require.True(t, valid, validationErrors)
	}
	assert.Equal(t, 1, cache.stores)
	assert.GreaterOrEqual(t, cache.loads, 2)
	validator.Release()
}

func TestAuthenticationInputUsesSharedRouterContext(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: auth route, version: 1.0.0}
servers:
  - url: /api/{version}
    variables: {version: {default: v1}}
components:
  securitySchemes: {key: {type: apiKey, in: header, name: X-Key}}
paths:
  /items/{id}:
    get:
      security: [{key: []}]
      responses: {"204": {description: ok}}`
	model := contentParameterModel(t, spec)
	routeFinder := router.NewRouter(model)
	called := false
	authentication := func(_ context.Context, input *config.AuthenticationInput) error {
		called = true
		assert.Equal(t, "/items/{id}", input.Path)
		assert.Equal(t, "item", input.PathParams["id"])
		assert.Equal(t, "v2", input.ServerParams["version"])
		assert.NotNil(t, input.Server)
		assert.NotNil(t, input.Operation)
		return nil
	}
	counter := &countingParameterRouter{Router: routeFinder}
	opts := config.NewValidationOptions(config.WithAuthenticationFunc(authentication))
	opts.Router = counter
	validator := NewParameterValidator(model, config.WithExistingOpts(opts))
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/api/v2/items/item", nil)
	resolved, resolveErr := routeFinder.FindRoute(request)
	require.NoError(t, resolveErr)
	valid, validationErrors := validator.ValidateSecurityWithPathItem(request, resolved.PathItem, resolved.Path)
	require.True(t, valid, validationErrors)
	assert.Equal(t, int64(1), counter.calls.Load())
	counter.calls.Store(0)
	restoreRoute := requeststate.AttachRoute(request, resolved)
	defer restoreRoute()
	valid, validationErrors = validator.ValidateSecurityWithPathItem(request, resolved.PathItem, resolved.Path)
	require.True(t, valid, validationErrors)
	assert.True(t, called)
	assert.Zero(t, counter.calls.Load())
	validator.Release()
	routeFinder.Release()
}

type countingParameterRouter struct {
	router.Router
	calls atomic.Int64
}

func (r *countingParameterRouter) FindRoute(request *http.Request) (*router.Route, error) {
	r.calls.Add(1)
	return r.Router.FindRoute(request)
}
