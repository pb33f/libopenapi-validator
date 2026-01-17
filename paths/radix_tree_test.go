// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPathRadixTree(t *testing.T) {
	tree := NewPathRadixTree()
	require.NotNil(t, tree)
	assert.Equal(t, 0, tree.Size())
}

func TestPathRadixTree_Insert_Lookup(t *testing.T) {
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

	// Get the PathItem from the model
	pair := model.Model.Paths.PathItems.First()
	require.NotNil(t, pair)

	tree := NewPathRadixTree()
	tree.Insert("/users", pair.Value())

	pathItem, path, found := tree.Lookup("/users")
	assert.True(t, found)
	assert.Equal(t, "/users", path)
	assert.NotNil(t, pathItem)
	assert.NotNil(t, pathItem.Get)
}

func TestPathRadixTree_LiteralOverParam(t *testing.T) {
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

	tree := BuildRadixTree(&model.Model)
	require.Equal(t, 2, tree.Size())

	// Literal match should win
	pathItem, path, found := tree.Lookup("/users/admin")
	assert.True(t, found)
	assert.Equal(t, "/users/admin", path)
	assert.NotNil(t, pathItem.Get)
	assert.Equal(t, "getAdmin", pathItem.Get.OperationId)

	// Parameterized should still work
	pathItem, path, found = tree.Lookup("/users/123")
	assert.True(t, found)
	assert.Equal(t, "/users/{id}", path)
	assert.NotNil(t, pathItem.Get)
	assert.Equal(t, "getUserById", pathItem.Get.OperationId)
}

func TestPathRadixTree_MultipleParams(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /orgs/{orgId}/teams/{teamId}/members/{memberId}:
    get:
      operationId: getOrgTeamMember
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildRadixTree(&model.Model)

	pathItem, path, found := tree.Lookup("/orgs/org1/teams/team2/members/member3")
	assert.True(t, found)
	assert.Equal(t, "/orgs/{orgId}/teams/{teamId}/members/{memberId}", path)
	assert.NotNil(t, pathItem.Get)
}

func TestPathRadixTree_NoMatch(t *testing.T) {
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

	tree := BuildRadixTree(&model.Model)

	_, _, found := tree.Lookup("/posts")
	assert.False(t, found)

	_, _, found = tree.Lookup("/users/123/extra")
	assert.False(t, found)
}

func TestPathRadixTree_Walk(t *testing.T) {
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
    get:
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildRadixTree(&model.Model)
	assert.Equal(t, 3, tree.Size())

	// Verify all paths are reachable
	_, _, found := tree.Lookup("/users")
	assert.True(t, found)
	_, _, found = tree.Lookup("/users/123")
	assert.True(t, found)
	_, _, found = tree.Lookup("/posts")
	assert.True(t, found)

	// Test Walk function
	var paths []string
	tree.Walk(func(path string, pathItem *v3.PathItem) bool {
		paths = append(paths, path)
		assert.NotNil(t, pathItem)
		return true
	})
	assert.Len(t, paths, 3)
}

func TestBuildRadixTree_NilDocument(t *testing.T) {
	tree := BuildRadixTree(nil)
	require.NotNil(t, tree)
	assert.Equal(t, 0, tree.Size())
}

func TestFindPathWithRadix_Success(t *testing.T) {
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
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildRadixTree(&model.Model)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)

	pathItem, validationErrors, matchedPath := FindPathWithRadix(req, &model.Model, tree)

	assert.Empty(t, validationErrors)
	assert.Equal(t, "/users/{id}", matchedPath)
	assert.NotNil(t, pathItem)
	assert.Equal(t, "getUserById", pathItem.Get.OperationId)
}

func TestFindPathWithRadix_NotFound(t *testing.T) {
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

	tree := BuildRadixTree(&model.Model)

	req := httptest.NewRequest(http.MethodGet, "/posts", nil)

	pathItem, validationErrors, matchedPath := FindPathWithRadix(req, &model.Model, tree)

	assert.Nil(t, pathItem)
	assert.NotEmpty(t, validationErrors)
	assert.Equal(t, "missing", validationErrors[0].ValidationSubType)
	assert.Empty(t, matchedPath)
}

func TestFindPathWithRadix_MethodNotFound(t *testing.T) {
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

	tree := BuildRadixTree(&model.Model)

	req := httptest.NewRequest(http.MethodPost, "/users", nil)

	pathItem, validationErrors, matchedPath := FindPathWithRadix(req, &model.Model, tree)

	assert.NotNil(t, pathItem) // Path exists but method doesn't
	assert.NotEmpty(t, validationErrors)
	assert.Equal(t, "missingOperation", validationErrors[0].ValidationSubType)
	assert.Equal(t, "/users", matchedPath)
}

func TestFindPathWithRadix_NilTree(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	// Should fall back to FindPath when tree is nil
	pathItem, validationErrors, matchedPath := FindPathWithRadix(req, &model.Model, nil)

	assert.Empty(t, validationErrors)
	assert.Equal(t, "/users", matchedPath)
	assert.NotNil(t, pathItem)
}

func TestFindPathWithRadix_WithBasePath(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://api.example.com/v1
paths:
  /users/{id}:
    get:
      operationId: getUserById
      responses:
        '200':
          description: OK
`
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	tree := BuildRadixTree(&model.Model)

	// Request with base path stripped
	req := httptest.NewRequest(http.MethodGet, "/v1/users/123", nil)

	pathItem, validationErrors, matchedPath := FindPathWithRadix(req, &model.Model, tree)

	assert.Empty(t, validationErrors)
	assert.Equal(t, "/users/{id}", matchedPath)
	assert.NotNil(t, pathItem)
}

// Benchmark to ensure radix tree performance

func BenchmarkFindPathWithRadix(b *testing.B) {
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
  /api/v3/ad_accounts/{ad_account_id}:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/ads:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/ads/{ad_id}:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/campaigns:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/campaigns/{campaign_id}:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/bulk_actions:
    post:
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

	tree := BuildRadixTree(&model.Model)
	req := httptest.NewRequest(http.MethodGet, "/api/v3/ad_accounts/acc123/campaigns/camp456", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindPathWithRadix(req, &model.Model, tree)
	}
}

func BenchmarkFindPath_Linear(b *testing.B) {
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
  /api/v3/ad_accounts/{ad_account_id}:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/ads:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/ads/{ad_id}:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/campaigns:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/campaigns/{campaign_id}:
    get:
      responses:
        '200':
          description: OK
  /api/v3/ad_accounts/{ad_account_id}/bulk_actions:
    post:
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

	req := httptest.NewRequest(http.MethodGet, "/api/v3/ad_accounts/acc123/campaigns/camp456", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindPath(req, &model.Model, nil)
	}
}
