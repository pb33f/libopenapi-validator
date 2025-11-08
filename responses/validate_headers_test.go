// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package responses

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
)

func TestValidateResponseHeaders(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Healthcheck
  version: '0.1.0'
paths:
  /health:
    get:
      responses:
        '200':
          headers:
            chicken-nuggets:
              description: chicken nuggets response
              required: true
              schema:
                type: integer
          description: pet response`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	// build a request
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/health", nil)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Chicken-Cakes", "I should fail")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(nil)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	headers := m.Model.Paths.PathItems.GetOrZero("/health").Get.Responses.Codes.GetOrZero("200").Headers

	// validate!
	valid, errors := ValidateResponseHeaders(request, response, headers, "/health", "200")

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, errors[0].Message, "Missing required header")
	assert.Equal(t, errors[0].Reason, "Required header 'chicken-nuggets' was not found in response")

	res = httptest.NewRecorder()
	handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Chicken-Nuggets", "I should fail")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(nil)
	}

	// fire the request
	handler(res, request)

	response = res.Result()

	headers = m.Model.Paths.PathItems.GetOrZero("/health").Get.Responses.Codes.GetOrZero("200").Headers

	// validate!
	valid, errors = ValidateResponseHeaders(request, response, headers, "/health", "200")

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, errors[0].Message, "header 'chicken-nuggets' failed to validate")
	assert.Equal(t, errors[0].Reason, "response header 'chicken-nuggets' is defined as an integer, however it failed to pass a schema validation")
}

func TestValidateResponseHeaders_Valid(t *testing.T) {
	spec := `openapi: "3.0.0"
info:
  title: Healthcheck
  version: '0.1.0'
paths:
  /health:
    get:
      responses:
        '200':
          headers:
            chicken-nuggets:
              description: chicken nuggets response
              required: false
              schema:
                type: integer
          description: pet response`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	// build a request
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/health", nil)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Chicken-Cakes", "I should fail")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(nil)
	}

	// fire the request
	handler(res, request)

	response := res.Result()

	headers := m.Model.Paths.PathItems.GetOrZero("/health").Get.Responses.Codes.GetOrZero("200").Headers

	// validate!
	valid, errors := ValidateResponseHeaders(request, response, headers, "/health", "200")

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}
