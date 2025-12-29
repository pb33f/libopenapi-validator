// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
)

func TestNewValidator_BadParam(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/doggy", nil)

	// load a doc
	b, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(b)

	m, _ := doc.BuildV3Model()

	pathItem, _, _ := FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
}

func TestNewValidator_GoodParamFloat(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/232.233", nil)

	b, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(b)
	m, _ := doc.BuildV3Model()

	pathItem, _, _ := FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
}

func TestNewValidator_GoodParamInt(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/pet/12334", nil)

	b, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(b)

	m, _ := doc.BuildV3Model()
	pathItem, _, _ := FindPath(request, &m.Model, nil)
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

	pathItem, _, _ := FindPath(request, &m.Model, nil)
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

	pathItem, _, _ := FindPath(request, &m.Model, &sync.Map{})
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

	pathItem, _, _ := FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "locateBurger", pathItem.Patch.OperationId)
}

func TestNewValidator_FindPathPost(t *testing.T) {
	// load a doc
	b, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(b)

	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/pet/12334", nil)

	pathItem, _, _ := FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
}

func TestNewValidator_FindPathDelete(t *testing.T) {
	// load a doc
	b, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(b)

	m, _ := doc.BuildV3Model()
	request, _ := http.NewRequest(http.MethodDelete, "https://things.com/pet/12334", nil)

	pathItem, _, _ := FindPath(request, &m.Model, &sync.Map{})
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

	pathItem, _, _ := FindPath(request, &m.Model, &sync.Map{})
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

	pathItem, _, _ := FindPath(request, &m.Model, nil)
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

	pathItem, _, _ := FindPath(request, &m.Model, &sync.Map{})
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

	pathItem, _, _ := FindPath(request, &m.Model, &sync.Map{})
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

	pathItem, _, _ := FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "locateBurger", pathItem.Head.OperationId)
}

func TestNewValidator_FindPathWithBaseURLInServer(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: https://things.com/base1
  - url: https://things.com/base2
  - url: https://things.com/base3/base4/base5/base6/
paths:
  /user:
    post:
      operationId: addUser
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// check against base1
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/base1/user", nil)
	pathItem, _, _ := FindPath(request, &m.Model, &sync.Map{})
	assert.NotNil(t, pathItem)
	assert.Equal(t, "addUser", pathItem.Post.OperationId)

	// check against base2
	request, _ = http.NewRequest(http.MethodPost, "https://things.com/base2/user", nil)
	pathItem, _, _ = FindPath(request, &m.Model, &sync.Map{})
	assert.NotNil(t, pathItem)
	assert.Equal(t, "addUser", pathItem.Post.OperationId)

	// check against a deeper base
	request, _ = http.NewRequest(http.MethodPost, "https://things.com/base3/base4/base5/base6/user", nil)
	pathItem, _, _ = FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "addUser", pathItem.Post.OperationId)
}

func TestNewValidator_FindPathWithBaseURLInServer_Args(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: https://things.com/base3/base4/base5/base6/
paths:
  /user/{userId}/thing/{thingId}:
    post:
      operationId: addUser
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// check against a deeper base
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/base3/base4/base5/base6/user/1234/thing/abcd", nil)
	pathItem, _, _ := FindPath(request, &m.Model, &sync.Map{})
	assert.NotNil(t, pathItem)
	assert.Equal(t, "addUser", pathItem.Post.OperationId)
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

	pathItem, errs, _ := FindPath(request, &m.Model, nil)
	assert.Nil(t, pathItem)
	assert.NotNil(t, errs)
	assert.Equal(t, "HEAD Path '/not/here' not found", errs[0].Message)
	assert.True(t, errs[0].IsPathMissingError())
}

func TestNewValidator_FindOperationMissing(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /burgers/{burgerId}:
    trace:
      operationId: locateBurger
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPut, "https://things.com/burgers/12345", nil)

	pathItem, errs, _ := FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
	assert.NotNil(t, errs)
	assert.Equal(t, "PUT Path '/burgers/12345' not found", errs[0].Message)
	assert.True(t, errs[0].IsOperationMissingError())
}

