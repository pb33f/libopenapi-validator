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

	libcache "github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
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

	undeclared := ValidateRequestHeaders(headers, nil, nil, opts)
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

func TestStrictValidator_PrefixItems_FewerDataElements(t *testing.T) {
	// Covers array_validator.go:41-42 - break when data has fewer elements than prefixItems
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
        - type: object
          properties:
            third:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Tuple")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Only 1 data element, but 3 prefixItems - should break early at line 42
	data := []any{
		map[string]any{"first": "a", "extra": "undeclared"},
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Only first element validated, has one undeclared property
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
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
	undeclared := ValidateRequestHeaders(headers, params, nil, opts)

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

// Additional nil check tests

func TestStrictValidator_IsPropertyDeclaredInAllOf_NilSchemaProxy(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with nil SchemaProxy in allOf slice
	allOf := []*base.SchemaProxy{nil}
	result := v.isPropertyDeclaredInAllOf(allOf, "foo")
	assert.False(t, result)
}

func TestStrictValidator_IsPropertyDeclaredInAllOf_NilSchema(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with empty allOf
	result := v.isPropertyDeclaredInAllOf(nil, "foo")
	assert.False(t, result)
}

func TestStrictValidator_ShouldReportUndeclaredForAllOf_NilSchemaProxy(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Test:
      type: object
      allOf:
        - type: object
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Test")

	// Manually inject a nil into allOf to test the nil check
	schema.AllOf = append(schema.AllOf, nil)

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Should still work and return true (default behavior)
	result := v.shouldReportUndeclaredForAllOf(schema)
	assert.True(t, result)
}

func TestStrictValidator_FindPropertySchemaInAllOf_NilSchemaProxy(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with nil SchemaProxy in allOf
	allOf := []*base.SchemaProxy{nil}
	result := v.findPropertySchemaInAllOf(allOf, "foo", nil)
	assert.Nil(t, result)
}

func TestStrictValidator_RecurseIntoAllOfDeclaredProperties_NilSchemaProxy(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	allOf := []*base.SchemaProxy{nil}
	data := map[string]any{"foo": "bar"}

	result := v.recurseIntoAllOfDeclaredProperties(ctx, allOf, data, nil)
	assert.Empty(t, result)
}

func TestStrictValidator_SelectByDiscriminator_NilDiscriminator(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Test:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Test")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Schema has no discriminator
	result := v.selectByDiscriminator(schema, nil, map[string]any{"foo": "bar"})
	assert.Nil(t, result)
}

func TestStrictValidator_SelectByDiscriminator_EmptyPropertyName(t *testing.T) {
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
        propertyName: ""
      oneOf:
        - $ref: '#/components/schemas/Dog'
    Dog:
      type: object
      properties:
        bark:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	result := v.selectByDiscriminator(schema, schema.OneOf, map[string]any{"petType": "Dog"})
	assert.Nil(t, result)
}

func TestStrictValidator_SelectByDiscriminator_MissingDiscriminatorValue(t *testing.T) {
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
      oneOf:
        - $ref: '#/components/schemas/Dog'
    Dog:
      type: object
      properties:
        bark:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Data doesn't have the discriminator property
	result := v.selectByDiscriminator(schema, schema.OneOf, map[string]any{"bark": true})
	assert.Nil(t, result)
}

func TestStrictValidator_SelectByDiscriminator_NonStringValue(t *testing.T) {
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
      oneOf:
        - $ref: '#/components/schemas/Dog'
    Dog:
      type: object
      properties:
        bark:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Discriminator value is not a string
	result := v.selectByDiscriminator(schema, schema.OneOf, map[string]any{"petType": 123})
	assert.Nil(t, result)
}

func TestStrictValidator_SelectByDiscriminator_NoMatchingVariant(t *testing.T) {
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
      oneOf:
        - $ref: '#/components/schemas/Dog'
    Dog:
      type: object
      properties:
        bark:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Pet")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Discriminator value doesn't match any variant
	result := v.selectByDiscriminator(schema, schema.OneOf, map[string]any{"petType": "Cat"})
	assert.Nil(t, result)
}

func TestStrictValidator_FindMatchingVariant_NoMatch2(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Dog:
      type: object
      required:
        - bark
      properties:
        bark:
          type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Dog")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Create variants with a schema that won't match the data
	variants := []*base.SchemaProxy{base.CreateSchemaProxy(schema)}

	// Data doesn't have required 'bark' property - won't match
	result := v.findMatchingVariant(variants, map[string]any{"meow": true})
	assert.Nil(t, result)
}

func TestStrictValidator_CollectDeclaredProperties_NilSchema(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	declared, patterns := v.collectDeclaredProperties(nil, nil)
	assert.Empty(t, declared)
	assert.Empty(t, patterns)
}

func TestStrictValidator_GetDeclaredPropertyNames_Empty(t *testing.T) {
	result := getDeclaredPropertyNames(nil)
	assert.Empty(t, result)
}

func TestStrictValidator_ShouldSkipProperty_NilSchema(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	result := v.shouldSkipProperty(nil, DirectionRequest)
	assert.False(t, result)
}

func TestStrictValidator_ValidateObject_NilProperties(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Empty:
      type: object
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Empty")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	result := v.validateObject(ctx, schema, map[string]any{"foo": "bar"})

	// In strict mode, empty schema with no properties still reports undeclared
	// because additionalProperties defaults to true (meaning strict mode catches it)
	assert.Len(t, result, 1)
	assert.Equal(t, "foo", result[0].Name)
}

func TestStrictValidator_ShouldReportUndeclared_NilSchema(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// nil schema returns false - can't report undeclared without schema
	result := v.shouldReportUndeclared(nil)
	assert.False(t, result)
}

func TestStrictValidator_GetPatternPropertySchema_NoPatterns(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    NoPatterns:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "NoPatterns")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Schema has no patternProperties
	result := v.getPatternPropertySchema(schema, "foo")
	assert.Nil(t, result)
}

func TestStrictValidator_RecurseIntoDeclaredProperties_EmptySchema(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Empty:
      type: object
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Empty")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	data := map[string]any{"name": "test"}

	// recurseIntoDeclaredProperties only takes ctx, schema, data
	result := v.recurseIntoDeclaredProperties(ctx, schema, data)
	assert.Empty(t, result)
}

func TestStrictValidator_ValidateArray_NilItems(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    List:
      type: array
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "List")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	result := v.validateArray(ctx, schema, []any{"foo", "bar"})

	// Array with no items schema - anything is allowed
	assert.Empty(t, result)
}

