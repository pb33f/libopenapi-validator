// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"net/http"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
)

func TestNewValidator_CookieNoPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/I/do/not/exist", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "1"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "", errors[0].SpecPath)
}

func TestNewValidator_CookieParamNumberValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "1"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamNumberValidFloat(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "123.455"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamNumberInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "false"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Convert the value 'false' into a number", errors[0].HowToFix)
}

func TestNewValidator_CookieParamIntegerValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "1"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamIntegerInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "false"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Convert the value 'false' into an integer", errors[0].HowToFix)
}

func TestNewValidator_CookieParamBooleanValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "true"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamEnumValidString(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: string
            enum:
              - beef
              - chicken
              - pea protein`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "chicken"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamEnumInvalidString(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: string
            enum:
              - beef
              - chicken
              - pea protein`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "milk"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t,
		"Instead of 'milk', use one of the allowed values: 'beef, chicken, pea protein'", errors[0].HowToFix)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/burgers/beef", errors[0].SpecPath)
}

func TestNewValidator_CookieParamBooleanInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "12345"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Convert the value '12345' into a true/false value", errors[0].HowToFix)
}

func TestNewValidator_CookieParamObjectValid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          explode: false
          schema:
            type: object
            properties:
              pink:
                type: boolean
              number:
                type: number
            required: [pink, number]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "pink,true,number,2"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamObjectInvalid(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          explode: false
          schema:
            type: object
            properties:
              pink:
                type: boolean
              number:
                type: number
            required: [pink, number]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "pink,2,number,2"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "got number, want boolean", errors[0].SchemaValidationErrors[0].Reason)
}

func TestNewValidator_CookieParamArrayValidNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2,3,4"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamArrayInvalidNumber(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2,true,4,'hello'"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 2)
}

func TestNewValidator_CookieParamArrayValidInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2,3,4"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamArrayInvalidInteger(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2,true,4,'hello'"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 2)
}

func TestNewValidator_CookieParamArrayValidBoolean(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "true,false,true,false,true"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamArrayString(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "true,1,hey,ho"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamArrayInvalidBoolean(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "true,false,pb33f,false,99.99"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 2)
}

func TestNewValidator_CookieParamArrayInvalidBooleanZeroOne(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "true,false,0,false,1"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 2)
}

func TestNewValidator_CookieParamArrayValidIntegerEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: integer
            enum: [1, 2, 99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamArrayInvalidIntegerEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: integer
            enum: [1, 2, 99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2500"}) // too many dude.

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Instead of '2500', use one of the allowed values: '1, 2, 99'", errors[0].HowToFix)
}

func TestNewValidator_CookieParamArrayValidNumberEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number
            enum: [1.2, 2.3, 99.0]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2.3"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamArrayInvalidNumberEnum(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number
            enum: [1.2, 2.3, 99.1]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2500"}) // too many dude.

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Instead of '2500', use one of the allowed values: '1.2, 2.3, 99.1'", errors[0].HowToFix)
}

func TestNewValidator_PresetPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: integer
            enum: [1, 2, 99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2500"}) // too many dude.

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model, &sync.Map{})

	valid, errors := v.ValidateCookieParamsWithPathItem(request, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Instead of '2500', use one of the allowed values: '1, 2, 99'", errors[0].HowToFix)
}

