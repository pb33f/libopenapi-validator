// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package openapi_vocabulary

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/message"
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
	assert.Contains(t, err.Error(), "The `nullable` keyword is not supported in OpenAPI 3.1+")
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

// Additional comprehensive tests

func TestNewOpenAPIVocabulary_Version30(t *testing.T) {
	vocab := NewOpenAPIVocabulary(Version30)
	assert.NotNil(t, vocab)
	assert.Equal(t, OpenAPIVocabularyURL, vocab.URL)
	assert.Nil(t, vocab.Schema)
	assert.NotNil(t, vocab.Compile)
}

func TestNewOpenAPIVocabulary_Version31(t *testing.T) {
	vocab := NewOpenAPIVocabulary(Version31)
	assert.NotNil(t, vocab)
	assert.Equal(t, OpenAPIVocabularyURL, vocab.URL)
	assert.Nil(t, vocab.Schema)
	assert.NotNil(t, vocab.Compile)
}

func TestNewOpenAPIVocabularyWithCoercion_Enabled(t *testing.T) {
	vocab := NewOpenAPIVocabularyWithCoercion(Version30, true)
	assert.NotNil(t, vocab)
	assert.Equal(t, OpenAPIVocabularyURL, vocab.URL)
	assert.Nil(t, vocab.Schema)
	assert.NotNil(t, vocab.Compile)
}

func TestNewOpenAPIVocabularyWithCoercion_Disabled(t *testing.T) {
	vocab := NewOpenAPIVocabularyWithCoercion(Version30, false)
	assert.NotNil(t, vocab)
	assert.Equal(t, OpenAPIVocabularyURL, vocab.URL)
	assert.Nil(t, vocab.Schema)
	assert.NotNil(t, vocab.Compile)
}

func TestCompileOpenAPIKeywords_EmptySchema(t *testing.T) {
	obj := map[string]any{}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, false)
	assert.NoError(t, err)
	assert.Nil(t, ext)
}

func TestCompileOpenAPIKeywords_NoOpenAPIKeywords(t *testing.T) {
	obj := map[string]any{
		"type":      "string",
		"minLength": 1,
	}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, false)
	assert.NoError(t, err)
	assert.Nil(t, ext)
}

func TestCompileOpenAPIKeywords_NullableOnly(t *testing.T) {
	obj := map[string]any{
		"type":     "string",
		"nullable": true,
	}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, false)
	assert.NoError(t, err)
	// Nullable compilation returns nil because the actual transformation is handled elsewhere
	// The vocabulary just validates that the keyword is not used in 3.1+
	assert.Nil(t, ext)
}

