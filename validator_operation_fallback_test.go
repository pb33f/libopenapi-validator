// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/require"
)

// These tests cover the operation resolution gaps flagged in the PR 340
// review: HEAD requests fall back to get, and QUERY (OpenAPI 3.2) plus
// additionalOperations methods resolve to their own operations. The
// operation level parameters now take part in validation.

func TestHEADRequestOnGetOnlyPathValidatesGetParameters(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: claims
  version: 1.0.0
paths:
  /reports:
    get:
      parameters:
        - name: limit
          in: query
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: ok
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)
	t.Cleanup(v.Release)

	request, err := http.NewRequest(http.MethodHead, "https://things.com/reports", nil)
	require.NoError(t, err)

	// HEAD is validated as the get operation, so its required query
	// parameter is checked: the request without limit fails.
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	require.False(t, valid, "expected HEAD to validate as the get operation")
	found := false
	for _, e := range validationErrors {
		if strings.Contains(e.Error(), "limit") {
			found = true
		}
	}
	require.True(t, found, "expected a missing limit parameter error, got %v", validationErrors)
}

func TestQueryOperationValidatesItsParameters(t *testing.T) {
	spec := `openapi: 3.2.0
info:
  title: claims
  version: 1.0.0
paths:
  /search:
    query:
      parameters:
        - name: limit
          in: query
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: ok
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)
	t.Cleanup(v.Release)

	request, err := http.NewRequest("QUERY", "https://things.com/search", nil)
	require.NoError(t, err)

	// the query operation resolves and its required query parameter is
	// checked: the request without limit fails validation.
	valid, validationErrors := v.ValidateHttpRequestSync(request)
	require.False(t, valid, "expected the query operation's required parameter to be enforced")
	require.NotEmpty(t, validationErrors)
}

func TestAdditionalOperationValidatesItsParameters(t *testing.T) {
	spec := `openapi: 3.2.0
info:
  title: claims
  version: 1.0.0
paths:
  /search:
    additionalOperations:
      CUSTOM:
        parameters:
          - name: range
            in: query
            required: true
            schema:
              type: string
        responses:
          '200':
            description: ok
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)
	t.Cleanup(v.Release)

	request, err := http.NewRequest("CUSTOM", "https://things.com/search", nil)
	require.NoError(t, err)

	// the custom method's operation resolves and its required query
	// parameter is checked: the request without range fails validation.
	_, validationErrors := v.ValidateHttpRequestSync(request)
	require.NotEmpty(t, validationErrors, "expected the custom method's required parameter to be enforced")
}
