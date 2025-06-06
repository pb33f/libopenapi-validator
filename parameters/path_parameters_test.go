// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/paths"
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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPatch, "https://things.com/burgers/1,2,3,4,5/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_SimpleArrayEncodedPath_InvalidInteger(t *testing.T) {
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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/1,pizza,3,4,false/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 2)
	assert.Equal(t, "Path array parameter 'burgerIds' is not a valid integer", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/burgers/{burgerIds*}/locate", errors[0].SpecPath)
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
            type: number
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/1,pizza,3,4,false/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 2)
	assert.Equal(t, "Path array parameter 'burgerIds' is not a valid number", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/burgers/{burgerIds*}/locate", errors[0].SpecPath)
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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/hello/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' is not a valid integer", errors[0].Message)
	assert.Equal(t, "The path parameter 'burgerId' is defined as being an integer, however the value 'hello' is not a valid integer", errors[0].Reason)
}

func TestNewValidator_SimpleEncodedPath_MinimumIntegerViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: integer
          minimum: 10
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/1/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: minimum: got 1, want 10, Location: /minimum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_SimpleEncodedPath_MinimumInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: integer
          minimum: 10
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/14/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Nil(t, errors)
}

func TestNewValidator_SimpleEncodedPath_MaximumIntegerViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: integer
          maximum: 10
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/11/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: maximum: got 11, want 10, Location: /maximum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_SimpleEncodedPath_MaximumInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: integer
          maximum: 10
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/4/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Nil(t, errors)
}

func TestNewValidator_SimpleEncodedPath_InvalidNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: number
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/hello/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' is not a valid number", errors[0].Message)
	assert.Equal(t, "The path parameter 'burgerId' is defined as being a number, however the value 'hello' is not a valid number", errors[0].Reason)
}

func TestNewValidator_SimpleEncodedPath_MinimumNumberViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: number
          minimum: 10.2
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/1.3/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: minimum: got 1.3, want 10.2, Location: /minimum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_SimpleEncodedPath_MinimumNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: number
          minimum: 10.3
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/14.5/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Nil(t, errors)
}

