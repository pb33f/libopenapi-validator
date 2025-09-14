package helpers

import (
	"encoding/json"
	"fmt"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
)

// A few simple JSON Schemas
const stringSchema = `{
  "type": "string",
  "format": "date",
  "minLength": 10
}`

const objectSchema = `{
  "type": "object",
  "title" : "Fish",
  "properties" : {
     "name" : {
	    "type": "string",
        "description": "The given name of the fish"
     },
	"name" : {
	    "type": "string",
		"format": "capital",
        "description": "The given name of the fish"
     },
     "species" : {
		"type" : "string",
		"enum" : [ "OTHER", "GUPPY", "PIKE", "BASS" ]
     }
  }
}`

func Test_SchemaWithNilOptions(t *testing.T) {
	jsch, err := NewCompiledSchema("test", []byte(stringSchema), nil)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_SchemaWithDefaultOptions(t *testing.T) {
	valOptions := config.NewValidationOptions()
	jsch, err := NewCompiledSchema("test", []byte(stringSchema), valOptions)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_SchemaWithOptions(t *testing.T) {
	valOptions := config.NewValidationOptions(
		config.WithFormatAssertions(),
		config.WithContentAssertions(),
		config.WithCustomFormat("capital", func(v any) error {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("expected string")
			}

			if s == "" {
				return nil
			}

			r := []rune(s)[0]

			if !unicode.IsUpper(r) {
				return fmt.Errorf("expected first latter to be uppercase")
			}

			return nil
		}),
	)

	jsch, err := NewCompiledSchema("test", []byte(stringSchema), valOptions)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_ObjectSchema(t *testing.T) {
	valOptions := config.NewValidationOptions()
	jsch, err := NewCompiledSchema("test", []byte(objectSchema), valOptions)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_ValidJSONSchemaWithInvalidContent(t *testing.T) {
	// An example of a dubious JSON Schema
	const badSchema = `{
  "type": "you-dont-know-me",
  "format": "date",
  "minLength": 10
}`

	jsch, err := NewCompiledSchema("test", []byte(badSchema), nil)

	assert.Error(t, err, "Expected an error to be thrown")
	assert.Nil(t, jsch, "invalid schema compiled!")
}

func Test_MalformedSONSchema(t *testing.T) {
	// An example of a JSON schema with malformed JSON
	const badSchema = `{
  "type": "you-dont-know-me",
  "format": "date"
  "minLength": 10
}`

	jsch, err := NewCompiledSchema("test", []byte(badSchema), nil)

	assert.Error(t, err, "Expected an error to be thrown")
	assert.Nil(t, jsch, "invalid schema compiled!")
}

func Test_ValidJSONSchemaWithIncompleteContent(t *testing.T) {
	// An example of a dJSON schema with references to non-existent stuff
	const incompleteSchema = `{
  "type": "object",
  "title" : "unresolvable",
  "properties" : {
     "name" : {
	    "type": "string",
     },
     "species" : {
      "$ref": "#/$defs/speciesEnum"
     }
  }
}`

	jsch, err := NewCompiledSchema("test", []byte(incompleteSchema), nil)

	assert.Error(t, err, "Expected an error to be thrown")
	assert.Nil(t, jsch, "invalid schema compiled!")
}

// Additional comprehensive tests for version-aware schema compilation

func TestNewCompiledSchemaWithVersion_OpenAPIMode_Version30(t *testing.T) {
	schemaJSON := `{
		"type": "string",
		"nullable": true
	}`

	options := config.NewValidationOptions(
		config.WithOpenAPIMode(),
	)

	// Test version 3.0 (< 3.05)
	jsch, err := NewCompiledSchemaWithVersion("test", []byte(schemaJSON), options, 3.0)
	require.NoError(t, err, "Should compile OpenAPI 3.0 schema with nullable")
	require.NotNil(t, jsch, "Should return compiled schema")
}

func TestNewCompiledSchemaWithVersion_OpenAPIMode_Version31(t *testing.T) {
	schemaJSON := `{
		"type": "string"
	}`

	options := config.NewValidationOptions(
		config.WithOpenAPIMode(),
	)

	// Test version 3.1 (>= 3.05)
	jsch, err := NewCompiledSchemaWithVersion("test", []byte(schemaJSON), options, 3.1)
	require.NoError(t, err, "Should compile OpenAPI 3.1 schema")
	require.NotNil(t, jsch, "Should return compiled schema")
}

func TestNewCompiledSchemaWithVersion_OpenAPIMode_Version31_NullableRejected(t *testing.T) {
	schemaJSON := `{
		"type": "string",
		"nullable": true
	}`

	options := config.NewValidationOptions(
		config.WithOpenAPIMode(),
	)

	// Test version 3.1 (>= 3.05) with nullable should fail
	jsch, err := NewCompiledSchemaWithVersion("test", []byte(schemaJSON), options, 3.1)
	assert.Error(t, err, "Should fail for nullable in OpenAPI 3.1")
	assert.Nil(t, jsch, "Should not return compiled schema")
	assert.Contains(t, err.Error(), "The `nullable` keyword is not supported in OpenAPI 3.1+")
}

func TestNewCompiledSchemaWithVersion_OpenAPIMode_ScalarCoercion(t *testing.T) {
	schemaJSON := `{
		"type": "boolean"
	}`

	options := config.NewValidationOptions(
		config.WithOpenAPIMode(),
		config.WithScalarCoercion(),
	)

	// Test with scalar coercion enabled
	jsch, err := NewCompiledSchemaWithVersion("test", []byte(schemaJSON), options, 3.0)
	require.NoError(t, err, "Should compile with scalar coercion")
	require.NotNil(t, jsch, "Should return compiled schema")

	// Test that coercion works
	err = jsch.Validate("true")
	assert.NoError(t, err, "Should allow string 'true' for boolean with coercion")

	err = jsch.Validate("invalid")
	assert.Error(t, err, "Should reject invalid boolean string")
}

func TestNewCompiledSchemaWithVersion_OpenAPIMode_NoScalarCoercion(t *testing.T) {
	schemaJSON := `{
		"type": "boolean"
	}`

	options := config.NewValidationOptions(
		config.WithOpenAPIMode(),
	)

	// Test with scalar coercion disabled (default)
	jsch, err := NewCompiledSchemaWithVersion("test", []byte(schemaJSON), options, 3.0)
	require.NoError(t, err, "Should compile without scalar coercion")
	require.NotNil(t, jsch, "Should return compiled schema")

	// Test that coercion doesn't work
	err = jsch.Validate("true")
	assert.Error(t, err, "Should reject string 'true' for boolean without coercion")

	err = jsch.Validate(true)
	assert.NoError(t, err, "Should accept actual boolean value")
}

func TestNewCompiledSchemaWithVersion_NonOpenAPIMode(t *testing.T) {
	schemaJSON := `{
		"type": "string",
		"nullable": true
	}`

	options := config.NewValidationOptions()
	// OpenAPIMode is false by default

	// Test that OpenAPI keywords are ignored when not in OpenAPI mode
	jsch, err := NewCompiledSchemaWithVersion("test", []byte(schemaJSON), options, 3.0)
	require.NoError(t, err, "Should compile in non-OpenAPI mode")
	require.NotNil(t, jsch, "Should return compiled schema")

	// String values should pass when OpenAPI mode is disabled
	err = jsch.Validate("test")
	assert.NoError(t, err, "Should accept string values")

	// When OpenAPI mode is disabled, nullable is ignored by JSON Schema
	// The behavior with null depends on the JSON Schema version and mode
}

func TestTransformOpenAPI30Schema_ValidJSON(t *testing.T) {
	input := []byte(`{
		"type": "string",
		"nullable": true
	}`)

	result := transformOpenAPI30Schema(input)

	var schema map[string]interface{}
	err := json.Unmarshal(result, &schema)
	require.NoError(t, err, "Result should be valid JSON")

	// Check that nullable was transformed
	schemaType, ok := schema["type"]
	assert.True(t, ok, "Should have type field")

	typeArray, ok := schemaType.([]interface{})
	assert.True(t, ok, "Type should be an array")
	assert.Contains(t, typeArray, "string")
	assert.Contains(t, typeArray, "null")

	_, hasNullable := schema["nullable"]
	assert.False(t, hasNullable, "Should not have nullable field after transformation")
}

func TestTransformOpenAPI30Schema_InvalidJSON(t *testing.T) {
	input := []byte(`{invalid json}`)

	result := transformOpenAPI30Schema(input)

	// Should return original when invalid
	assert.Equal(t, input, result)
}

func TestTransformNullableInSchema_MapWithNullableTrue(t *testing.T) {
	schema := map[string]interface{}{
		"type":     "string",
		"nullable": true,
	}

	result := transformNullableInSchema(schema)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)

	schemaType, ok := resultMap["type"]
	require.True(t, ok)

	typeArray, ok := schemaType.([]interface{})
	require.True(t, ok)
	assert.Contains(t, typeArray, "string")
	assert.Contains(t, typeArray, "null")

	_, hasNullable := resultMap["nullable"]
	assert.False(t, hasNullable)
}

func TestTransformNullableInSchema_MapWithNullableFalse(t *testing.T) {
	schema := map[string]interface{}{
		"type":     "string",
		"nullable": false,
	}

	result := transformNullableInSchema(schema)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)

	// nullable: false should just remove nullable, keep type as is
	schemaType, ok := resultMap["type"]
	require.True(t, ok)
	assert.Equal(t, "string", schemaType)

	_, hasNullable := resultMap["nullable"]
	assert.False(t, hasNullable)
}

func TestTransformNullableInSchema_Array(t *testing.T) {
	schema := []interface{}{
		map[string]interface{}{
			"type":     "string",
			"nullable": true,
		},
		"other-item",
	}

	result := transformNullableInSchema(schema)

	resultArray, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, resultArray, 2)

	firstItem, ok := resultArray[0].(map[string]interface{})
	require.True(t, ok)

	schemaType := firstItem["type"].([]interface{})
	assert.Contains(t, schemaType, "string")
	assert.Contains(t, schemaType, "null")

	_, hasNullable := firstItem["nullable"]
	assert.False(t, hasNullable)
}

