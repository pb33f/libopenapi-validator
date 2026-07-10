// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
	"go.yaml.in/yaml/v4"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
	validationerrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/internal/requeststate"
	"github.com/pb33f/libopenapi-validator/router"
)

const defaultsSpec = `openapi: 3.1.0
info: {title: defaults, version: 1.0.0}
paths:
  /items:
    post:
      parameters:
        - {name: limit, in: query, schema: {type: integer, default: 10}}
        - name: tags
          in: query
          explode: true
          schema: {type: array, items: {type: string}, default: [a, b]}
        - {name: X-Mode, in: header, schema: {type: string, default: safe}}
        - {name: session, in: cookie, schema: {type: string, default: abc}}
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [requiredName]
              properties:
                requiredName: {type: string}
                mode: {type: [string, "null"], default: fast}
                nested:
                  type: object
                  properties:
                    count: {type: integer, default: 2}
                list:
                  type: array
                  items:
                    type: object
                    properties:
                      enabled: {type: boolean, default: true}
                branch:
                  oneOf:
                    - type: object
                      properties:
                        ignored: {type: string, default: no}
              allOf:
                - type: object
                  properties:
                    inherited: {type: string, default: yes}
      responses:
        "200":
          description: ok
          headers:
            X-Rate: {required: true, schema: {type: integer}}
          content:
            application/json:
              schema: {type: object, required: [ok], properties: {ok: {type: boolean}}}
  /undeclared:
    post:
      responses: {"204": {description: ok}}
`

func parityValidator(t *testing.T, specification string, opts ...config.Option) Validator {
	t.Helper()
	document, err := libopenapi.NewDocument([]byte(specification))
	require.NoError(t, err)
	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	return NewValidatorFromV3Model(&model.Model, opts...)
}

