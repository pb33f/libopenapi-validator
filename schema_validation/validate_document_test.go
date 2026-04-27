// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"fmt"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"

	"github.com/pb33f/libopenapi-validator/config"
	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

func TestValidateDocument(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/petstorev3.json")

	doc, _ := libopenapi.NewDocument(petstore)

	// validate!
	valid, errors := ValidateOpenAPIDocument(doc)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateDocument_31(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/valid_31.yaml")

	doc, _ := libopenapi.NewDocument(petstore)

	// validate!
	valid, errors := ValidateOpenAPIDocument(doc)

	assert.True(t, valid)
	assert.Len(t, errors, 0)
}

func TestValidateDocument_Invalid31(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/invalid_31.yaml")

	doc, _ := libopenapi.NewDocument(petstore)

	// validate!
	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 6)
}

func TestValidateDocument_UnquotedIntegerResponseCodeHelpfulError(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        200:
          description: OK`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "OpenAPI document validation failed", errors[0].Message)
	assert.Contains(t, errors[0].Reason, "Response status code keys must be strings")
	assert.Contains(t, errors[0].Reason, `quote 200 as "200"`)
	assert.Contains(t, errors[0].Reason, "/paths/~1test/get/responses/200")
	assert.NotContains(t, errors[0].Reason, "got null, want object")
	assert.Contains(t, errors[0].HowToFix, `"200"`)
	assert.Equal(t, 9, errors[0].SpecLine)
	assert.Equal(t, 9, errors[0].SpecCol)
	assert.Empty(t, errors[0].SchemaValidationErrors)
}

func TestValidateDocument_QuotedResponseCodeValid(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        "200":
          description: OK`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestValidateDocument_YAMLMergeKeyValid(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
x-base-response: &baseResponse
  description: OK
paths:
  /test:
    get:
      responses:
        "200":
          <<: *baseResponse`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestValidateDocument_YAMLMergeKeyDoesNotHideInvalidResponseCode(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
x-base-responses: &baseResponses
  default:
    description: Default
paths:
  /test:
    get:
      responses:
        <<: *baseResponses
        200:
          description: OK`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Reason, "Response status code keys must be strings")
	assert.Contains(t, errors[0].Reason, `quote 200 as "200"`)
	assert.NotContains(t, errors[0].Reason, `merge key "<<"`)
	assert.Equal(t, 13, errors[0].SpecLine)
	assert.Empty(t, errors[0].SchemaValidationErrors)
}

func TestValidateDocument_GenericNonStringMappingKeyHelpfulError(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths: {}
x-values:
  1: one`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "OpenAPI document validation failed", errors[0].Message)
	assert.Contains(t, errors[0].Reason, "OpenAPI documents require string mapping keys")
	assert.Contains(t, errors[0].Reason, `int key "1"`)
	assert.Contains(t, errors[0].Reason, "/x-values/1")
	assert.NotContains(t, errors[0].Reason, "got null, want object")
	assert.Contains(t, errors[0].HowToFix, "Quote YAML mapping keys")
	assert.Equal(t, 7, errors[0].SpecLine)
	assert.Equal(t, 3, errors[0].SpecCol)
	assert.Empty(t, errors[0].SchemaValidationErrors)
}

func TestNormalizeJSON_ReturnsMarshalError(t *testing.T) {
	payload := map[string]interface{}{
		"openapi": "3.1.0",
		"invalid": map[interface{}]interface{}{
			1: "one",
		},
	}

	normalized, err := normalizeJSON(payload)

	assert.Nil(t, normalized)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type: map[interface {}]interface {}")
}

func TestValidateDocument_NormalizationErrorDoesNotValidateNil(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths: {}`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	badSpecJSON := map[string]interface{}{
		"openapi": "3.1.0",
		"invalid": map[interface{}]interface{}{
			1: "one",
		},
	}
	info := doc.GetSpecInfo()
	info.SpecJSON = &badSpecJSON
	info.SpecJSONBytes = nil

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "OpenAPI document validation failed", errors[0].Message)
	assert.Contains(t, errors[0].Reason, "cannot be converted to JSON")
	assert.Contains(t, errors[0].Reason, "unsupported type: map[interface {}]interface {}")
	assert.NotContains(t, errors[0].Reason, "got null, want object")
	assert.Empty(t, errors[0].SchemaValidationErrors)
}

