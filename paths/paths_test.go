// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "net/http"
    "os"
    "testing"
)

func TestNewValidator_BadParam(t *testing.T) {

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/doggy", nil)

    // load a doc
    b, _ := os.ReadFile("../test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()

    _, errs, _ := FindPath(request, &m.Model)

    assert.Equal(t, "Match for path '/pet/doggy', but the parameter 'doggy' is not a number",
        errs[0].Message)
    assert.Equal(t, "The parameter 'petId' is defined as a number, but the value 'doggy' is not a number",
        errs[0].Reason)
    assert.Equal(t, 306, errs[0].SpecLine)
}

func TestNewValidator_GoodParamFloat(t *testing.T) {

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/232.233", nil)

    b, _ := os.ReadFile("../test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)
    m, _ := doc.BuildV3Model()

    pathItem, _, _ := FindPath(request, &m.Model)
    assert.NotNil(t, pathItem)
}

func TestNewValidator_GoodParamInt(t *testing.T) {

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/12334", nil)

    b, _ := os.ReadFile("../test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()
    pathItem, _, _ := FindPath(request, &m.Model)
    assert.NotNil(t, pathItem)
}

func TestNewValidator_FindSimpleEncodedArrayPath(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId*}/locate:
    patch:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    request, _ := http.NewRequest(http.MethodPatch, "https://things.com/burgers/1,2,3,4,5/locate", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Patch.OperationId)
}

func TestNewValidator_FindSimpleEncodedObjectPath(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId*}/locate:
    patch:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    request, _ := http.NewRequest(http.MethodPatch, "https://things.com/burgers/bish=bosh,wish=wash/locate", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Patch.OperationId)
}

func TestNewValidator_FindLabelEncodedArrayPath(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/{.burgerId}/locate:
    patch:
      operationId: locateBurger
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()
    request, _ := http.NewRequest(http.MethodPatch, "https://things.com/burgers/.1.2.3.4.5/locate", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Patch.OperationId)
}

func TestNewValidator_FindPathPost(t *testing.T) {

    // load a doc
    b, _ := os.ReadFile("../test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()

    request, _ := http.NewRequest(http.MethodPost, "https://things.com/pet/12334", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
    assert.NotNil(t, pathItem)
}

func TestNewValidator_FindPathDelete(t *testing.T) {

    // load a doc
    b, _ := os.ReadFile("../test_specs/petstorev3.json")
    doc, _ := libopenapi.NewDocument(b)

    m, _ := doc.BuildV3Model()
    request, _ := http.NewRequest(http.MethodDelete, "https://things.com/pet/12334", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
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

    request, _ := http.NewRequest(http.MethodPatch, "https://things.com/burgers/12345", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
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
    request, _ := http.NewRequest(http.MethodOptions, "https://things.com/burgers/12345", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
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

    request, _ := http.NewRequest(http.MethodTrace, "https://things.com/burgers/12345", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
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

    request, _ := http.NewRequest(http.MethodPut, "https://things.com/burgers/12345", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
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

    request, _ := http.NewRequest(http.MethodHead, "https://things.com/burgers/12345", nil)

    pathItem, _, _ := FindPath(request, &m.Model)
    assert.NotNil(t, pathItem)
    assert.Equal(t, "locateBurger", pathItem.Head.OperationId)

}

func TestNewValidator_FindPathMissing(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /a/fishy/on/a/dishy:
    head:
      operationId: locateFishy
`

    doc, _ := libopenapi.NewDocument([]byte(spec))

    m, _ := doc.BuildV3Model()

    request, _ := http.NewRequest(http.MethodHead, "https://things.com/not/here", nil)

    pathItem, errs, _ := FindPath(request, &m.Model)
    assert.Nil(t, pathItem)
    assert.NotNil(t, errs)
    assert.Equal(t, "Path '/not/here' not found", errs[0].Message)

}