func TestRequestDefaultsAtomicSuccessSyncAndAsync(t *testing.T) {
	for _, synchronous := range []bool{true, false} {
		t.Run(map[bool]string{true: "sync", false: "async"}[synchronous], func(t *testing.T) {
			v := parityValidator(t, defaultsSpec, config.WithRequestDefaults())
			t.Cleanup(v.Release)
			request, err := http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"item","nested":{},"list":[{}],"branch":{}}`))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			var valid bool
			var validationErrors []*ValidationErrorAlias
			if synchronous {
				valid, validationErrors = validateSync(v, request)
			} else {
				valid, validationErrors = validateAsync(v, request)
			}
			require.True(t, valid, validationErrors)
			require.Empty(t, validationErrors)
			assert.Equal(t, "10", request.URL.Query().Get("limit"))
			assert.Equal(t, []string{"a", "b"}, request.URL.Query()["tags"])
			assert.Equal(t, "safe", request.Header.Get("X-Mode"))
			cookie, cookieErr := request.Cookie("session")
			require.NoError(t, cookieErr)
			assert.Equal(t, "abc", cookie.Value)
			body, readErr := io.ReadAll(request.Body)
			require.NoError(t, readErr)
			var decoded map[string]any
			require.NoError(t, json.Unmarshal(body, &decoded))
			assert.Equal(t, "fast", decoded["mode"])
			assert.Equal(t, float64(2), decoded["nested"].(map[string]any)["count"])
			assert.Equal(t, true, decoded["list"].([]any)[0].(map[string]any)["enabled"])
			assert.Equal(t, "yes", decoded["inherited"])
			assert.NotContains(t, decoded["branch"].(map[string]any), "ignored")
			replayed, replayErr := request.GetBody()
			require.NoError(t, replayErr)
			replayedBody, _ := io.ReadAll(replayed)
			assert.Equal(t, body, replayedBody)
			assert.Equal(t, int64(len(body)), request.ContentLength)
		})
	}
}

type ValidationErrorAlias = validationerrors.ValidationError

func validateSync(v Validator, request *http.Request) (bool, []*ValidationErrorAlias) {
	return v.ValidateHttpRequestSync(request)
}

func validateAsync(v Validator, request *http.Request) (bool, []*ValidationErrorAlias) {
	return v.ValidateHttpRequest(request)
}

func TestRequestDefaultsPreserveExplicitNullAndDisabledMode(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"item","mode":null}`))
	request.Header.Set("Content-Type", "application/json")
	v := parityValidator(t, defaultsSpec, config.WithRequestDefaults())
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	body, _ := io.ReadAll(request.Body)
	assert.Contains(t, string(body), `"mode":null`)
	v.Release()

	request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"item"}`))
	request.Header.Set("Content-Type", "application/json")
	v = parityValidator(t, defaultsSpec)
	valid, validationErrors = v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	assert.Empty(t, request.URL.RawQuery)
	assert.Empty(t, request.Header.Get("X-Mode"))
	body, _ = io.ReadAll(request.Body)
	assert.NotContains(t, string(body), "mode")
	v.Release()
}

func TestRequestDefaultsPreserveExplicitEmptyHeader(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"item"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header["X-Mode"] = []string{""}
	v := parityValidator(t, defaultsSpec, config.WithRequestDefaults())
	t.Cleanup(v.Release)
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	assert.Equal(t, []string{""}, request.Header.Values("X-Mode"))
}

func TestRequestDefaultsRejectUnsupportedObjectParameterSerialization(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: object default, version: 1.0.0}
paths:
  /items:
    get:
      parameters:
        - name: filter
          in: query
          schema: {type: object, default: {active: true}}
      responses: {"204": {description: ok}}`
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items", nil)
	v := parityValidator(t, spec, config.WithRequestDefaults()).(*validator)
	t.Cleanup(v.Release)
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "unsupported object serialization")
	assert.Empty(t, request.URL.RawQuery)

	explicit, _ := http.NewRequest(http.MethodGet, "http://example.com/items?filter=active%2Ctrue", nil)
	pathItem := v.v3Model.Paths.PathItems.GetOrZero("/items")
	staged, stageErrors := v.stageRequestDefaults(explicit, pathItem)
	require.Empty(t, stageErrors)
	require.NotNil(t, staged)
	assert.Equal(t, "filter=active%2Ctrue", staged.URL.RawQuery)

	assert.False(t, containsObjectDefault("value"))
	assert.False(t, containsObjectDefault([]any{"value", 1.0}))
	assert.True(t, containsObjectDefault([]any{map[string]any{"active": true}}))
	assert.True(t, headerExists(http.Header{"x-direct": {""}}, "X-Direct"))
	assert.False(t, headerExists(http.Header{}, "missing"))
	assert.False(t, parameterNeedsDefault(nil, nil, true))
	assert.False(t, parameterNeedsDefault(request, &v3.Parameter{In: helpers.Path}, true))
}

func TestHighLevelValidationResolvesRouteOnce(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: route sharing, version: 1.0.0}
components:
  securitySchemes:
    first: {type: apiKey, in: header, name: X-First}
    second: {type: apiKey, in: header, name: X-Second}
paths:
  /items/{id}:
    get:
      security: [{first: [], second: []}]
      parameters:
        - name: id
          in: path
          required: true
          schema: {type: string}
        - name: filter
          in: query
          content: {application/json: {schema: {type: boolean}}}
      responses: {"204": {description: ok}}`
	decoder := func(_ context.Context, input *config.ContentParameterInput) (any, *base.Schema, error) {
		assert.Equal(t, "abc", input.PathParams["id"])
		return true, input.DefaultSchema, nil
	}
	v := parityValidator(t, spec,
		config.WithAuthenticationFunc(func(context.Context, *config.AuthenticationInput) error { return nil }),
		config.WithContentParameterDecoder(decoder),
	).(*validator)
	t.Cleanup(v.Release)
	counter := &countingRouter{Router: v.options.Router}
	v.options.Router = counter
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/abc?filter=true", nil)
	valid, validationErrors := v.ValidateHttpRequest(request)
	require.True(t, valid, validationErrors)
	assert.Equal(t, int64(1), counter.calls.Load())
	assert.Nil(t, requeststate.Route(request), "route context must be scoped to validation")
}

func TestWithPathItemResolvesAndScopesRoute(t *testing.T) {
	v := parityValidator(t, defaultsSpec).(*validator)
	t.Cleanup(v.Release)
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"item"}`))
	request.Header.Set("Content-Type", "application/json")
	pathItem := v.v3Model.Paths.PathItems.GetOrZero("/items")
	valid, validationErrors := v.ValidateHttpRequestSyncWithPathItem(request, pathItem, "/items")
	require.True(t, valid, validationErrors)
	assert.Nil(t, requeststate.Route(request))
}