func TestNewValidator_GetLiteralMatch(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/store/inventory", nil)

	// load a doc
	b, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(b)

	m, _ := doc.BuildV3Model()

	_, errs, _ := FindPath(request, &m.Model, nil)

	assert.Len(t, errs, 0)
}

func TestNewValidator_PostLiteralMatch(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/user", nil)

	// load a doc
	b, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(b)

	m, _ := doc.BuildV3Model()

	_, errs, _ := FindPath(request, &m.Model, nil)

	assert.Len(t, errs, 0)
}

func TestNewValidator_PutLiteralMatch(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/burger:
    put:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPut, "https://things.com/pizza/burger", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})

	assert.Len(t, errs, 0)
}

func TestNewValidator_PutMatch_Error(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/{cakes}:
    put:
      operationId: locateBurger
      parameters:
        - name: cakes
          in: path
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/pizza/1234", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})

	assert.Len(t, errs, 1)
	assert.Equal(t, "POST Path '/pizza/1234' not found", errs[0].Message)
}

func TestNewValidator_OptionsMatch_Error(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/{cakes}:
    options:
      operationId: locateBurger
      parameters:
        - name: cakes
          in: path
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/pizza/1234", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})

	assert.Len(t, errs, 1)
	assert.Equal(t, "POST Path '/pizza/1234' not found", errs[0].Message)
}

func TestNewValidator_PatchLiteralMatch(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/burger:
    patch:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPatch, "https://things.com/pizza/burger", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})

	assert.Len(t, errs, 0)
}

func TestNewValidator_PatchMatch_Error(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/{cakes}:
    patch:
      operationId: locateBurger
      parameters:
        - name: cakes
          in: path
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/pizza/1234", nil)

	_, errs, _ := FindPath(request, &m.Model, nil)

	assert.Len(t, errs, 1)
	assert.Equal(t, "POST Path '/pizza/1234' not found", errs[0].Message)
}

func TestNewValidator_DeleteLiteralMatch(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/burger:
    delete:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodDelete, "https://things.com/pizza/burger", nil)

	_, errs, _ := FindPath(request, &m.Model, nil)

	assert.Len(t, errs, 0)
}

func TestNewValidator_OptionsLiteralMatch(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/burger:
    options:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodOptions, "https://things.com/pizza/burger", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})

	assert.Len(t, errs, 0)
}

func TestNewValidator_HeadLiteralMatch(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/burger:
    head:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodHead, "https://things.com/pizza/burger", nil)

	_, errs, _ := FindPath(request, &m.Model, nil)

	assert.Len(t, errs, 0)
}

func TestNewValidator_TraceLiteralMatch(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/burger:
    trace:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodTrace, "https://things.com/pizza/burger", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})

	assert.Len(t, errs, 0)
}

func TestNewValidator_TraceMatch_Error(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/{cakes}:
    trace:
      operationId: locateBurger
      parameters:
        - name: cakes
          in: path
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/pizza/1234", nil)

	_, errs, _ := FindPath(request, &m.Model, nil)

	assert.Len(t, errs, 1)
	assert.Equal(t, "POST Path '/pizza/1234' not found", errs[0].Message)
}

func TestNewValidator_DeleteMatch_Error(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/{cakes}:
    delete:
      operationId: locateBurger
      parameters:
        - name: cakes
          in: path
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/pizza/1234", nil)

	_, errs, _ := FindPath(request, &m.Model, nil)

	assert.Len(t, errs, 1)
	assert.Equal(t, "POST Path '/pizza/1234' not found", errs[0].Message)
}

