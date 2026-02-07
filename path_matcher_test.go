// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/radix"
)

func createTestDocument(paths map[string]bool) *v3.Document {
	doc := &v3.Document{
		Paths: &v3.Paths{
			PathItems: orderedmap.New[string, *v3.PathItem](),
		},
	}
	for path := range paths {
		pathItem := &v3.PathItem{
			Get: &v3.Operation{},
		}
		doc.Paths.PathItems.Set(path, pathItem)
	}
	return doc
}

func TestMatcherChain_Empty(t *testing.T) {
	chain := matcherChain(nil)
	result := chain.Match("/users", createTestDocument(map[string]bool{"/users": true}))
	assert.Nil(t, result, "empty chain should return nil")
}

func TestMatcherChain_SingleMatcher(t *testing.T) {
	doc := createTestDocument(map[string]bool{"/users/{id}": true})
	tree := radix.BuildPathTree(doc)

	chain := matcherChain{
		&radixMatcher{pathLookup: tree},
	}

	result := chain.Match("/users/123", doc)
	require.NotNil(t, result, "should find match")
	assert.NotNil(t, result.pathItem, "pathItem should not be nil")
	assert.Equal(t, "/users/{id}", result.matchedPath)
	assert.Equal(t, map[string]string{"id": "123"}, result.pathParams)
}

func TestMatcherChain_FirstWins(t *testing.T) {
	doc := createTestDocument(map[string]bool{"/users/{id}": true})
	tree := radix.BuildPathTree(doc)

	radixM := &radixMatcher{pathLookup: tree}
	regexM := &regexMatcher{regexCache: &sync.Map{}}

	chain := matcherChain{radixM, regexM}

	result := chain.Match("/users/123", doc)
	require.NotNil(t, result, "should find match")
	assert.Equal(t, "/users/{id}", result.matchedPath)
}

func TestMatcherChain_Fallback(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      responses:
        '200':
          description: OK
  /matrix;id=123:
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := radix.BuildPathTree(&model.Model)

	radixM := &radixMatcher{pathLookup: tree}
	regexM := &regexMatcher{regexCache: &sync.Map{}}

	chain := matcherChain{radixM, regexM}

	result := chain.Match("/matrix;id=123", &model.Model)
	require.NotNil(t, result, "should find match via regex fallback")
	assert.Equal(t, "/matrix;id=123", result.matchedPath)
}

func TestRadixMatcher_NilPathLookup(t *testing.T) {
	matcher := &radixMatcher{pathLookup: nil}
	result := matcher.Match("/users/123", createTestDocument(map[string]bool{"/users/{id}": true}))
	assert.Nil(t, result, "nil PathLookup should return nil")
}

func TestRadixMatcher_WithMatch(t *testing.T) {
	doc := createTestDocument(map[string]bool{"/users/{id}": true})
	tree := radix.BuildPathTree(doc)

	matcher := &radixMatcher{pathLookup: tree}
	result := matcher.Match("/users/123", doc)

	require.NotNil(t, result, "should find match")
	assert.NotNil(t, result.pathItem, "pathItem should not be nil")
	assert.Equal(t, "/users/{id}", result.matchedPath)
	assert.Equal(t, map[string]string{"id": "123"}, result.pathParams)
}

func TestRadixMatcher_NoMatch(t *testing.T) {
	doc := createTestDocument(map[string]bool{"/users/{id}": true})
	tree := radix.BuildPathTree(doc)

	matcher := &radixMatcher{pathLookup: tree}
	result := matcher.Match("/posts/123", doc)

	assert.Nil(t, result, "should not find match")
}

func TestRegexMatcher_WithMatch(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	matcher := &regexMatcher{regexCache: &sync.Map{}}
	result := matcher.Match("/users/123", &model.Model)

	require.NotNil(t, result, "should find match")
	assert.NotNil(t, result.pathItem, "pathItem should not be nil")
	assert.Equal(t, "/users/{id}", result.matchedPath)
	assert.Nil(t, result.pathParams, "regex matcher should not extract params yet")
}

func TestRegexMatcher_NilDoc(t *testing.T) {
	matcher := &regexMatcher{regexCache: &sync.Map{}}
	result := matcher.Match("/users/123", nil)
	assert.Nil(t, result, "nil doc should return nil")
}

func TestRegexMatcher_NilPaths(t *testing.T) {
	doc := &v3.Document{}
	matcher := &regexMatcher{regexCache: &sync.Map{}}
	result := matcher.Match("/users/123", doc)
	assert.Nil(t, result, "nil paths should return nil")
}

func TestRegexMatcher_NoMatch(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	matcher := &regexMatcher{regexCache: &sync.Map{}}
	result := matcher.Match("/posts/123", &model.Model)
	assert.Nil(t, result, "should not find match")
}