func TestNewValidator_PresetPath_notfound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: integer
            enum: [1, 2, 99]`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/pizza/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2500"}) // too many dude.

	// preset the path
	path, _, pv := paths.FindPath(request, &m.Model, nil)

	valid, errors := v.ValidateCookieParamsWithPathItem(request, path, pv)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "GET Path '/pizza/beef' not found", errors[0].Message)
}

// Tests for required cookie validation (GitHub issue #183)

func TestNewValidator_CookieRequiredMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// No cookie added - this should fail validation

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'PattyPreference' is missing", errors[0].Message)
	assert.Equal(t, "The cookie parameter 'PattyPreference' is defined as being required, "+
		"however it's missing from the request", errors[0].Reason)
	assert.Equal(t, helpers.ParameterValidation, errors[0].ValidationType)
	assert.Equal(t, helpers.ParameterValidationCookie, errors[0].ValidationSubType)
}

func TestNewValidator_CookieOptionalMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: false
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// No cookie added - this should pass validation since it's optional

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieOptionalMissingNoRequiredField(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// No cookie added - this should pass validation since required defaults to false

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieMultipleRequiredOneMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number
        - name: BunType
          in: cookie
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// Only add one cookie
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "1.5"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'BunType' is missing", errors[0].Message)
}

func TestNewValidator_CookieMultipleRequiredBothMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number
        - name: BunType
          in: cookie
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// No cookies added

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 2)
}

func TestNewValidator_CookieMultipleRequiredAllPresent(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number
        - name: BunType
          in: cookie
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "1.5"})
	request.AddCookie(&http.Cookie{Name: "BunType", Value: "sesame"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieCaseSensitive(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// Add cookie with different case - should not match
	request.AddCookie(&http.Cookie{Name: "pattypreference", Value: "1.5"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'PattyPreference' is missing", errors[0].Message)
}

func TestNewValidator_CookieRequiredWithInvalidValue(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "not-a-number"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	// Should be a type error, not a missing error
	assert.Contains(t, errors[0].Message, "not a valid number")
}

func TestNewValidator_CookieMixedRequiredOptional(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number
        - name: ExtraCheese
          in: cookie
          required: false
          schema:
            type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// Only add the required cookie
	request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "2.5"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieRequiredIntegerMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyCount
          in: cookie
          required: true
          schema:
            type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'PattyCount' is missing", errors[0].Message)
}

func TestNewValidator_CookieRequiredBooleanMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: ExtraCheese
          in: cookie
          required: true
          schema:
            type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'ExtraCheese' is missing", errors[0].Message)
}

func TestNewValidator_CookieRequiredStringMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: CustomerName
          in: cookie
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'CustomerName' is missing", errors[0].Message)
}

func TestNewValidator_CookieRequiredArrayMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Toppings
          in: cookie
          required: true
          schema:
            type: array
            items:
              type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'Toppings' is missing", errors[0].Message)
}

func TestNewValidator_CookieRequiredObjectMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Preferences
          in: cookie
          required: true
          explode: false
          schema:
            type: object
            properties:
              pink:
                type: boolean
              number:
                type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'Preferences' is missing", errors[0].Message)
}

func TestNewValidator_CookieRequiredWithPathItem(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	// No cookie added

	// Use the WithPathItem variant
	path, _, pv := paths.FindPath(request, &m.Model, &sync.Map{})

	valid, errors := v.ValidateCookieParamsWithPathItem(request, path, pv)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'PattyPreference' is missing", errors[0].Message)
}

// Tests for string schema validation (GitHub issue #184)

func TestNewValidator_CookieParamStringValidPattern(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: SessionID
          in: cookie
          required: true
          schema:
            type: string
            pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "SessionID", Value: "550e8400-e29b-41d4-a716-446655440000"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamStringInvalidPattern(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: SessionID
          in: cookie
          required: true
          schema:
            type: string
            pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "SessionID", Value: "invalid_value"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "does not match pattern")
}

func TestNewValidator_CookieParamStringValidFormat(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: SessionID
          in: cookie
          required: true
          schema:
            type: string
            format: uuid`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model, config.WithFormatAssertions())

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "SessionID", Value: "550e8400-e29b-41d4-a716-446655440000"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamStringInvalidFormat(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: SessionID
          in: cookie
          required: true
          schema:
            type: string
            format: uuid`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model, config.WithFormatAssertions())

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "SessionID", Value: "not-a-valid-uuid"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "uuid")
}

func TestNewValidator_CookieParamStringValidMinLength(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Token
          in: cookie
          required: true
          schema:
            type: string
            minLength: 10`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "Token", Value: "abcdefghij"}) // exactly 10 chars

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamStringInvalidMinLength(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Token
          in: cookie
          required: true
          schema:
            type: string
            minLength: 10`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "Token", Value: "short"}) // only 5 chars

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "minLength")
}

func TestNewValidator_CookieParamStringValidMaxLength(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Token
          in: cookie
          required: true
          schema:
            type: string
            maxLength: 10`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "Token", Value: "short"}) // 5 chars

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamStringInvalidMaxLength(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Token
          in: cookie
          required: true
          schema:
            type: string
            maxLength: 10`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "Token", Value: "this-is-way-too-long"}) // 20 chars

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "maxLength")
}

func TestNewValidator_CookieParamStringValidPatternAndMinMaxLength(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Code
          in: cookie
          required: true
          schema:
            type: string
            pattern: '^[A-Z]+$'
            minLength: 3
            maxLength: 10`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "Code", Value: "ABCDEF"}) // 6 chars, all uppercase

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamStringInvalidPatternButValidLength(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: Code
          in: cookie
          required: true
          schema:
            type: string
            pattern: '^[A-Z]+$'
            minLength: 3
            maxLength: 10`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "Code", Value: "abcdef"}) // 6 chars, but lowercase - fails pattern

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "does not match pattern")
}

func TestNewValidator_CookieParamStringEmailFormat(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: UserEmail
          in: cookie
          required: true
          schema:
            type: string
            format: email`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model, config.WithFormatAssertions())

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "UserEmail", Value: "user@example.com"})

	valid, errors := v.ValidateCookieParams(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamStringInvalidEmailFormat(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: UserEmail
          in: cookie
          required: true
          schema:
            type: string
            format: email`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model, config.WithFormatAssertions())

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
	request.AddCookie(&http.Cookie{Name: "UserEmail", Value: "not-an-email"})

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "email")
}

func TestNewValidator_CookieParamMissingRequired(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: session_id
          in: cookie
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Create request WITHOUT the required cookie
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)

	valid, errors := v.ValidateCookieParams(request)

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "Cookie parameter 'session_id' is missing", errors[0].Message)
	assert.Contains(t, errors[0].Reason, "required")
}