func TestTransformNullableInSchema_OtherTypes(t *testing.T) {
	stringSchema := "string-value"
	result := transformNullableInSchema(stringSchema)
	assert.Equal(t, stringSchema, result)

	numberSchema := 123
	result = transformNullableInSchema(numberSchema)
	assert.Equal(t, numberSchema, result)

	var nilSchema interface{} = nil
	result = transformNullableInSchema(nilSchema)
	assert.Equal(t, nilSchema, result)
}

func TestTransformNullableSchema_ArrayType(t *testing.T) {
	schema := map[string]interface{}{
		"type":     []interface{}{"string", "number"},
		"nullable": true,
	}

	result := transformNullableSchema(schema)

	schemaType, ok := result["type"]
	require.True(t, ok)

	typeArray, ok := schemaType.([]interface{})
	require.True(t, ok)
	assert.Contains(t, typeArray, "string")
	assert.Contains(t, typeArray, "number")
	assert.Contains(t, typeArray, "null")

	_, hasNullable := result["nullable"]
	assert.False(t, hasNullable)
}

func TestTransformNullableSchema_ArrayTypeWithNull(t *testing.T) {
	schema := map[string]interface{}{
		"type":     []interface{}{"string", "null"},
		"nullable": true,
	}

	result := transformNullableSchema(schema)

	schemaType, ok := result["type"]
	require.True(t, ok)

	typeArray, ok := schemaType.([]interface{})
	require.True(t, ok)
	assert.Contains(t, typeArray, "string")
	assert.Contains(t, typeArray, "null")
	// Should not have duplicate "null"
	nullCount := 0
	for _, item := range typeArray {
		if item == "null" {
			nullCount++
		}
	}
	assert.Equal(t, 1, nullCount)

	_, hasNullable := result["nullable"]
	assert.False(t, hasNullable)
}

