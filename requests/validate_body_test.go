// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/stretchr/testify/assert"
)

func TestValidateBody_NotRequiredBody(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        required: false
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
	v := NewRequestBodyValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger", http.NoBody)

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_UnknownContentType(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        required: true
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
	v := NewRequestBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "thomas/tank-engine") // wtf kinda content type is this?

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST operation request content type 'thomas/tank-engine' does not exist", errors[0].Message)
	assert.Equal(t, "The content type is invalid, Use one of the 1 "+
		"supported types for this operation: application/json", errors[0].HowToFix)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/burgers/createBurger", errors[0].SpecPath)
}

func TestValidateBody_SkipValidationForNonJSON(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/yaml:
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
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/yaml")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_PathNotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
	v := NewRequestBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/I do not exist",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST Path '/I do not exist' not found", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "", errors[0].SpecPath)
}

func TestValidateBody_OperationNotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
	v := NewRequestBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	pathItem, validationErrors, pathValue := paths.FindPath(request, &m.Model)
	assert.Len(t, validationErrors, 0)

	request2, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request2.Header.Set("Content-Type", "application/json")
	valid, errors := v.ValidateRequestBodyWithPathItem(request2, pathItem, pathValue)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "GET operation request content type 'GET' does not exist", errors[0].Message)
	assert.Equal(t, request2.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/burgers/createBurger", errors[0].SpecPath)
}

func TestValidateBody_SetPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
	v := NewRequestBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBodyWithPathItem(request, nil, "")

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST Path '/burgers/createBurger' not found", errors[0].Message)

}

func TestValidateBody_ContentTypeNotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        required: true
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
	v := NewRequestBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("content-type", "application/not-json")

	pathItem, validationErrors, pathValue := paths.FindPath(request, &m.Model)
	assert.Len(t, validationErrors, 0)
	valid, errors := v.ValidateRequestBodyWithPathItem(request, pathItem, pathValue)

	assert.False(t, valid)
	assert.Len(t, errors, 1)

}

func TestValidateBody_ContentTypeNotSet(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        required: true
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
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))

	pathItem, validationErrors, pathValue := paths.FindPath(request, &m.Model)
	assert.Len(t, validationErrors, 0)
	valid, errors := v.ValidateRequestBodyWithPathItem(request, pathItem, pathValue)

	assert.False(t, valid)
	assert.Len(t, errors, 1)

}

func TestValidateBody_InvalidBasicSchema_NotRequired(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        required: false
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
	v := NewRequestBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	// double-tap to hit the cache
	_, _ = v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Equal(t, "POST request body for '/burgers/createBurger' failed to validate schema", errors[0].Message)

}

func TestValidateBody_InvalidBasicSchema_Required(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        required: false
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
	v := NewRequestBodyValidator(&m.Model)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false,
		"vegetarian": 2,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	// double-tap to hit the cache
	_, _ = v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Equal(t, "POST request body for '/burgers/createBurger' failed to validate schema", errors[0].Message)

}

func TestValidateBody_ValidBasicSchema(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

}

func TestValidateBody_ValidBasicSchema_WithFullContentTypeHeader(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json; charset=utf-8; boundary=12345")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

}

func TestValidateBody_ValidSchemaUsingAllOf(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody' 
components:
  schema_validation:
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
      allOf:
        - $ref: '#/components/schema_validation/Nutrients'
      properties:
        name:
          type: string
        patties:
          type: integer
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidSchemaUsingAllOf(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody' 
components:
  schema_validation:
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
      allOf:
        - $ref: '#/components/schema_validation/Nutrients'
      properties:
        name:
          type: string
        patties:
          type: integer
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       false,    // invalid
		"meat":       "turkey", // invalid
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 3) // throws 'allOf failure' in addition
}

func TestValidateBody_ValidSchemaUsingAllOfAnyOf(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
      required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

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

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateBody_InvalidSchemaUsingOneOf(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
      required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 3)
	assert.Equal(t, "oneOf failed", errors[0].SchemaValidationErrors[0].Reason)
	assert.Equal(t, "missing properties: 'uncookedWeight', 'uncookedHeight'", errors[0].SchemaValidationErrors[1].Reason)
	assert.Equal(t, "missing properties: 'usedOil', 'usedAnimalFat'", errors[0].SchemaValidationErrors[2].Reason)

}

func TestValidateBody_InvalidSchemaMinMax(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody' 
components:
  schema_validation:
    TestBody:
      type: object
      properties:
        name:
          type: string
        patties:
          type: integer
          maximum: 3
          minimum: 1
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    5,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "must be <= 3 but found 5", errors[0].SchemaValidationErrors[0].Reason)

}

