// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestNewValidator_QueryParamMissing(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "Query parameter 'fishy' is missing", errors[0].Message)
}

func TestNewValidator_QueryParamNotMissing(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamPost(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    post:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamPut(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    put:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPut, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamDelete(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    delete:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodDelete, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamOptions(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    options:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodOptions, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamHead(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    head:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodHead, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamPatch(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    patch:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPatch, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamTrace(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    trace:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodTrace, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamBadPath(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: number
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/Not/Found/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.Nil(t, pathItem)
	assert.NotNil(t, errors)
}

func TestNewValidator_QueryParamWrongTypeNumber(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: number
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.NotNil(t, errors)
	assert.Equal(t, "Query parameter 'fishy' is not a valid number", errors[0].Message)
}

func TestNewValidator_QueryParamValidTypeNumber(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: number
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=123", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamWrongTypeBool(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: boolean
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.NotNil(t, errors)
	assert.Equal(t, "Query parameter 'fishy' is not a valid boolean", errors[0].Message)
}

func TestNewValidator_QueryParamValidTypeBool(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: boolean
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=true", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamValidTypeArrayString(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: array
            items:
              type: string
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod&fishy=haddock", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Nil(t, errors)
}

func TestNewValidator_QueryParamInvalidTypeArrayNumber(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
 /a/fishy/on/a/dishy:
   get:
     parameters:
       - name: fishy
         in: query
         required: true
         schema:
           type: array
           items:
             type: number
     operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod&fishy=haddock", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 2)
	assert.Equal(t, "Query array parameter 'fishy' is not a valid number", errors[0].Message)
	assert.Equal(t, "The query parameter (which is an array) 'fishy' is defined as being a number, "+
		"however the value 'cod' is not a valid number", errors[0].Reason)
	assert.Equal(t, "Query array parameter 'fishy' is not a valid number", errors[1].Message)
	assert.Equal(t, "The query parameter (which is an array) 'fishy' is defined as being a number, "+
		"however the value 'haddock' is not a valid number", errors[1].Reason)
}

func TestNewValidator_QueryParamValidExplodedType(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
 /a/fishy/on/a/dishy:
   get:
     parameters:
       - name: fishy
         in: query
         required: true
         explode: true
         schema:
           type: array
           items:
             type: string
     operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod,haddock", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 0)
}

func TestNewValidator_QueryParamInvalidExplodedArray(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
 /a/fishy/on/a/dishy:
   get:
     parameters:
       - name: fishy
         in: query
         required: true
         explode: true
         schema:
           type: array
           items:
             type: number
     operationId: locateFishy`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=1&fishy=2", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 2)
	assert.Equal(t, "Query array parameter 'fishy' has not been exploded correctly", errors[0].Message)
	assert.Equal(t, "The query parameter (which is an array) 'fishy' is defined as being exploded, "+
		"however the value '1' is not comma delimited. There are multiple parameters with the same name in the request (2)", errors[0].Reason)
	assert.Equal(t, "The query parameter (which is an array) 'fishy' is defined as being exploded, "+
		"however the value '2' is not comma delimited. There are multiple parameters with the same name in the request (2)", errors[1].Reason)
}

func TestNewValidator_QueryParamInvalidExplodedArrayAndInvalidType(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
 /a/fishy/on/a/dishy:
   get:
     parameters:
       - name: fishy
         in: query
         required: true
         explode: true
         schema:
           type: array
           items:
             type: number
     operationId: locateFishy`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=haddock&fishy=cod", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 4)
}

func TestNewValidator_QueryParamValidExploded(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
 /a/fishy/on/a/dishy:
   get:
     parameters:
       - name: fishy
         in: query
         required: true
         explode: true
         schema:
           type: array
           items:
             type: string
     operationId: locateFishy`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod,haddock,mackrel", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 0)
}

func TestNewValidator_QueryParamInvalidTypeArrayBool(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
 /a/fishy/on/a/dishy:
   get:
     parameters:
       - name: fishy
         in: query
         required: true
         schema:
           type: array
           items:
             type: boolean 
operationId: locateFishy`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy=cod&fishy=haddock", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 2)
	assert.Equal(t, "Query array parameter 'fishy' is not a valid boolean", errors[0].Message)
	assert.Equal(t, "The query parameter (which is an array) 'fishy' is defined as being a boolean, "+
		"however the value 'cod' is not a valid true/false value", errors[0].Reason)
	assert.Equal(t, "Query array parameter 'fishy' is not a valid boolean", errors[1].Message)
	assert.Equal(t, "The query parameter (which is an array) 'fishy' is defined as being a boolean, "+
		"however the value 'haddock' is not a valid true/false value", errors[1].Reason)
}

func TestNewValidator_QueryParamValidTypeObject(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: array
            items:
              type: object
              properties:
                vinegar:
                  type: boolean
                chips:
                  type: number
              required:
                - vinegar
                - chips
      operationId: locateFishy`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy={\"cod\":\"cakes\"}&fishy={\"crab\":\"legs\"}", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 2)
	assert.Equal(t, "Query array parameter 'fishy' failed to validate", errors[0].Message)
	assert.Equal(t, "The query parameter (which is an array) 'fishy' is defined as an object, "+
		"however it failed to pass a schema validation", errors[0].Reason)
	assert.Equal(t, "missing properties: 'vinegar', 'chips'", errors[0].ValidationError.Reason)
	assert.Equal(t, "/required", errors[0].ValidationError.Location)
}

func TestNewValidator_QueryParamValidTypeObjectPropType(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            type: array
            items:
              type: object
              properties:
                vinegar:
                  type: boolean
                chips:
                  type: number
              required:
                - vinegar
                - chips
      operationId: locateFishy`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy={\"vinegar\":\"cakes\",\"chips\":\"hello\"}&fishy={\"vinegar\":true,\"chips\":123}", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 2)

}

func TestNewValidator_QueryParamInvalidTypeObjectArrayPropType_Ref(t *testing.T) {

	spec := `openapi: 3.1.0
components:
  parameters:
    something:
      name: somethingElse
      in: query
      schema:
        type: array
        items:
          type: object
          properties:
            vinegar:
              type: boolean
            chips:
              type: number
          required:
            - vinegar
            - chips
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            $ref: "#/components/parameters/something/schema"
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy={\"vinegar\":\"cakes\",\"chips\":\"hello\"}&fishy={\"vinegar\":true,\"chips\":123}", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 2)

}

func TestNewValidator_QueryParamValidTypeObjectArrayPropType_Ref(t *testing.T) {

	spec := `openapi: 3.1.0
components:
  parameters:
    something:
      name: somethingElse
      in: query
      schema:
        type: array
        items:
          type: object
          properties:
            vinegar:
              type: boolean
            chips:
              type: number
          required:
            - vinegar
            - chips
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - name: fishy
          in: query
          required: true
          schema:
            $ref: "#/components/parameters/something/schema"
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy={\"vinegar\":false,\"chips\":999}&fishy={\"vinegar\":true,\"chips\":123}", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 0)

}

func TestNewValidator_QueryParamValidTypeObjectPropType_Ref(t *testing.T) {

	spec := `openapi: 3.1.0
components:
  parameters:
    fishy:
      name: fishy
      in: query
      schema:
        type: object
        properties:
          vinegar:
            type: boolean
          chips:
            type: number
        required:
          - vinegar
          - chips
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - $ref: "#/components/parameters/fishy"
      operationId: locateFishy
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy={\"vinegar\":false,\"chips\":999}", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.True(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 0)

}

func TestNewValidator_QueryParamInvalidTypeObjectPropType_Ref(t *testing.T) {

	spec := `openapi: 3.1.0
components:
  schemas:
    chippy:
      type: object
      properties:
        vinegar:
          type: boolean
        chips:
          type: number
      required:
        - vinegar
        - chips
  parameters:
    fishy:
      name: fishy
      in: query
      schema:
        $ref: "#/components/schemas/chippy"
paths:
  /a/fishy/on/a/dishy:
    get:
      parameters:
        - $ref: "#/components/parameters/fishy"
      operationId: locateFishy`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/a/fishy/on/a/dishy?fishy={\"vinegar\":1234,\"chips\":false}", nil)
	pathItem, _ := v.FindPath(request)
	valid, errors := v.ValidateQueryParams(request)
	assert.False(t, valid)
	assert.NotNil(t, pathItem)
	assert.Len(t, errors, 2)

}