func TestTransformSchemaForCoercion_ValidJSON(t *testing.T) {
	input := []byte(`{
		"type": "boolean"
	}`)

	result := transformSchemaForCoercion(input)

	var schema map[string]interface{}
	err := json.Unmarshal(result, &schema)
	require.NoError(t, err, "Result should be valid JSON")

	// Check that type was transformed to include string
	schemaType, ok := schema["type"]
	assert.True(t, ok, "Should have type field")

	typeArray, ok := schemaType.([]interface{})
	assert.True(t, ok, "Type should be an array")
	assert.Contains(t, typeArray, "boolean")
	assert.Contains(t, typeArray, "string")
}

func TestTransformSchemaForCoercion_InvalidJSON(t *testing.T) {
	input := []byte(`{invalid json}`)

	result := transformSchemaForCoercion(input)

	// Should return original when invalid
	assert.Equal(t, input, result)
}

func TestTransformOpenAPI30Schema_MarshalError(t *testing.T) {
	// Create a transformation that could potentially cause marshal issues
	// This is hard to test because Go's json.Marshal rarely fails
	// The error path exists for defensive programming
	input := []byte(`{
		"type": "string",
		"nullable": true
	}`)

	result := transformOpenAPI30Schema(input)

	// Should return valid transformed JSON even if marshal could theoretically fail
	var schema map[string]interface{}
	err := json.Unmarshal(result, &schema)
	assert.NoError(t, err)
}