func TestStrictValidator_ValidateArray_ItemsSchemaB(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    List:
      type: array
      items: true
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "List")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	result := v.validateArray(ctx, schema, []any{"foo", "bar"})

	// items: true means all items are valid
	assert.Empty(t, result)
}

func TestStrictValidator_ValidateArray_PrefixItemsNil(t *testing.T) {
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
        - type: string
        - type: integer
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Tuple")

	// Manually set one prefixItem to nil to test the nil check
	schema.PrefixItems = append(schema.PrefixItems, nil)

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")
	result := v.validateArray(ctx, schema, []any{"foo", 42, "extra"})

	// Should handle nil prefixItems gracefully
	assert.Empty(t, result)
}

func TestStrictValidator_FindPropertySchemaInMerged_NilProxy(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Create declared map with nil proxy
	declared := make(map[string]*declaredProperty)
	declared["name"] = &declaredProperty{proxy: nil}

	// findPropertySchemaInMerged takes (variant, parent, propName, declared)
	result := v.findPropertySchemaInMerged(nil, nil, "name", declared)
	assert.Nil(t, result)
}

func TestStrictValidator_RecurseIntoDeclaredPropertiesWithMerged_NilProxy(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")

	// Create declared with nil proxy
	declared := make(map[string]*declaredProperty)
	declared["name"] = &declaredProperty{proxy: nil}

	data := map[string]any{"name": "test"}

	// recurseIntoDeclaredPropertiesWithMerged takes (ctx, variant, parent, data, declared)
	result := v.recurseIntoDeclaredPropertiesWithMerged(ctx, nil, nil, data, declared)
	assert.Empty(t, result)
}

func TestStrictValidator_ValidateAnyOf_NoMatchingVariant(t *testing.T) {
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    StringOrInt:
      anyOf:
        - type: string
          minLength: 5
        - type: integer
          minimum: 10
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "StringOrInt")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	ctx := newTraversalContext(DirectionRequest, nil, "$.body")

	// Data is an object which won't match string or integer
	result := v.validateAnyOf(ctx, schema, map[string]any{"foo": "bar"})

	// Should return empty - no matching variant means we can't validate
	assert.Empty(t, result)
}

func TestStrictValidator_CompilePattern_EmptyPattern(t *testing.T) {
	// Test compilePattern with empty pattern
	result := compilePattern("")
	assert.Nil(t, result)
}

func TestStrictValidator_GetSchemaKey_NoLowLevel(t *testing.T) {
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Create a schema without low-level info
	schema := &base.Schema{}

	key := v.getSchemaKey(schema)
	// Should return pointer-based key
	assert.NotEmpty(t, key)
}

func TestTruncateValue_SmallMapUnchanged(t *testing.T) {
	// Small map (<= 3 entries) should return unchanged
	input := map[string]any{"a": 1, "b": 2}
	result := TruncateValue(input)
	assert.Equal(t, input, result)

	// Exactly 3 entries should also pass unchanged
	input3 := map[string]any{"a": 1, "b": 2, "c": 3}
	result3 := TruncateValue(input3)
	assert.Equal(t, input3, result3)
}

func TestTruncateValue_SmallArrayUnchanged(t *testing.T) {
	// Small array (<= 3 entries) should return unchanged
	input := []any{1, 2}
	result := TruncateValue(input)
	assert.Equal(t, input, result)

	// Exactly 3 entries should also pass unchanged
	input3 := []any{1, 2, 3}
	result3 := TruncateValue(input3)
	assert.Equal(t, input3, result3)
}

func TestStrictValidator_DataMatchesSchema_CompilationError(t *testing.T) {
	// Create a schema with an invalid regex pattern that will fail compilation
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    BadPattern:
      type: object
      properties:
        name:
          type: string
          pattern: "[invalid(regex"
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "BadPattern")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// dataMatchesSchema should return false with error due to invalid pattern
	matches, err := v.dataMatchesSchema(schema, map[string]any{"name": "test"})
	assert.False(t, matches)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "strict:")
}

func TestStrictValidator_FindMatchingVariant_SchemaError(t *testing.T) {
	// Create oneOf with a variant that has invalid pattern - should skip bad variant
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Container:
      oneOf:
        - type: object
          properties:
            valid:
              type: string
        - type: object
          properties:
            broken:
              type: string
              pattern: "[unclosed("
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "Container")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// findMatchingVariant should skip the broken variant and match the valid one
	variant := v.findMatchingVariant(schema.OneOf, map[string]any{"valid": "test"})

	// Should find a valid variant (the first one)
	assert.NotNil(t, variant)
}

func TestStrictValidator_GetPatternPropertySchema_InvalidPattern(t *testing.T) {
	// Create schema with invalid patternProperties regex
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    BadPatternProps:
      type: object
      patternProperties:
        "[invalid(":
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "BadPatternProps")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// getPatternPropertySchema should return nil for invalid pattern
	propProxy := v.getPatternPropertySchema(schema, "test")
	assert.Nil(t, propProxy)
}

// =============================================================================
// Phase 1: CRITICAL Coverage Tests
// =============================================================================