type countingRouter struct {
	router.Router
	calls atomic.Int64
}

func (r *countingRouter) FindRoute(request *http.Request) (*router.Route, error) {
	r.calls.Add(1)
	return r.Router.FindRoute(request)
}

func TestRequestDefaultsRollbackOnValidationFailure(t *testing.T) {
	v := parityValidator(t, defaultsSpec, config.WithRequestDefaults())
	t.Cleanup(v.Release)
	original := `{"nested":{}}`
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/items?keep=yes", strings.NewReader(original))
	request.Header.Set("Content-Type", "application/json")
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
	assert.Equal(t, "keep=yes", request.URL.RawQuery)
	assert.Empty(t, request.Header.Get("X-Mode"))
	assert.Empty(t, request.Header.Get("Cookie"))
	body, _ := io.ReadAll(request.Body)
	assert.JSONEq(t, original, string(body))
}

func TestRequestDefaultsCustomCodecAndMissingEncoder(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: custom, version: 1.0.0}
paths:
  /custom:
    post:
      requestBody:
        required: true
        content:
          application/custom:
            schema:
              type: object
              properties: {added: {type: string, default: yes}}
      responses: {"204": {description: ok}}`
	decoder := content.DecoderFunc(func(input *content.DecodeInput) (any, error) {
		var value any
		err := json.NewDecoder(input.Body).Decode(&value)
		return value, err
	})
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/custom", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/custom")
	v := parityValidator(t, spec, config.WithRequestDefaults(), config.WithBodyDecoder("application/custom", decoder))
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "encoder")
	body, _ := io.ReadAll(request.Body)
	assert.Equal(t, `{}`, string(body))
	v.Release()

	noDefaultSpec := strings.Replace(spec, ", default: yes", "", 1)
	request, _ = http.NewRequest(http.MethodPost, "http://example.com/custom", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/custom")
	v = parityValidator(t, noDefaultSpec, config.WithRequestDefaults(), config.WithBodyDecoder("application/custom", decoder))
	valid, validationErrors = v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	v.Release()

	encoder := content.EncoderFunc(func(input *content.EncodeInput) ([]byte, error) { return json.Marshal(input.Value) })
	request, _ = http.NewRequest(http.MethodPost, "http://example.com/custom", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/custom")
	v = parityValidator(t, spec, config.WithRequestDefaults(), config.WithBodyDecoder("application/custom", decoder), config.WithBodyEncoder("application/custom", encoder))
	valid, validationErrors = v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	body, _ = io.ReadAll(request.Body)
	assert.JSONEq(t, `{"added":"yes"}`, string(body))
	v.Release()
}

func TestRequestDefaultsForJSONContentParameter(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: content default, version: 1.0.0}
paths:
  /content:
    get:
      parameters:
        - name: filter
          in: query
          content:
            application/json:
              schema:
                type: object
                default: {active: true}
                properties: {active: {type: boolean}}
      responses: {"204": {description: ok}}`
	v := parityValidator(t, spec, config.WithRequestDefaults())
	t.Cleanup(v.Release)
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/content", nil)
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	assert.JSONEq(t, `{"active":true}`, request.URL.Query().Get("filter"))
}

func TestAuthenticationRouteContextAndReplayableBody(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: auth, version: 1.0.0}
servers: [{url: /api}]
components:
  securitySchemes:
    first: {type: apiKey, in: header, name: X-Key}
    second: {type: oauth2, flows: {clientCredentials: {tokenUrl: /token, scopes: {read: Read}}}}