func TestNewValidator_PostMatch_Error(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/{cakes}:
    post:
      operationId: locateBurger
      parameters:
        - name: cakes
          in: path
          required: true
          schema:
            type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPut, "https://things.com/pizza/1234", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})

	assert.Len(t, errs, 1)
	assert.Equal(t, "PUT Path '/pizza/1234' not found", errs[0].Message)
}

func TestNewValidator_FindPathWithFragment(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /hashy#one:
    post:
      operationId: one
  /hashy#two:
    post:
      operationId: two
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/hashy#one", nil)

	pathItem, errs, _ := FindPath(request, &m.Model, &sync.Map{})
	assert.Len(t, errs, 0)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "one", pathItem.Post.OperationId)

	request, _ = http.NewRequest(http.MethodPost, "https://things.com/hashy#two", nil)
	pathItem, errs, _ = FindPath(request, &m.Model, &sync.Map{})
	assert.Len(t, errs, 0)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "two", pathItem.Post.OperationId)
}

func TestNewValidator_FindPathMissingWithBaseURLInServer(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: 'https://things.com/'
paths:
  /dishy:
    get:
      operationId: one
`

	doc, err := libopenapi.NewDocument([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/not_here", nil)

	_, errs, _ := FindPath(request, &m.Model, nil)
	assert.Len(t, errs, 1)
	assert.Equal(t, "GET Path '/not_here' not found", errs[0].Message)
}

func TestGetBasePaths(t *testing.T) {
	spec := `openapi: 3.1.0
servers:
  - url: 'https://things.com/'
  - url: 'https://things.com/some/path'
  - url: 'https://things.com/more//paths//please'
  - url: 'https://{invalid}.com/'
  - url: 'https://{invalid}.com/some/path'
  - url: 'https://{invalid}.com/more//paths//please'
  - url: 'https://{invalid}.com//even//more//paths//please'
paths:
  /dishy:
    get:
      operationId: one
`

	doc, err := libopenapi.NewDocument([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := doc.BuildV3Model()

	basePaths := getBasePaths(&m.Model)

	expectedPaths := []string{
		"/",
		"/some/path",
		"/more//paths//please",
		"/",
		"/some/path",
		"/more//paths//please",
		"/even//more//paths//please",
	}

	assert.Equal(t, expectedPaths, basePaths)
}

func TestNewValidator_FindPathWithEncodedArg(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /something/{string_contains_encoded}:
    put:
      operationId: putSomething
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodPut, "https://things.com/something/pkg%3Agithub%2Frs%2Fzerolog%40v1.18.0", nil)

	pathItem, errs, _ := FindPath(request, &m.Model, nil)

	assert.Equal(t, 0, len(errs), "Errors found: %v", errs)
	assert.NotNil(t, pathItem)
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

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/entities('1')", nil)

	pathItem, _, _ := FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "one", pathItem.Get.OperationId)

	request, _ = http.NewRequest(http.MethodGet, "https://things.com/orders(RelationshipNumber='1234',ValidityEndDate=datetime'1492041600000')", nil)

	pathItem, _, _ = FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "one", pathItem.Get.OperationId)

	request, _ = http.NewRequest(http.MethodGet, "https://things.com/orders(RelationshipNumber='dummy',ValidityEndDate=datetime'1492041600000')", nil)

	pathItem, _, _ = FindPath(request, &m.Model, nil)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "one", pathItem.Get.OperationId)
}

func TestNewValidator_ODataFormattedOpenAPISpecsWithRegexCache(t *testing.T) {
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

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/entities('1')", nil)

	regexCache := &sync.Map{}

	pathItem, _, _ := FindPath(request, &m.Model, regexCache)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "one", pathItem.Get.OperationId)

	request, _ = http.NewRequest(http.MethodGet, "https://things.com/orders(RelationshipNumber='1234',ValidityEndDate=datetime'1492041600000')", nil)

	pathItem, _, _ = FindPath(request, &m.Model, regexCache)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "one", pathItem.Get.OperationId)

	request, _ = http.NewRequest(http.MethodGet, "https://things.com/orders(RelationshipNumber='dummy',ValidityEndDate=datetime'1492041600000')", nil)

	pathItem, _, _ = FindPath(request, &m.Model, regexCache)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "one", pathItem.Get.OperationId)
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

	request, _ := http.NewRequest(http.MethodGet, "https://things.com/entities('1')", nil)

	_, errs, _ := FindPath(request, &m.Model, &sync.Map{})
	assert.NotEmpty(t, errs)
}

func TestNewValidator_FindPathWithRegexpCache(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /pizza/{sauce}/{fill}/hamburger/pizza:
    head:
      operationId: locateBurger`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodHead, "https://things.com/pizza/tomato/pepperoni/hamburger/pizza", nil)

	syncMap := sync.Map{}

	_, errs, _ := FindPath(request, &m.Model, &syncMap)

	keys := []string{}
	addresses := make(map[string]bool)

	syncMap.Range(func(key, value any) bool {
		keys = append(keys, key.(string))
		addresses[fmt.Sprintf("%p", value)] = true
		return true
	})

	cached, found := syncMap.Load("pizza")

	assert.True(t, found)
	assert.True(t, cached.(*regexp.Regexp).MatchString("pizza"))
	assert.Len(t, errs, 0)
	assert.Len(t, keys, 4)
	assert.Len(t, addresses, 3)
}

