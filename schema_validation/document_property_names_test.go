// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"fmt"
	"testing"

	"github.com/pb33f/jsonschema/v6"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

func TestDocumentPropertyNamesKeepEachLocation(t *testing.T) {
	for _, version := range []string{"3.1.0", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			doc, err := libopenapi.NewDocument([]byte(fmt.Sprintf(`openapi: %s
info:
  title: Property name locations
  version: '1'
paths: {}
components:
  schemas:
    $bad:
      type: object
    "$a'~/b":
      type: string
  responses:
    $bad:
      description: Invalid component name
  x-example:
    $bad: This extension key is valid
`, version)))
			require.NoError(t, err)
			valid, failures := ValidateOpenAPIDocument(doc)
			require.False(t, valid)
			require.Len(t, failures, 1)
			require.Len(t, failures[0].SchemaValidationErrors, 3)
			byLine := make(map[int]*liberrors.SchemaValidationFailure)
			for _, failure := range failures[0].SchemaValidationErrors {
				byLine[failure.Line] = failure
			}
			for _, want := range []struct {
				line    int
				name    string
				section string
				path    string
			}{
				{8, "$bad", "schemas", "$.components.schemas['$bad']"},
				{10, "$a'~/b", "schemas", "$.components.schemas['$a\\'~/b']"},
				{13, "$bad", "responses", "$.components.responses['$bad']"},
			} {
				failure := byLine[want.line]
				require.NotNil(t, failure, "missing error at line %d", want.line)
				assert.Equal(t, 5, failure.Column)
				assert.Equal(t, want.name, failure.FieldName)
				assert.Equal(t, want.path, failure.FieldPath)
				assert.Equal(t, []string{"components", want.section, want.name}, failure.InstancePath)
				assert.Contains(t, failure.Reason, want.name)
				assert.Contains(t, failure.Reason, "does not match pattern")
				assert.Equal(t, want.name, failure.ReferenceObject)
				assert.NotNil(t, failure.OriginalJsonSchemaError)
			}
		})
	}
}

func TestDocumentPropertyNamesDoNotReplaceValueErrors(t *testing.T) {
	doc, err := libopenapi.NewDocument([]byte(`openapi: 3.1.0
info:
  title: Mixed errors
  version: 42
components:
  schemas:
    $bad:
      type: object
`))
	require.NoError(t, err)
	valid, failures := ValidateOpenAPIDocument(doc)
	require.False(t, valid)
	require.Len(t, failures, 1)
	var nameError, valueError bool
	for _, failure := range failures[0].SchemaValidationErrors {
		if failure.FieldName == "$bad" {
			nameError = true
			assert.Equal(t, 7, failure.Line)
		}
		if failure.FieldPath == "$.info.version" {
			valueError = true
			assert.NotContains(t, failure.Reason, "$bad")
			assert.Equal(t, 4, failure.Line)
		}
	}
	assert.True(t, nameError)
	assert.True(t, valueError)
}

func TestDocumentPropertyNamesPrecompiledRootConstraint(t *testing.T) {
	doc, err := libopenapi.NewDocument([]byte(`openapi: 3.1.0
info: {title: Test, version: '1'}
x: true
`))
	require.NoError(t, err)
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource("root.json", map[string]any{
		"propertyNames":     map[string]any{"minLength": 2},
		"dependentRequired": map[string]any{"x": []any{"missing"}},
	}))
	schema, err := compiler.Compile("root.json")
	require.NoError(t, err)
	valid, failures := ValidateOpenAPIDocumentWithPrecompiled(doc, schema)
	require.False(t, valid)
	require.Len(t, failures, 1)
	require.Len(t, failures[0].SchemaValidationErrors, 2)
	failure := failures[0].SchemaValidationErrors[0]
	assert.Equal(t, "x", failure.FieldName)
	assert.Equal(t, "$.x", failure.FieldPath)
	assert.Equal(t, []string{"x"}, failure.InstancePath)
	assert.Equal(t, 3, failure.Line)
	assert.Equal(t, 1, failure.Column)
	assert.Contains(t, failure.Reason, "minLength")
	assert.Contains(t, failures[0].SchemaValidationErrors[1].Reason, "missing")
	assert.Equal(t, helpers.Schema, failures[0].ValidationType)
}

func TestDocumentPropertyNamesKeepFollowingArrayErrors(t *testing.T) {
	for _, rule := range []any{false, map[string]any{"minLength": 2}} {
		t.Run(fmt.Sprint(rule), func(t *testing.T) {
			doc, err := libopenapi.NewDocument([]byte(`openapi: 3.1.0
info: {title: Test, version: '1'}
x-items:
  - x: true
  - 42
`))
			require.NoError(t, err)
			compiler := jsonschema.NewCompiler()
			require.NoError(t, compiler.AddResource("array.json", map[string]any{
				"properties": map[string]any{"x-items": map[string]any{
					"items": map[string]any{"type": "object", "propertyNames": rule},
				}},
			}))
			schema, err := compiler.Compile("array.json")
			require.NoError(t, err)
			valid, failures := ValidateOpenAPIDocumentWithPrecompiled(doc, schema)
			require.False(t, valid)
			require.Len(t, failures, 1)
			require.Len(t, failures[0].SchemaValidationErrors, 2)
			name, value := failures[0].SchemaValidationErrors[0], failures[0].SchemaValidationErrors[1]
			assert.Equal(t, "$['x-items'][0].x", name.FieldPath)
			assert.Equal(t, 4, name.Line)
			assert.Equal(t, 5, name.Column)
			assert.Equal(t, "$['x-items'][1]", value.FieldPath)
			assert.Contains(t, value.Reason, "want object")
		})
	}
}
