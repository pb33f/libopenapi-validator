// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/schema_validation"
)

func TestOpenAPI32DocumentValidationYAMLAndRootValidator(t *testing.T) {
	specification, err := os.ReadFile("test_specs/valid_32.yaml")
	require.NoError(t, err)
	document, err := libopenapi.NewDocument(specification)
	require.NoError(t, err)
	valid, validationErrors := schema_validation.ValidateOpenAPIDocument(document)
	require.True(t, valid, validationErrors)
	require.Empty(t, validationErrors)

	validator, constructionErrors := NewValidator(document)
	require.Empty(t, constructionErrors)
	t.Cleanup(validator.Release)
	valid, validationErrors = validator.ValidateDocument()
	require.True(t, valid, validationErrors)
	require.Empty(t, validationErrors)
}

func TestOpenAPI32DocumentValidationJSON(t *testing.T) {
	document, err := libopenapi.NewDocument([]byte(`{
  "openapi": "3.2.0",
  "$self": "https://example.com/openapi.json",
  "info": {"title": "JSON 3.2", "version": "1.0.0"},
  "paths": {
    "/search": {
      "query": {"responses": {"200": {"description": "ok"}}}
    }
  }
}`))
	require.NoError(t, err)
	valid, validationErrors := schema_validation.ValidateOpenAPIDocument(document)
	require.True(t, valid, validationErrors)
	require.Empty(t, validationErrors)
}

func TestOpenAPI32InvalidDocumentDiagnostics(t *testing.T) {
	specification, err := os.ReadFile("test_specs/invalid_32.yaml")
	require.NoError(t, err)
	document, err := libopenapi.NewDocument(specification)
	require.NoError(t, err)
	valid, validationErrors := schema_validation.ValidateOpenAPIDocument(document)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Greater(t, validationErrors[0].SpecLine, 0)
	assert.GreaterOrEqual(t, validationErrors[0].SpecCol, 0)
	assert.NotEmpty(t, validationErrors[0].SchemaValidationErrors)
	assert.Contains(t, validationErrors[0].SchemaValidationErrors[0].FieldPath, "info")
	assert.NotEmpty(t, validationErrors[0].SchemaValidationErrors[0].ReferenceObject)
}
