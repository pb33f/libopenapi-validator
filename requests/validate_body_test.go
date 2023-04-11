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

func TestValidateBody_ValidSchemaUsingAllOf(t *testing.T) {
    spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/TestBody' 
components:
  schemas:
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
        - $ref: '#/components/schemas/Nutrients'
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
              $ref: '#/components/schemas/TestBody' 
components:
  schemas:
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
        - $ref: '#/components/schemas/Nutrients'
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
    assert.Len(t, errors[0].SchemaValidationErrors, 3)
}
