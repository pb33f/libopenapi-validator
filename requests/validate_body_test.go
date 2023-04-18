// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
    "bytes"
    "encoding/json"
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "net/http"
    "testing"
)

func TestValidateBody_MissingContentType(t *testing.T) {
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

    request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
        bytes.NewBuffer(bodyBytes))
    request.Header.Set("Content-Type", "thomas/tank-engine") // wtf kinda content type is this?

    valid, errors := v.ValidateRequestBody(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "POST operation request content type 'thomas/tank-engine' does not exist", errors[0].Message)
    assert.Equal(t, "The content type is invalid, Use one of the 1 "+
        "supported types for this operation: application/json", errors[0].HowToFix)
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
    request.Header.Set("Content-Type", "application/json") // wtf kinda content type is this?

    valid, errors := v.ValidateRequestBody(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Path '/I do not exist' not found", errors[0].Message)
}

func TestValidateBody_InvalidBasicSchema(t *testing.T) {
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

    request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
        bytes.NewBuffer(bodyBytes))
    request.Header.Set("Content-Type", "application/json")

    valid, errors := v.ValidateRequestBody(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "POST request body for '/burgers/createBurger' failed to validate schema", errors[0].Message)
    assert.Equal(t, "expected integer, but got boolean", errors[0].SchemaValidationErrors[0].Reason)

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