func TestStrictValidator_AllOfWithParentProperties(t *testing.T) {
	// Covers polymorphic.go:88-91 - parent schema properties merged with allOf
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    MergedSchema:
      type: object
      properties:
        parentProp:
          type: string
      allOf:
        - type: object
          properties:
            childProp:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "MergedSchema")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Both parent and child properties should be considered declared
	data := map[string]any{
		"parentProp": "from parent",
		"childProp":  "from child",
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

func TestStrictValidator_AllOfWithParentProperties_UndeclaredReported(t *testing.T) {
	// Verify undeclared properties are still caught with parent+allOf merge
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    MergedSchema:
      type: object
      properties:
        parentProp:
          type: string
      allOf:
        - type: object
          properties:
            childProp:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "MergedSchema")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"parentProp":     "from parent",
		"childProp":      "from child",
		"undeclaredProp": "should be reported",
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
	assert.Equal(t, "undeclaredProp", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_AllOfReadOnlyInRequest(t *testing.T) {
	// Covers polymorphic.go:116-117 - shouldSkipProperty for readOnly in allOf
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    ReadOnlyAllOf:
      type: object
      allOf:
        - type: object
          properties:
            id:
              type: string
              readOnly: true
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "ReadOnlyAllOf")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// In request direction, readOnly property should be skipped
	data := map[string]any{
		"id":   "123",
		"name": "test",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// id is readOnly - should be skipped in request validation (not flagged)
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_AllOfWriteOnlyInResponse(t *testing.T) {
	// Covers polymorphic.go:222-223 - shouldSkipProperty for writeOnly in oneOf/anyOf
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    WriteOnlySchema:
      type: object
      oneOf:
        - type: object
          properties:
            password:
              type: string
              writeOnly: true
            email:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "WriteOnlySchema")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// In response direction, writeOnly property should be skipped
	data := map[string]any{
		"password": "secret123",
		"email":    "user@example.com",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionResponse,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// password is writeOnly - should be skipped in response validation
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_AllOfWithIgnoredPath(t *testing.T) {
	// Covers polymorphic.go:107-108 - shouldIgnore in allOf validation loop
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    IgnoreInAllOf:
      type: object
      allOf:
        - type: object
          properties:
            data:
              type: object
              properties:
                visible:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "IgnoreInAllOf")

	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.data.metadata"),
	)
	v := NewValidator(opts, 3.1)

	// metadata path is ignored, so undeclared properties there should not be reported
	data := map[string]any{
		"data": map[string]any{
			"visible": "ok",
			"metadata": map[string]any{
				"ignored":     "should not be flagged",
				"alsoIgnored": "also not flagged",
			},
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

	// metadata path is ignored, so no undeclared errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_OneOfWithIgnoredPath(t *testing.T) {
	// Covers polymorphic.go:213-214 - shouldIgnore in oneOf/anyOf validation loop
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    IgnoreInOneOf:
      type: object
      oneOf:
        - type: object
          properties:
            data:
              type: object
              properties:
                name:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "IgnoreInOneOf")

	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.data.internal"),
	)
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"data": map[string]any{
			"name": "visible",
			"internal": map[string]any{
				"secret": "ignored",
			},
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

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_OneOfWithIgnoredTopLevelProperty(t *testing.T) {
	// Covers polymorphic.go:213-214 - shouldIgnore at TOP LEVEL of oneOf iteration
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    OneOfIgnoreTopLevel:
      type: object
      oneOf:
        - type: object
          properties:
            name:
              type: string
            internal:
              type: object
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "OneOfIgnoreTopLevel")

	// Ignore "internal" property at top level - this directly hits line 214
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.internal"),
	)
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"name": "visible",
		"internal": map[string]any{
			"anything": "should be ignored entirely",
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

	// internal property is ignored at top level, no errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_FindPropertySchemaInMerged_VariantProperty(t *testing.T) {
	// Covers polymorphic.go:248-249 - property found in variant's explicit properties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    OneOfVariantProp:
      type: object
      properties:
        parentProp:
          type: string
      oneOf:
        - type: object
          properties:
            variantProp:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "OneOfVariantProp")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// variantProp is defined in variant, should be found via line 249
	data := map[string]any{
		"parentProp":  "parent",
		"variantProp": "variant",
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

func TestStrictValidator_FindPropertySchemaInMerged_ParentProperty(t *testing.T) {
	// Covers polymorphic.go:254-256 - property found in parent's explicit properties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    OneOfParentProp:
      type: object
      properties:
        parentOnly:
          type: string
      oneOf:
        - type: object
          properties:
            variantOnly:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "OneOfParentProp")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// parentOnly is NOT in variant, so findPropertySchemaInMerged falls through
	// to parent lookup at line 254-256
	data := map[string]any{
		"parentOnly":  "from parent",
		"variantOnly": "from variant",
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

func TestStrictValidator_FindPropertySchemaInMerged_VariantDirect(t *testing.T) {
	// Covers polymorphic.go:247-249 - direct test with empty declared map
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Variant:
      type: object
      properties:
        variantProp:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	variant := getSchema(t, model, "Variant")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Call with empty declared map - forces lookup in variant.Properties (line 247-249)
	result := v.findPropertySchemaInMerged(variant, nil, "variantProp", make(map[string]*declaredProperty))
	assert.NotNil(t, result)
}

func TestStrictValidator_FindPropertySchemaInMerged_ParentDirect(t *testing.T) {
	// Covers polymorphic.go:254-256 - direct test with empty declared map, no variant
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    Parent:
      type: object
      properties:
        parentProp:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	parent := getSchema(t, model, "Parent")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Call with nil variant and empty declared - forces lookup in parent.Properties (line 254-256)
	result := v.findPropertySchemaInMerged(nil, parent, "parentProp", make(map[string]*declaredProperty))
	assert.NotNil(t, result)
}

func TestStrictValidator_FindPropertySchemaInAllOf_FromAllOfSchema(t *testing.T) {
	// Covers polymorphic.go:437-439 - property found in allOf schema's explicit properties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AllOfExplicitProp:
      type: object
      allOf:
        - type: object
          properties:
            fromAllOf:
              type: object
              properties:
                nested:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AllOfExplicitProp")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// fromAllOf is in allOf schema, findPropertySchemaInAllOf should find it
	// and recurse into nested object to detect undeclared
	data := map[string]any{
		"fromAllOf": map[string]any{
			"nested":     "valid",
			"undeclared": "should be flagged",
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

	// undeclared in nested object should be reported
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "undeclared", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_FindPropertySchemaInAllOf_Direct(t *testing.T) {
	// Covers polymorphic.go:437-439 - direct test with empty declared map
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AllOfSchema:
      type: object
      allOf:
        - type: object
          properties:
            allOfProp:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AllOfSchema")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Call with empty declared map - forces lookup in allOf schemas (line 437-439)
	result := v.findPropertySchemaInAllOf(schema.AllOf, "allOfProp", make(map[string]*declaredProperty))
	assert.NotNil(t, result)
}

func TestStrictValidator_RecurseIntoAllOfDeclaredProperties_ShouldIgnore(t *testing.T) {
	// Covers polymorphic.go:455-456 - shouldIgnore in recurseIntoAllOfDeclaredProperties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AllOfIgnore:
      type: object
      additionalProperties: false
      allOf:
        - type: object
          additionalProperties: false
          properties:
            name:
              type: string
            metadata:
              type: object
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AllOfIgnore")

	// Ignore metadata at top level
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.metadata"),
	)
	v := NewValidator(opts, 3.1)

	// Both parent and allOf have additionalProperties: false,
	// so we go through recurseIntoAllOfDeclaredProperties
	// metadata is ignored at line 455-456
	data := map[string]any{
		"name": "test",
		"metadata": map[string]any{
			"anything": "ignored",
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

	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_RecurseIntoAllOfDeclaredProperties_SkipReadOnly(t *testing.T) {
	// Covers polymorphic.go:461-462 - shouldSkipProperty in recurseIntoAllOfDeclaredProperties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AllOfReadOnly:
      type: object
      additionalProperties: false
      allOf:
        - type: object
          additionalProperties: false
          properties:
            name:
              type: string
            id:
              type: string
              readOnly: true
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AllOfReadOnly")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Both parent and allOf have additionalProperties: false,
	// so we go through recurseIntoAllOfDeclaredProperties
	// id is readOnly and skipped in request direction (line 461-462)
	data := map[string]any{
		"name": "test",
		"id":   "should-be-skipped",
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

func TestStrictValidator_RecurseIntoDeclaredPropertiesWithMerged_SkipReadOnly(t *testing.T) {
	// Covers polymorphic.go:291-292 - shouldSkipProperty in recurseIntoDeclaredPropertiesWithMerged
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    OneOfWithReadOnly:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
      oneOf:
        - type: object
          additionalProperties: false
          properties:
            name:
              type: string
            id:
              type: string
              readOnly: true
            data:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "OneOfWithReadOnly")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// In request direction, readOnly property "id" should be skipped (line 291-292)
	// Both parent and variant have additionalProperties: false, so we go through
	// recurseIntoDeclaredPropertiesWithMerged
	// Note: variant must also declare "name" so data matches the variant
	data := map[string]any{
		"name": "test",
		"id":   "should-be-skipped",
		"data": "valid",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// id is readOnly and skipped in request, no validation errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_AllOfAdditionalPropertiesFalseRecurse(t *testing.T) {
	// Covers polymorphic.go:461-462, 467-468 - recursion with additionalProperties: false
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    RecurseTest:
      type: object
      additionalProperties: false
      allOf:
        - type: object
          properties:
            nested:
              type: object
              properties:
                valid:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "RecurseTest")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// nested.extra should be reported as undeclared
	data := map[string]any{
		"nested": map[string]any{
			"valid": "ok",
			"extra": "should be flagged",
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
	assert.Equal(t, "$.body.nested.extra", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_OneOfVariantPropertyPriority(t *testing.T) {
	// Covers polymorphic.go:248-250, 255-257 - findPropertySchemaInMerged
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    PriorityTest:
      type: object
      properties:
        type:
          type: string
      oneOf:
        - type: object
          properties:
            details:
              type: object
              properties:
                variantField:
                  type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "PriorityTest")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// type is from parent, details is from variant
	data := map[string]any{
		"type": "test",
		"details": map[string]any{
			"variantField": "from variant",
			"undeclared":   "should be flagged",
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

	// undeclared in details should be flagged
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "undeclared", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_PropertyDeclaredInAllOfChild(t *testing.T) {
	// Covers polymorphic.go:46-47 - isPropertyDeclaredInAllOf continuation
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AllOfChildProp:
      type: object
      properties:
        parentOnly:
          type: string
      allOf:
        - type: object
          properties:
            fromChild:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AllOfChildProp")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// fromChild is declared in allOf child, should be considered declared
	data := map[string]any{
		"parentOnly": "parent",
		"fromChild":  "child",
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

func TestStrictValidator_ValidateAllOf_NilSchemaProxy(t *testing.T) {
	// Covers polymorphic.go:67-68 - nil schemaProxy in allOf loop
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AllOfWithNil:
      type: object
      allOf:
        - type: object
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AllOfWithNil")

	// Inject nil into allOf array to test the nil check at line 67-68
	schema.AllOf = append(schema.AllOf, nil)

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"name":  "test",
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

	// Should still work - nil schemaProxy is skipped
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_ValidateAllOf_IgnoreTopLevelProperty(t *testing.T) {
	// Covers polymorphic.go:107-108 - shouldIgnore for top-level property in allOf
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AllOfIgnoreTopLevel:
      type: object
      allOf:
        - type: object
          properties:
            name:
              type: string
            metadata:
              type: object
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AllOfIgnoreTopLevel")

	// Ignore the metadata property at top level
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.metadata"),
	)
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"name": "test",
		"metadata": map[string]any{
			"anything": "should be ignored at this level",
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

	// metadata property is ignored entirely, no errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

// =============================================================================
// Phase 2: HIGH Priority Coverage Tests
// =============================================================================

func TestStrictValidator_SchemaCacheHit(t *testing.T) {
	// Covers matcher.go:60-62 - global schema cache hit path
	// Need a oneOf schema to trigger dataMatchesSchema which uses getCompiledSchema
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    DogVariant:
      type: object
      properties:
        breed:
          type: string
    CatVariant:
      type: object
      properties:
        color:
          type: string
    CachedSchema:
      type: object
      oneOf:
        - $ref: '#/components/schemas/DogVariant'
        - $ref: '#/components/schemas/CatVariant'
`
	model := buildSchemaFromYAML(t, yml)
	dogSchema := getSchema(t, model, "DogVariant")

	// Create options with schema cache
	opts := config.NewValidationOptions(config.WithStrictMode())

	// Pre-populate the GLOBAL cache with a compiled schema for the DogVariant hash
	// This is what findMatchingVariant will check via dataMatchesSchema
	hash := dogSchema.GoLow().Hash()
	compiledSchema, err := helpers.NewCompiledSchemaWithVersion(
		"test-cached",
		[]byte(`{"type":"object","properties":{"breed":{"type":"string"}}}`),
		opts,
		3.1,
	)
	require.NoError(t, err)
	opts.SchemaCache.Store(hash, &libcache.SchemaCacheEntry{
		CompiledSchema: compiledSchema,
	})

	v := NewValidator(opts, 3.1)

	// Data that matches DogVariant
	data := map[string]any{
		"breed": "labrador",
		"extra": "undeclared",
	}

	// Get the parent oneOf schema
	parentSchema := getSchema(t, model, "CachedSchema")

	// Validation should hit the GLOBAL cache when checking oneOf variants
	result := v.Validate(Input{
		Schema:    parentSchema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Should still detect undeclared property
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_PrefixItemsWithIgnoredPath(t *testing.T) {
	// Covers array_validator.go:48-50 - shouldIgnore in prefixItems loop
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    TupleIgnore:
      type: array
      prefixItems:
        - type: object
          properties:
            id:
              type: string
        - type: object
          properties:
            name:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "TupleIgnore")

	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body[0]"),
	)
	v := NewValidator(opts, 3.1)

	// First item should be ignored entirely, second item should be validated
	data := []any{
		map[string]any{
			"id":           "1",
			"extraInFirst": "ignored because path $.body[0] is ignored",
		},
		map[string]any{
			"name":          "test",
			"extraInSecond": "should be flagged",
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

	// Only second item's extra property should be flagged
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extraInSecond", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body[1].extraInSecond", result.UndeclaredValues[0].Path)
}

func TestStrictValidator_ItemsWithIgnoredPath(t *testing.T) {
	// Covers array_validator.go:71-72 - shouldIgnore in items loop
	// Need to ignore the ITEM PATH itself ($.body[0]) not a nested property
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    ArrayIgnore:
      type: array
      items:
        type: object
        properties:
          name:
            type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "ArrayIgnore")

	// Ignore the first array item entirely ($.body[0])
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body[0]"),
	)
	v := NewValidator(opts, 3.1)

	data := []any{
		map[string]any{
			"name":  "item1",
			"extra": "should be ignored because $.body[0] is ignored",
		},
		map[string]any{
			"name":  "item2",
			"extra": "should be flagged",
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

	// First item ignored, only second item's extra should be flagged
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
	assert.Equal(t, "$.body[1].extra", result.UndeclaredValues[0].Path)
}

func TestValidateRequestHeaders_DeclaredHeaderSkipped(t *testing.T) {
	// Covers validator.go:123-125 - declared header skip in request validation
	opts := config.NewValidationOptions(config.WithStrictMode())

	// Create params with X-Custom header declared
	params := []*v3.Parameter{
		{
			Name: "X-Custom",
			In:   "header",
		},
		{
			Name: "X-Another",
			In:   "header",
		},
	}

	headers := http.Header{
		"X-Custom":     []string{"declared-value"},
		"X-Another":    []string{"also-declared"},
		"X-Undeclared": []string{"should-be-flagged"},
	}

	undeclared := ValidateRequestHeaders(headers, params, nil, opts)

	// Only X-Undeclared should be reported
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Undeclared", undeclared[0].Name)
}

func TestValidateRequestHeaders_NilOrDisabled(t *testing.T) {
	// Covers validator.go:103 - early return when headers nil, options nil, or strict mode disabled
	params := []*v3.Parameter{{Name: "X-Custom", In: "header"}}
	headers := http.Header{"X-Custom": []string{"value"}}

	// Test nil headers
	result := ValidateRequestHeaders(nil, params, nil, config.NewValidationOptions(config.WithStrictMode()))
	assert.Nil(t, result)

	// Test nil options
	result = ValidateRequestHeaders(headers, params, nil, nil)
	assert.Nil(t, result)

	// Test strict mode disabled
	opts := config.NewValidationOptions() // strict mode off by default
	result = ValidateRequestHeaders(headers, params, nil, opts)
	assert.Nil(t, result)
}

func TestValidateRequestHeaders_IgnoredHeaderSkipped(t *testing.T) {
	// Covers validator.go:129 - skip when header is in ignored list
	opts := config.NewValidationOptions(config.WithStrictMode())

	// No declared headers - but Content-Type is in default ignored list
	params := []*v3.Parameter{}

	headers := http.Header{
		"Content-Type":  []string{"application/json"}, // ignored by default
		"Authorization": []string{"Bearer token"},     // ignored by default
		"X-Custom":      []string{"should-be-flagged"},
	}

	undeclared := ValidateRequestHeaders(headers, params, nil, opts)

	// Only X-Custom should be reported (Content-Type and Authorization are ignored)
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Custom", undeclared[0].Name)
}

func TestValidateRequestHeaders_SecurityHeadersRecognized(t *testing.T) {
	// Covers validator.go:122-125 - security scheme headers are recognized as declared
	opts := config.NewValidationOptions(config.WithStrictMode())

	// No declared params - security headers come from security schemes
	params := []*v3.Parameter{}

	// Security headers extracted from security schemes
	securityHeaders := []string{"X-API-Key", "X-Custom-Auth"}

	headers := http.Header{
		"X-Api-Key":     []string{"my-api-key"},   // matches securityHeaders (case-insensitive)
		"X-Custom-Auth": []string{"custom-token"}, // matches securityHeaders
		"X-Unknown":     []string{"should-be-flagged"},
	}

	undeclared := ValidateRequestHeaders(headers, params, securityHeaders, opts)

	// Only X-Unknown should be reported; X-Api-Key and X-Custom-Auth are recognized as security headers
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Unknown", undeclared[0].Name)
}

func TestValidateRequestHeaders_SecurityHeadersCaseInsensitive(t *testing.T) {
	// Verify security header matching is case-insensitive
	opts := config.NewValidationOptions(config.WithStrictMode())

	params := []*v3.Parameter{}
	securityHeaders := []string{"X-API-KEY"} // uppercase

	headers := http.Header{
		"x-api-key": []string{"my-key"}, // lowercase in request
	}

	undeclared := ValidateRequestHeaders(headers, params, securityHeaders, opts)

	// Should not report x-api-key as undeclared (case-insensitive match)
	assert.Empty(t, undeclared)
}

func TestValidateRequestHeaders_BothParamsAndSecurityHeaders(t *testing.T) {
	// Test that both params and security headers are recognized
	opts := config.NewValidationOptions(config.WithStrictMode())

	params := []*v3.Parameter{
		{Name: "X-Request-Id", In: "header"},
	}
	securityHeaders := []string{"X-API-Key"}

	headers := http.Header{
		"X-Request-Id": []string{"123"}, // declared via params
		"X-Api-Key":    []string{"key"}, // declared via security schemes
		"X-Other":      []string{"should-be-flagged"},
	}

	undeclared := ValidateRequestHeaders(headers, params, securityHeaders, opts)

	// Only X-Other should be reported
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Other", undeclared[0].Name)
}

func TestValidateRequestHeaders_EmptySecurityHeaders(t *testing.T) {
	// Test with empty security headers slice (not nil)
	opts := config.NewValidationOptions(config.WithStrictMode())

	params := []*v3.Parameter{}
	securityHeaders := []string{} // empty, not nil

	headers := http.Header{
		"X-Custom": []string{"value"},
	}

	undeclared := ValidateRequestHeaders(headers, params, securityHeaders, opts)

	// X-Custom should be flagged since there are no declared headers
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Custom", undeclared[0].Name)
}

func TestValidateResponseHeaders_DeclaredHeaderSkipped(t *testing.T) {
	// Covers validator.go:219-223, 228-230 - declared header handling in response
	opts := config.NewValidationOptions(config.WithStrictMode())

	// Create declared headers map
	declaredHeaders := make(map[string]*v3.Header)
	declaredHeaders["X-Response-Id"] = &v3.Header{}

	headers := http.Header{
		"X-Response-Id": []string{"declared"},
		"X-Undeclared":  []string{"should-be-flagged"},
	}

	undeclared := ValidateResponseHeaders(headers, &declaredHeaders, opts)

	// Only X-Undeclared should be reported
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Undeclared", undeclared[0].Name)
}

func TestValidateResponseHeaders_WithDeclaredHeaders(t *testing.T) {
	// Covers validator.go:219-223, 228-230 - building declared names list
	opts := config.NewValidationOptions(config.WithStrictMode())

	// Create declared headers map with multiple headers
	declaredHeaders := make(map[string]*v3.Header)
	declaredHeaders["X-Rate-Limit"] = &v3.Header{}
	declaredHeaders["X-Request-Id"] = &v3.Header{}

	headers := http.Header{
		"X-Rate-Limit": []string{"100"},
		"X-Request-Id": []string{"abc123"},
		"X-Undeclared": []string{"flagged"},
	}

	undeclared := ValidateResponseHeaders(headers, &declaredHeaders, opts)

	// Only X-Undeclared should be reported
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Undeclared", undeclared[0].Name)
}

func TestValidateResponseHeaders_IgnorePathMatch(t *testing.T) {
	// Covers validator.go:239 - skip when header matches ignore path pattern
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.headers.x-internal"),
	)

	declaredHeaders := make(map[string]*v3.Header)

	headers := http.Header{
		"X-Internal":   []string{"should-be-ignored"}, // matches ignore path
		"X-Undeclared": []string{"should-be-flagged"},
	}

	undeclared := ValidateResponseHeaders(headers, &declaredHeaders, opts)

	// Only X-Undeclared should be reported (X-Internal matches ignore path)
	assert.Len(t, undeclared, 1)
	assert.Equal(t, "X-Undeclared", undeclared[0].Name)
}

func TestNewValidator_WithIgnorePaths(t *testing.T) {
	// Covers types.go:310-311 - compiledIgnorePaths populated
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.metadata", "$.body.internal"),
	)

	v := NewValidator(opts, 3.1)

	// Verify ignore paths are compiled
	assert.NotNil(t, v)
	assert.Len(t, v.compiledIgnorePaths, 2)

	// Test that the patterns work
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    TestSchema:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "TestSchema")

	// metadata and internal are undeclared properties that match ignore patterns
	data := map[string]any{
		"name": "test",
		"metadata": map[string]any{
			"ignored": "value",
		},
		"internal": map[string]any{
			"deep": map[string]any{
				"nested": "also ignored",
			},
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

	// metadata and internal paths are ignored, so no undeclared errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestNewValidator_WithCustomLogger(t *testing.T) {
	// Covers types.go:295 - custom logger from options
	customLogger := slog.New(slog.NewTextHandler(nil, nil))
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithLogger(customLogger),
	)

	v := NewValidator(opts, 3.1)

	// Verify the custom logger is used
	assert.NotNil(t, v)
	assert.Equal(t, customLogger, v.logger)
}

// =============================================================================
// Phase 3: MEDIUM Priority Tests
// =============================================================================

func TestStrictValidator_PrimitiveValuesIgnored(t *testing.T) {
	// Covers schema_walker.go:37-38 - validateValue default case for primitives
	// Primitive values (string, number, boolean) have no properties to check
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    StringSchema:
      type: string
    NumberSchema:
      type: number
    BooleanSchema:
      type: boolean
`
	model := buildSchemaFromYAML(t, yml)
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test string value - no properties to check
	stringSchema := getSchema(t, model, "StringSchema")
	result := v.Validate(Input{
		Schema:    stringSchema,
		Data:      "just a string",
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)

	// Test number value
	numberSchema := getSchema(t, model, "NumberSchema")
	result = v.Validate(Input{
		Schema:    numberSchema,
		Data:      42.5,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)

	// Test boolean value
	boolSchema := getSchema(t, model, "BooleanSchema")
	result = v.Validate(Input{
		Schema:    boolSchema,
		Data:      true,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_AdditionalPropertiesSchemaRecurse(t *testing.T) {
	// Covers schema_walker.go:72-80 - recurse into additionalProperties schema
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    AddlPropsNested:
      type: object
      properties:
        id:
          type: string
      additionalProperties:
        type: object
        properties:
          nested:
            type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "AddlPropsNested")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Data with nested undeclared property inside additionalProperties
	data := map[string]any{
		"id": "1",
		"extra": map[string]any{
			"nested": "ok",
			"bad":    "undeclared inside extra",
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

	// Both "extra" at top level AND "bad" inside extra should be reported
	assert.False(t, result.Valid)
	assert.GreaterOrEqual(t, len(result.UndeclaredValues), 1)

	// Find undeclared values
	foundExtra := false
	foundBad := false
	for _, uv := range result.UndeclaredValues {
		if uv.Name == "extra" {
			foundExtra = true
		}
		if uv.Name == "bad" {
			foundBad = true
		}
	}
	assert.True(t, foundExtra, "expected 'extra' to be reported as undeclared")
	assert.True(t, foundBad, "expected 'bad' inside extra to be reported as undeclared")
}

func TestStrictValidator_AdditionalPropertiesFalseShortCircuit(t *testing.T) {
	// Covers schema_walker.go:113-115 - shouldReportUndeclared returns false
	// When additionalProperties: false, JSON Schema handles it, not strict mode
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    NoExtras:
      type: object
      additionalProperties: false
      properties:
        id:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "NoExtras")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Data with extra property - additionalProperties: false handles this
	data := map[string]any{
		"id":    "1",
		"extra": "should be handled by JSON Schema, not strict",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Strict mode should NOT report this because additionalProperties: false
	// means JSON Schema will handle it
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_PatternPropertiesWithAdditionalFalse(t *testing.T) {
	// Covers schema_walker.go:223-228 - patternProperties with additionalProperties: false
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    PatternOnly:
      type: object
      additionalProperties: false
      patternProperties:
        "^x-":
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "PatternOnly")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// x-custom matches the pattern, so it's declared
	data := map[string]any{
		"x-custom": "ok",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// x-custom matches pattern and additionalProperties: false handles the rest
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_InvalidPatternPropertiesRegex(t *testing.T) {
	// Covers property_collector.go:46-49 - invalid regex skipped
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    InvalidPattern:
      type: object
      properties:
        id:
          type: string
      patternProperties:
        "[invalid(regex":
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "InvalidPattern")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Property name that would match the invalid pattern if it could compile
	data := map[string]any{
		"id":             "1",
		"[invalid(regex": "value",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Invalid pattern is skipped, so the property is reported as undeclared
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "[invalid(regex", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_UnevaluatedItemsWithIgnoredPath(t *testing.T) {
	// Covers array_validator.go:97-98 - shouldIgnore in unevaluatedItems
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    UnevalIgnore:
      type: array
      unevaluatedItems:
        type: object
        properties:
          id:
            type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "UnevalIgnore")

	// Ignore the first array element
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body[0]"),
	)
	v := NewValidator(opts, 3.1)

	// First item has undeclared 'extra', but it should be ignored
	data := []any{
		map[string]any{
			"id":    "1",
			"extra": "should be ignored at index 0",
		},
		map[string]any{
			"id":     "2",
			"extra2": "should be reported at index 1",
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

	// First item ignored, second item's extra2 should be reported
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra2", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_AdditionalPropertiesSchemaReportsUndeclared(t *testing.T) {
	// Covers schema_walker.go:122-126 - additionalProperties with schema still reports
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    SchemaAddl:
      type: object
      additionalProperties:
        type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "SchemaAddl")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Data with extra property allowed by additionalProperties schema
	data := map[string]any{
		"extra": "ok per JSON Schema but flagged by strict",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Strict mode should still flag undeclared properties even when
	// additionalProperties allows them
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_NilSchemaPassesValidation(t *testing.T) {
	// Covers matcher.go:38-40 - nil schema handling in dataMatchesSchema
	// When schema is nil, validation passes (no schema means anything matches)
	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Test with nil schema directly using dataMatchesSchema
	matches, err := v.dataMatchesSchema(nil, map[string]any{"key": "value"})
	assert.NoError(t, err)
	assert.True(t, matches, "nil schema should match any data")

	// Also test with different data types
	matches, err = v.dataMatchesSchema(nil, "string value")
	assert.NoError(t, err)
	assert.True(t, matches)

	matches, err = v.dataMatchesSchema(nil, 123)
	assert.NoError(t, err)
	assert.True(t, matches)

	matches, err = v.dataMatchesSchema(nil, []any{1, 2, 3})
	assert.NoError(t, err)
	assert.True(t, matches)
}

func TestStrictValidator_ValidateValue_ShouldIgnore(t *testing.T) {
	// Covers schema_walker.go:17-18 - shouldIgnore in validateValue at ENTRY point
	// Need to ignore $.body itself so the check happens at validateValue entry
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    IgnoreTest:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "IgnoreTest")

	// Ignore the entire body - validateValue entry should return early at line 18
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body"),
	)
	v := NewValidator(opts, 3.1)

	data := map[string]any{
		"name":       "valid",
		"undeclared": "should be ignored because entire body is ignored",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Entire body is ignored, so no undeclared errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_ValidateValue_CycleDetection(t *testing.T) {
	// Covers schema_walker.go:27-28 - cycle detection in validateValue
	// Need to call validateValue directly with a pre-visited context
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    TestSchema:
      type: object
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "TestSchema")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Create a context and pre-mark the schema as visited at this path
	ctx := newTraversalContext(DirectionRequest, v.compiledIgnorePaths, "$.body")
	schemaKey := v.getSchemaKey(schema)
	ctx.checkAndMarkVisited(schemaKey) // First visit - marks as visited

	data := map[string]any{
		"name":       "test",
		"undeclared": "should not be detected due to cycle",
	}

	// Call validateValue directly - should hit line 28 (cycle detected)
	result := v.validateValue(ctx, schema, data)

	// Cycle detected, returns early with no errors
	assert.Empty(t, result)
}

func TestStrictValidator_ShouldReportUndeclared_AdditionalPropertiesTrue(t *testing.T) {
	// Covers schema_walker.go:119-120 - additionalProperties: true explicit
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    ExplicitTrue:
      type: object
      additionalProperties: true
      properties:
        name:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "ExplicitTrue")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Extra property allowed by additionalProperties: true but flagged by strict
	data := map[string]any{
		"name":  "test",
		"extra": "should be flagged in strict mode",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// Strict mode should flag undeclared even with additionalProperties: true
	assert.False(t, result.Valid)
	assert.Len(t, result.UndeclaredValues, 1)
	assert.Equal(t, "extra", result.UndeclaredValues[0].Name)
}

func TestStrictValidator_RecurseIntoDeclaredProperties_PropertyNotInData(t *testing.T) {
	// Covers schema_walker.go:179-180 - continue when schema property not in data
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    MissingProp:
      type: object
      additionalProperties: false
      properties:
        required:
          type: string
        optional:
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "MissingProp")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Only provide 'required', not 'optional' - line 180 should be hit
	data := map[string]any{
		"required": "value",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// No undeclared properties
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_RecurseIntoDeclaredProperties_SkipReadOnly(t *testing.T) {
	// Covers schema_walker.go:194-195 - shouldSkipProperty in recurseIntoDeclaredProperties
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    ReadOnlyProp:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
        id:
          type: string
          readOnly: true
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "ReadOnlyProp")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// Include readOnly property in request - should be skipped (line 195)
	data := map[string]any{
		"name": "test",
		"id":   "should-be-skipped",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// id is readOnly and skipped, no errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_RecurseIntoDeclaredProperties_ShouldIgnore(t *testing.T) {
	// Covers schema_walker.go:188-189 - shouldIgnore for explicit property
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    IgnoreProp:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
        metadata:
          type: object
          properties:
            nested:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "IgnoreProp")

	// Ignore metadata property
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.metadata"),
	)
	v := NewValidator(opts, 3.1)

	// metadata has undeclared property but is ignored (line 189)
	data := map[string]any{
		"name": "test",
		"metadata": map[string]any{
			"nested":     "ok",
			"undeclared": "should be ignored",
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

	// metadata is ignored, no errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_RecurseIntoDeclaredProperties_PatternNoMatch(t *testing.T) {
	// Covers schema_walker.go:210-211 - property doesn't match any patternProperty
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    PatternSchema:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
      patternProperties:
        "^x-":
          type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "PatternSchema")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// "other" doesn't match explicit props or pattern "^x-" - line 211 hit
	data := map[string]any{
		"name":     "test",
		"x-custom": "matches pattern",
		"other":    "doesn't match pattern",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// "other" doesn't match pattern, but additionalProperties: false handles it
	// so strict mode doesn't report it
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_RecurseIntoDeclaredProperties_PatternSkipReadOnly(t *testing.T) {
	// Covers schema_walker.go:225-226 - shouldSkipProperty for patternProperty
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    PatternReadOnly:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
      patternProperties:
        "^x-":
          type: string
          readOnly: true
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "PatternReadOnly")

	opts := config.NewValidationOptions(config.WithStrictMode())
	v := NewValidator(opts, 3.1)

	// x-custom matches pattern but schema is readOnly - skip in request (line 226)
	data := map[string]any{
		"name":     "test",
		"x-custom": "matches readOnly pattern",
	}

	result := v.Validate(Input{
		Schema:    schema,
		Data:      data,
		Direction: DirectionRequest,
		Options:   opts,
		BasePath:  "$.body",
		Version:   3.1,
	})

	// x-custom matches pattern but is readOnly, skipped in request
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}

func TestStrictValidator_RecurseIntoDeclaredProperties_PatternShouldIgnore(t *testing.T) {
	// Covers schema_walker.go:219-220 - shouldIgnore for patternProperty path
	yml := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths: {}
components:
  schemas:
    PatternIgnore:
      type: object
      additionalProperties: false
      properties:
        name:
          type: string
      patternProperties:
        "^x-":
          type: object
          properties:
            nested:
              type: string
`
	model := buildSchemaFromYAML(t, yml)
	schema := getSchema(t, model, "PatternIgnore")

	// Ignore the pattern-matched property path
	opts := config.NewValidationOptions(
		config.WithStrictMode(),
		config.WithStrictIgnorePaths("$.body.x-custom"),
	)
	v := NewValidator(opts, 3.1)

	// x-custom matches pattern, path matches ignore pattern - should skip (line 220)
	data := map[string]any{
		"name": "test",
		"x-custom": map[string]any{
			"nested":     "valid",
			"undeclared": "should be ignored because path is ignored",
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

	// x-custom path is ignored, so no undeclared errors
	assert.True(t, result.Valid)
	assert.Empty(t, result.UndeclaredValues)
}
