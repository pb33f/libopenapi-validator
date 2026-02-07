// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestContext(t *testing.T) {
	spec := `openapi: "3.1.0"
info:
  title: Test
  version: "0.1.0"
paths:
  /users:
    get:
      operationId: listUsers
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
      responses:
        "200":
          description: OK
  /users/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: getUser
      security:
        - bearerAuth:
            - read
      responses:
        "200":
          description: OK
    post:
      operationId: createUser
      requestBody:
        content:
          application/json:
            schema:
              type: object
      responses:
        "201":
          description: Created
  /health:
    get:
      operationId: healthCheck
      responses:
        "200":
          description: OK
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	m, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	v := NewValidatorFromV3Model(&m.Model).(*validator)

	tests := []struct {
		name           string
		method         string
		url            string
		expectErr      bool
		expectPath     string
		expectSegments []string
		expectVersion  float32
		expectParamLen int  // expected number of parameters (-1 to skip check)
		expectSecurity bool // expect non-nil security
	}{
		{
			name:           "success - simple path",
			method:         http.MethodGet,
			url:            "/health",
			expectErr:      false,
			expectPath:     "/health",
			expectSegments: []string{"health"},
			expectVersion:  3.1,
			expectParamLen: 0,
			expectSecurity: false,
		},
		{
			name:           "success - path with parameters",
			method:         http.MethodGet,
			url:            "/users/abc123",
			expectErr:      false,
			expectPath:     "/users/{id}",
			expectSegments: []string{"users", "abc123"},
			expectVersion:  3.1,
			expectParamLen: 1, // path-level "id" param
			expectSecurity: true,
		},
		{
			name:           "success - path with query params",
			method:         http.MethodGet,
			url:            "/users",
			expectErr:      false,
			expectPath:     "/users",
			expectSegments: []string{"users"},
			expectVersion:  3.1,
			expectParamLen: 1, // operation-level "limit" param
			expectSecurity: false,
		},
		{
			name:      "path not found",
			method:    http.MethodGet,
			url:       "/nonexistent",
			expectErr: true,
		},
		{
			name:      "method not found",
			method:    http.MethodDelete,
			url:       "/health",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, tc.url, nil)
			ctx, validationErrs := v.buildRequestContext(req)

			if tc.expectErr {
				assert.Nil(t, ctx)
				assert.NotEmpty(t, validationErrs)
				return
			}

			require.NotNil(t, ctx)
			assert.Empty(t, validationErrs)
			assert.Equal(t, tc.expectPath, ctx.route.matchedPath)
			assert.Equal(t, tc.expectSegments, ctx.segments)
			assert.Equal(t, tc.expectVersion, ctx.version)
			assert.NotNil(t, ctx.operation)
			assert.Equal(t, req, ctx.request)

			if tc.expectParamLen >= 0 {
				assert.Len(t, ctx.parameters, tc.expectParamLen)
			}

			if tc.expectSecurity {
				assert.NotNil(t, ctx.security)
				assert.NotEmpty(t, ctx.security)
			} else {
				assert.Nil(t, ctx.security)
			}
		})
	}
}

func TestBuildRequestContext_Version30(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Test
  version: "0.1.0"
paths:
  /ping:
    get:
      operationId: ping
      responses:
        "200":
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	m, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	v := NewValidatorFromV3Model(&m.Model).(*validator)

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	ctx, validationErrs := v.buildRequestContext(req)

	require.NotNil(t, ctx)
	assert.Empty(t, validationErrs)
	assert.Equal(t, float32(3.0), ctx.version)
}
