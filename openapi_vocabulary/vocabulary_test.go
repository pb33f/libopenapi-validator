// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package openapi_vocabulary

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
)

func TestNullableKeyword_Version30_Compilation(t *testing.T) {
	// Test that nullable: true compiles successfully in OpenAPI 3.0
	schemaJSON := `{
		"type": "string",
		"nullable": true
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Should compile successfully in 3.0 (validation behavior handled by transformation)
	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)
	assert.NotNil(t, compiledSchema)
}

func TestNullableKeyword_Version30_WithoutNullable(t *testing.T) {
	// Test that without nullable: true, null values are rejected
	schemaJSON := `{
		"type": "string"
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	// Test null value - should fail without nullable: true
	err = compiledSchema.Validate(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "got null, want string")

	// Test string value - should pass
	err = compiledSchema.Validate("hello")
	assert.NoError(t, err)
}

func TestNullableKeyword_Version31_Rejected(t *testing.T) {
	// Test that nullable keyword is rejected in OpenAPI 3.1
	schemaJSON := `{
		"type": "string",
		"nullable": true
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version31))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Compilation should fail due to nullable in 3.1
	_, err = compiler.Compile("test.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nullable keyword is not allowed in OpenAPI 3.1+")
}

func TestNullableKeyword_InvalidType(t *testing.T) {
	// Test that nullable must be a boolean
	schemaJSON := `{
		"type": "string",
		"nullable": "yes"
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Compilation should fail due to invalid nullable type
	_, err = compiler.Compile("test.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nullable must be a boolean value")
}

func TestDiscriminatorKeyword_ValidStructure(t *testing.T) {
	// Test discriminator with valid structure
	schemaJSON := `{
		"type": "object",
		"discriminator": {
			"propertyName": "type",
			"mapping": {
				"dog": "#/components/schemas/Dog",
				"cat": "#/components/schemas/Cat"
			}
		}
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	// Test object with discriminator property
	testData := map[string]any{
		"type": "dog",
		"name": "Buddy",
	}
	err = compiledSchema.Validate(testData)
	assert.NoError(t, err)
}

func TestDiscriminatorKeyword_MissingPropertyName(t *testing.T) {
	// Test discriminator without propertyName
	schemaJSON := `{
		"type": "object",
		"discriminator": {
			"mapping": {
				"dog": "#/components/schemas/Dog"
			}
		}
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Compilation should fail due to missing propertyName
	_, err = compiler.Compile("test.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discriminator must have a propertyName field")
}

func TestExampleKeyword_Valid(t *testing.T) {
	// Test example keyword with any value
	schemaJSON := `{
		"type": "string",
		"example": "hello world"
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)
	assert.NotNil(t, compiledSchema)
}

func TestDeprecatedKeyword_Valid(t *testing.T) {
	// Test deprecated keyword with boolean value
	schemaJSON := `{
		"type": "string",
		"deprecated": true
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)
	assert.NotNil(t, compiledSchema)
}

func TestDeprecatedKeyword_InvalidType(t *testing.T) {
	// Test deprecated keyword with non-boolean value
	schemaJSON := `{
		"type": "string",
		"deprecated": "yes"
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Compilation should fail due to invalid deprecated type
	_, err = compiler.Compile("test.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deprecated must be a boolean value")
}

func TestMultipleKeywords_Combined(t *testing.T) {
	// Test multiple OpenAPI keywords compile successfully in the same schema
	schemaJSON := `{
		"type": "string",
		"nullable": true,
		"example": "test value",
		"deprecated": false
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Should compile successfully in 3.0 (actual nullable behavior handled by transformation)
	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)
	assert.NotNil(t, compiledSchema)
}

func TestNoOpenAPIKeywords_NoExtension(t *testing.T) {
	// Test that schemas without OpenAPI keywords don't get extensions
	schemaJSON := `{
		"type": "string",
		"minLength": 1
	}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabulary(Version30))
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)

	// Should compile successfully even without OpenAPI keywords
	err = compiledSchema.Validate("hello")
	assert.NoError(t, err)

	err = compiledSchema.Validate("")
	assert.Error(t, err) // Should fail minLength validation
}
