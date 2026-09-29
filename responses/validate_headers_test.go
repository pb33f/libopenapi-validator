// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package responses

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
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

func TestValidateResponseHeaders_StrictMode(t *testing.T) {
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
            x-request-id:
              description: request ID
              required: false
              schema:
                type: string
          description: healthy response`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// build a request
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/health", nil)

	// simulate a response with an undeclared header
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc-123")
		w.Header().Set("X-Undeclared-Header", "should fail in strict mode")
		w.WriteHeader(http.StatusOK)
	}

	handler(res, request)
	response := res.Result()

	headers := m.Model.Paths.PathItems.GetOrZero("/health").Get.Responses.Codes.GetOrZero("200").Headers

	// validate with strict mode - should find undeclared header
	valid, errors := ValidateResponseHeaders(request, response, headers, "/health", "200", config.WithStrictMode())

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "X-Undeclared-Header")
	assert.Contains(t, errors[0].Message, "not declared")
}

func TestValidateResponseHeaders_StrictMode_NoUndeclared(t *testing.T) {
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
            x-request-id:
              description: request ID
              required: false
              schema:
                type: string
          description: healthy response`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/health", nil)

	// response with only declared headers (x-request-id is declared, Content-Type is default-ignored)
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc-123")
		w.WriteHeader(http.StatusOK)
	}

	handler(res, request)
	response := res.Result()

	headers := m.Model.Paths.PathItems.GetOrZero("/health").Get.Responses.Codes.GetOrZero("200").Headers

	// validate with strict mode - should pass (no undeclared headers)
	valid, errors := ValidateResponseHeaders(request, response, headers, "/health", "200", config.WithStrictMode())

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateResponseHeaders_OptionalHeaderIsValidatedWhenPresent(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Headers
  version: 1.0.0
paths:
  /things:
    get:
      responses:
        '200':
          description: ok
          headers:
            X-Rate-Limit:
              schema:
                type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	headers := m.Model.Paths.PathItems.GetOrZero("/things").Get.Responses.Codes.GetOrZero("200").Headers
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/things", nil)

	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Rate-Limit": {"abc"}}}
	valid, errors := ValidateResponseHeaders(request, response, headers, "/things", "200")
	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "header 'x-rate-limit' failed to validate", errors[0].Message)

	response = &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}
	valid, errors = ValidateResponseHeaders(request, response, headers, "/things", "200")
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestValidateResponseHeaders_ContentHeaderIsNotSchemaValidated(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Headers
  version: 1.0.0
paths:
  /things:
    get:
      responses:
        '200':
          description: ok
          headers:
            X-Payload:
              content:
                application/json:
                  schema:
                    type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	headers := m.Model.Paths.PathItems.GetOrZero("/things").Get.Responses.Codes.GetOrZero("200").Headers
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/things", nil)

	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Payload": {`{"a":1}`}}}
	valid, errors := ValidateResponseHeaders(request, response, headers, "/things", "200")
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestValidateResponseHeaders_ValuesDecodeBySchemaType(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Headers
  version: 1.0.0
paths:
  /things:
    get:
      responses:
        '200':
          description: ok
          headers:
            X-String:
              schema: {type: string, maxLength: 5}
            X-Integer:
              schema: {type: integer, maximum: 10}
            X-Number:
              schema: {type: number, minimum: 1.5}
            X-Boolean:
              schema: {type: boolean}
            X-Array:
              schema: {type: array, items: {type: integer}, maxItems: 3}
            X-Object:
              schema:
                type: object
                properties:
                  id: {type: integer}
            X-Exploded:
              explode: true
              schema:
                type: object
                properties:
                  id: {type: integer}
            X-Enum:
              schema: {enum: [1, 2]}
            X-NaN:
              schema: {type: number, maximum: 10}
            X-Either:
              schema: {type: [number, boolean]}
            X-Nullable:
              schema: {type: [integer, "null"]}
            X-Short:
              schema: {type: [string, integer], maxLength: 2}
            X-StringEnum:
              schema: {enum: ["1", "2"]}
            X-Wrapped:
              schema:
                allOf:
                  - type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	headers := m.Model.Paths.PathItems.GetOrZero("/things").Get.Responses.Codes.GetOrZero("200").Headers
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/things", nil)

	for _, test := range []struct {
		header string
		value  string
		valid  bool
	}{
		{"X-String", "123", true},
		{"X-String", "true", true},
		{"X-String", "null", true},
		{"X-String", "too long", false},
		{"X-Integer", "7", true},
		{"X-Integer", "7.0", true},
		{"X-Integer", "11", false},
		{"X-Integer", "seven", false},
		{"X-Number", "2.5", true},
		{"X-Number", "1", false},
		{"X-Boolean", "false", true},
		{"X-Boolean", "maybe", false},
		{"X-Array", "1", true},
		{"X-Array", "1, 2,3", true},
		{"X-Array", "1,two", false},
		{"X-Array", "1,2,3,4", false},
		{"X-Object", "id,5", true},
		{"X-Object", "id,five", false},
		{"X-Exploded", "id=5", true},
		{"X-Exploded", "id=five", false},
		{"X-Enum", "2", true},
		{"X-Enum", "3", false},
		{"X-NaN", "NaN", false},
		{"X-Either", "1.5", true},
		{"X-Either", "true", true},
		{"X-Either", "maybe", false},
		{"X-Nullable", "null", true},
		{"X-Nullable", "7", true},
		{"X-Short", "12345", true},
		{"X-StringEnum", "1", true},
		{"X-Wrapped", "123", true},
		{"X-Array", "[1,2]", true},
	} {
		t.Run(test.header+"="+test.value, func(t *testing.T) {
			response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}
			response.Header.Set(test.header, test.value)

			valid, errors := ValidateResponseHeaders(request, response, headers, "/things", "200")
			assert.Equal(t, test.valid, valid, errors)
		})
	}
}

