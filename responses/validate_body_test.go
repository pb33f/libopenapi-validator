// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
)

type validateResponseTestBed struct {
	responseBodyValidator ResponseBodyValidator
	httpTestServer        *httptest.Server
	responseHandlerFunc   http.HandlerFunc
}

func newvalidateResponseTestBed(
	t *testing.T,
	openApiSpec []byte,
) *validateResponseTestBed {
	doc, err := libopenapi.NewDocument(openApiSpec)
	if err != nil {
		t.Fatalf("failed to create openapi document: %v", err)
	}

	m, buildV3ModelErr := doc.BuildV3Model()
	if buildV3ModelErr != nil {
		t.Fatalf("failed to build v3 model: %v", err)
	}

	tb := validateResponseTestBed{responseBodyValidator: NewResponseBodyValidator(&m.Model, config.WithXmlBodyValidation())}
	tb.httpTestServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tb.responseHandlerFunc != nil {
			tb.responseHandlerFunc(w, r)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		tb.httpTestServer.Close()
	})

	return &tb
}

func (tb *validateResponseTestBed) makeRequestWithReponse(
	t *testing.T,
	method string,
	path string,
	responseHandler http.HandlerFunc,
) (
	*http.Request,
	*http.Response,
) {
	tb.responseHandlerFunc = responseHandler

	req, err := http.NewRequestWithContext(context.TODO(), method, tb.httpTestServer.URL+path, nil)
	if err != nil {
		t.Fatalf("failed to create http request: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to perform http request: %v", err)
	}

	return req, res
}

func TestValidateBody_MissingContentType(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"Big Mac","patties":false,"vegetarian":2}`))
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST / 200 operation response content type 'cheeky/monkey' does not exist", errors[0].Message)
	assert.Equal(t, "The content type is invalid, Use one of the 1 supported types for this operation: application/json", errors[0].HowToFix)
	assert.Equal(t, req.Method, errors[0].RequestMethod)
	assert.Equal(t, req.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/burgers/createBurger", errors[0].SpecPath)
}

func TestValidateBody_MissingContentType4XX(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        4XX:
          content:
            application/json:
              schema:
                type: object
                properties:
                  error:
                    type: string`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    false,
				"vegetarian": 2,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST / 4XX operation response content type 'cheeky/monkey' does not exist", errors[0].Message)
	assert.Equal(t, "The content type is invalid, Use one of the 1 supported types for this operation: application/json", errors[0].HowToFix)
	assert.Equal(t, req.Method, errors[0].RequestMethod)
	assert.Equal(t, req.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/burgers/createBurger", errors[0].SpecPath)
}

func TestValidateBody_MissingPath(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/I do not exist",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    false,
				"vegetarian": 2,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey") // won't even matter!
			w.WriteHeader(http.StatusUnprocessableEntity)              // does not matter.
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST Path '/I do not exist' not found", errors[0].Message)
	assert.Equal(t, req.Method, errors[0].RequestMethod)
	assert.Equal(t, req.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "", errors[0].SpecPath)
}

func TestValidateBody_SetPath(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/I do not exist",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    false,
				"vegetarian": 2,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey") // won't even matter!
			w.WriteHeader(http.StatusUnprocessableEntity)              // does not matter.
			_, _ = w.Write(bodyBytes)
		},
	)

	// preset the path
	m := tb.responseBodyValidator.(*responseBodyValidator).document
	path, _, pv := paths.FindPath(req, m, nil)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBodyWithPathItem(req, res, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST Path '/I do not exist' not found", errors[0].Message)
}

func TestValidateBody_SetPath_missing_operation(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    2,
				"vegetarian": false,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType) // won't even matter!
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// preset the path
	m := tb.responseBodyValidator.(*responseBodyValidator).document
	path, _, pv := paths.FindPath(req, m, nil)

	// Create a different request with GET method to test missing operation
	request2, _ := http.NewRequest(http.MethodGet, req.URL.String(), nil)
	request2.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBodyWithPathItem(request2, res, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "GET operation request content type 'GET' does not exist", errors[0].Message)
}

