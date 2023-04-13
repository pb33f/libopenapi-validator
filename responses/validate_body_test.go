// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
    "bytes"
    "encoding/json"
    "github.com/pb33f/libopenapi"
    "github.com/pb33f/libopenapi-validator/helpers"
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
    assert.Equal(t, 6, errors[0].SchemaValidationErrors[0].Line)
    assert.Equal(t, 8, errors[0].SchemaValidationErrors[1].Line)
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

    // primitves are now correct.
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
