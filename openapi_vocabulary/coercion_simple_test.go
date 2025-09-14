// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package openapi_vocabulary

import (
	"fmt"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
)

func TestCoercion_Vocabulary_CompilationSuccess(t *testing.T) {
	// Test that coercion vocabulary compiles successfully for all scalar types
	testCases := []string{
		`{"type": "boolean"}`,
		`{"type": "number"}`,
		`{"type": "integer"}`,
		`{"type": ["boolean", "null"]}`,
		`{"type": "string"}`, // Should not get coercion extension
	}

	for i, schemaJSON := range testCases {
		t.Run(fmt.Sprintf("Schema_%d", i), func(t *testing.T) {
			schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
			assert.NoError(t, err)

			compiler := jsonschema.NewCompiler()
			compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, true))
			compiler.AssertVocabs()

			err = compiler.AddResource("test.json", schema)
			assert.NoError(t, err)

			// Should compile successfully
			compiledSchema, err := compiler.Compile("test.json")
			assert.NoError(t, err)
			assert.NotNil(t, compiledSchema)
		})
	}
}

func TestCoercion_Vocabulary_DisabledCompilation(t *testing.T) {
	// Test that vocabulary compiles successfully when coercion is disabled
	schemaJSON := `{"type": "boolean"}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, false)) // Disabled
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Should compile successfully even with coercion disabled
	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)
	assert.NotNil(t, compiledSchema)
}

// Additional comprehensive coercion tests

func TestCoercionExtension_Validate_CoercionDisabled(t *testing.T) {
	// Test with coercion disabled via vocabulary
	schemaJSON := `{"type": "boolean"}`
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, false))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	// With coercion disabled, string "true" should fail normal validation
	err = compiledSchema.Validate("true")
	assert.Error(t, err, "Should fail without coercion")

	// But actual boolean should pass
	err = compiledSchema.Validate(true)
	assert.NoError(t, err, "Should pass with actual boolean")
}

func TestCoercionExtension_Validate_NonStringValue(t *testing.T) {
	// Test that non-string values don't trigger coercion
	schemaJSON := `{"type": "boolean"}`
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, true))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	// Non-string values should be handled by normal JSON Schema validation
	err = compiledSchema.Validate(true)
	assert.NoError(t, err, "Boolean should pass")

	err = compiledSchema.Validate(123)
	assert.Error(t, err, "Number should fail for boolean type")

	err = compiledSchema.Validate(nil)
	assert.Error(t, err, "Null should fail for boolean type")
}

func TestCoercionExtension_Validate_BooleanCoercion_Valid(t *testing.T) {
	// Create a schema that allows both boolean and string types for coercion to work
	schemaJSON := `{"type": ["boolean", "string"]}`
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, true))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	testCases := []string{"true", "false"}

	for _, testCase := range testCases {
		err = compiledSchema.Validate(testCase)
		assert.NoError(t, err, "Should pass for valid boolean string: %s", testCase)
	}
}

func TestCoercionExtension_Validate_BooleanCoercion_Invalid(t *testing.T) {
	schemaJSON := `{"type": ["boolean", "string"]}`
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, true))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	testCases := []string{"yes", "no", "1", "0", "True", "FALSE", ""}

	for _, testCase := range testCases {
		err = compiledSchema.Validate(testCase)
		assert.Error(t, err, "Should fail for invalid boolean string: %s", testCase)
		assert.Contains(t, err.Error(), "cannot coerce", "Should contain coercion error message")
	}
}

func TestCoercionExtension_HasType_Methods(t *testing.T) {
	// Test hasType method with string type
	ext := &coercionExtension{
		schemaType: "boolean",
	}

	assert.True(t, ext.hasType("boolean"))
	assert.False(t, ext.hasType("number"))
	assert.False(t, ext.hasType("integer"))

	// Test hasType method with array type
	ext = &coercionExtension{
		schemaType: []any{"boolean", "null"},
	}

	assert.True(t, ext.hasType("boolean"))
	assert.True(t, ext.hasType("null"))
	assert.False(t, ext.hasType("number"))

	// Test hasType method with invalid array items
	ext = &coercionExtension{
		schemaType: []any{123, "boolean"},
	}

	assert.True(t, ext.hasType("boolean"))
	assert.False(t, ext.hasType("number"))

	// Test hasType method with non-string, non-array
	ext = &coercionExtension{
		schemaType: 123,
	}

	assert.False(t, ext.hasType("boolean"))
	assert.False(t, ext.hasType("number"))
}

func TestCoercionExtension_ValidationMethods(t *testing.T) {
	ext := &coercionExtension{}

	// Test boolean validation
	assert.True(t, ext.isValidBooleanString("true"))
	assert.True(t, ext.isValidBooleanString("false"))
	assert.False(t, ext.isValidBooleanString("True"))
	assert.False(t, ext.isValidBooleanString("FALSE"))
	assert.False(t, ext.isValidBooleanString("yes"))
	assert.False(t, ext.isValidBooleanString("no"))
	assert.False(t, ext.isValidBooleanString("1"))
	assert.False(t, ext.isValidBooleanString("0"))
	assert.False(t, ext.isValidBooleanString(""))

	// Test number validation
	validNumbers := []string{
		"123", "-123", "0", "-0",
		"123.45", "-123.45", "0.0",
		"1e5", "1E5", "1e+5", "1e-5",
		"1.23e10", "1.23E-10",
	}

	for _, num := range validNumbers {
		assert.True(t, ext.isValidNumberString(num), "Should be valid number: %s", num)
	}

	invalidNumbers := []string{
		"abc", "12.34.56", "1e", "e5",
		"1.23.45e10", "", "null", "true",
		"Infinity", "NaN", "+123",
	}

	for _, num := range invalidNumbers {
		assert.False(t, ext.isValidNumberString(num), "Should be invalid number: %s", num)
	}

	// Test integer validation
	validIntegers := []string{"123", "-123", "0"}

	for _, num := range validIntegers {
		assert.True(t, ext.isValidIntegerString(num), "Should be valid integer: %s", num)
	}

	invalidIntegers := []string{
		"123.45", "abc", "007", "1e5",
		"", "null", "true", "+123",
	}

	for _, num := range invalidIntegers {
		assert.False(t, ext.isValidIntegerString(num), "Should be invalid integer: %s", num)
	}
}

func TestCompileCoercion_CoercionDisabled(t *testing.T) {
	obj := map[string]any{
		"type": "boolean",
	}

	ext, err := CompileCoercion(nil, obj, false)
	assert.NoError(t, err)
	assert.Nil(t, ext)
}

func TestCompileCoercion_NoType(t *testing.T) {
	obj := map[string]any{
		"minLength": 1,
	}

	ext, err := CompileCoercion(nil, obj, true)
	assert.NoError(t, err)
	assert.Nil(t, ext)
}

func TestCompileCoercion_NonCoercibleType(t *testing.T) {
	obj := map[string]any{
		"type": "string",
	}

	ext, err := CompileCoercion(nil, obj, true)
	assert.NoError(t, err)
	assert.Nil(t, ext)
}

func TestCompileCoercion_CoercibleTypes(t *testing.T) {
	coercibleTypes := []string{"boolean", "number", "integer"}

	for _, schemaType := range coercibleTypes {
		obj := map[string]any{
			"type": schemaType,
		}

		ext, err := CompileCoercion(nil, obj, true)
		assert.NoError(t, err, "Should compile for type: %s", schemaType)
		assert.NotNil(t, ext, "Should return extension for type: %s", schemaType)

		coercionExt, ok := ext.(*coercionExtension)
		assert.True(t, ok, "Should be coercionExtension")
		assert.Equal(t, schemaType, coercionExt.schemaType)
		assert.True(t, coercionExt.allowCoercion)
	}
}

func TestIsCoercibleType_String(t *testing.T) {
	assert.True(t, IsCoercibleType("boolean"))
	assert.True(t, IsCoercibleType("number"))
	assert.True(t, IsCoercibleType("integer"))
	assert.False(t, IsCoercibleType("string"))
	assert.False(t, IsCoercibleType("object"))
	assert.False(t, IsCoercibleType("array"))
}

func TestCoercionExtension_ShouldCoerceToMethods(t *testing.T) {
	// Test shouldCoerceToNumber method
	ext := &coercionExtension{
		schemaType: "number",
	}
	assert.True(t, ext.shouldCoerceToNumber())
	assert.False(t, ext.shouldCoerceToBoolean())
	assert.False(t, ext.shouldCoerceToInteger())

	// Test shouldCoerceToInteger method
	ext = &coercionExtension{
		schemaType: "integer",
	}
	assert.True(t, ext.shouldCoerceToInteger())
	assert.False(t, ext.shouldCoerceToBoolean())
	assert.False(t, ext.shouldCoerceToNumber())

	// Test shouldCoerceToBoolean method
	ext = &coercionExtension{
		schemaType: "boolean",
	}
	assert.True(t, ext.shouldCoerceToBoolean())
	assert.False(t, ext.shouldCoerceToNumber())
	assert.False(t, ext.shouldCoerceToInteger())
}

func TestCoercionExtension_Validate_NumberCoercion(t *testing.T) {
	// Test number coercion path in Validate method
	schemaJSON := `{"type": ["number", "string"]}`
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, true))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	// Test valid number strings
	err = compiledSchema.Validate("123.45")
	assert.NoError(t, err)

	err = compiledSchema.Validate("-123")
	assert.NoError(t, err)

	// Test invalid number string
	err = compiledSchema.Validate("abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot coerce")
}

func TestCoercionExtension_Validate_IntegerCoercion(t *testing.T) {
	// Test integer coercion path in Validate method
	schemaJSON := `{"type": ["integer", "string"]}`
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, true))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	// Test valid integer strings
	err = compiledSchema.Validate("123")
	assert.NoError(t, err)

	err = compiledSchema.Validate("-123")
	assert.NoError(t, err)

	// Test invalid integer string
	err = compiledSchema.Validate("123.45")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot coerce")
}

func TestMetadataExtensions_Validate_Coverage(t *testing.T) {
	// Test example extension validate method (just for coverage)
	exampleExt := &exampleExtension{example: "test"}
	exampleExt.Validate(nil, "any-value") // No-op, just for coverage

	// Test deprecated extension validate method (just for coverage)
	deprecatedExt := &deprecatedExtension{deprecated: true}
	deprecatedExt.Validate(nil, "any-value") // No-op, just for coverage
}