paths:
  /secure/{id}:
    post:
      security: [{first: [], second: [read]}]
      parameters: [{name: id, in: path, required: true, schema: {type: string}}]
      requestBody:
        required: true
        content: {application/json: {schema: {type: object, required: [ok], properties: {ok: {type: boolean}}}}}
      responses: {"204": {description: ok}}`
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("key"), "value")
	var schemes []string
	var bodies []string
	auth := func(callbackContext context.Context, input *config.AuthenticationInput) error {
		schemes = append(schemes, input.SecuritySchemeName)
		body, err := io.ReadAll(input.Request.Body)
		bodies = append(bodies, string(body))
		require.NoError(t, err)
		assert.Equal(t, "value", callbackContext.Value(contextKey("key")))
		assert.Equal(t, "/secure/{id}", input.Path)
		assert.Equal(t, "a/b", input.PathParams["id"])
		assert.NotNil(t, input.PathItem)
		assert.NotNil(t, input.Operation)
		assert.NotNil(t, input.Server)
		assert.Equal(t, "/api", input.Server.URL)
		return nil
	}
	v := parityValidator(t, spec, config.WithAuthenticationFunc(auth))
	t.Cleanup(v.Release)
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://example.com/api/secure/a%2Fb", strings.NewReader(`{"ok":true}`))
	request.Header.Set("Content-Type", "application/json")
	valid, validationErrors := v.ValidateHttpRequest(request)
	require.True(t, valid, validationErrors)
	assert.Equal(t, []string{"first", "second"}, schemes)
	assert.Equal(t, []string{`{"ok":true}`, `{"ok":true}`}, bodies)
	body, _ := io.ReadAll(request.Body)
	assert.JSONEq(t, `{"ok":true}`, string(body))
}

func TestAuthenticationReceivesCancellation(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: auth cancel, version: 1.0.0}
components:
  securitySchemes: {key: {type: apiKey, in: header, name: X-Key}}
paths:
  /secure:
    get:
      security: [{key: []}]
      responses: {"204": {description: ok}}`
	called := false
	v := parityValidator(t, spec, config.WithAuthenticationFunc(func(ctx context.Context, _ *config.AuthenticationInput) error {
		called = true
		assert.ErrorIs(t, ctx.Err(), context.Canceled)
		return ctx.Err()
	}))
	t.Cleanup(v.Release)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/secure", nil)
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	assert.False(t, valid)
	assert.True(t, called)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, context.Canceled.Error())
}