func TestValidateDocument_CorruptSpecJSONBytesFallbackNormalizationError(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths: {}`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	badSpecJSON := map[string]interface{}{
		"openapi": "3.1.0",
		"invalid": map[interface{}]interface{}{
			1: "one",
		},
	}
	corrupt := []byte(`{not valid json!!!}`)
	info := doc.GetSpecInfo()
	info.SpecJSON = &badSpecJSON
	info.SpecJSONBytes = &corrupt

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Reason, "cannot be converted to JSON")
	assert.NotContains(t, errors[0].Reason, "got null, want object")
	assert.Empty(t, errors[0].SchemaValidationErrors)
}

func TestValidateDocumentHelpers_DefensiveBranches(t *testing.T) {
	assert.Nil(t, findNonStringMappingKey(nil))
	assert.Nil(t, findNonStringMappingKeyInNode(nil, nil))
	assert.Nil(t, findNonStringMappingKeyInMergeValue(nil, nil))
	assert.False(t, isStringMappingKey(nil))
	assert.False(t, isMergeMappingKey(nil))
	assert.Equal(t, "", buildJSONPointer(nil))
	assert.Equal(t, "non-string", yamlKeyType(nil))
	assert.False(t, isOperationResponseStatusCodeKey([]string{"paths", "/test", "parameters", "responses", "200"}))

	sequenceKey := &nonStringMappingKey{Sequence: true}
	assert.Equal(t, "sequence", yamlKeyType(sequenceKey))

	mergeKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!merge", Value: "<<"}
	assert.True(t, isMergeMappingKey(mergeKey))

	intKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "1", Line: 2, Column: 5}
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "one"}
	mapping := &yaml.Node{
		Kind:    yaml.MappingNode,
		Content: []*yaml.Node{intKey, value},
	}
	sequence := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Content: []*yaml.Node{mapping},
	}

	found := findNonStringMappingKeyInNode(sequence, []string{"items"})
	assert.NotNil(t, found)
	assert.Equal(t, []string{"items", "0", "1"}, found.Path)
	assert.Equal(t, "1", found.Value)

	found = findNonStringMappingKeyInMergeValue(&yaml.Node{Kind: yaml.AliasNode, Alias: mapping}, []string{"merged"})
	assert.NotNil(t, found)
	assert.Equal(t, []string{"merged", "1"}, found.Path)

	mergeMapping := &yaml.Node{
		Kind:    yaml.MappingNode,
		Content: []*yaml.Node{mergeKey, mapping},
	}
	found = findNonStringMappingKeyInNode(mergeMapping, []string{"mergeTarget"})
	assert.NotNil(t, found)
	assert.Equal(t, []string{"mergeTarget", "1"}, found.Path)

	mergeSequence := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{mapping}}
	found = findNonStringMappingKeyInMergeValue(mergeSequence, []string{"mergedSequence"})
	assert.NotNil(t, found)
	assert.Equal(t, []string{"mergedSequence", "1"}, found.Path)

	assert.Nil(t, findNonStringMappingKeyInMergeValue(&yaml.Node{Kind: yaml.SequenceNode}, []string{"empty"}))
	assert.Nil(t, findNonStringMappingKeyInMergeValue(&yaml.Node{Kind: yaml.AliasNode}, []string{"merged"}))
}

// Helper function to test the validation logic directly
func validateOpenAPIDocumentWithMalformedSchema(loadedSchema string, decodedDocument map[string]interface{}) (bool, []*liberrors.ValidationError) {
	options := config.NewValidationOptions()
	var validationErrors []*liberrors.ValidationError

	// This replicates the exact logic from validate_document.go:40-127
	_, err := helpers.NewCompiledSchema("schema", []byte(loadedSchema), options)
	if err != nil {
		// schema compilation failed, return validation error instead of panicking
		// NO SchemaValidationFailure for pre-validation errors like compilation failures
		validationErrors = append(validationErrors, &liberrors.ValidationError{
			ValidationType:    helpers.Schema,
			ValidationSubType: "compilation",
			Message:           "OpenAPI document schema compilation failed",
			Reason:            fmt.Sprintf("The OpenAPI schema failed to compile: %s", err.Error()),
			SpecLine:          1,
			SpecCol:           0,
			HowToFix:          "check the OpenAPI schema for invalid JSON Schema syntax, complex regex patterns, or unsupported schema constructs",
			Context:           loadedSchema,
		})
		return false, validationErrors
	}

	// If compilation succeeded, continue with normal validation logic
	// (This would be the rest of the validate_document.go logic)
	return true, nil
}

func TestValidateDocument_SchemaCompilationFailure(t *testing.T) {
	// Test the schema compilation error handling by providing invalid JSON schema
	malformedSchema := `{"type": "object", "properties": {"test": invalid_json_here}}`
	decodedDocument := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
	}

	// Call our helper function which replicates the exact logic from validate_document.go
	valid, errors := validateOpenAPIDocumentWithMalformedSchema(malformedSchema, decodedDocument)

	// Should fail validation due to schema compilation error
	assert.False(t, valid)
	assert.NotEmpty(t, errors)

	// Verify we got a schema compilation error with the exact same structure
	validationError := errors[0]
	assert.Equal(t, helpers.Schema, validationError.ValidationType)
	assert.Equal(t, "compilation", validationError.ValidationSubType)
	assert.Equal(t, "OpenAPI document schema compilation failed", validationError.Message)
	assert.Contains(t, validationError.Reason, "The OpenAPI schema failed to compile")
	assert.Contains(t, validationError.HowToFix, "complex regex patterns")
	assert.Equal(t, malformedSchema, validationError.Context)
	assert.Equal(t, 1, validationError.SpecLine)
	assert.Equal(t, 0, validationError.SpecCol)

	// Schema compilation errors don't have SchemaValidationFailure objects
	assert.Empty(t, validationError.SchemaValidationErrors)
}

// TestValidateDocument_CompilationFailure tests the actual ValidateOpenAPIDocument function
// with a corrupted document that causes schema compilation to fail
func TestValidateDocument_CompilationFailure(t *testing.T) {
	doc, _ := libopenapi.NewDocumentWithTypeCheck([]byte(`{}`), true)
	doc.GetSpecInfo().APISchema = `{"type": "object", "properties": {"test": :bad"": JSON: } here.}}`
	// validate!
	valid, errors := ValidateOpenAPIDocument(doc)
	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Reason, "The OpenAPI schema failed to compile")
	assert.Nil(t, errors[0].SchemaValidationErrors, "Compilation errors should not have SchemaValidationErrors")
}

func TestValidateSchema_ValidateLicenseIdentifier(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  version: 1.0.0
  title: Test
  license:
    name: Apache 2.0
    url: https://opensource.org/licenses/Apache-2.0
    identifier: Apache-2.0
components:
  schemas:
    Pet:
      type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	// validate!
	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
}

func TestValidateSchema_GeneratePointlessValidation(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  version: 1
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	// validate!
	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 6)
}

func TestValidateDocument_NilSpecJSON(t *testing.T) {
	// Create a document with minimal valid OpenAPI content
	spec := `openapi: 3.1.0