func TestNewValidator_SimpleEncodedPath_MaximumNumberViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: number
          maximum: 10.2
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/11.2/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: maximum: got 11.2, want 10.2, Location: /maximum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_SimpleEncodedPath_MaximumNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: number
          maximum: 10.2
    get:
      operationId: locateBurgers`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/4.5/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Nil(t, errors)
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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.hello/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' is not a valid integer", errors[0].Message)
}

func TestNewValidator_LabelEncodedPath_MinimumIntegerViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: integer
          minimum: 10
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: minimum: got 3, want 10, Location: /minimum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_LabelEncodedPath_MaximumIntegerViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: integer
          maximum: 10
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.32/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: maximum: got 32, want 10, Location: /maximum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_LabelEncodedPath_InvalidBoolean(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: boolean
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.hello/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' is not a valid boolean", errors[0].Message)
}

func TestNewValidator_LabelEncodedPath_ValidBoolean(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: boolean
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.true/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_LabelEncodedPath_ValidArray_Integer(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3,4/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_LabelEncodedPath_ValidArray_Integer_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        explode: true
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3.4/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_LabelEncodedPath_InvalidArray_Integer_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        explode: true
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3.Not an_integer.5.6/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path array parameter 'burgerId' is not a valid integer", errors[0].Message)
}

func TestNewValidator_LabelEncodedPath_InvalidArray_Integer(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3,4,Not an_integer,6/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path array parameter 'burgerId' is not a valid integer", errors[0].Message)
}

func TestNewValidator_LabelEncodedPath_ValidArray_Number(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: array
          items:
            type: number
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3.4,5.6/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_LabelEncodedPath_ValidArray_Number_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        explode: true
        schema:
          type: array
          items:
            type: number
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3.4.5.6/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_LabelEncodedPath_InvalidArray_Number_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        explode: true
        schema:
          type: array
          items:
            type: number
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3.Not a number.5.6/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path array parameter 'burgerId' is not a valid number", errors[0].Message)
}

func TestNewValidator_LabelEncodedPath_InvalidArray_Number(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: array
          items:
            type: number
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.3,4,Not a number,6/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path array parameter 'burgerId' is not a valid number", errors[0].Message)
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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.id=1234.vegetarian=true/locate/bigMac", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
}

func TestNewValidator_MatrixEncodedPath_ValidInteger(t *testing.T) {
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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=5/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_MatrixEncodedPath_InvalidInteger(t *testing.T) {
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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=I am not a number/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' is not a valid integer", errors[0].Message)
}

func TestNewValidator_MatrixEncodedPath_MinimumIntegerViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: integer
          minimum: 5
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=3/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: minimum: got 3, want 5, Location: /minimum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_MatrixEncodedPath_MaximumIntegerViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: integer
          maximum: 5
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=30/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: maximum: got 30, want 5, Location: /maximum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_MatrixEncodedPath_InvalidNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: number
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=I am not a number/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' is not a valid number", errors[0].Message)
}

func TestNewValidator_MatrixEncodedPath_MinimumNumberViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: number
          minimum: 5
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=3/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: minimum: got 3, want 5, Location: /minimum", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_MatrixEncodedPath_MaximumNumberViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: number
          maximum: 5
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=30/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: maximum: got 30, want 5, Location: /maximum", errors[0].SchemaValidationErrors[0].Error())
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

	v := NewParameterValidator(&m.Model)

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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=I am also not a bool/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' is not a valid boolean", errors[0].Message)
}

func TestNewValidator_MatrixEncodedPath_ValidObject(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burger=id,1234,vegetarian,false/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_MatrixEncodedPath_InvalidObject(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
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

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burger=id,1234,vegetarian,I am not a bool/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "got string, want boolean", errors[0].SchemaValidationErrors[0].Reason)
}

func TestNewValidator_MatrixEncodedPath_ValidObject_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger*}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
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
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;id=1234;vegetarian=false/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_MatrixEncodedPath_InvalidObject_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger*}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
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
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;id=1234;vegetarian=I am not a boolean/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "got string, want boolean", errors[0].SchemaValidationErrors[0].Reason)
}

func TestNewValidator_MatrixEncodedPath_ValidArray(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger*}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burger=1,2,3,4,5/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_MatrixEncodedPath_InvalidArray(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger*}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burger=1,2,not a number,4,false/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 2)
}

func TestNewValidator_MatrixEncodedPath_ValidArray_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger*}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
        explode: true
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burger=1;burger=2;burger=3/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_MatrixEncodedPath_InvalidArray_Exploded(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger*}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
        explode: true
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burger=1;burger=I am not an int;burger=3/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path array parameter 'burger' is not a valid integer", errors[0].Message)
}

func TestNewValidator_PathParams_PathNotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burger*}/locate:
    parameters:
      - name: burger
        in: path
        style: matrix
        explode: true
        schema:
          type: array
          items:
            type: integer
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/I do not exist", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
}

func TestNewValidator_PathParamStringEnumValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: string
          enum: [bigMac, whopper, mcCrispy]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/bigMac/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_PathParamStringEnumInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: string
          enum: [bigMac, whopper, mcCrispy]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/hello/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' does not match allowed values", errors[0].Message)
	assert.Equal(t, "Instead of 'hello', use one of the allowed values: 'bigMac, whopper, mcCrispy'", errors[0].HowToFix)
}

func TestNewValidator_PathParamStringMinLengthViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: string
          minLength: 4
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/big/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: minLength: got 3, want 4, Location: /minLength", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_PathParamStringMaxLengthViolation(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: string
          maxLength: 1
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/big/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' failed to validate", errors[0].Message)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "Reason: maxLength: got 3, want 1, Location: /maxLength", errors[0].SchemaValidationErrors[0].Error())
}

func TestNewValidator_PathParamIntegerEnumValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: integer
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/2/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_PathParamIntegerEnumInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: integer
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/3284/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' does not match allowed values", errors[0].Message)
}

func TestNewValidator_PathParamNumberEnumValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: number
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/2/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_PathParamNumberEnumInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        schema:
          type: number
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/3284/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' does not match allowed values", errors[0].Message)
}

func TestNewValidator_PathLabelEumValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: number
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.2/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_PathLabelEumInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: label
        schema:
          type: number
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/.22334/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' does not match allowed values", errors[0].Message)
	assert.Equal(t, "Instead of '22334', use one of the allowed values: '1, 2, 99, 100'", errors[0].HowToFix)
}

func TestNewValidator_PathMatrixEumInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: number
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=22334/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' does not match allowed values", errors[0].Message)
	assert.Equal(t, "Instead of '22334', use one of the allowed values: '1, 2, 99, 100'", errors[0].HowToFix)
}

func TestNewValidator_SetPathForPathParam(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: number
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/;burgerId=22334/locate", nil)

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model)

	valid, errors := v.ValidatePathParamsWithPathItem(request, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Path parameter 'burgerId' does not match allowed values", errors[0].Message)
	assert.Equal(t, "Instead of '22334', use one of the allowed values: '1, 2, 99, 100'", errors[0].HowToFix)
}

func TestNewValidator_SetPathForPathParam_notfound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{;burgerId}/locate:
    parameters:
      - name: burgerId
        in: path
        style: matrix
        schema:
          type: number
          enum: [1,2,99,100]
    get:
      operationId: locateBurgers`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/pizza/;burgerId=22334/locate", nil)

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model)

	valid, errors := v.ValidatePathParamsWithPathItem(request, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "GET Path '/pizza/;burgerId=22334/locate' not found", errors[0].Message)
}