func TestUndeclaredBodyAndHighLevelPolicies(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/undeclared", strings.NewReader("payload"))
	v := parityValidator(t, defaultsSpec)
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	v.Release()

	request, _ = http.NewRequest(http.MethodPost, "http://example.com/undeclared", strings.NewReader("payload"))
	v = parityValidator(t, defaultsSpec, config.WithRejectUndeclaredRequestBody())
	valid, validationErrors = v.ValidateHttpRequestSync(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	body, _ := io.ReadAll(request.Body)
	assert.Equal(t, "payload", string(body))
	v.Release()

	request, _ = http.NewRequest(http.MethodPost, "http://example.com/undeclared", strings.NewReader("payload"))
	v = parityValidator(t, defaultsSpec, config.WithRejectUndeclaredRequestBody(), config.WithoutRequestBodyValidation())
	valid, validationErrors = v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	v.Release()
}

func TestQueryResponseBodyAndStatusExclusions(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/items?limit=bad", strings.NewReader(`{"requiredName":"item"}`))
	request.Header.Set("Content-Type", "application/json")
	v := parityValidator(t, defaultsSpec, config.WithoutRequestQueryParameterValidation())
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	v.Release()

	request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"item"}`))
	request.Header.Set("Content-Type", "application/json")
	response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}, "X-Rate": {"2"}}, Body: io.NopCloser(strings.NewReader(`{"ok":"wrong"}`))}
	v = parityValidator(t, defaultsSpec, config.WithoutResponseBodyValidation())
	valid, validationErrors = v.ValidateHttpResponse(request, response)
	require.True(t, valid, validationErrors)
	body, _ := io.ReadAll(response.Body)
	assert.JSONEq(t, `{"ok":"wrong"}`, string(body))
	v.Release()

	response = &http.Response{StatusCode: 299, Header: make(http.Header), Body: http.NoBody}
	v = parityValidator(t, defaultsSpec, config.WithoutResponseStatusValidation())
	valid, validationErrors = v.ValidateHttpResponse(request, response)
	require.True(t, valid, validationErrors)
	v.Release()
}

func TestRequestDefaultPreparationErrors(t *testing.T) {
	v := parityValidator(t, defaultsSpec, config.WithRequestDefaults())
	concrete := v.(*validator)
	valid, validationErrors := concrete.validateWithRequestDefaults(nil, nil, "", concrete.validateHttpRequestSyncWithPathItem)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
	v.Release()

	assert.Equal(t, "x", serializeDefault("x", ","))
	assert.Nil(t, requestMediaType(nil, "application/json"))
	changed, err := applySchemaDefaults(nil, nil)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestRequestDefaultBranchCoverage(t *testing.T) {
	t.Run("unreadable body", func(t *testing.T) {
		v := parityValidator(t, defaultsSpec, config.WithRequestDefaults()).(*validator)
		defer v.Release()
		request := &http.Request{Method: http.MethodPost, URL: mustURL(t, "http://example.com/items"), Header: make(http.Header), Body: &parityErrorBody{}}
		_, errs := v.stageRequestDefaults(request, v.v3Model.Paths.PathItems.GetOrZero("/items"))
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Message, "staged")
	})

	t.Run("operation and empty body exits", func(t *testing.T) {
		v := parityValidator(t, defaultsSpec, config.WithRequestDefaults()).(*validator)
		defer v.Release()
		request := &http.Request{Method: http.MethodGet, URL: mustURL(t, "http://example.com/none")}
		staged, errs := v.stageRequestDefaults(request, &v3.PathItem{})
		require.Empty(t, errs)
		assert.NotNil(t, staged.Header)

		request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", http.NoBody)
		staged, errs = v.stageRequestDefaults(request, v.v3Model.Paths.PathItems.GetOrZero("/items"))
		require.Empty(t, errs)
		assert.NotNil(t, staged)
	})

	t.Run("media and codec failures", func(t *testing.T) {
		v := parityValidator(t, defaultsSpec, config.WithRequestDefaults()).(*validator)
		item := v.v3Model.Paths.PathItems.GetOrZero("/items")
		request, _ := http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"x"}`))
		request.Header.Set("Content-Type", "text/plain")
		staged, errs := v.stageRequestDefaults(request, item)
		require.Empty(t, errs)
		assert.NotNil(t, staged)
		v.Release()

		yamlSpec := strings.Replace(defaultsSpec, "application/json:", "application/yaml:", 1)
		v = parityValidator(t, yamlSpec, config.WithRequestDefaults()).(*validator)
		request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader("requiredName: x"))
		request.Header.Set("Content-Type", "application/yaml")
		_, errs = v.stageRequestDefaults(request, v.v3Model.Paths.PathItems.GetOrZero("/items"))
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Message, "decoder")
		v.Release()

		decodeFailure := content.DecoderFunc(func(*content.DecodeInput) (any, error) { return nil, assert.AnError })
		v = parityValidator(t, defaultsSpec, config.WithRequestDefaults(), config.WithBodyDecoder("application/json", decodeFailure)).(*validator)
		request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		_, errs = v.stageRequestDefaults(request, v.v3Model.Paths.PathItems.GetOrZero("/items"))
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Message, "decoded")
		v.Release()

		canonicalFailure := content.DecoderFunc(func(*content.DecodeInput) (any, error) { return map[any]any{1: "bad"}, nil })
		v = parityValidator(t, defaultsSpec, config.WithRequestDefaults(), config.WithBodyDecoder("application/json", canonicalFailure)).(*validator)
		request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		_, errs = v.stageRequestDefaults(request, v.v3Model.Paths.PathItems.GetOrZero("/items"))
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Message, "canonicalized")
		v.Release()
	})

	t.Run("apply and encode failures", func(t *testing.T) {
		v := parityValidator(t, defaultsSpec, config.WithRequestDefaults()).(*validator)
		item := v.v3Model.Paths.PathItems.GetOrZero("/items")
		schema := item.Post.RequestBody.Content.GetOrZero("application/json").Schema.Schema()
		mode := schema.Properties.GetOrZero("mode").Schema()
		mode.Default = &yaml.Node{Kind: 99}
		request, _ := http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"x"}`))
		request.Header.Set("Content-Type", "application/json")
		_, errs := v.stageRequestDefaults(request, item)
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Message, "applied")
		v.Release()

		encodeFailure := content.EncoderFunc(func(*content.EncodeInput) ([]byte, error) { return nil, assert.AnError })
		v = parityValidator(t, defaultsSpec, config.WithRequestDefaults(), config.WithBodyEncoder("application/json", encodeFailure)).(*validator)
		request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"requiredName":"x"}`))
		request.Header.Set("Content-Type", "application/json")
		_, errs = v.stageRequestDefaults(request, v.v3Model.Paths.PathItems.GetOrZero("/items"))
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Message, "encoded")
		v.Release()
	})

	t.Run("unchanged body needs no rewrite", func(t *testing.T) {
		v := parityValidator(t, defaultsSpec, config.WithRequestDefaults()).(*validator)
		defer v.Release()
		body := `{"requiredName":"x","mode":"set","nested":{"count":1},"list":[{"enabled":false}],"inherited":"set"}`
		request, _ := http.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		staged, errs := v.stageRequestDefaults(request, v.v3Model.Paths.PathItems.GetOrZero("/items"))
		require.Empty(t, errs)
		stagedBody, _ := io.ReadAll(staged.Body)
		assert.JSONEq(t, body, string(stagedBody))
	})
}