info:
  version: 1.0.0
  title: Test
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	// Simulate the nil SpecJSON scenario by setting both to nil
	info := doc.GetSpecInfo()
	info.SpecJSON = nil
	info.SpecJSONBytes = nil

	// validate!
	valid, errors := ValidateOpenAPIDocument(doc)

	// Should fail validation due to nil SpecJSON
	assert.False(t, valid)
	assert.Len(t, errors, 1)

	// Verify error structure
	validationError := errors[0]
	assert.Equal(t, helpers.Schema, validationError.ValidationType)
	assert.Equal(t, "document", validationError.ValidationSubType)
	assert.Equal(t, "OpenAPI document validation failed", validationError.Message)
	assert.Contains(t, validationError.Reason, "SpecJSON is nil")
	assert.Contains(t, validationError.HowToFix, "ensure the OpenAPI document is valid")
	assert.Equal(t, 1, validationError.SpecLine)
	assert.Equal(t, 0, validationError.SpecCol)

	// Pre-validation errors should not have SchemaValidationErrors
	assert.Empty(t, validationError.SchemaValidationErrors)
}

func TestValidateDocument_WithPrecompiledSchema(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(petstore)

	info := doc.GetSpecInfo()

	// Pre-compile the schema
	options := config.NewValidationOptions()
	compiledSchema, err := helpers.NewCompiledSchema("schema", []byte(info.APISchema), options)
	assert.NoError(t, err)

	// Validate with precompiled schema
	valid, errs := ValidateOpenAPIDocumentWithPrecompiled(doc, compiledSchema)
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	// Validate without precompiled schema (should produce identical results)
	valid2, errs2 := ValidateOpenAPIDocument(doc)
	assert.True(t, valid2)
	assert.Len(t, errs2, 0)
}

