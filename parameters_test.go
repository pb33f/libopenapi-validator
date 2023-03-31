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
