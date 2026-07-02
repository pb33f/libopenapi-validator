// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
)

func TestXquikSearchFixture_ValidateRequestAndResponse(t *testing.T) {
	spec := []byte(`openapi: 3.1.0
info:
  title: Xquik API
  version: "1.0"
servers:
  - url: https://xquik.com
paths:
  /api/v1/x/tweets/search:
    get:
      operationId: searchTweets
      summary: Search Tweets
      security:
        - apiKey: []
      parameters:
        - name: query
          in: query
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Search results
          content:
            application/json:
              schema:
                type: object
                required:
                  - data
                properties:
                  data:
                    type: array
                    items:
                      $ref: "#/components/schemas/Tweet"
components:
  schemas:
    Tweet:
      type: object
      required:
        - id
        - text
      properties:
        id:
          type: string
        text:
          type: string
  securitySchemes:
    apiKey:
      type: apiKey
      in: header
      name: x-api-key
`)

	document, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	docValidator, validatorErrs := NewValidator(document)
	require.Empty(t, validatorErrs)

	valid, validationErrs := docValidator.ValidateDocument()
	assert.True(t, valid)
	assert.Empty(t, validationErrs)

	request, err := http.NewRequest(
		http.MethodGet,
		"https://xquik.com/api/v1/x/tweets/search?query=golang",
		nil,
	)
	require.NoError(t, err)
	request.Header.Set("x-api-key", "test-key")

	valid, validationErrs = docValidator.ValidateHttpRequest(request)
	assert.True(t, valid)
	assert.Empty(t, validationErrs)

	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewBufferString(`{
			"data": [
				{
					"id": "1900000000000000000",
					"text": "OpenAPI validation fixture"
				}
			]
		}`)),
		Request: request,
	}

	valid, validationErrs = docValidator.ValidateHttpResponse(request, response)
	assert.True(t, valid)
	assert.Empty(t, validationErrs)
}
