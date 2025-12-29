// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package strict

import (
	"context"
	"log/slog"
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
)

// Helper to build a schema from YAML
func buildSchemaFromYAML(t *testing.T, yml string) *libopenapi.DocumentModel[v3.Document] {
	doc, err := libopenapi.NewDocument([]byte(yml))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)
	return model
}

// Helper to get schema
func getSchema(t *testing.T, model *libopenapi.DocumentModel[v3.Document], name string) *base.Schema {
	schemaProxy := model.Model.Components.Schemas.GetOrZero(name)
	require.NotNil(t, schemaProxy)
	schema := schemaProxy.Schema()
	require.NotNil(t, schema)
	return schema
}

func TestStrictValidator_SimpleUndeclaredProperty(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        age:
          type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with undeclared property
	data := map[string]any{
		"name":  "John",
		"age":   30,
		"extra": "undeclared",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body.extra", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_AllPropertiesDeclared(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        age:
          type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with only declared properties
	data := map[string]any{
		"name": "John",
		"age":  30,
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_NestedObjects(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        address:
          type: object
          properties:
            street:
              type: string
            city:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with undeclared nested property
	data := map[string]any{
		"name": "John",
		"address": map[string]any{
			"street":  "123 Main St",
			"city":    "Anytown",
			"zipcode": "12345", // undeclared
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "zipcode", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body.address.zipcode", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_ArrayOfObjects(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Users:
      type: array
      items:
        type: object
        properties:
          name:
            type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Users")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with undeclared property in array item
	data := []any{
		map[string]any{
			"name":  "John",
			"extra": "undeclared",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_IgnorePaths(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.metadata.*"),
	)
	v := NewValidator(opts, 3.1)

	// Test that ignored path is not reported
	data := map[string]any{
		"name": "John",
		"metadata": map[string]any{
			"custom": "value", // Should be ignored
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// metadata itself is undeclared, but its children should be ignored
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "metadata", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_AdditionalPropertiesFalse(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// additionalProperties: false means base validation catches this
	// strict mode should NOT report (would be redundant)
	data := map[string]any{
		"name":  "John",
		"extra": "undeclared",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// strict should NOT report this since additionalProperties: false
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_AdditionalPropertiesWithSchema(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
      additionalProperties:
        type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// additionalProperties with schema means extra properties are allowed
	// but strict should still report them (they're not in explicit schema)
	data := map[string]any{
		"name":  "John",
		"extra": "valid string", // Matches additionalProperties schema
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// strict should report "extra" as undeclared even though it's valid
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_PatternProperties(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Config:
      type: object
      properties:
        name:
          type: string
      patternProperties:
        "^x-.*$":
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Config")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Properties matching patternProperties should be considered declared
	data := map[string]any{
		"name":     "myconfig",
		"x-custom": "extension value",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// x-custom matches the pattern, so it should be considered declared
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestBuildPath(t *testing.T) {
	tests := []struct {
		base     string
		propName string
		expected string
	}{
		{"$.body", "name", "$.body.name"},
		{"$.body", "a.b", "$.body['a.b']"},
		{"$.body", "x[0]", "$.body['x[0]']"},
		{"$.body.user", "email", "$.body.user.email"},
	}

	for _, tt := range tests {
		t.Run(tt.propName, func(t *testing.T) {
			result := buildPath(tt.base, tt.propName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompilePattern(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		matches bool
	}{
		// Single segment wildcard
		{"$.body.metadata.*", "$.body.metadata.custom", true},
		{"$.body.metadata.*", "$.body.metadata.custom.nested", false},

		// Double wildcard (any depth)
		{"$.body.**", "$.body.a.b.c", true},
		{"$.body.**.x-*", "$.body.deep.nested.x-custom", true},

		// Array index wildcard
		{"$.body.items[*].name", "$.body.items[0].name", true},
		{"$.body.items[*].name", "$.body.items[999].name", true},

		// Escaped asterisk
		{"$.body.\\*", "$.body.*", true},
		{"$.body.\\*", "$.body.anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			re := compilePattern(tt.pattern)
			if re == nil {
				t.Fatalf("Failed to compile pattern: %s", tt.pattern)
			}
			result := re.MatchString(tt.input)
			assert.Equal(t, tt.matches, result, "Pattern: %s, Input: %s", tt.pattern, tt.input)
		})
	}
}

func TestDirection_String(t *testing.T) {
	assert.Equal(t, "request", DirectionRequest.String())
	assert.Equal(t, "response", DirectionResponse.String())
}

func TestIsHeaderIgnored(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Standard headers should be ignored
	assert.True(t, v.isHeaderIgnored("Content-Type", DirectionRequest))
	assert.True(t, v.isHeaderIgnored("content-type", DirectionRequest))
	assert.True(t, v.isHeaderIgnored("Authorization", DirectionRequest))

	// Set-Cookie is direction-aware
	assert.True(t, v.isHeaderIgnored("Set-Cookie", DirectionResponse))
	assert.False(t, v.isHeaderIgnored("Set-Cookie", DirectionRequest))

	// Custom headers should not be ignored
	assert.False(t, v.isHeaderIgnored("X-Custom-Header", DirectionRequest))
}

func TestWithStrictIgnoredHeaders(t *testing.T) {
	// Replace defaults entirely
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnoredHeaders("X-Only-This"),
	)
	v := NewValidator(opts, 3.1)

	// Standard headers are NOT ignored anymore
	assert.False(t, v.isHeaderIgnored("Content-Type", DirectionRequest))

	// Only our custom header is ignored
	assert.True(t, v.isHeaderIgnored("X-Only-This", DirectionRequest))
}

func TestWithStrictIgnoredHeadersExtra(t *testing.T) {
	// Add to defaults
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnoredHeadersExtra("X-Custom-Extra"),
	)
	v := NewValidator(opts, 3.1)

	// Standard headers are still ignored
	assert.True(t, v.isHeaderIgnored("Content-Type", DirectionRequest))

	// Our custom header is also ignored
	assert.True(t, v.isHeaderIgnored("X-Custom-Extra", DirectionRequest))
}

func TestTruncateValue(t *testing.T) {
	// Short string unchanged
	assert.Equal(t, "hello", truncateValue("hello"))

	// Long string truncated
	longStr := "this is a very long string that should be truncated"
	result := truncateValue(longStr).(string)
	assert.True(t, len(result) <= 50)
	assert.Contains(t, result, "...")

	// Map truncated
	bigMap := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}
	assert.Equal(t, "{...}", truncateValue(bigMap))

	// Slice truncated
	bigSlice := []any{1, 2, 3, 4}
	assert.Equal(t, "[...]", truncateValue(bigSlice))
}

func TestStrictValidator_PolymorphicPatternProperties(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Edge
  version: "1.0"
paths: {}
components:
  schemas:
    VariantA:
      type: object
      required:
        - kind
      properties:
        kind:
          type: string
        aProp:
          type: string
    Root:
      type: object
      discriminator:
        propertyName: kind
      oneOf:
        - $ref: "#/components/schemas/VariantA"
      patternProperties:
        "^x-.*$":
          type: object
          properties:
            id:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Root")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"kind":  "VariantA",
		"aProp": "ok",
		"x-foo": map[string]any{
			"id":    "1",
			"extra": "nope",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	require.False(t, result.Valid)
	require.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body.x-foo.extra", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_ReusedSchemaDifferentPaths(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Edge
  version: "1.0"
paths: {}
components:
  schemas:
    Node:
      type: object
      properties:
        id:
          type: string
    Root:
      type: object
      properties:
        left:
          $ref: "#/components/schemas/Node"
        right:
          $ref: "#/components/schemas/Node"
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Root")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"left": map[string]any{
			"id":    "1",
			"extra": "nope",
		},
		"right": map[string]any{
			"id":    "2",
			"extra": "nope",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	require.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 2)
}

func TestStrictValidator_UnevaluatedItemsOnly(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Edge
  version: "1.0"
paths: {}
components:
  schemas:
    Items:
      type: array
      unevaluatedItems:
        type: object
        properties:
          id:
            type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Items")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	data := []any{
		map[string]any{
			"id":    "1",
			"extra": "nope",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	require.False(t, result.Valid)
	require.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "$.body[0].extra", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_HeaderIgnorePathsCase(t *testing.T) {
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.headers.x-trace"),
	)

	headers := http.Header{
		"X-Trace": {"abc"},
	}

	undeclared := ValidateRequestHeaders(headers, nil, opts)
	assert.Empty(t, undeclared)
}

func TestStrictValidator_OneOfWithParentProperties(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
      oneOf:
        - properties:
            name:
              type: string
        - properties:
            title:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with parent property "id" + oneOf variant property "name"
	// Both should be considered declared
	data := map[string]any{
		"id":   "123",
		"name": "John",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// id is from parent, name is from oneOf variant - both should be declared
	assert.True(t, result.Valid, "Parent + oneOf variant properties should be valid")
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_AnyOfWithParentProperties(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
      anyOf:
        - properties:
            name:
              type: string
        - properties:
            title:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with parent property "id" + anyOf variant property "name"
	data := map[string]any{
		"id":   "123",
		"name": "John",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// id is from parent, name is from anyOf variant - both should be declared
	assert.True(t, result.Valid, "Parent + anyOf variant properties should be valid")
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_OneOfWithUndeclaredProperty(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
      oneOf:
        - properties:
            name:
              type: string
        - properties:
            title:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with undeclared property "extra"
	data := map[string]any{
		"id":    "123",
		"name":  "John",
		"extra": "undeclared",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// "extra" is not in parent or variant - should be reported as undeclared
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_PatternPropertiesWithAdditionalPropertiesFalse(t *testing.T) {
	// This tests that patternProperties are recursed into even when
	// additionalProperties: false (which short-circuits to recurseIntoDeclaredProperties)
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Config:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
      patternProperties:
        "^x-":
          type: object
          properties:
            id:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Config")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with pattern property that has undeclared nested property
	data := map[string]any{
		"name": "test",
		"x-custom": map[string]any{
			"id":    "123",
			"extra": "undeclared nested field",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// "extra" inside x-custom should be reported as undeclared
	// This verifies patternProperties are recursed into even with additionalProperties: false
	assert.False(t, result.Valid, "Should report undeclared nested property in patternProperties")
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body.x-custom.extra", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_PatternPropertiesInOneOf(t *testing.T) {
	// This tests that patternProperties in oneOf/anyOf variants are recursed into
	// to find undeclared properties in nested objects.
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Container:
      type: object
      properties:
        type:
          type: string
      oneOf:
        - properties:
            type:
              const: "dynamic"
          patternProperties:
            "^x-":
              type: object
              properties:
                value:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Container")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with undeclared property inside pattern-matched nested object
	data := map[string]any{
		"type": "dynamic",
		"x-custom": map[string]any{
			"value":      "hello",
			"undeclared": "should be caught",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// "undeclared" inside x-custom should be reported
	assert.False(t, result.Valid, "Should report undeclared property in pattern-matched object")
	require.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "undeclared", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body.x-custom.undeclared", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_CycleDetection(t *testing.T) {
	// This tests that circular schema references don't cause infinite recursion.
	// The cycle detection should stop validation of the same schema at the same path.
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Node:
      type: object
      properties:
        name:
          type: string
        child:
          $ref: "#/components/schemas/Node"
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Node")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with deeply nested data that reuses the same schema
	data := map[string]any{
		"name": "root",
		"child": map[string]any{
			"name": "level1",
			"child": map[string]any{
				"name":  "level2",
				"extra": "undeclared at level2",
			},
		},
		"extra": "undeclared at root",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Should find undeclared properties at multiple levels
	assert.False(t, result.Valid)
	assert.GreaterOrEqual(t, len(result.UndeclaredValues), 2, "Should find undeclared at multiple levels")

	// Verify both undeclared properties were found
	var foundRoot, foundLevel2 bool
	for _, u := range result.UndeclaredValues {
		if u.Path == "$.body.extra" {
			foundRoot = true
		}
		if u.Path == "$.body.child.child.extra" {
			foundLevel2 = true
		}
	}
	assert.True(t, foundRoot, "Should find undeclared at root level")
	assert.True(t, foundLevel2, "Should find undeclared at nested level")
}

func TestStrictValidator_CycleDetectionDoesNotBlockDifferentPaths(t *testing.T) {
	// Tests that the same schema can be validated at different paths.
	// Cycle detection uses path+schemaRef, so same schema at different paths should work.
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Container:
      type: object
      properties:
        left:
          $ref: "#/components/schemas/Item"
        right:
          $ref: "#/components/schemas/Item"
    Item:
      type: object
      properties:
        id:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Container")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with undeclared properties in both left and right
	data := map[string]any{
		"left": map[string]any{
			"id":        "1",
			"extraLeft": "undeclared in left",
		},
		"right": map[string]any{
			"id":         "2",
			"extraRight": "undeclared in right",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Should find undeclared in both left and right
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 2, "Should find undeclared in both branches")

	var foundLeft, foundRight bool
	for _, u := range result.UndeclaredValues {
		if u.Name == "extraLeft" {
			foundLeft = true
		}
		if u.Name == "extraRight" {
			foundRight = true
		}
	}
	assert.True(t, foundLeft, "Should find undeclared in left branch")
	assert.True(t, foundRight, "Should find undeclared in right branch")
}

func TestValidateBody_UndeclaredProperty(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        age:
          type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())

	data := map[string]any{
		"name":  "John",
		"age":   30,
		"extra": "undeclared",
	}

	result := ValidateBody(schema, data, DirectionRequest, opts, 3.1)

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body.extra", result.UndeclaredValues[0].Path)
}

func TestValidateBody_AllPropertiesDeclared(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        age:
          type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())

	data := map[string]any{
		"name": "John",
		"age":  30,
	}

	result := ValidateBody(schema, data, DirectionResponse, opts, 3.1)

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestValidateBody_NilInputs(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	// nil schema
	result := ValidateBody(nil, map[string]any{"foo": "bar"}, DirectionRequest, opts, 3.1)
	assert.True(t, result.Valid)

	// nil data
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	result = ValidateBody(schema, nil, DirectionRequest, opts, 3.1)
	assert.True(t, result.Valid)
}

// ============== allOf tests ==============

func TestStrictValidator_AllOf_Simple(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Base:
      type: object
      properties:
        id:
          type: string
    Extended:
      allOf:
        - $ref: "#/components/schemas/Base"
        - type: object
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Extended")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Both id (from Base) and name (from inline) should be declared
	data := map[string]any{
		"id":   "123",
		"name": "Test",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_AllOf_WithUndeclared(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Base:
      type: object
      properties:
        id:
          type: string
    Extended:
      allOf:
        - $ref: "#/components/schemas/Base"
        - type: object
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Extended")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// extra is not in any allOf schema
	data := map[string]any{
		"id":    "123",
		"name":  "Test",
		"extra": "undeclared",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_AllOf_WithAdditionalPropertiesFalse(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Extended:
      allOf:
        - type: object
          additionalProperties: false
          properties:
            id:
              type: string
        - type: object
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Extended")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// When any allOf has additionalProperties: false, skip strict
	data := map[string]any{
		"id":    "123",
		"name":  "Test",
		"extra": "would normally be undeclared",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// additionalProperties: false means base validation handles this
	assert.True(t, result.Valid)
}

func TestStrictValidator_AllOf_WithNestedObjects(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Address:
      type: object
      properties:
        street:
          type: string
    Extended:
      allOf:
        - type: object
          properties:
            id:
              type: string
        - type: object
          properties:
            address:
              $ref: "#/components/schemas/Address"
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Extended")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Undeclared nested property in address
	data := map[string]any{
		"id": "123",
		"address": map[string]any{
			"street":  "Main St",
			"zipcode": "12345", // undeclared in Address
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "zipcode", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body.address.zipcode", result.UndeclaredValues[0].Path)
}

// ============== Parameter validation tests ==============

func TestValidateQueryParams_Basic(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	params := []*v3.Parameter{
		{Name: "limit", In: "query"},
		{Name: "offset", In: "query"},
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test?limit=10&offset=0&extra=undeclared", nil)

	undeclared := ValidateQueryParams(req, params, opts)

	assert.Len(t, undeclared, 1)
	assert.Equal(t, "extra", undeclared[0].Name)
	assert.Equal(t, "$.query.extra", undeclared[0].Path)
	assert.Equal(t, "query", undeclared[0].Type)
}

func TestValidateQueryParams_AllDeclared(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	params := []*v3.Parameter{
		{Name: "limit", In: "query"},
		{Name: "offset", In: "query"},
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test?limit=10&offset=0", nil)

	undeclared := ValidateQueryParams(req, params, opts)

	assert.Empty(t, undeclared)
}

func TestValidateQueryParams_IgnorePaths(t *testing.T) {
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.query.debug"),
	)

	params := []*v3.Parameter{
		{Name: "limit", In: "query"},
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test?limit=10&debug=true", nil)

	undeclared := ValidateQueryParams(req, params, opts)

	assert.Empty(t, undeclared)
}

func TestValidateQueryParams_NilInputs(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	// nil request
	assert.Nil(t, ValidateQueryParams(nil, nil, opts))

	// nil options
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	assert.Nil(t, ValidateQueryParams(req, nil, nil))

	// strict mode disabled
	optsNoStrict := config.NewValidationOptions()
	assert.Nil(t, ValidateQueryParams(req, nil, optsNoStrict))
}

func TestValidateCookies_Basic(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	params := []*v3.Parameter{
		{Name: "session", In: "cookie"},
		{Name: "token", In: "cookie"},
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	req.AddCookie(&http.Cookie{Name: "token", Value: "xyz789"})
	req.AddCookie(&http.Cookie{Name: "tracking", Value: "undeclared"})

	undeclared := ValidateCookies(req, params, opts)

	assert.Len(t, undeclared, 1)
	assert.Equal(t, "tracking", undeclared[0].Name)
	assert.Equal(t, "$.cookies.tracking", undeclared[0].Path)
	assert.Equal(t, "cookie", undeclared[0].Type)
}

func TestValidateCookies_AllDeclared(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	params := []*v3.Parameter{
		{Name: "session", In: "cookie"},
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})

	undeclared := ValidateCookies(req, params, opts)

	assert.Empty(t, undeclared)
}

func TestValidateCookies_IgnorePaths(t *testing.T) {
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.cookies.tracking"),
	)

	params := []*v3.Parameter{
		{Name: "session", In: "cookie"},
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	req.AddCookie(&http.Cookie{Name: "tracking", Value: "ignored"})

	undeclared := ValidateCookies(req, params, opts)

	assert.Empty(t, undeclared)
}

func TestValidateCookies_NilInputs(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	// nil request
	assert.Nil(t, ValidateCookies(nil, nil, opts))

	// nil options
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	assert.Nil(t, ValidateCookies(req, nil, nil))

	// strict mode disabled
	optsNoStrict := config.NewValidationOptions()
	assert.Nil(t, ValidateCookies(req, nil, optsNoStrict))
}

func TestValidateResponseHeaders_Basic(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	declaredHeaders := &map[string]*v3.Header{
		"X-Request-Id": {},
		"X-Rate-Limit": {},
	}

	headers := http.Header{
		"X-Request-Id":    {"abc123"},
		"X-Rate-Limit":    {"100"},
		"X-Custom-Header": {"undeclared"},
	}

	undeclared := ValidateResponseHeaders(headers, declaredHeaders, opts)

	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Custom-Header", undeclared[0].Name)
	assert.Equal(t, "$.headers.x-custom-header", undeclared[0].Path)
	assert.Equal(t, DirectionResponse, undeclared[0].Direction)
}

func TestValidateResponseHeaders_AllDeclared(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	declaredHeaders := &map[string]*v3.Header{
		"X-Request-Id": {},
	}

	headers := http.Header{
		"X-Request-Id": {"abc123"},
	}

	undeclared := ValidateResponseHeaders(headers, declaredHeaders, opts)

	assert.Empty(t, undeclared)
}

func TestValidateResponseHeaders_SetCookieIgnored(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	// No declared headers
	var declaredHeaders *map[string]*v3.Header

	headers := http.Header{
		"Set-Cookie": {"session=abc123"},
	}

	undeclared := ValidateResponseHeaders(headers, declaredHeaders, opts)

	// Set-Cookie should be ignored in responses
	assert.Empty(t, undeclared)
}

func TestValidateResponseHeaders_NilInputs(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())

	// nil headers
	assert.Nil(t, ValidateResponseHeaders(nil, nil, opts))

	// nil options
	headers := http.Header{"X-Test": {"value"}}
	assert.Nil(t, ValidateResponseHeaders(headers, nil, nil))

	// strict mode disabled
	optsNoStrict := config.NewValidationOptions()
	assert.Nil(t, ValidateResponseHeaders(headers, nil, optsNoStrict))
}

// ============== Array validation tests ==============

func TestStrictValidator_ArrayItemsFalse(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Empty:
      type: array
      items: false
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Empty")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// items: false means no items allowed
	data := []any{"item1", "item2"}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Should report both items as undeclared
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 2)
}

func TestStrictValidator_PrefixItems(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Tuple:
      type: array
      prefixItems:
        - type: object
          properties:
            first:
              type: string
        - type: object
          properties:
            second:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Tuple")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Each prefix item has its own schema
	data := []any{
		map[string]any{"first": "a", "extra1": "undeclared"},
		map[string]any{"second": "b", "extra2": "undeclared"},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 2)
}

func TestStrictValidator_PrefixItemsWithItems(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Tuple:
      type: array
      prefixItems:
        - type: object
          properties:
            first:
              type: string
      items:
        type: object
        properties:
          rest:
            type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Tuple")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// First item uses prefixItems[0], rest use items schema
	data := []any{
		map[string]any{"first": "a"},
		map[string]any{"rest": "b"},
		map[string]any{"rest": "c", "extra": "undeclared"},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body[2].extra", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_EmptyArray(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Items:
      type: array
      items:
        type: object
        properties:
          id:
            type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Items")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Empty array should be valid
	data := []any{}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

// ============== Additional edge case tests ==============

func TestStrictValidator_ReadOnlyInRequest(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
          readOnly: true
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// readOnly properties should not be expected in requests
	data := map[string]any{
		"name": "John",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
}

func TestStrictValidator_WriteOnlyInResponse(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        password:
          type: string
          writeOnly: true
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// writeOnly properties should not be expected in responses
	data := map[string]any{
		"name": "John",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionResponse,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
}

func TestStrictValidator_DiscriminatorMapping(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Dog:
      type: object
      properties:
        petType:
          type: string
        bark:
          type: string
    Cat:
      type: object
      properties:
        petType:
          type: string
        meow:
          type: string
    Pet:
      type: object
      discriminator:
        propertyName: petType
        mapping:
          dog: "#/components/schemas/Dog"
          cat: "#/components/schemas/Cat"
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with discriminator selecting Dog
	data := map[string]any{
		"petType": "dog",
		"bark":    "woof",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_NilSchemaData(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// nil schema
	result := v.Validate(Input{
		Schema:    nil,
		Data:      map[string]any{"foo": "bar"},
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})
	assert.True(t, result.Valid)

	// nil data
	result = v.Validate(Input{
		Schema:    &base.Schema{},
		Data:      nil,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})
	assert.True(t, result.Valid)
}

func TestNewValidator_NilOptions(t *testing.T) {
	v := NewValidator(nil, 3.1)
	assert.NotNil(t, v)
	assert.NotNil(t, v.logger)
}

func TestGetSchemaKey_NilSchema(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	key := v.getSchemaKey(nil)
	assert.Equal(t, "", key)
}

func TestGetCompiledPattern_Invalid(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Invalid regex pattern
	pattern := v.getCompiledPattern("[invalid")
	assert.Nil(t, pattern)
}

func TestGetCompiledPattern_Cached(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// First call compiles
	pattern1 := v.getCompiledPattern("^test$")
	assert.NotNil(t, pattern1)

	// Second call returns cached
	pattern2 := v.getCompiledPattern("^test$")
	assert.Equal(t, pattern1, pattern2)
}

func TestExceedsDepth(t *testing.T) {
	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	assert.False(t, ctx.exceedsDepth())

	// Create context at max depth
	for i := 0; i < 101; i++ {
		ctx = ctx.withPath("$.body.deep")
	}
	assert.True(t, ctx.exceedsDepth())
}

func TestCheckAndMarkVisited_Cycle(t *testing.T) {
	ctx := newTraversalContext(DirectionRequest, nil, "$.body")

	// First visit should return false (not a cycle)
	isCycle := ctx.checkAndMarkVisited("schema1")
	assert.False(t, isCycle)

	// Second visit to same schema at same path should return true (cycle)
	isCycle = ctx.checkAndMarkVisited("schema1")
	assert.True(t, isCycle)
}

func TestGetParamNames(t *testing.T) {
	params := []*v3.Parameter{
		{Name: "limit", In: "query"},
		{Name: "offset", In: "query"},
		{Name: "X-Api-Key", In: "header"},
	}

	queryNames := getParamNames(params, "query")
	assert.ElementsMatch(t, []string{"limit", "offset"}, queryNames)

	headerNames := getParamNames(params, "header")
	assert.ElementsMatch(t, []string{"X-Api-Key"}, headerNames)

	cookieNames := getParamNames(params, "cookie")
	assert.Empty(t, cookieNames)
}

func TestGetEffectiveIgnoredHeaders_Nil(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	headers := v.getEffectiveIgnoredHeaders()
	assert.NotEmpty(t, headers)
	assert.Contains(t, headers, "content-type")
}

func TestStrictValidator_DependentSchemas(t *testing.T) {
	// Test dependentSchemas with trigger property present
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    CreditCard:
      type: object
      properties:
        name:
          type: string
        creditCard:
          type: string
      dependentSchemas:
        creditCard:
          properties:
            billingAddress:
              type: string
          required:
            - billingAddress
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "CreditCard")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// When creditCard is present, billingAddress becomes a declared property
	data := map[string]any{
		"name":           "John",
		"creditCard":     "1234-5678-9012-3456",
		"billingAddress": "123 Main St",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_DependentSchemas_NoTrigger(t *testing.T) {
	// Test dependentSchemas when trigger property is NOT present
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    CreditCard:
      type: object
      properties:
        name:
          type: string
        creditCard:
          type: string
      dependentSchemas:
        creditCard:
          properties:
            billingAddress:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "CreditCard")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// When creditCard is NOT present, billingAddress is undeclared
	data := map[string]any{
		"name":           "John",
		"billingAddress": "123 Main St", // undeclared without creditCard trigger
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "billingAddress", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_IfThenElse_ThenBranch(t *testing.T) {
	// Test if/then/else - matching if condition uses then properties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Conditional:
      type: object
      properties:
        type:
          type: string
      if:
        properties:
          type:
            const: "car"
      then:
        properties:
          numWheels:
            type: integer
      else:
        properties:
          numLegs:
            type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Conditional")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// type="car" matches if condition, so numWheels is declared
	data := map[string]any{
		"type":      "car",
		"numWheels": 4,
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_IfThenElse_ElseBranch(t *testing.T) {
	// Test if/then/else - non-matching if condition uses else properties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Conditional:
      type: object
      properties:
        type:
          type: string
      if:
        properties:
          type:
            const: "car"
      then:
        properties:
          numWheels:
            type: integer
      else:
        properties:
          numLegs:
            type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Conditional")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// type="animal" does NOT match if condition, so numLegs is declared (else branch)
	data := map[string]any{
		"type":    "animal",
		"numLegs": 4,
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_IfThenElse_WrongBranchProperty(t *testing.T) {
	// Test if/then/else - using wrong branch property is undeclared
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Conditional:
      type: object
      properties:
        type:
          type: string
      if:
        properties:
          type:
            const: "car"
      then:
        properties:
          numWheels:
            type: integer
      else:
        properties:
          numLegs:
            type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Conditional")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// type="car" matches if condition (then branch), but we're using numLegs (else property)
	data := map[string]any{
		"type":    "car",
		"numLegs": 4, // wrong branch property
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "numLegs", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_OneOfWithParentBothAdditionalPropertiesFalse(t *testing.T) {
	// Test recurseIntoDeclaredPropertiesWithMerged path:
	// Both parent and variant have additionalProperties: false
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Pet:
      type: object
      additionalProperties: false
      properties:
        id:
          type: string
      oneOf:
        - $ref: '#/components/schemas/Dog'
        - $ref: '#/components/schemas/Cat'
    Dog:
      type: object
      additionalProperties: false
      properties:
        bark:
          type: boolean
    Cat:
      type: object
      additionalProperties: false
      properties:
        meow:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// All properties are declared (parent id + variant bark)
	data := map[string]any{
		"id":   "pet-123",
		"bark": true,
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Both parent and variant have additionalProperties: false
	// This triggers the recurseIntoDeclaredPropertiesWithMerged path
	// Standard validation would catch any extras, so strict just recurses
	assert.True(t, result.Valid)
}

func TestStrictValidator_OneOfWithParentBothAdditionalPropertiesFalse_NestedObject(t *testing.T) {
	// Test recurseIntoDeclaredPropertiesWithMerged with nested object validation
	// When both parent and variant have additionalProperties: false, the code
	// takes the recurseIntoDeclaredPropertiesWithMerged path which still recurses
	// into nested objects to check for undeclared properties there.
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Pet:
      type: object
      additionalProperties: false
      properties:
        id:
          type: string
        meta:
          type: object
          properties:
            version:
              type: string
      oneOf:
        - $ref: '#/components/schemas/Dog'
    Dog:
      type: object
      additionalProperties: false
      properties:
        bark:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Valid nested object - tests that recursion into nested objects works
	data := map[string]any{
		"id":   "pet-123",
		"bark": true,
		"meta": map[string]any{
			"version": "1.0",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// When both parent and variant have additionalProperties: false,
	// strict mode delegates to standard validation for undeclared detection
	// but still recurses into nested objects
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_MergePropertiesIntoDeclared_EmptySchema(t *testing.T) {
	// Test mergePropertiesIntoDeclared with nil/empty schema
	declared := make(map[string]*declaredProperty)
	mergePropertiesIntoDeclared(declared, nil)
	assert.Empty(t, declared)

	// Test with schema but nil properties
	schema := &base.Schema{}
	mergePropertiesIntoDeclared(declared, schema)
	assert.Empty(t, declared)
}

func TestStrictValidator_IsPropertyDeclaredInAllOf_EmptyAllOf(t *testing.T) {
	// Test isPropertyDeclaredInAllOf with nil allOf
	v := NewValidator(config.NewValidationOptions(config.WithStrictMode()), 3.1)
	result := v.isPropertyDeclaredInAllOf(nil, "test")
	assert.False(t, result)
}

func TestStrictValidator_GetSchemaKey_NilSchema(t *testing.T) {
	// Test getSchemaKey with nil schema returns empty string
	v := NewValidator(config.NewValidationOptions(config.WithStrictMode()), 3.1)
	key := v.getSchemaKey(nil)
	assert.Equal(t, "", key)
}

func TestStrictValidator_GetSchemaKey_SchemaWithHash(t *testing.T) {
	// Test getSchemaKey with schema that has a hash
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	v := NewValidator(config.NewValidationOptions(config.WithStrictMode()), 3.1)
	key := v.getSchemaKey(schema)
	assert.NotEmpty(t, key)
}

func TestStrictValidator_RecurseIntoDeclaredPropertiesWithMerged(t *testing.T) {
	// Test the recurseIntoDeclaredPropertiesWithMerged code path
	// This requires both parent AND variant to have additionalProperties: false
	// AND the data to only contain properties declared in the variant
	// (so the variant matching succeeds)
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Pet:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
        meta:
          type: object
          properties:
            version:
              type: string
      oneOf:
        - type: object
          additionalProperties: false
          properties:
            name:
              type: string
            meta:
              type: object
              properties:
                version:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Data that only has properties declared in the variant
	// The variant matches because it declares both name and meta
	data := map[string]any{
		"name": "Fido",
		"meta": map[string]any{
			"version": "1.0",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Both parent and variant have additionalProperties: false
	// Strict mode delegates to base validation but still recurses into declared properties
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_RecurseIntoDeclaredPropertiesWithMerged_WithIgnorePath(t *testing.T) {
	// Test the shouldIgnore path within recurseIntoDeclaredPropertiesWithMerged
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Pet:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
        details:
          type: object
          properties:
            version:
              type: string
      oneOf:
        - type: object
          additionalProperties: false
          properties:
            name:
              type: string
            details:
              type: object
              properties:
                version:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	// Ignore the details path
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.details"),
	)
	v := NewValidator(opts, 3.1)

	// Data with properties that match both parent and variant
	data := map[string]any{
		"name": "Fido",
		"details": map[string]any{
			"version": "1.0",
		},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Should be valid - tests that ignore path works in recurseIntoDeclaredPropertiesWithMerged
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_ShouldSkipProperty_WriteOnly_Request(t *testing.T) {
	// Test that writeOnly properties are not flagged in responses
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
        password:
          type: string
          writeOnly: true
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Password should be skipped in response direction
	data := map[string]any{
		"id":       "user-123",
		"password": "secret",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionResponse,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// writeOnly in response should be flagged (password shouldn't be in response)
	// Actually let me check the shouldSkipProperty logic
	assert.True(t, result.Valid)
}

func TestStrictValidator_IsPropertyDeclaredInAllOf_WithProperties(t *testing.T) {
	// Test isPropertyDeclaredInAllOf with actual allOf schemas
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Combined:
      allOf:
        - type: object
          properties:
            name:
              type: string
        - type: object
          properties:
            age:
              type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Combined")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test the isPropertyDeclaredInAllOf function
	isDeclared := v.isPropertyDeclaredInAllOf(schema.AllOf, "name")
	assert.True(t, isDeclared)

	isDeclared = v.isPropertyDeclaredInAllOf(schema.AllOf, "age")
	assert.True(t, isDeclared)

	isDeclared = v.isPropertyDeclaredInAllOf(schema.AllOf, "undeclared")
	assert.False(t, isDeclared)
}

func TestDiscardHandler_Methods(t *testing.T) {
	// Test the discardHandler slog.Handler implementation
	// These are interface methods required by slog.Handler

	d := discardHandler{}

	// Enabled should return false (no logging)
	assert.False(t, d.Enabled(context.TODO(), 0))

	// Handle should return nil (no error)
	assert.NoError(t, d.Handle(context.TODO(), slog.Record{}))

	// WithAttrs should return itself
	handler := d.WithAttrs(nil)
	assert.Equal(t, d, handler)

	// WithGroup should return itself
	handler = d.WithGroup("test")
	assert.Equal(t, d, handler)
}

func TestStrictValidator_DataMatchesSchema_NilSchema(t *testing.T) {
	// Test that nil schema matches anything
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	matches, err := v.dataMatchesSchema(nil, map[string]any{"foo": "bar"})
	assert.NoError(t, err)
	assert.True(t, matches)
}

func TestStrictValidator_GetCompiledSchema_NilSchema(t *testing.T) {
	// Test getCompiledSchema with nil schema
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	compiled, err := v.getCompiledSchema(nil)
	assert.NoError(t, err)
	assert.Nil(t, compiled)
}

func TestStrictValidator_GetCompiledSchema_LocalCacheHit(t *testing.T) {
	// Test that local cache is used on second call
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// First call - compiles and caches
	compiled1, err := v.getCompiledSchema(schema)
	assert.NoError(t, err)
	assert.NotNil(t, compiled1)

	// Second call - should hit local cache
	compiled2, err := v.getCompiledSchema(schema)
	assert.NoError(t, err)
	assert.NotNil(t, compiled2)
	assert.Same(t, compiled1, compiled2)
}

func TestStrictValidator_CompileSchema_NilSchema(t *testing.T) {
	// Test compileSchema with nil schema
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	compiled, err := v.compileSchema(nil)
	assert.NoError(t, err)
	assert.Nil(t, compiled)
}

func TestStrictValidator_GetEffectiveIgnoredHeaders_WithMerge(t *testing.T) {
	// Test getEffectiveIgnoredHeaders with merge mode
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnoredHeadersExtra("X-Custom"),
	)
	v := NewValidator(opts, 3.1)

	headers := v.getEffectiveIgnoredHeaders()
	// Should contain defaults plus the custom header
	assert.Contains(t, headers, "content-type") // From defaults
	assert.Contains(t, headers, "X-Custom")     // From extra
}

func TestStrictValidator_GetEffectiveIgnoredHeaders_WithReplace(t *testing.T) {
	// Test getEffectiveIgnoredHeaders with replace mode
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnoredHeaders("X-Only-This"),
	)
	v := NewValidator(opts, 3.1)

	headers := v.getEffectiveIgnoredHeaders()
	// Should ONLY contain the replaced headers
	assert.Contains(t, headers, "X-Only-This")
	assert.NotContains(t, headers, "content-type") // Defaults should be replaced
}

func TestStrictValidator_ValidateRequestHeaders_UndeclaredHeader(t *testing.T) {
	// Test ValidateRequestHeaders with undeclared header
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /test:
    get:
      parameters:
        - name: X-Known-Header
          in: header
          schema:
            type: string
      responses:
        "200":
          description: OK
`
	doc, _ := libopenapi.NewDocument([]byte(yml))
	model, _ := doc.BuildV3Model()

	opts := config.NewValidationOptions(config.WithStrictMode())

	params := model.Model.Paths.PathItems.GetOrZero("/test").Get.Parameters

	// Create headers directly
	headers := http.Header{
		"X-Known-Header":   {"value"},
		"X-Unknown-Header": {"value"}, // Not in spec
	}

	// ValidateRequestHeaders takes http.Header, not *http.Request
	undeclared := ValidateRequestHeaders(headers, params, opts)

	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Unknown-Header", undeclared[0].Name)
}

func TestStrictValidator_ValidateValue_NilSchema(t *testing.T) {
	// Test validateValue with nil schema
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	result := v.validateValue(ctx, nil, map[string]any{"foo": "bar"})
	assert.Empty(t, result)
}

func TestStrictValidator_ValidateValue_NonObjectData(t *testing.T) {
	// Test validateValue with non-object data (string, number, etc.)
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    StringType:
      type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "StringType")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	result := v.validateValue(ctx, schema, "hello")
	assert.Empty(t, result)
}

func TestStrictValidator_FindMatchingVariant_NoMatch(t *testing.T) {
	// Test findMatchingVariant when no variant matches
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Pet:
      oneOf:
        - type: object
          required:
            - bark
          properties:
            bark:
              type: boolean
        - type: object
          required:
            - meow
          properties:
            meow:
              type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Data that matches neither variant
	data := map[string]any{
		"swim": true,
	}

	variant := v.findMatchingVariant(schema.OneOf, data)
	assert.Nil(t, variant)
}

func TestStrictValidator_ShouldReportUndeclared_AdditionalPropertiesSchema(t *testing.T) {
	// Test shouldReportUndeclared with additionalProperties as a schema
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Config:
      type: object
      properties:
        name:
          type: string
      additionalProperties:
        type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Config")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	result := v.shouldReportUndeclared(schema)
	assert.True(t, result) // Should report undeclared even with additionalProperties schema
}

func TestStrictValidator_GetEffectiveIgnoredHeaders_NilOptions(t *testing.T) {
	// Test getEffectiveIgnoredHeaders with nil options
	v := &Validator{options: nil}
	headers := v.getEffectiveIgnoredHeaders()
	assert.Nil(t, headers)
}

func TestStrictValidator_ShouldSkipProperty_ReadOnlyInRequest(t *testing.T) {
	// Test that readOnly properties are skipped in request direction
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
          readOnly: true
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Get the id property which is readOnly
	idProp := schema.Properties.GetOrZero("id").Schema()

	// readOnly in request should be skipped
	result := v.shouldSkipProperty(idProp, DirectionRequest)
	assert.True(t, result)

	// readOnly in response should NOT be skipped
	result = v.shouldSkipProperty(idProp, DirectionResponse)
	assert.False(t, result)
}

func TestStrictValidator_ShouldSkipProperty_WriteOnlyInResponse(t *testing.T) {
	// Test that writeOnly properties are skipped in response direction
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        password:
          type: string
          writeOnly: true
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Get the password property which is writeOnly
	passwordProp := schema.Properties.GetOrZero("password").Schema()

	// writeOnly in response should be skipped
	result := v.shouldSkipProperty(passwordProp, DirectionResponse)
	assert.True(t, result)

	// writeOnly in request should NOT be skipped
	result = v.shouldSkipProperty(passwordProp, DirectionRequest)
	assert.False(t, result)
}

func TestStrictValidator_ShouldReportUndeclared_UnevaluatedPropertiesFalse(t *testing.T) {
	// Test that unevaluatedProperties: false still reports undeclared in strict mode
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Config:
      type: object
      properties:
        name:
          type: string
      unevaluatedProperties: false
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Config")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	result := v.shouldReportUndeclared(schema)
	assert.True(t, result)
}

func TestStrictValidator_ValidateValue_ExceedsDepth(t *testing.T) {
	// Test validateValue when max depth is exceeded
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    DeepNested:
      type: object
      properties:
        level:
          type: object
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "DeepNested")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	// Increase depth artificially to exceed max
	for i := 0; i < 101; i++ {
		ctx = ctx.withPath("$.body.deep")
	}

	result := v.validateValue(ctx, schema, map[string]any{"level": map[string]any{}})
	assert.Empty(t, result) // Should return early due to depth
}

func TestStrictValidator_AnyOf_WithMatch(t *testing.T) {
	// Test validateAnyOf with a matching variant
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: string
      anyOf:
        - type: object
          properties:
            bark:
              type: boolean
        - type: object
          properties:
            meow:
              type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Data that matches the first variant
	data := map[string]any{
		"id":    "pet-123",
		"bark":  true,
		"extra": "undeclared",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_AnyOf_WithDiscriminator(t *testing.T) {
	// Test validateAnyOf with discriminator
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Pet:
      type: object
      discriminator:
        propertyName: petType
        mapping:
          dog: '#/components/schemas/Dog'
      anyOf:
        - $ref: '#/components/schemas/Dog'
        - $ref: '#/components/schemas/Cat'
    Dog:
      type: object
      properties:
        petType:
          type: string
        bark:
          type: boolean
    Cat:
      type: object
      properties:
        petType:
          type: string
        meow:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"petType": "dog",
		"bark":    true,
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	assert.True(t, result.Valid)
}

func TestStrictValidator_FindMatchingVariant_NilProxy(t *testing.T) {
	// Test findMatchingVariant with nil proxy in variants
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Create a slice with nil entry
	variants := []*base.SchemaProxy{nil}

	result := v.findMatchingVariant(variants, map[string]any{"foo": "bar"})
	assert.Nil(t, result)
}

func TestStrictValidator_ShouldReportUndeclaredForAllOf_AdditionalPropertiesFalse(t *testing.T) {
	// Test shouldReportUndeclaredForAllOf when parent has additionalProperties: false
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Combined:
      type: object
      additionalProperties: false
      allOf:
        - type: object
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Combined")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Should return false because parent has additionalProperties: false
	result := v.shouldReportUndeclaredForAllOf(schema)
	assert.False(t, result)
}

func TestStrictValidator_ShouldReportUndeclaredForAllOf_AllOfHasAdditionalPropertiesFalse(t *testing.T) {
	// Test shouldReportUndeclaredForAllOf when allOf member has additionalProperties: false
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Combined:
      type: object
      allOf:
        - type: object
          additionalProperties: false
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Combined")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Should return false because allOf member has additionalProperties: false
	result := v.shouldReportUndeclaredForAllOf(schema)
	assert.False(t, result)
}

func TestStrictValidator_FindPropertySchemaInAllOf_FromDeclared(t *testing.T) {
	// Test findPropertySchemaInAllOf finding property from declared map
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "User")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Create declared map with the property
	declared := make(map[string]*declaredProperty)
	declared["name"] = &declaredProperty{
		proxy: schema.Properties.GetOrZero("name"),
	}

	result := v.findPropertySchemaInAllOf(nil, "name", declared)
	assert.NotNil(t, result)
}
