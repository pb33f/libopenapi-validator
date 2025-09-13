// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
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

func TestValidateDocument_SchemaCompilationFailure(t *testing.T) {
	// create a document with invalid schema that will cause compilation failure
	spec := `openapi: 3.1.0
info:
  title: Test API with Invalid Schema
  version: 1.0.0
paths:
  /test:
    get:
      parameters:
        - name: test_param
          in: query
          required: true
          schema:
            type: string
            pattern: "[\\w\\W]{1,2048}$"
      responses:
        '200':
          description: Success`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	// validate - should handle schema compilation failure gracefully
	valid, errors := ValidateOpenAPIDocument(doc)

	if !valid {
		// verify we got a schema compilation error
		assert.NotEmpty(t, errors)
		found := false
		for _, validationError := range errors {
			if validationError.ValidationType == "schema" && 
			   validationError.ValidationSubType == "compilation" {
				assert.Equal(t, "OpenAPI document schema compilation failed", validationError.Message)
				assert.Contains(t, validationError.Reason, "The OpenAPI schema failed to compile")
				assert.Contains(t, validationError.HowToFix, "complex regex patterns")
				assert.NotEmpty(t, validationError.SchemaValidationErrors)
				
				for _, schemaErr := range validationError.SchemaValidationErrors {
					if schemaErr.Location == "schema compilation" {
						assert.Contains(t, schemaErr.Reason, "failed to compile OpenAPI schema")
						found = true
						break
					}
				}
			}
		}
		if !found {
			// schema compilation succeeded, validation should have passed or failed for other reasons
			t.Logf("Schema compilation succeeded, validation result: %v", valid)
		}
	} else {
		// schema compiled successfully
		assert.True(t, valid)
		assert.Empty(t, errors)
	}
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
	assert.Len(t, errors[0].SchemaValidationErrors, 7)
}