// Test cases for path precedence - Issue #181
// According to OpenAPI spec, literal paths take precedence over parameterized paths

func TestFindPath_LiteralTakesPrecedenceOverParameter(t *testing.T) {
	// This is the exact bug case from issue #181
	spec := `openapi: 3.1.0
info:
  title: Path Precedence Bug
  version: 1.0.0
paths:
  /Messages/{message_id}:
    parameters:
      - name: message_id
        in: path
        required: true
        schema:
          type: string
          pattern: '^comms_message_[0-7][a-hjkmnpqrstv-z0-9]{25,34}'
    get:
      operationId: getMessage
      responses:
        '200':
          description: OK
  /Messages/Operations:
    get:
      operationId: getOperations
      summary: List Operations
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// Request to literal path should match literal, not parameter
	request, _ := http.NewRequest(http.MethodGet, "https://api.com/Messages/Operations", nil)
	pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

	assert.Nil(t, errs, "Expected no errors")
	assert.NotNil(t, pathItem, "Expected pathItem to be found")
	assert.Equal(t, "getOperations", pathItem.Get.OperationId, "Should match literal path")
	assert.Equal(t, "/Messages/Operations", foundPath)
}

func TestFindPath_LiteralPrecedence_ReverseOrder(t *testing.T) {
	// Same test but with paths defined in opposite order
	// Result should be the same - literal always wins
	spec := `openapi: 3.1.0
info:
  title: Path Precedence Test
  version: 1.0.0
paths:
  /Messages/Operations:
    get:
      operationId: getOperations
      responses:
        '200':
          description: OK
  /Messages/{message_id}:
    get:
      operationId: getMessage
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodGet, "https://api.com/Messages/Operations", nil)
	pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

	assert.Nil(t, errs)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "getOperations", pathItem.Get.OperationId)
	assert.Equal(t, "/Messages/Operations", foundPath)
}

func TestFindPath_ParameterStillMatchesNonLiteral(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Path Precedence Test
  version: 1.0.0
paths:
  /Messages/{message_id}:
    get:
      operationId: getMessage
      responses:
        '200':
          description: OK
  /Messages/Operations:
    get:
      operationId: getOperations
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// Request to a non-literal value should match parameter path
	request, _ := http.NewRequest(http.MethodGet, "https://api.com/Messages/12345", nil)
	pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

	assert.Nil(t, errs)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "getMessage", pathItem.Get.OperationId)
	assert.Equal(t, "/Messages/{message_id}", foundPath)
}

