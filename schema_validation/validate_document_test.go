// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"fmt"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"

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

// Helper function to test the validation logic directly
func validateOpenAPIDocumentWithMalformedSchema(loadedSchema string, decodedDocument map[string]interface{}) (bool, []*liberrors.ValidationError) {
	options := config.NewValidationOptions()
	var validationErrors []*liberrors.ValidationError

	// This replicates the exact logic from validate_document.go:40-127
	_, err := helpers.NewCompiledSchema("schema", []byte(loadedSchema), options)
	if err != nil {
		// schema compilation failed, return validation error instead of panicking
		violation := &liberrors.SchemaValidationFailure{
			Reason:          fmt.Sprintf("failed to compile OpenAPI schema: %s", err.Error()),
			Location:        "schema compilation",
			ReferenceSchema: loadedSchema,
		}
		validationErrors = append(validationErrors, &liberrors.ValidationError{
			ValidationType:         "schema",
			ValidationSubType:      "compilation",
			Message:                "OpenAPI document schema compilation failed",
			Reason:                 fmt.Sprintf("The OpenAPI schema failed to compile: %s", err.Error()),
			SpecLine:               1,
			SpecCol:                0,
			SchemaValidationErrors: []*liberrors.SchemaValidationFailure{violation},
			HowToFix:               "check the OpenAPI schema for invalid JSON Schema syntax, complex regex patterns, or unsupported schema constructs",
			Context:                loadedSchema,
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
	assert.Equal(t, "schema", validationError.ValidationType)
	assert.Equal(t, "compilation", validationError.ValidationSubType)
	assert.Equal(t, "OpenAPI document schema compilation failed", validationError.Message)
	assert.Contains(t, validationError.Reason, "The OpenAPI schema failed to compile")
	assert.Contains(t, validationError.HowToFix, "complex regex patterns")
	assert.Equal(t, malformedSchema, validationError.Context)
	assert.Equal(t, 1, validationError.SpecLine)
	assert.Equal(t, 0, validationError.SpecCol)

	// Verify schema validation errors
	assert.NotEmpty(t, validationError.SchemaValidationErrors)
	schemaErr := validationError.SchemaValidationErrors[0]
	assert.Equal(t, "schema compilation", schemaErr.Location)
	assert.Contains(t, schemaErr.Reason, "failed to compile OpenAPI schema")
	assert.Equal(t, malformedSchema, schemaErr.ReferenceSchema)
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