func TestValidateDocument_WithPrecompiledSchema_Invalid(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/invalid_31.yaml")
	doc, _ := libopenapi.NewDocument(petstore)

	info := doc.GetSpecInfo()

	// Pre-compile the schema
	options := config.NewValidationOptions()
	compiledSchema, err := helpers.NewCompiledSchema("schema", []byte(info.APISchema), options)
	assert.NoError(t, err)

	// Validate with precompiled schema
	valid, errs := ValidateOpenAPIDocumentWithPrecompiled(doc, compiledSchema)
	assert.False(t, valid)
	assert.Len(t, errs, 1)
	assert.Len(t, errs[0].SchemaValidationErrors, 6)

	// Validate without precompiled schema (should produce identical error count)
	valid2, errs2 := ValidateOpenAPIDocument(doc)
	assert.False(t, valid2)
	assert.Len(t, errs2, 1)
	assert.Len(t, errs2[0].SchemaValidationErrors, 6)
}

func TestValidateDocument_SpecJSONBytesPath(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(petstore)

	info := doc.GetSpecInfo()

	// Nil out SpecJSON but leave SpecJSONBytes intact — forces the SpecJSONBytes path
	assert.NotNil(t, info.SpecJSONBytes, "SpecJSONBytes should be populated by libopenapi")
	info.SpecJSON = nil

	valid, errs := ValidateOpenAPIDocument(doc)
	assert.True(t, valid)
	assert.Len(t, errs, 0)
}

func TestValidateDocument_SpecJSONBytesCorrupt_NilSpecJSON(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(petstore)

	info := doc.GetSpecInfo()

	// Put corrupt bytes in SpecJSONBytes so UnmarshalJSON fails,
	// and nil out SpecJSON so the fallback normalizeJSON path is skipped.
	// This exercises the nil guard on SpecJSON inside the error branch.
	corrupt := []byte(`{not valid json!!!}`)
	info.SpecJSONBytes = &corrupt
	info.SpecJSON = nil

	// Validation should fail before JSON Schema validation instead of validating nil.
	valid, errs := ValidateOpenAPIDocument(doc)
	assert.False(t, valid)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Reason, "SpecJSONBytes cannot be decoded as JSON")
	assert.Empty(t, errs[0].SchemaValidationErrors)
}

func TestValidateDocument_SpecJSONBytesCorrupt_FallbackToSpecJSON(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/petstorev3.json")
	doc, _ := libopenapi.NewDocument(petstore)

	info := doc.GetSpecInfo()

	// Put corrupt bytes in SpecJSONBytes so UnmarshalJSON fails,
	// but leave SpecJSON intact so the fallback to normalizeJSON executes.
	corrupt := []byte(`{not valid json!!!}`)
	info.SpecJSONBytes = &corrupt

	// Should still validate successfully via the SpecJSON fallback
	valid, errs := ValidateOpenAPIDocument(doc)
	assert.True(t, valid)
	assert.Len(t, errs, 0)
}

func TestValidateDocument_SpecJSONBytesPath_Invalid(t *testing.T) {
	petstore, _ := os.ReadFile("../test_specs/invalid_31.yaml")
	doc, _ := libopenapi.NewDocument(petstore)

	info := doc.GetSpecInfo()

	// Nil out SpecJSON but leave SpecJSONBytes intact
	assert.NotNil(t, info.SpecJSONBytes, "SpecJSONBytes should be populated by libopenapi")
	info.SpecJSON = nil

	valid, errs := ValidateOpenAPIDocument(doc)
	assert.False(t, valid)
	assert.Len(t, errs, 1)
	assert.NotEmpty(t, errs[0].SchemaValidationErrors)
}