func TestTransformSchemaForCoercion_MarshalError(t *testing.T) {
	// Create a transformation that could potentially cause marshal issues
	// This is hard to test because Go's json.Marshal rarely fails
	// The error path exists for defensive programming
	input := []byte(`{
		"type": "boolean"
	}`)

	result := transformSchemaForCoercion(input)

	// Should return valid transformed JSON even if marshal could theoretically fail
	var schema map[string]interface{}
	err := json.Unmarshal(result, &schema)
	assert.NoError(t, err)
}

func TestTransformCoercionInSchema_Array(t *testing.T) {
	schema := []interface{}{
		map[string]interface{}{
			"type": "number",
		},
		"other-item",
	}

	result := transformCoercionInSchema(schema)

	resultArray, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, resultArray, 2)

	firstItem, ok := resultArray[0].(map[string]interface{})
	require.True(t, ok)

	schemaType := firstItem["type"].([]interface{})
	assert.Contains(t, schemaType, "number")
	assert.Contains(t, schemaType, "string")
}

func TestTransformCoercionInSchema_OtherTypes(t *testing.T) {
	stringSchema := "string-value"
	result := transformCoercionInSchema(stringSchema)
	assert.Equal(t, stringSchema, result)
}

func TestTransformTypeForCoercion_ArrayWithNonStringItems(t *testing.T) {
	input := []interface{}{"boolean", 123, "null"}

	result := transformTypeForCoercion(input)

	typeArray, ok := result.([]interface{})
	require.True(t, ok)
	assert.Contains(t, typeArray, "boolean")
	assert.Contains(t, typeArray, 123)
	assert.Contains(t, typeArray, "null")
	assert.Contains(t, typeArray, "string")
}

func TestTransformTypeForCoercion_OtherTypes(t *testing.T) {
	result := transformTypeForCoercion(123)
	assert.Equal(t, 123, result)

	result = transformTypeForCoercion(nil)
	assert.Equal(t, nil, result)

	result = transformTypeForCoercion(map[string]interface{}{})
	assert.Equal(t, map[string]interface{}{}, result)
}
