// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "net/http"
    "testing"
)

func TestNewValidator_SimpleArrayEncodedPath(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerIds*}/locate:
    parameters:
      - name: burgerIds
        in: path
        schema:
          type: array
          items:
            type: integer
    patch:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodPatch, "https://things.com/burgers/1,2,3,4,5/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_SimpleArrayEncodedPath_InvalidNumber(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerIds*}/locate:
    parameters:
      - name: burgerIds
        in: path
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/1,pizza,3,4,false/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 2)
    assert.Equal(t, "Path array parameter 'burgerIds' is not a valid number", errors[0].Message)
}

func TestNewValidator_SimpleArrayEncodedPath_InvalidBool(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerIds*}/locate:
    parameters:
      - name: burgerIds
        in: path
        schema:
          type: array
          items:
            type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/1,true,0,frogs,false/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 3)
    assert.Equal(t, "Path array parameter 'burgerIds' is not a valid boolean", errors[0].Message)
}

func TestNewValidator_SimpleObjectEncodedPath(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burger}/locate:
    parameters:
      - name: burger
        in: path
        schema:
          type: object
          properties:
            id:
               type: integer
            vegetarian:
               type: boolean
    get:
      operationId: locateBurger`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/id,1234,vegetarian,true/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_SimpleObjectEncodedPath_Invalid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burger}/locate:
    parameters:
      - name: burger
        in: path
        schema:
          type: object
          properties:
            id:
               type: integer
            vegetarian:
               type: boolean
    get:
      operationId: locateBurger`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/id,hello,vegetarian,there/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Len(t, errors[0].SchemaValidationErrors, 2)
}

func TestNewValidator_SimpleObjectEncodedPath_Exploded(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burger}/locate:
    parameters:
      - name: burger
        in: path
        explode: true
        schema:
          type: object
          properties:
            id:
               type: integer
            vegetarian:
               type: boolean
    get:
      operationId: locateBurger`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/id=1234,vegetarian=true/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_SimpleObjectEncodedPath_ExplodedInvalid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burger}/locate:
    parameters:
      - name: burger
        in: path
        explode: true
        schema:
          type: object
          properties:
            id:
               type: integer
            vegetarian:
               type: boolean
    get:
      operationId: locateBurger`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/id=toast,vegetarian=chicken/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Len(t, errors[0].SchemaValidationErrors, 2)
}

func TestNewValidator_ObjectEncodedPath(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burger}/locate:
    parameters:
      - name: burger
        in: path
        schema:
          type: object
          properties:
            id:
               type: integer
            vegetarian:
               type: boolean
    get:
      operationId: locateBurger`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/id,1234,vegetarian,true/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_SimpleEncodedPath_InvalidInteger(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: integer
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/hello/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Path parameter 'burgerId' is not a valid number", errors[0].Message)
}

func TestNewValidator_SimpleEncodedPath_InvalidBoolean(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/hello/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Path parameter 'burgerId' is not a valid boolean", errors[0].Message)
}

func TestNewValidator_LabelEncodedPath_InvalidInteger(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: integer
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.hello/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Path parameter 'burgerId' is not a valid number", errors[0].Message)
}

func TestNewValidator_LabelEncodedPath_InvalidObject(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: object
          properties:
            id:
              type: integer
            vegetarian:
              type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.id,hello,vegetarian,why/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Len(t, errors[0].SchemaValidationErrors, 2)
}

func TestNewValidator_LabelEncodedPath_InvalidObject_Exploded(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        explode: true
        schema:
          type: object
          properties:
            id:
              type: integer
            vegetarian:
              type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.id=hello.vegetarian=why/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Len(t, errors[0].SchemaValidationErrors, 2)
}

func TestNewValidator_LabelEncodedPath_ValidMultiParam(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate/{.query}:
    parameters:
      - name: query
        in: path
        style: label
        schema:
          type: string
      - name: burgerId
        in: path
        style: label
        explode: true
        schema:
          type: object
          properties:
            id:
              type: integer
            vegetarian:
              type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.id=1234.vegetarian=true/locate/bigMac", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_LabelEncodedPath_InvalidMultiParam(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate/{.query}:
    parameters:
      - name: query
        in: path
        style: label
        schema:
          type: integer
      - name: burgerId
        in: path
        style: label
        explode: true
        schema:
          type: object
          properties:
            id:
              type: integer
            vegetarian:
              type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.id=1234.vegetarian=true/locate/bigMac", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
}

func TestNewValidator_MatrixEncodedPath_ValidPrimitiveNumber(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: integer
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=5/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_MatrixEncodedPath_InvalidPrimitiveNumber(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: integer
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=I am not a number/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Path parameter 'burgerId' is not a valid number", errors[0].Message)
}

func TestNewValidator_MatrixEncodedPath_ValidPrimitiveBoolean(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=false/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_MatrixEncodedPath_InvalidPrimitiveBoolean(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: boolean
    get:
      operationId: locateBurgers`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=I am also not a bool/locate", nil)
    valid, errors := v.ValidatePathParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Path parameter 'burgerId' is not a valid boolean", errors[0].Message)

}
