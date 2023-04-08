// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "net/http"
    "testing"
)

func TestNewValidator_FindSimpleArrayEncodedPath(t *testing.T) {

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

func TestNewValidator_FindSimpleArrayEncodedPath_Invalid(t *testing.T) {

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