func TestValidateBody_MissingStatusCode(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    false,
				"vegetarian": 2,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey") // won't even matter!
			w.WriteHeader(http.StatusUnprocessableEntity)              // undefined in the spec.
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST operation request response code '422' does not exist", errors[0].Message)
	assert.Equal(t, "The service is responding with a code that is not defined in the spec, fix the service or add the code to the specification", errors[0].HowToFix)
}

func TestValidateBody_InvalidBasicSchema(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			// mix up the primitives to fire two schema violations.
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    false,
				"vegetarian": 2,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	// doubletap to hit cache
	_, _ = tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
}

func TestValidateBody_NoBody(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			// Don't write anything - this creates a response with no body
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	// doubletap to hit cache
	_, _ = tb.responseBodyValidator.ValidateResponseBody(req, res)

	// With the real HTTP server, an empty body is now properly detected
	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST response object is missing for '/burgers/createBurger'", errors[0].Message)
}

func TestValidateBody_InvalidResponseBodyNil(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			// Don't write anything - this creates a response with no body
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	// doubletap to hit cache
	_, _ = tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.ErrorContains(t, errors[0], "response object is missing")
}

func TestValidateBody_InvalidResponseBodyError(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			// Don't write anything - this creates a response with no body
		},
	)

	// simulate an error reading the body
	res.Body = &errorReader{}

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	// doubletap to hit cache
	_, _ = tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.ErrorContains(t, errors[0], "The response body cannot be decoded: some io error")
}

func TestValidateBody_InvalidBasicSchema_SetPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", bytes.NewReader(bodyBytes))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bodyBytes)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model, nil)

	// validate!
	valid, errors := v.ValidateResponseBodyWithPathItem(request, response, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Equal(t, "200 response body for '/burgers/createBurger' failed to validate schema", errors[0].Message)
}

func TestValidateBody_ValidComplexSchema(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schema_validation/TestBody'
components:
  schema_validation:
    Uncooked:
      type: object
      required: [uncookedWeight, uncookedHeight]
      properties:
        uncookedWeight:
          type: number
        uncookedHeight:
          type: number
    Cooked:
      type: object
      required: [usedOil, usedAnimalFat]
      properties:
        usedOil:
          type: boolean
        usedAnimalFat:
          type: boolean
    Nutrients:
      type: object
      required: [fat, salt, meat]
      properties:
        fat:
          type: number
        salt:
          type: number
        meat:
          type: string
          enum:
            - beef
            - pork
            - lamb
            - vegetables
    TestBody:
      type: object
      oneOf:
        - $ref: '#/components/schema_validation/Uncooked'
        - $ref: '#/components/schema_validation/Cooked'
      allOf:
        - $ref: '#/components/schema_validation/Nutrients'
      properties:
        name:
          type: string
        patties:
          type: integer
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]`,
		),
	)

	body := map[string]interface{}{
		"name":          "Big Mac",
		"patties":       2,
		"vegetarian":    true,
		"fat":           10.0,
		"salt":          0.5,
		"meat":          "beef",
		"usedOil":       true,
		"usedAnimalFat": false,
	}

	bodyBytes, _ := json.Marshal(body)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidComplexSchema(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schema_validation/TestBody'
components:
  schema_validation:
    Uncooked:
      type: object
      required: [uncookedWeight, uncookedHeight]
      properties:
        uncookedWeight:
          type: number
        uncookedHeight:
          type: number
    Cooked:
      type: object
      required: [usedOil, usedAnimalFat]
      properties:
        usedOil:
          type: boolean
        usedAnimalFat:
          type: boolean
    Nutrients:
      type: object
      required: [fat, salt, meat]
      properties:
        fat:
          type: number
        salt:
          type: number
        meat:
          type: string
          enum:
            - beef
            - pork
            - lamb
            - vegetables
    TestBody:
      type: object
      oneOf:
        - $ref: '#/components/schema_validation/Uncooked'
        - $ref: '#/components/schema_validation/Cooked'
      allOf:
        - $ref: '#/components/schema_validation/Nutrients'
      properties:
        name:
          type: string
        patties:
          type: integer
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":          "Big Mac",
				"patties":       2,
				"vegetarian":    true,
				"fat":           10.0,
				"salt":          0.5,
				"meat":          "beef",
				"usedOil":       12345, // invalid, should be bool
				"usedAnimalFat": false,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Equal(t, "missing properties 'uncookedWeight', 'uncookedHeight'", errors[0].SchemaValidationErrors[0].Reason)
}

func TestValidateBody_ValidBasicSchema(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    2,
				"vegetarian": false,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_ValidBasicSchema_WithFullContentTypeHeader(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    2,
				"vegetarian": false,
			})

			require.NoError(t, err, "failed to marshal body")

			// inject a full content type header, including charset and boundary
			w.Header().Set(helpers.ContentTypeHeader,
				fmt.Sprintf("%s; charset=utf-8; boundary=---12223344", helpers.JSONContentType))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_ValidBasicSchemaUsingDefault(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        default:
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    2,
				"vegetarian": false,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidBasicSchemaUsingDefault_MissingContentType(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        default:
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			// primitives are now correct.
			bodyBytes, err := json.Marshal(map[string]interface{}{
				"name":       "Big Mac",
				"patties":    2,
				"vegetarian": false,
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, "chicken/nuggets;chicken=soup")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST / 200 operation response content type 'chicken/nuggets' does not exist", errors[0].Message)
}

func TestValidateBody_InvalidSchemaMultiple(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  required:
                    - name
                  properties:
                    name:
                      type: string
                    patties:
                      type: integer
                    vegetarian:
                      type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := json.Marshal([]map[string]interface{}{
				{
					"patties":    1,
					"vegetarian": true,
				},
				{
					"name":       "Quarter Pounder",
					"patties":    true,
					"vegetarian": false,
				},
				{
					"name":       "Big Mac",
					"patties":    2,
					"vegetarian": false,
				},
			})

			require.NoError(t, err, "failed to marshal body")

			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bodyBytes)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Equal(t, "200 response body for '/burgers/createBurger' failed to validate schema", errors[0].Message)
}

func TestValidateBody_EmptyContentType_Valid(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: "3.0.0"
info:
  title: Healthcheck
  version: '0.1.0'
paths:
  /health:
    get:
      responses:
        '200':
          description: pet response
          content: {}`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodGet,
		"/health",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(nil)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidBodyJSON(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        default:
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodPost,
		"/burgers/createBurger",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{\"bad\": \"json\",}"))
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST response body for '/burgers/createBurger' failed to validate schema", errors[0].Message)
	assert.Equal(t, "invalid character '}' looking for beginning of object key string", errors[0].SchemaValidationErrors[0].Reason)
}