func TestRequestDefaultPureHelpers(t *testing.T) {
	query := make(url.Values)
	parameter := &v3.Parameter{Name: "items"}
	writeQueryDefault(query, parameter, []any{"a", "b"})
	assert.Equal(t, "a,b", query.Get("items"))
	assert.Equal(t, "a|b", serializeDefault([]any{"a", "b"}, "|"))
	encoded := serializeContentDefault(map[string]any{"ok": true})
	assert.JSONEq(t, `{"ok":true}`, encoded)
	assert.Nil(t, requestMediaType(&v3.Operation{}, "application/json"))
	assert.Nil(t, requestMediaType(&v3.Operation{RequestBody: &v3.RequestBody{}}, "application/json"))

	wildcardSpec := `openapi: 3.1.0
info: {title: wildcard, version: 1.0.0}
paths:
  /wild:
    parameters:
      - name: filter
        in: query
        content: {application/vnd.filter+json: {schema: {type: object}}}
    post:
      requestBody:
        content:
          application/*: {schema: {type: string}}
      responses: {"204": {description: ok}}`
	v := parityValidator(t, wildcardSpec).(*validator)
	operation := v.v3Model.Paths.PathItems.GetOrZero("/wild").Post
	assert.NotNil(t, requestMediaType(operation, "application/json"))
	assert.Nil(t, requestMediaType(operation, "invalid"))
	assert.Nil(t, requestMediaType(operation, "text/plain"))
	v.Release()

	badNode := &yaml.Node{Kind: 99}
	_, err := defaultValue(badNode)
	assert.Error(t, err)
	var nonStringDefault yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("1: value"), &nonStringDefault))
	_, err = defaultValue(nonStringDefault.Content[0])
	assert.Error(t, err)
}

