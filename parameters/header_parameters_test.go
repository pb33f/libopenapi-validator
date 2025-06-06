// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"

	"github.com/pb33f/libopenapi-validator/paths"
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
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/bish/bosh", nil)

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "Header parameter 'bash' is missing", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/bish/bosh", errors[0].SpecPath)
}

func TestNewValidator_HeaderPathMissing(t *testing.T) {
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
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/I/do/not/exist", nil)

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "GET Path '/I/do/not/exist' not found", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "", errors[0].SpecPath)
}

func TestNewValidator_HeaderParamDefaultEncoding_InvalidParamTypeInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "two") // headers are case-insensitive

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "Header parameter 'coffeeCups' is not a valid integer", errors[0].Message)
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
	v := NewParameterValidator(&m.Model)

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
	v := NewParameterValidator(&m.Model)

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
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "I am not an object") // headers are case-insensitive

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "Header parameter 'coffeeCups' cannot be decoded", errors[0].Message)
}

func TestNewValidator_HeaderParamDefaultEncoding_InvalidParamTypeObjectInteger(t *testing.T) {
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
                type: integer
              sugar:
                type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "milk,true,sugar,true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "got boolean, want integer", errors[0].SchemaValidationErrors[0].Reason)
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
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "milk,true,sugar,true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "got boolean, want number", errors[0].SchemaValidationErrors[0].Reason)
}

func TestNewValidator_HeaderParamDefaultEncoding_ValidParamTypeObjectBoolean(t *testing.T) {
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
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "milk,123,sugar,true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamInvalidSimpleEncoding(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          explode: false
          schema:
            type: object
            properties:
              milk:
                type: number
              sugar:
                type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "milk,123,sugar,true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_ValidParamTypeObject(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          explode: true
          schema:
            type: object
            properties:
              milk:
                type: number
              sugar:
                type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "milk=123,sugar=true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_InvalidParamTypeObjectNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          explode: true
          schema:
            type: object
            properties:
              milk:
                type: number
              sugar:
                type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "milk=true,sugar=true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "got boolean, want number", errors[0].SchemaValidationErrors[0].Reason)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_InvalidParamTypeObjectInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          explode: true
          schema:
            type: object
            properties:
              milk:
                type: integer
              sugar:
                type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "milk=true,sugar=true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "got boolean, want integer", errors[0].SchemaValidationErrors[0].Reason)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_ValidParamTypeArrayString(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: array
            items:
              type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "1,2,3,4,5") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_ValidParamTypeArrayNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: array
            items:
              type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "1.22,2.33,3.44,4.55,5.66") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_ValidParamTypeArrayInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: array
            items:
              type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "1,2,3,4,5") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_ValidParamTypeArrayBool(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: array
            items:
              type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "true,false,true,false,true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_InvalidParamTypeArrayNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: array
            items:
              type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "true,false,true,false,true") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 5)
}

func TestNewValidator_HeaderParamNonDefaultEncoding_InvalidParamTypeArrayBool(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: array
            items:
              type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "1,false,2,true,5,false") // default encoding.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 3)
}

func TestNewValidator_HeaderParamStringValidEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: string
            enum: [glass, china, thermos]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "glass")

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamStringInvalidEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: string
            enum: [glass, china, thermos]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "microwave") // this is not a cup!

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Instead of 'microwave', "+
		"use one of the allowed values: 'glass, china, thermos'", errors[0].HowToFix)
}

func TestNewValidator_HeaderParamIntegerValidEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: integer
            enum: [1,2,99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "2")

	valid, errors := v.ValidateHeaderParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_HeaderParamNumberInvalidEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: number
            enum: [1.2,2.3,99.8]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "1200.3") // that's a lot of cups dude, we only have one dishwasher.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Instead of '1200.3', "+
		"use one of the allowed values: '1.2, 2.3, 99.8'", errors[0].HowToFix)
}

func TestNewValidator_HeaderParamIntegerInvalidEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: integer
            enum: [1,2,99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "1200") // that's a lot of cups dude, we only have one dishwasher.

	valid, errors := v.ValidateHeaderParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Instead of '1200', "+
		"use one of the allowed values: '1, 2, 99'", errors[0].HowToFix)
}

func TestNewValidator_HeaderParamSetPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: integer
            enum: [1,2,99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/vending/drinks", nil)
	request.Header.Set("coffeecups", "1200") // that's a lot of cups dude, we only have one dishwasher.

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model)

	valid, errors := v.ValidateHeaderParamsWithPathItem(request, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Instead of '1200', "+
		"use one of the allowed values: '1, 2, 99'", errors[0].HowToFix)
}

func TestNewValidator_HeaderParamSetPath_notfound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /vending/drinks:
    get:
      parameters:
        - name: coffeeCups
          in: header
          required: true
          schema:
            type: integer
            enum: [1,2,99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/buying/drinks", nil)
	request.Header.Set("coffeecups", "1200") // that's a lot of cups dude, we only have one dishwasher.

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model)

	valid, errors := v.ValidateHeaderParamsWithPathItem(request, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "GET Path '/buying/drinks' not found", errors[0].Message)
}