func TestCompileOpenAPIKeywords_DiscriminatorOnly(t *testing.T) {
	obj := map[string]any{
		"type": "object",
		"discriminator": map[string]any{
			"propertyName": "type",
		},
	}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, false)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestCompileOpenAPIKeywords_ExampleOnly(t *testing.T) {
	obj := map[string]any{
		"type":    "string",
		"example": "test",
	}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, false)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestCompileOpenAPIKeywords_DeprecatedOnly(t *testing.T) {
	obj := map[string]any{
		"type":       "string",
		"deprecated": true,
	}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, false)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestCompileOpenAPIKeywords_CoercionOnly(t *testing.T) {
	obj := map[string]any{
		"type": "boolean",
	}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, true)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestCompileOpenAPIKeywords_AllKeywordsCombined(t *testing.T) {
	obj := map[string]any{
		"type":     "string",
		"nullable": true,
		"discriminator": map[string]any{
			"propertyName": "type",
		},
		"example":    "test",
		"deprecated": false,
	}

	ext, err := compileOpenAPIKeywords(nil, obj, Version30, true)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

// Error type tests
func TestOpenAPIKeywordError_Error(t *testing.T) {
	err := &OpenAPIKeywordError{
		Keyword: "nullable",
		Message: "test message",
	}

	expected := "OpenAPI keyword 'nullable': test message"
	assert.Equal(t, expected, err.Error())
}

func TestDiscriminatorPropertyMissingError_KeywordPath(t *testing.T) {
	err := &DiscriminatorPropertyMissingError{
		PropertyName: "type",
	}

	path := err.KeywordPath()
	assert.Equal(t, []string{"discriminator"}, path)
}

func TestDiscriminatorPropertyMissingError_LocalizedString(t *testing.T) {
	err := &DiscriminatorPropertyMissingError{
		PropertyName: "type",
	}

	printer := message.NewPrinter(message.MatchLanguage("en"))
	result := err.LocalizedString(printer)
	expected := "discriminator property 'type' is missing"
	assert.Equal(t, expected, result)
}

func TestDiscriminatorPropertyMissingError_Error(t *testing.T) {
	err := &DiscriminatorPropertyMissingError{
		PropertyName: "type",
	}

	expected := "discriminator property 'type' is missing"
	assert.Equal(t, expected, err.Error())
}

func TestCoercionError_KeywordPath(t *testing.T) {
	err := &CoercionError{
		SourceType: "string",
		TargetType: "boolean",
		Value:      "yes",
		Message:    "invalid value",
	}

	path := err.KeywordPath()
	assert.Equal(t, []string{"type"}, path)
}

func TestCoercionError_LocalizedString(t *testing.T) {
	err := &CoercionError{
		SourceType: "string",
		TargetType: "boolean",
		Value:      "yes",
		Message:    "invalid value",
	}

	printer := message.NewPrinter(message.MatchLanguage("en"))
	result := err.LocalizedString(printer)
	expected := "cannot coerce string 'yes' to boolean: invalid value"
	assert.Equal(t, expected, result)
}

func TestCoercionError_Error(t *testing.T) {
	err := &CoercionError{
		SourceType: "string",
		TargetType: "boolean",
		Value:      "yes",
		Message:    "invalid value",
	}

	expected := "cannot coerce string 'yes' to boolean: invalid value"
	assert.Equal(t, expected, err.Error())
}

// Test metadata keywords compilation
func TestMetadataKeywords_ExampleCompilation(t *testing.T) {
	obj := map[string]any{
		"type": "string",
		"example": map[string]any{
			"nested": "value",
		},
	}

	ext, err := CompileExample(nil, obj, Version30)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestMetadataKeywords_DeprecatedTrueCompilation(t *testing.T) {
	obj := map[string]any{
		"type":       "string",
		"deprecated": true,
	}

	ext, err := CompileDeprecated(nil, obj, Version30)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestMetadataKeywords_DeprecatedFalseCompilation(t *testing.T) {
	obj := map[string]any{
		"type":       "string",
		"deprecated": false,
	}

	ext, err := CompileDeprecated(nil, obj, Version30)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestMetadataKeywords_DeprecatedInvalidType(t *testing.T) {
	obj := map[string]any{
		"type":       "string",
		"deprecated": "invalid",
	}

	ext, err := CompileDeprecated(nil, obj, Version30)
	assert.Error(t, err)
	assert.Nil(t, ext)
	assert.Contains(t, err.Error(), "deprecated must be a boolean value")
}

func TestMetadataKeywords_NoKeywords(t *testing.T) {
	obj := map[string]any{
		"type": "string",
	}

	ext, err := CompileExample(nil, obj, Version30)
	assert.NoError(t, err)
	assert.Nil(t, ext)

	ext, err = CompileDeprecated(nil, obj, Version30)
	assert.NoError(t, err)
	assert.Nil(t, ext)
}

// Test discriminator compilation
func TestDiscriminatorKeyword_ValidCompilation(t *testing.T) {
	obj := map[string]any{
		"discriminator": map[string]any{
			"propertyName": "type",
			"mapping": map[string]any{
				"dog": "#/components/schemas/Dog",
			},
		},
	}

	ext, err := CompileDiscriminator(nil, obj, Version30)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestDiscriminatorKeyword_MissingPropertyName_Comprehensive(t *testing.T) {
	obj := map[string]any{
		"discriminator": map[string]any{
			"mapping": map[string]any{
				"dog": "#/components/schemas/Dog",
			},
		},
	}

	ext, err := CompileDiscriminator(nil, obj, Version30)
	assert.Error(t, err)
	assert.Nil(t, ext)
	assert.Contains(t, err.Error(), "discriminator must have a propertyName field")
}

func TestDiscriminatorKeyword_PropertyNameNotString(t *testing.T) {
	obj := map[string]any{
		"discriminator": map[string]any{
			"propertyName": 123,
		},
	}

	ext, err := CompileDiscriminator(nil, obj, Version30)
	assert.Error(t, err)
	assert.Nil(t, ext)
	assert.Contains(t, err.Error(), "discriminator propertyName must be a string")
}

func TestDiscriminatorKeyword_NotObject(t *testing.T) {
	obj := map[string]any{
		"discriminator": "invalid",
	}

	ext, err := CompileDiscriminator(nil, obj, Version30)
	assert.Error(t, err)
	assert.Nil(t, ext)
	assert.Contains(t, err.Error(), "discriminator must be an object")
}

func TestDiscriminatorKeyword_NoDiscriminator(t *testing.T) {
	obj := map[string]any{
		"type": "object",
	}

	ext, err := CompileDiscriminator(nil, obj, Version30)
	assert.NoError(t, err)
	assert.Nil(t, ext)
}

// Test end-to-end discriminator validation
func TestDiscriminatorValidation_PropertyMissing(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"discriminator": {
			"propertyName": "type"
		},
		"properties": {
			"name": {
				"type": "string"
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

	// Test object missing discriminator property
	testData := map[string]any{
		"name": "test",
		// Missing "type" property
	}
	err = compiledSchema.Validate(testData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discriminator property 'type' is missing")
}

func TestDiscriminatorValidation_PropertyPresent(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"discriminator": {
			"propertyName": "type"
		},
		"properties": {
			"type": {
				"type": "string"
			},
			"name": {
				"type": "string"
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

func TestDiscriminatorValidation_NonObjectValue(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"discriminator": {
			"propertyName": "type"
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

	// Test with non-object (should pass discriminator validation but fail type validation)
	err = compiledSchema.Validate("not an object")
	assert.Error(t, err)
	// Should get type validation error, not discriminator error
	assert.NotContains(t, err.Error(), "discriminator property")
}
