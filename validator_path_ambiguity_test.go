// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
	validationErrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/parameters"
)

// Operations that a single request reaches under different readings of its path.
const ambiguousRouteSpec = `openapi: 3.1.0
info: {title: ambiguous routes, version: "1"}
servers: [{url: https://example.com/api}]
components:
  securitySchemes:
    key: {type: apiKey, in: header, name: X-API-Key}
paths:
  /users/{id}:
    get:
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
      responses: {"200": {description: ok}}
  /users/me:
    get:
      security: [{key: []}]
      responses: {"200": {description: ok}}
  /users/{id}/secrets:
    get:
      security: [{key: []}]
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
      responses: {"200": {description: ok}}
  /status:
    get:
      responses: {"200": {description: ok}}
  /{id}:
    get:
      security: [{key: []}]
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
      responses: {"200": {description: ok}}
`

func TestValidateHttpRequest_AmbiguousPathsAreRejected(t *testing.T) {
	document, err := libopenapi.NewDocument([]byte(ambiguousRouteSpec))
	require.NoError(t, err)
	model, modelErr := document.BuildV3Model()
	require.NoError(t, modelErr)

	calls := 0
	authenticate := config.WithAuthenticationFunc(func(_ context.Context, input *config.AuthenticationInput) error {
		calls++
		if input.Request.Header.Get("X-API-Key") != "secret" {
			return errors.New("bad key")
		}
		return nil
	})

	high, buildErrs := NewValidator(document, authenticate)
	require.Empty(t, buildErrs)
	strict, buildErrs := NewValidator(document, authenticate, config.WithStrictServerMatching())
	require.Empty(t, buildErrs)
	standalone := parameters.NewParameterValidator(&model.Model, authenticate)

	validators := map[string]func(*http.Request) (bool, []*validationErrors.ValidationError){
		"path-only": high.ValidateHttpRequest,
		"strict":    strict.ValidateHttpRequest,
		"standalone": func(request *http.Request) (bool, []*validationErrors.ValidationError) {
			return standalone.ValidateSecurity(request)
		},
	}

	for name, validate := range validators {
		for _, target := range []string{
			"https://example.com/api/users/m%65",
			"https://example.com/api/users/5%2Fsecrets",
		} {
			request, _ := http.NewRequest(http.MethodGet, target, nil)
			valid, errs := validate(request)
			assert.False(t, valid, "%s %s", name, target)
			if assert.NotEmpty(t, errs, "%s %s", name, target) {
				assert.Equal(t, helpers.ValidationAmbiguousPath, errs[0].ValidationSubType, "%s %s", name, target)
			}
		}

		// the operations still behave as declared
		calls = 0
		request, _ := http.NewRequest(http.MethodGet, "https://example.com/api/users/alice", nil)
		valid, _ := validate(request)
		assert.True(t, valid, name)
		assert.Zero(t, calls, name)

		request, _ = http.NewRequest(http.MethodGet, "https://example.com/api/users/me", nil)
		valid, _ = validate(request)
		assert.False(t, valid, name)
		assert.Equal(t, 1, calls, name)

		request, _ = http.NewRequest(http.MethodGet, "https://example.com/api/users/me", nil)
		request.Header.Set("X-API-Key", "secret")
		valid, _ = validate(request)
		assert.True(t, valid, name)
	}

	// a base path is only removed on a segment boundary, so "/apistatus" is not "/status"
	for name, validate := range map[string]func(*http.Request) (bool, []*validationErrors.ValidationError){
		"path-only":  validators["path-only"],
		"standalone": validators["standalone"],
	} {
		calls = 0
		request, _ := http.NewRequest(http.MethodGet, "https://example.com/apistatus", nil)
		valid, _ := validate(request)
		assert.False(t, valid, name)
		assert.Equal(t, 1, calls, name)
	}
}
