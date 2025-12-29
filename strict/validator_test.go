// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package strict

import (
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
