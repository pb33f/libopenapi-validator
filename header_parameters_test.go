// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "net/http"
    "testing"
)

func TestNewValidator_HeaderParamMissing(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /bish/bosh:
    get:
      parameters:
        - name: bash
          in: header
          required: true
          schema:
            type: string
`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/bish/bosh", nil)

    valid, errors := v.ValidateHeaderParams(request)

    assert.False(t, valid)
    assert.Equal(t, 1, len(errors))
    assert.Equal(t, "Header parameter 'bash' is missing", errors[0].Message)
}

func TestNewValidator_HeaderParamUndefined(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: fishy
          in: header
          schema:
            type: string
`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
    request.Header.Set("Mushypeas", "yes please") //https://github.com/golang/go/issues/5022

    valid, errors := v.ValidateHeaderParams(request)

    assert.False(t, valid)
    assert.Equal(t, 1, len(errors))
    assert.Equal(t, "Header parameter 'Mushypeas' is not defined", errors[0].Message)
}

func TestNewValidator_HeaderParamDefaultEncoding_InvalidParamTypeNumber(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: number`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
    request.Header.Set("coffeecups", "two") // headers are case-insensitive

    valid, errors := v.ValidateHeaderParams(request)

    assert.False(t, valid)
    assert.Equal(t, 1, len(errors))
    assert.Equal(t, "Header parameter 'coffeeCups' is not a valid number", errors[0].Message)
}

func TestNewValidator_HeaderParamDefaultEncoding_InvalidParamTypeBoolean(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: boolean`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
    request.Header.Set("coffeecups", "two") // headers are case-insensitive

    valid, errors := v.ValidateHeaderParams(request)

    assert.False(t, valid)
    assert.Equal(t, 1, len(errors))
    assert.Equal(t, "Header parameter 'coffeeCups' is not a valid boolean", errors[0].Message)
}

func TestNewValidator_HeaderParamDefaultEncoding_InvalidParamTypeObjectInvalid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: object
            properties:
              milk:
                type: boolean
              sugar:
                type: boolean`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
    request.Header.Set("coffeecups", "I am not an object") // headers are case-insensitive

    valid, errors := v.ValidateHeaderParams(request)

    assert.False(t, valid)
    assert.Equal(t, 1, len(errors))
    assert.Equal(t, "Header parameter 'coffeeCups' cannot be decoded", errors[0].Message)
}

func TestNewValidator_HeaderParamDefaultEncoding_InvalidParamTypeObjectNumber(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: object
            properties:
              milk:
                type: number
              sugar:
                type: boolean`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
    request.Header.Set("coffeecups", "milk,true,sugar,true") // default encoding.

    valid, errors := v.ValidateHeaderParams(request)

    assert.False(t, valid)
    assert.Equal(t, 1, len(errors))
    assert.Equal(t, "expected number, but got boolean", errors[0].SchemaValidationErrors[0].Reason)
}

func TestNewValidator_HeaderParamDefaultEncoding_InvalidParamTypeObjectBoolean(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: object
            properties:
              milk:
                type: number
              sugar:
                type: boolean`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
    request.Header.Set("coffeecups", "milk,true,sugar,true") // default encoding.

    valid, errors := v.ValidateHeaderParams(request)

    assert.False(t, valid)
    assert.Equal(t, 1, len(errors))
    assert.Equal(t, "expected number, but got boolean", errors[0].SchemaValidationErrors[0].Reason)
}
