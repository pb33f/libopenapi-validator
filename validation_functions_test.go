// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "net/http"
    "os"
    "testing"
)

func TestNewValidator_BadParam(t *testing.T) {

    // load a doc
    b, _ := os.ReadFile("test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/doggy", nil)

    pathItem := v.FindPath(request)
    assert.Nil(t, pathItem)
    assert.Equal(t, "Match for path '/pet/doggy', but the parameter '{petId}' is not a number",
        v.ValidationErrors()[0].Message)
    assert.Equal(t, "The parameter 'petId' is defined as a number, but the value 'doggy' is not a number",
        v.ValidationErrors()[0].Reason)
    assert.Equal(t, 306, v.ValidationErrors()[0].SpecLine)
}

func TestNewValidator_GoodParamFloat(t *testing.T) {

    // load a doc
    b, _ := os.ReadFile("test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/232.233", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
}

func TestNewValidator_GoodParamInt(t *testing.T) {

    // load a doc
    b, _ := os.ReadFile("test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/12334", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
}

func TestNewValidator_FindPathPost(t *testing.T) {

    // load a doc
    b, _ := os.ReadFile("test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodPost, "https://things.com/pet/12334", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
}

func TestNewValidator_FindPathDelete(t *testing.T) {

    // load a doc
    b, _ := os.ReadFile("test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodDelete, "https://things.com/pet/12334", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
}

func TestNewValidator_FindPathPatch(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}:
    patch:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodPatch, "https://things.com/burgers/12345", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Patch.OperationId)

}

func TestNewValidator_FindPathOptions(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}:
    options:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodOptions, "https://things.com/burgers/12345", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Options.OperationId)

}

func TestNewValidator_FindPathTrace(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}:
    trace:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodTrace, "https://things.com/burgers/12345", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Trace.OperationId)

}

func TestNewValidator_FindPathPut(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}:
    put:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodPut, "https://things.com/burgers/12345", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Put.OperationId)

}

func TestNewValidator_FindPathHead(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}:
    head:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodHead, "https://things.com/burgers/12345", nil)

    pathItem := v.FindPath(request)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Head.OperationId)

}