func TestValidateBody_InvalidSchemaMaxItems(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody' 
components:
  schema_validation:
    TestBody:
      type: array
      maxItems: 2
      items:
        type: object
        properties:
          name:
            type: string
          patties:
            type: integer
            maximum: 3
            minimum: 1
          vegetarian:
            type: boolean
        required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
		"fat":        10.0,
		"salt":       0.5,
		"meat":       "beef",
	}
	bodyArray := []interface{}{body, body, body, body} // two too many!
	bodyBytes, _ := json.Marshal(bodyArray)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "maximum 2 items required, but found 4 items", errors[0].SchemaValidationErrors[0].Reason)
	assert.Equal(t, 2, errors[0].SchemaValidationErrors[0].Line)
	assert.Equal(t, "maximum 2 items required, but found 4 items", errors[0].SchemaValidationErrors[0].Reason)
	assert.Equal(t, 11, errors[0].SchemaValidationErrors[0].Column)
}

func TestValidateBody_SchemaHasNoRequestBody(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		http.NoBody)
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

}

func TestValidateBody_MediaTypeHasNullSchema(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		http.NoBody)
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

}

func TestValidateBody_MissingBody(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody' 
components:
  schema_validation:
    TestBody:
      type: array
      maxItems: 2
      items:
        type: object
        properties:
          name:
            type: string
          patties:
            type: integer
            maximum: 3
            minimum: 1
          vegetarian:
            type: boolean
        required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		http.NoBody)
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)

}

func TestValidateBody_NoBodyNoNothing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		http.NoBody)
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

}

func TestValidateBody_InvalidSchemaMultipleItems(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
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
                    type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	var items []map[string]interface{}
	items = append(items, map[string]interface{}{
		"patties":    1,
		"vegetarian": true,
	})
	items = append(items, map[string]interface{}{
		"name":       "Quarter Pounder",
		"patties":    true,
		"vegetarian": false,
	})
	items = append(items, map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": false,
	})

	bodyBytes, _ := json.Marshal(items)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	// double-tap to hit the cache
	_, _ = v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Equal(t, "POST request body for '/burgers/createBurger' failed to validate schema", errors[0].Message)

}

func TestValidateBody_InvalidSchema_BadDecode(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schema_validation/TestBody' 
components:
  schema_validation:
    TestBody:
      type: object
      properties:
        name:
          type: string
        patties:
          type: integer
          maximum: 3
          minimum: 1
        vegetarian:
          type: boolean
      required: [name, patties, vegetarian]    `

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewRequestBodyValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer([]byte("{\"bad\": \"json\",}")))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateRequestBody(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "invalid character '}' looking for beginning of object key string", errors[0].SchemaValidationErrors[0].Reason)

}

func TestValidateBody_SchemaNoType_Issue75(t *testing.T) {

	spec := `{
  "openapi": "3.0.1",
  "info": {
    "title": "testing",
    "description": "<p>This is for testing purpose</p>",
    "version": "1.0",
    "x-targetEndpoint": "https://mocktarget.apigee.net/json"
  },
  "servers": [
    {
      "url": "https://some-url.com"
    }
  ],
  "paths": {
    "/path1": {
      "put": {
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "anyOf": [
                  {
                    "type": "object",
                    "properties": {
                      "name": {
                        "type": "string",
                        "minLength": 1
                      },
                      "age": {
                        "type": "integer"
                      }
                    },
                    "required": [
                      "name"
                    ]
                  },
                  {
                    "type": "object",
                    "properties": {
                      "email": {
                        "type": "string"
                      },
                      "address": {
                        "type": "string"
                      }
                    },
                    "required": [
                      "email"
                    ]
                  }
                ]
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    },
    "/path2": {
      "get": {
        "parameters": [
          {
            "name": "X-My-Header",
            "in": "header",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    },
    "/path3": {
      "get": {
        "parameters": [
          {
            "name": "id",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    }
  }
}
`

	doc, err := libopenapi.NewDocument([]byte(spec))
	if err != nil {
		fmt.Println("error while creating open api spec document", err)
		return
	}

	req, err := http.NewRequest("PUT", "/path1", nil)
	if err != nil {
		fmt.Println("error while creating new HTTP request", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	v3Model, errs := doc.BuildV3Model()
	if len(errs) > 0 {
		fmt.Println("error while building a Open API spec V3 model", errs)
		return
	}

	reqBodyValidator := NewRequestBodyValidator(&v3Model.Model)
	isSuccess, valErrs := reqBodyValidator.ValidateRequestBody(req)

	assert.False(t, isSuccess)
	assert.Len(t, valErrs, 1)
	assert.Equal(t, "PUT request body is empty for '/path1'", valErrs[0].Message)

}
