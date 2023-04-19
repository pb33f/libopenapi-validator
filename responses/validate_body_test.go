// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateBody_MissingContentType(t *testing.T) {
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
		w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bodyBytes)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST / 200 operation response content type 'cheeky/monkey' does not exist", errors[0].Message)
	assert.Equal(t, "The content type is invalid, Use one of the 1 "+
		"supported types for this operation: application/json", errors[0].HowToFix)
}

func TestValidateBody_MissingPath(t *testing.T) {
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

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/I do not exist", bytes.NewReader(bodyBytes))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey") // won't even matter!
		w.WriteHeader(http.StatusUnprocessableEntity)              // does not matter.
		_, _ = w.Write(bodyBytes)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path '/I do not exist' not found", errors[0].Message)
}

func TestValidateBody_SetPath(t *testing.T) {
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

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/I do not exist", bytes.NewReader(bodyBytes))
	request.Header.Set(helpers.ContentTypeHeader, helpers.JSONContentType)

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model)
	v.SetPathItem(path, pv)

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey") // won't even matter!
		w.WriteHeader(http.StatusUnprocessableEntity)              // does not matter.
		_, _ = w.Write(bodyBytes)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path '/I do not exist' not found", errors[0].Message)
}

func TestValidateBody_MissingStatusCode(t *testing.T) {
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
		w.Header().Set(helpers.ContentTypeHeader, "cheeky/monkey") // won't even matter!
		w.WriteHeader(http.StatusUnprocessableEntity)              // undefined in the spec.
		_, _ = w.Write(bodyBytes)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST operation request response code '422' does not exist", errors[0].Message)
	assert.Equal(t, "The service is responding with a code that is not defined in the spec, fix the service!", errors[0].HowToFix)
}

func TestValidateBody_InvalidBasicSchema(t *testing.T) {
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

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Equal(t, "200 response body for '/burgers/createBurger' failed to validate schema", errors[0].Message)
	assert.Equal(t, "expected integer, but got boolean", errors[0].SchemaValidationErrors[0].Reason)
}

func TestValidateBody_ValidComplexSchema(t *testing.T) {
	spec := `openapi: 3.1.0
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
      required: [name, patties, vegetarian]`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model)

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

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidComplexSchema(t *testing.T) {
	spec := `openapi: 3.1.0
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
      required: [name, patties, vegetarian]`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":          "Big Mac",
		"patties":       2,
		"vegetarian":    true,
		"fat":           10.0,
		"salt":          0.5,
		"meat":          "beef",
		"usedOil":       12345, // invalid, should be bool
		"usedAnimalFat": false,
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

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 3)
	assert.Equal(t, "expected boolean, but got number", errors[0].SchemaValidationErrors[2].Reason)
}

func TestValidateBody_ValidBasicSchema(t *testing.T) {
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

	body := map[string]interface{}{
		"name":       "Big Mac",
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

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_ValidBasicSchema_WithFullContentTypeHeader(t *testing.T) {
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

	body := map[string]interface{}{
		"name":       "Big Mac",
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

		// inject a full content type header, including charset and boundary
		w.Header().Set(helpers.ContentTypeHeader,
			fmt.Sprintf("%s; charset=utf-8; boundary=---12223344", helpers.JSONContentType))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bodyBytes)
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

func TestValidateBody_ValidBasicSchemaUsingDefault(t *testing.T) {
	spec := `openapi: 3.1.0
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
                    type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
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

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidBasicSchemaUsingDefault_MissingContentType(t *testing.T) {
	spec := `openapi: 3.1.0
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
                    type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewResponseBodyValidator(&m.Model)

	// primitives are now correct.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": false,
	}

	bodyBytes, _ := json.Marshal(body)

	// build a request
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", bytes.NewReader(bodyBytes))
	request.Header.Set(helpers.ContentTypeHeader, "chicken/nuggets;chicken=soup")

	// simulate a request/response
	res := httptest.NewRecorder()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(helpers.ContentTypeHeader, r.Header.Get(helpers.ContentTypeHeader))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bodyBytes)
	}

	// fire the request
	handler(res, request)

	// record response
	response := res.Result()

	// validate!
	valid, errors := v.ValidateResponseBody(request, response)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST / 200 operation response content type 'chicken/nuggets;chicken=soup' does not exist", errors[0].Message)
}