func TestFindPath_MultipleParameterLevels(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Path Precedence Test
  version: 1.0.0
paths:
  /api/{version}/users/{id}:
    get:
      operationId: getUserVersioned
      responses:
        '200':
          description: OK
  /api/v1/users/{id}:
    get:
      operationId: getUserV1
      responses:
        '200':
          description: OK
  /api/v1/users/me:
    get:
      operationId: getCurrentUser
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	tests := []struct {
		url          string
		expectedOp   string
		expectedPath string
	}{
		// Most specific: all literals
		{"https://api.com/api/v1/users/me", "getCurrentUser", "/api/v1/users/me"},
		// More specific: 3 literals + 1 param
		{"https://api.com/api/v1/users/123", "getUserV1", "/api/v1/users/{id}"},
		// Least specific: 2 literals + 2 params
		{"https://api.com/api/v2/users/123", "getUserVersioned", "/api/{version}/users/{id}"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			request, _ := http.NewRequest(http.MethodGet, tt.url, nil)
			pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

			assert.Nil(t, errs)
			assert.NotNil(t, pathItem)
			assert.Equal(t, tt.expectedOp, pathItem.Get.OperationId)
			assert.Equal(t, tt.expectedPath, foundPath)
		})
	}
}

func TestFindPath_TieBreaker_DefinitionOrder(t *testing.T) {
	// When two paths have equal specificity (same number of literals/params),
	// the first defined path should win
	spec := `openapi: 3.1.0
info:
  title: Path Precedence Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      operationId: getPetById
      responses:
        '200':
          description: OK
  /pets/{petName}:
    get:
      operationId: getPetByName
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	request, _ := http.NewRequest(http.MethodGet, "https://api.com/pets/fluffy", nil)
	pathItem, _, foundPath := FindPath(request, &m.Model, nil)

	// First defined path wins when scores are equal
	assert.Equal(t, "getPetById", pathItem.Get.OperationId)
	assert.Equal(t, "/pets/{petId}", foundPath)
}

func TestFindPath_PetsMinePrecedence(t *testing.T) {
	// Classic example from OpenAPI spec: /pets/mine vs /pets/{petId}
	spec := `openapi: 3.1.0
info:
  title: Petstore
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      responses:
        '200':
          description: OK
  /pets/mine:
    get:
      operationId: getMyPets
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// /pets/mine should match literal path
	request, _ := http.NewRequest(http.MethodGet, "https://api.com/pets/mine", nil)
	pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

	assert.Nil(t, errs)
	assert.Equal(t, "getMyPets", pathItem.Get.OperationId)
	assert.Equal(t, "/pets/mine", foundPath)

	// /pets/123 should match parameter path
	request, _ = http.NewRequest(http.MethodGet, "https://api.com/pets/123", nil)
	pathItem, errs, foundPath = FindPath(request, &m.Model, nil)

	assert.Nil(t, errs)
	assert.Equal(t, "getPet", pathItem.Get.OperationId)
	assert.Equal(t, "/pets/{petId}", foundPath)
}

func TestFindPath_DeepNestedPrecedence(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Nested Paths
  version: 1.0.0
paths:
  /api/{version}/resources/{id}/actions/{action}:
    get:
      operationId: genericAction
      responses:
        '200':
          description: OK
  /api/v1/resources/{id}/actions/delete:
    get:
      operationId: deleteResource
      responses:
        '200':
          description: OK
  /api/v1/resources/special/actions/delete:
    get:
      operationId: deleteSpecial
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	tests := []struct {
		url          string
		expectedOp   string
		expectedPath string
	}{
		// All literals - most specific
		{"https://api.com/api/v1/resources/special/actions/delete", "deleteSpecial", "/api/v1/resources/special/actions/delete"},
		// 5 literals + 1 param
		{"https://api.com/api/v1/resources/123/actions/delete", "deleteResource", "/api/v1/resources/{id}/actions/delete"},
		// 3 literals + 3 params - least specific
		{"https://api.com/api/v2/resources/123/actions/update", "genericAction", "/api/{version}/resources/{id}/actions/{action}"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			request, _ := http.NewRequest(http.MethodGet, tt.url, nil)
			pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

			assert.Nil(t, errs)
			assert.NotNil(t, pathItem)
			assert.Equal(t, tt.expectedOp, pathItem.Get.OperationId)
			assert.Equal(t, tt.expectedPath, foundPath)
		})
	}
}