func TestHeaderValueReadings(t *testing.T) {
	assert.Equal(t, []any{"5", float64(5)}, headerValueReadings("5", &base.Schema{Type: []string{"string"}}, false))
	assert.Equal(t, []any{int64(5), 5.0, float64(5), "5"},
		headerValueReadings("5", &base.Schema{Type: []string{"integer", "number"}}, false))
	assert.Equal(t, []any{[]any{"a", "b"}, "a,b"}, headerValueReadings("a,b", &base.Schema{Type: []string{"array"}}, false))
	assert.Equal(t, []any{"not-json"}, headerValueReadings("not-json", &base.Schema{}, false))
	assert.Equal(t, []any{map[string]any{"a": float64(1)}, `{"a":1}`}, headerValueReadings(`{"a":1}`, &base.Schema{}, false))
}

func TestValidateResponseHeaders_ContentTypeIsIgnored(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Headers
  version: 1.0.0
paths:
  /things:
    get:
      responses:
        '200':
          description: ok
          headers:
            Content-Type:
              required: true
              schema: {type: string, enum: [application/json]}`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	headers := m.Model.Paths.PathItems.GetOrZero("/things").Get.Responses.Codes.GetOrZero("200").Headers
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/things", nil)

	for _, header := range []http.Header{{"Content-Type": {"application/json; charset=utf-8"}}, {}} {
		valid, errors := ValidateResponseHeaders(request, &http.Response{StatusCode: http.StatusOK, Header: header}, headers, "/things", "200")
		assert.True(t, valid, errors)
	}
}

func TestValidateResponseHeaders_OpenAPI30Keywords(t *testing.T) {
	spec := `openapi: 3.0.3
info:
  title: Headers
  version: 1.0.0
paths:
  /things:
    get:
      responses:
        '200':
          description: ok
          headers:
            X-Rate:
              schema: {type: integer, nullable: true, minimum: 1, exclusiveMinimum: true}`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	headers := m.Model.Paths.PathItems.GetOrZero("/things").Get.Responses.Codes.GetOrZero("200").Headers
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/things", nil)

	for value, valid := range map[string]bool{"5": true, "null": true, "1": false, "abc": false} {
		response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Rate": {value}}}
		ok, errors := ValidateResponseHeaders(request, response, headers, "/things", "200")
		assert.Equal(t, valid, ok, "%s: %v", value, errors)
	}
}
