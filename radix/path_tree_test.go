// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package radix

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func TestNewPathTree(t *testing.T) {
	tree := NewPathTree()
	require.NotNil(t, tree)
	assert.Equal(t, 0, tree.Size())
}

func TestPathTree_ImplementsPathLookup(t *testing.T) {
	// Compile-time check that PathTree implements PathLookup
	var _ PathLookup = (*PathTree)(nil)
}

func TestPathTree_Insert_Lookup(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	pair := model.Model.Paths.PathItems.First()
	require.NotNil(t, pair)

	tree := NewPathTree()
	tree.Insert("/users", pair.Value())

	pathItem, path, found := tree.Lookup("/users")
	assert.True(t, found)
	assert.Equal(t, "/users", path)
	assert.NotNil(t, pathItem)
	assert.NotNil(t, pathItem.Get)
}

func TestPathTree_Walk(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
  /posts:
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildPathTree(&model.Model)
	assert.Equal(t, 2, tree.Size())

	var paths []string
	tree.Walk(func(path string, pathItem *v3.PathItem) bool {
		paths = append(paths, path)
		assert.NotNil(t, pathItem)
		return true
	})
	assert.Len(t, paths, 2)
}

func TestBuildPathTree(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
  /users/{id}:
    get:
      responses:
        '200':
          description: OK
  /posts:
    post:
      responses:
        '201':
          description: Created
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildPathTree(&model.Model)

	assert.Equal(t, 3, tree.Size())

	// Test lookups
	pathItem, path, found := tree.Lookup("/users")
	assert.True(t, found)
	assert.Equal(t, "/users", path)
	assert.NotNil(t, pathItem.Get)

	pathItem, path, found = tree.Lookup("/users/123")
	assert.True(t, found)
	assert.Equal(t, "/users/{id}", path)
	assert.NotNil(t, pathItem.Get)

	pathItem, path, found = tree.Lookup("/posts")
	assert.True(t, found)
	assert.Equal(t, "/posts", path)
	assert.NotNil(t, pathItem.Post)
}

func TestBuildPathTree_NilDocument(t *testing.T) {
	tree := BuildPathTree(nil)
	require.NotNil(t, tree)
	assert.Equal(t, 0, tree.Size())
}

func TestBuildPathTree_NilPaths(t *testing.T) {
	doc := &v3.Document{}
	tree := BuildPathTree(doc)
	require.NotNil(t, tree)
	assert.Equal(t, 0, tree.Size())
}

func TestPathTree_LiteralOverParam(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      operationId: getUserById
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
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildPathTree(&model.Model)

	// Literal should win
	pathItem, path, found := tree.Lookup("/users/admin")
	assert.True(t, found)
	assert.Equal(t, "/users/admin", path)
	assert.Equal(t, "getAdmin", pathItem.Get.OperationId)

	// Param should match other values
	pathItem, path, found = tree.Lookup("/users/123")
	assert.True(t, found)
	assert.Equal(t, "/users/{id}", path)
	assert.Equal(t, "getUserById", pathItem.Get.OperationId)
}

func TestPathTree_LookupWithParams(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
  /users/{id}:
    get:
      responses:
        '200':
          description: OK
  /users/{userId}/posts/{postId}:
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildPathTree(&model.Model)

	tests := []struct {
		name           string
		lookupPath     string
		expectedPath   string
		expectedParams map[string]string
		expectedFound  bool
	}{
		{
			name:           "Literal path - no params",
			lookupPath:     "/users",
			expectedPath:   "/users",
			expectedParams: nil,
			expectedFound:  true,
		},
		{
			name:           "Single param",
			lookupPath:     "/users/123",
			expectedPath:   "/users/{id}",
			expectedParams: map[string]string{"id": "123"},
			expectedFound:  true,
		},
		{
			name:           "Multiple params",
			lookupPath:     "/users/abc/posts/xyz",
			expectedPath:   "/users/{userId}/posts/{postId}",
			expectedParams: map[string]string{"id": "abc", "postId": "xyz"},
			expectedFound:  true,
		},
		{
			name:           "Single param matches when multiple paths exist",
			lookupPath:     "/users/123",
			expectedPath:   "/users/{id}",
			expectedParams: map[string]string{"id": "123"},
			expectedFound:  true,
		},
		{
			name:           "Not found",
			lookupPath:     "/posts/123",
			expectedPath:   "",
			expectedParams: nil,
			expectedFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathItem, path, params, found := tree.LookupWithParams(tt.lookupPath)

			assert.Equal(t, tt.expectedFound, found, "found mismatch")
			if tt.expectedFound {
				assert.NotNil(t, pathItem, "pathItem should not be nil")
				assert.Equal(t, tt.expectedPath, path, "path mismatch")
				if tt.expectedParams == nil {
					assert.Nil(t, params, "params should be nil")
				} else {
					assert.Equal(t, tt.expectedParams, params, "params mismatch")
				}
			} else {
				assert.Nil(t, pathItem, "pathItem should be nil")
				assert.Empty(t, path, "path should be empty")
				assert.Nil(t, params, "params should be nil")
			}
		})
	}
}

// Benchmark

func BenchmarkPathTree_Lookup(b *testing.B) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /api/v3/ad_accounts:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{id}:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{id}/campaigns:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{id}/campaigns/{campaign_id}:
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	if err != nil {
		b.Fatal(err)
	}

	model, modelErr := doc.BuildV3Model()
	if modelErr != nil {
		b.Fatal(modelErr)
	}

	tree := BuildPathTree(&model.Model)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tree.Lookup("/api/v3/ad_accounts/acc123/campaigns/camp456")
	}
}