func TestNewValidator_ServerPathPrefixInRequestPath(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: https://api.pb33f.io/lorem/ipsum
    description: Live production endpoint for general use.
paths:
  /burgers/{burger}/locate:
    parameters:
      - name: burger
        in: path
        schema:
          type: string
          format: uuid
    get:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/lorem/ipsum/burgers/d6d8d513-686c-466f-9f5a-1c051b6b4f3f/locate", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_MandatoryPathSegmentEmpty(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
- url: https://api.pb33f.io
  description: Live production endpoint for general use.
paths:
  /burgers/{burger}:
    get:
      parameters:
        - name: burger
          in: path
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/", nil)
	valid, errors := v.ValidatePathParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
}

func TestNewValidator_ODataFormattedOpenAPISpecs(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /entities('{Entity}'):
    parameters:
    - description: 'key: Entity'
      in: path
      name: Entity
      required: true
      schema:
        type: integer
    get:
      operationId: one
  /orders(RelationshipNumber='{RelationshipNumber}',ValidityEndDate=datetime'{ValidityEndDate}'):
    parameters:
    - name: RelationshipNumber
      in: path
      required: true
      schema:
        type: integer
    - name: ValidityEndDate
      in: path
      required: true
      schema:
        type: string
    get:
      operationId: one
`
	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/entities('1')", nil)

	valid, errors := v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

	request, _ = http.NewRequest(http.MethodGet, "https://things.com/orders(RelationshipNumber='1234',ValidityEndDate=datetime'1492041600000')", nil)

	valid, errors = v.ValidatePathParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

	request, _ = http.NewRequest(http.MethodGet, "https://things.com/orders(RelationshipNumber='dummy',ValidityEndDate=datetime'1492041600000')", nil)

	valid, errors = v.ValidatePathParams(request)
	assert.False(t, valid)
	assert.Len(t, errors, 1)
}

func TestNewValidator_ODataFormattedOpenAPISpecs_Error(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /entities('{Entity'):
    parameters:
    - in: path
      name: Entity
      required: true
      schema:
        type: integer
    get:
      operationId: one
`
	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/entities('1')", nil)

	valid, errors := v.ValidatePathParams(request)
	assert.False(t, valid)
	assert.NotEmpty(t, errors)
}

func TestNewValidator_ODataFormattedOpenAPISpecs_ErrorEmptyParameter(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /entities('{Entity}'):
    parameters:
    - in: path
      name: Entity
      required: true
      schema:
        type: integer
    get:
      operationId: one
`
	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/entities('')", nil)

	valid, errors := v.ValidatePathParams(request)
	assert.False(t, valid)
	assert.NotEmpty(t, errors)
}
