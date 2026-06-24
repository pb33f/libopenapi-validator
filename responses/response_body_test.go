// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi-validator/config"
)

func TestResponseBodyValidator_Release(t *testing.T) {
	schemaCache := cache.NewDefaultCache()
	schemaCache.Store(1, &cache.SchemaCacheEntry{RenderedInline: []byte("schema")})

	resourceCache := cache.NewDefaultSchemaResourceCache()
	resourceCache.Store("resource", &cache.SchemaResourceCacheEntry{RenderedInline: []byte("resource")})

	v := NewResponseBodyValidator(
		&v3.Document{},
		config.WithSchemaCache(schemaCache),
		config.WithSchemaResourceCache(resourceCache),
	)

	validator := v.(*responseBodyValidator)
	require.NotNil(t, validator.options)
	require.NotNil(t, validator.document)

	v.Release()

	assert.Nil(t, validator.options)
	assert.Nil(t, validator.document)

	_, schemaFound := schemaCache.Load(1)
	assert.False(t, schemaFound)

	_, resourceFound := resourceCache.Load("resource")
	assert.False(t, resourceFound)

	v.Release()

	var nilValidator *responseBodyValidator
	nilValidator.Release()
}