func TestValidateBody_NoContentType_Valid(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: "3.0.0"
info:
  title: Healthcheck
  version: '0.1.0'
paths:
  /health:
    get:
      responses:
        '200':
          description: pet response`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodGet,
		"/health",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(nil)
		},
	)

	// validate!
	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

// https://github.com/pb33f/libopenapi-validator/issues/107
// https://github.com/pb33f/libopenapi-validator/issues/103
func TestNewValidator_TestCircularRefsInValidation_Response(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`openapi: 3.1.0
info:
  title: Panic at response validation
  version: 1.0.0
paths:
  /operations:
    delete:
      description: Delete operations
      responses:
        default:
          description: Any response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

components:
  schemas:
    Error:
      type: object
      properties:
        code:
          type: string
        details:
          type: array
          items:
            $ref: '#/components/schemas/Error'`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodDelete,
		"/operations",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(nil)
		},
	)

	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	// The error message may vary depending on whether the circular reference is caught
	// during rendering or compilation, so we check for either pattern
	assert.True(t,
		strings.Contains(errors[0].Reason, "circular reference") ||
			strings.Contains(errors[0].Reason, "json-pointer") ||
			strings.Contains(errors[0].Reason, "not found"),
		"Expected error about circular reference or JSON pointer not found, got: %s", errors[0].Reason)
}

func TestValidateResponseBody_XMLMarshalError(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`
openapi: 3.1.0
info:
  title: Test Spec
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
          content:
            application/xml:
              schema:
                type: object
                properties:
                  bad_number:
                    type: number
`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodGet,
		"/test",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<bad_number>NaN</bad_number>"))
		},
	)

	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, errors[0].Message, "xml example is malformed")
}

func TestValidateResponseBody_NilSchema(t *testing.T) {
	tb := newvalidateResponseTestBed(
		t,
		[]byte(`
openapi: 3.1.0
info:
  title: Test Spec
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: Success
          content:
            application/json: {}
`,
		),
	)

	req, res := tb.makeRequestWithReponse(
		t,
		http.MethodGet,
		"/test",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(nil)
		},
	)

	valid, errors := tb.responseBodyValidator.ValidateResponseBody(req, res)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_CheckHeader(t *testing.T) {
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
	v := NewResponseBodyValidator(&m.Model)

	// build a request
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/health", nil)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(nil)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Missing required header", errors[0].Message)
	assert.Equal(t, "Required header 'chicken-nuggets' was not found in response", errors[0].Reason)
}

// TestValidateBody_ComplexRegexSchemaCompilationError tests that complex regex patterns
// that cause schema compilation to fail are handled gracefully instead of causing panics
func TestValidateBody_ComplexRegexSchemaCompilationError(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                    pattern: "[\\w\\W]{1,1024}$"
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac test",
		"patties":    2,
		"vegetarian": false,
	}

	bodyBytes, _ := json.Marshal(body)

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", bytes.NewReader(bodyBytes))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, helpers.JSONContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bodyBytes)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate - this should not panic even if schema compilation fails
	valid, validationErrors := v.ValidateResponseBody(request, response)

	// if schema compilation failed due to complex regex, we should get a validation error instead of a panic
	if !valid {
		// verify we got a schema compilation error instead of a panic
		assert.NotEmpty(t, validationErrors)
		found := false
		for _, err := range validationErrors {
			if err.ValidationSubType == helpers.Schema &&
				err.SchemaValidationErrors != nil &&
				len(err.SchemaValidationErrors) > 0 {
				for _, schemaErr := range err.SchemaValidationErrors {
					if schemaErr.Location == "schema compilation" &&
						schemaErr.Reason != "" {
						found = true
						assert.Contains(t, schemaErr.Reason, "failed to compile JSON schema")
						assert.Contains(t, err.HowToFix, "complex regex patterns")
						break
					}
				}
			}
		}
		if !found {
			// if it didn't fail compilation, it should have succeeded
			t.Logf("Schema compilation succeeded, validation result: %v", valid)
		}
	} else {
		// schema compiled and validated successfully
		assert.True(t, valid)
		assert.Empty(t, validationErrors)
	}
}

func TestValidateBody_StrictMode_UndeclaredProperty(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/getBurger:
    get:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model, config.WithStrictMode())

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/getBurger", nil)

	// Response with undeclared property 'extra'
	responseBody := `{"name": "Big Mac", "patties": 2, "extra": "undeclared"}`
	response := &http.Response{
		Header:     http.Header{},
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}
	response.Header.Set("Content-Type", "application/json")

	valid, errs := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, "extra")
	assert.Contains(t, errs[0].Message, "not declared")
}

func TestValidateBody_StrictMode_ValidResponse(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/getBurger:
    get:
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model, config.WithStrictMode())

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/getBurger", nil)

	// Response with only declared properties
	responseBody := `{"name": "Big Mac", "patties": 2}`
	response := &http.Response{
		Header:     http.Header{},
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}
	response.Header.Set("Content-Type", "application/json")

	valid, errs := v.ValidateResponseBody(request, response)

	assert.True(t, valid)
	assert.Len(t, errs, 0)
}

func TestValidateBody_ValidXmlDecode(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        default:
          content:
            application/xml:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model, config.WithXmlBodyValidation())

	body := "<name>test</name><patties>2</patties>"

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", bytes.NewReader([]byte(body)))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_ValidXmlFailedValidation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        default:
          content:
            application/xml:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model, config.WithXmlBodyValidation())

	body := "<name>20</name><patties>text</patties>"

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", bytes.NewReader([]byte(body)))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
}

func TestValidateBody_IgnoreXmlValidation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        default:
          content:
            application/xml:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model)

	body := "invalidbodycausenoxml"

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", bytes.NewReader([]byte(body)))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidXmlParse(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      responses:
        default:
          content:
            application/xml:
              schema:
                type: object
                properties:
                  name:
                    type: string
                  patties:
                    type: integer
                  vegetarian:
                    type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model, config.WithXmlBodyValidation())

	body := ""

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", bytes.NewReader([]byte(body)))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "xml example is malformed", errors[0].Message)
}

type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("some io error")
}

func (er *errorReader) Close() error {
	return nil
}