func TestFindPath_MethodMismatchUsesHighestScore(t *testing.T) {
	// When path matches but method doesn't exist, error should reference
	// the most specific matching path
	spec := `openapi: 3.1.0
info:
  title: Method Mismatch Test
  version: 1.0.0
paths:
  /users/{id}:
    get:
      operationId: getUser
      responses:
        '200':
          description: OK
  /users/admin:
    get:
      operationId: getAdmin
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// POST to /users/admin - literal path should be chosen for error
	request, _ := http.NewRequest(http.MethodPost, "https://api.com/users/admin", nil)
	pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

	assert.NotNil(t, errs)
	assert.Len(t, errs, 1)
	assert.Equal(t, "/users/admin", foundPath)
	assert.NotNil(t, pathItem)
	assert.True(t, errs[0].IsOperationMissingError())
}

func TestFindPath_WithQueryParams(t *testing.T) {
	// Ensure query params don't affect path matching precedence
	spec := `openapi: 3.1.0
info:
  title: Query Params Test
  version: 1.0.0
paths:
  /Messages/{message_id}:
    get:
      operationId: getMessage
      responses:
        '200':
          description: OK
  /Messages/Operations:
    get:
      operationId: getOperations
      parameters:
        - name: start_date
          in: query
          schema:
            type: string
        - name: end_date
          in: query
          schema:
            type: string
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// This is the exact request from issue #181
	request, _ := http.NewRequest(http.MethodGet,
		"https://api.com/Messages/Operations?start_date=2020-01-01T00:00:00Z&end_date=2025-12-31T23:59:59Z&page_size=10", nil)
	pathItem, errs, foundPath := FindPath(request, &m.Model, nil)

	assert.Nil(t, errs)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "getOperations", pathItem.Get.OperationId)
	assert.Equal(t, "/Messages/Operations", foundPath)
}

func TestFindPath_WithRegexCache(t *testing.T) {
	// Ensure precedence works correctly with regex cache
	spec := `openapi: 3.1.0
info:
  title: Cache Test
  version: 1.0.0
paths:
  /Messages/{message_id}:
    get:
      operationId: getMessage
      responses:
        '200':
          description: OK
  /Messages/Operations:
    get:
      operationId: getOperations
      responses:
        '200':
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	regexCache := &sync.Map{}

	// First request - populates cache
	request, _ := http.NewRequest(http.MethodGet, "https://api.com/Messages/Operations", nil)
	pathItem, errs, foundPath := FindPath(request, &m.Model, regexCache)

	assert.Nil(t, errs)
	assert.Equal(t, "getOperations", pathItem.Get.OperationId)
	assert.Equal(t, "/Messages/Operations", foundPath)

	// Second request - uses cache
	request, _ = http.NewRequest(http.MethodGet, "https://api.com/Messages/12345", nil)
	pathItem, errs, foundPath = FindPath(request, &m.Model, regexCache)

	assert.Nil(t, errs)
	assert.Equal(t, "getMessage", pathItem.Get.OperationId)
	assert.Equal(t, "/Messages/{message_id}", foundPath)

	// Third request - still works correctly
	request, _ = http.NewRequest(http.MethodGet, "https://api.com/Messages/Operations", nil)
	pathItem, errs, foundPath = FindPath(request, &m.Model, regexCache)

	assert.Nil(t, errs)
	assert.Equal(t, "getOperations", pathItem.Get.OperationId)
	assert.Equal(t, "/Messages/Operations", foundPath)
}
