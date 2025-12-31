// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package strict

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSchemaLocation_NilSchema(t *testing.T) {
	line, col := extractSchemaLocation(nil)
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, col)
}

func TestExtractSchemaLocation_WithValidSchema(t *testing.T) {
	spec := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	schema := model.Model.Paths.PathItems.GetOrZero("/test").Get.Responses.Codes.GetOrZero("200").Content.GetOrZero("application/json").Schema.Schema()
	require.NotNil(t, schema)

	line, col := extractSchemaLocation(schema)
	// The schema starts at line 14 (type: object)
	assert.Greater(t, line, 0, "line should be greater than 0")
	assert.Greater(t, col, 0, "col should be greater than 0")
}

func TestExtractSchemaLocation_InlineSchema(t *testing.T) {
	// Test with a schema that has GoLow() returning non-nil but RootNode nil
	// This is hard to construct directly, but we can test the nil case
	line, col := extractSchemaLocation(nil)
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, col)
}

func TestNewUndeclaredProperty_WithLocation(t *testing.T) {
	spec := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	schema := model.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema.Schema()
	require.NotNil(t, schema)

	undeclared := newUndeclaredProperty(
		"$.body.extra",
		"extra",
		"value",
		[]string{"name"},
		DirectionRequest,
		schema,
	)

	assert.Equal(t, "$.body.extra", undeclared.Path)
	assert.Equal(t, "extra", undeclared.Name)
	assert.Equal(t, "property", undeclared.Type)
	assert.Greater(t, undeclared.SpecLine, 0, "SpecLine should be set")
	assert.Greater(t, undeclared.SpecCol, 0, "SpecCol should be set")
}

func TestNewUndeclaredProperty_WithNilSchema(t *testing.T) {
	undeclared := newUndeclaredProperty(
		"$.body.extra",
		"extra",
		"value",
		[]string{"name"},
		DirectionRequest,
		nil, // nil schema
	)

	assert.Equal(t, "$.body.extra", undeclared.Path)
	assert.Equal(t, "extra", undeclared.Name)
	assert.Equal(t, "property", undeclared.Type)
	assert.Equal(t, 0, undeclared.SpecLine, "SpecLine should be 0 for nil schema")
	assert.Equal(t, 0, undeclared.SpecCol, "SpecCol should be 0 for nil schema")
}