func TestRequestDefaultParameterAndTraversalEdges(t *testing.T) {
	t.Run("parameter skips and malformed default", func(t *testing.T) {
		v := parityValidator(t, defaultsSpec, config.WithRequestDefaults()).(*validator)
		defer v.Release()
		item := v.v3Model.Paths.PathItems.GetOrZero("/items")
		item.Post.Parameters = append(item.Post.Parameters, nil, &v3.Parameter{})
		limit := item.Post.Parameters[0].Schema.Schema()
		original := limit.Default
		limit.Default = nil
		request, _ := http.NewRequest(http.MethodPost, "http://example.com/items", http.NoBody)
		_, errs := v.stageRequestDefaults(request, item)
		require.Empty(t, errs)
		limit.Default = &yaml.Node{Kind: 99}
		request, _ = http.NewRequest(http.MethodPost, "http://example.com/items", http.NoBody)
		_, errs = v.stageRequestDefaults(request, item)
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Message, "parameter default")
		limit.Default = original
	})

	t.Run("schema traversal error and skip branches", func(t *testing.T) {
		v := parityValidator(t, defaultsSpec).(*validator)
		defer v.Release()
		schema := v.v3Model.Paths.PathItems.GetOrZero("/items").Post.RequestBody.Content.GetOrZero("application/json").Schema.Schema()
		schema.AllOf = append(schema.AllOf, nil)
		schema.Properties.Set("nil-proxy", nil)
		schema.Properties.Set("empty-proxy", &base.SchemaProxy{})
		mode := schema.Properties.GetOrZero("mode").Schema()
		readOnly := true
		mode.ReadOnly = &readOnly
		changed, err := applySchemaDefaults(map[string]any{}, schema)
		require.NoError(t, err)
		assert.True(t, changed)
		assert.NotNil(t, mode.ReadOnly)

		inherited := schema.AllOf[0].Schema().Properties.GetOrZero("inherited").Schema()
		inherited.Default = &yaml.Node{Kind: 99}
		_, err = applySchemaDefaults(map[string]any{}, schema)
		assert.Error(t, err)

		inherited.Default = nil
		nested := schema.Properties.GetOrZero("nested").Schema().Properties.GetOrZero("count").Schema()
		nested.Default = &yaml.Node{Kind: 99}
		_, err = applySchemaDefaults(map[string]any{"nested": map[string]any{}}, schema)
		assert.Error(t, err)

		nested.Default = nil
		itemSchema := schema.Properties.GetOrZero("list").Schema().Items.A.Schema()
		itemSchema.Properties.GetOrZero("enabled").Schema().Default = &yaml.Node{Kind: 99}
		_, err = applySchemaDefaults([]any{map[string]any{}}, schema.Properties.GetOrZero("list").Schema())
		assert.Error(t, err)
	})
}

func TestRequestDefaultAllOfConflictIsDeterministic(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: conflicts, version: 1.0.0}
components:
  schemas:
    Conflict:
      type: object
      allOf:
        - type: object
          properties: {value: {type: string, default: first}}
        - type: object
          properties: {value: {type: string, default: second}}`
	document, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	schema := model.Model.Components.Schemas.GetOrZero("Conflict").Schema()
	value := map[string]any{}
	changed, err := applySchemaDefaults(value, schema)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "first", value["value"])
}

func TestValidatorConstructorRegexCacheAndAuthReadError(t *testing.T) {
	cache := &sync.Map{}
	v := parityValidator(t, defaultsSpec, config.WithRegexCache(cache))
	v.Release()

	authSpec := `openapi: 3.1.0
info: {title: auth, version: 1.0.0}
components:
  securitySchemes: {key: {type: apiKey, in: header, name: X-Key}}
paths:
  /secure:
    post:
      security: [{key: []}]
      requestBody: {content: {application/json: {schema: {type: object}}}}
      responses: {"204": {description: ok}}`
	v = parityValidator(t, authSpec, config.WithAuthenticationFunc(func(context.Context, *config.AuthenticationInput) error { return nil }))
	request := &http.Request{Method: http.MethodPost, URL: mustURL(t, "http://example.com/secure"), Header: http.Header{"Content-Type": {"application/json"}}, Body: &parityErrorBody{}}
	valid, errs := v.ValidateHttpRequest(request)
	assert.False(t, valid)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "authentication")
	v.Release()
}

func TestHighLevelStrictServerMatchingOptIn(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: strict, version: 1.0.0}
servers: [{url: https://api.example.com/v1}]
paths:
  /things:
    get: {responses: {"204": {description: ok}}}`
	compatible := parityValidator(t, spec)
	request, _ := http.NewRequest(http.MethodGet, "http://wrong.example.com/v1/things", nil)
	valid, validationErrors := compatible.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	compatible.Release()

	strict := parityValidator(t, spec, config.WithStrictServerMatching())
	request, _ = http.NewRequest(http.MethodGet, "http://wrong.example.com/v1/things", nil)
	valid, validationErrors = strict.ValidateHttpRequestSync(request)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
	request, _ = http.NewRequest(http.MethodGet, "https://api.example.com/v1/things", nil)
	valid, validationErrors = strict.ValidateHttpRequestSync(request)
	require.True(t, valid, validationErrors)
	strict.Release()
}

func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	require.NoError(t, err)
	return parsed
}

type parityErrorBody struct{}

func (*parityErrorBody) Read([]byte) (int, error) { return 0, assert.AnError }
func (*parityErrorBody) Close() error             { return nil }
