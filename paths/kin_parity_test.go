// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/router"
)

func TestFindPathRouterCompatibilityAdapter(t *testing.T) {
	document, err := libopenapi.NewDocument([]byte(`openapi: 3.1.0
info: {title: adapter, version: 1.0.0}
paths:
  /items/{id}:
    get: {responses: {"204": {description: ok}}}`))
	require.NoError(t, err)
	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	routeFinder := router.NewRouter(&model.Model, router.WithPathOnlyMatching())
	t.Cleanup(routeFinder.Release)
	options := config.NewValidationOptions()
	options.Router = routeFinder

	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items/1", nil)
	legacyRoute, legacyErrors := ResolveRoute(request, &model.Model, nil)
	require.NotNil(t, legacyRoute)
	assert.Empty(t, legacyErrors)
	assert.Equal(t, "/items/{id}", legacyRoute.Path)
	assert.NotNil(t, legacyRoute.Operation)

	pathItem, validationErrors, path := FindPath(request, &model.Model, options)
	assert.NotNil(t, pathItem)
	assert.Empty(t, validationErrors)
	assert.Equal(t, "/items/{id}", path)

	request, _ = http.NewRequest(http.MethodPost, "http://example.com/items/1", nil)
	pathItem, validationErrors, path = FindPath(request, &model.Model, options)
	assert.NotNil(t, pathItem)
	require.NotEmpty(t, validationErrors)
	assert.True(t, validationErrors[0].IsOperationMissingError())
	assert.Equal(t, "/items/{id}", path)

	request, _ = http.NewRequest(http.MethodGet, "http://example.com/missing", nil)
	pathItem, validationErrors, path = FindPath(request, &model.Model, options)
	assert.Nil(t, pathItem)
	require.NotEmpty(t, validationErrors)
	assert.True(t, validationErrors[0].IsPathMissingError())
	assert.Empty(t, path)
	legacyRoute, legacyErrors = ResolveRoute(request, &model.Model, nil)
	assert.Nil(t, legacyRoute)
	require.NotEmpty(t, legacyErrors)

	nilOptions := config.NewValidationOptions()
	nilOptions.Router = nilResultRouter{}
	nilRoute, nilErrors := ResolveRoute(request, &model.Model, nilOptions)
	assert.Nil(t, nilRoute)
	require.NotEmpty(t, nilErrors)
}

type nilResultRouter struct{}

func (nilResultRouter) FindRoute(*http.Request) (*router.Route, error) { return nil, nil }
func (nilResultRouter) Release()                                       {}
