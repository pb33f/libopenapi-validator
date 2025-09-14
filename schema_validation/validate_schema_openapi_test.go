// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"encoding/json"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/stretchr/testify/assert"
)

func TestSchemaValidator_NullableKeyword_OpenAPI30_Success(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                  nullable: true`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	body := map[string]interface{}{
		"name": nil, // This should be valid with nullable: true
	}
	bodyBytes, _ := json.Marshal(body)

	// Test with version 3.0 - should pass
	v := NewSchemaValidator()
	valid, errors := v.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.True(t, valid, "Validation should pass with nullable: true in OpenAPI 3.0")
	assert.Empty(t, errors, "Should have no validation errors")
}

func TestSchemaValidator_NullableKeyword_OpenAPI31_Fails(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                  nullable: true`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	body := map[string]interface{}{
		"name": nil,
	}
	bodyBytes, _ := json.Marshal(body)

	// Test with version 3.1 - should fail due to nullable keyword
	v := NewSchemaValidator()
	valid, errors := v.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.1)
	assert.False(t, valid, "Validation should fail with nullable keyword in OpenAPI 3.1")
	assert.NotEmpty(t, errors, "Should have validation errors")

	// Check that error mentions nullable keyword not allowed
	found := false
	for _, err := range errors {
		if err.Reason != "" && contains(err.Reason, "nullable") {
			found = true
			break
		}
		for _, schErr := range err.SchemaValidationErrors {
			if schErr.Reason != "" && contains(schErr.Reason, "nullable") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "Error should mention nullable keyword is not allowed")
}

func TestSchemaValidator_DefaultBehavior_RejectsNullable(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  nullable: true`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	body := map[string]interface{}{
		"name": nil,
	}
	bodyBytes, _ := json.Marshal(body)

	// Test default behavior (should be 3.1+ strict mode) - should fail
	v := NewSchemaValidator()
	valid, errors := v.ValidateSchemaString(schema.Schema(), string(bodyBytes))
	assert.False(t, valid, "Default validation should fail with nullable keyword")
	assert.NotEmpty(t, errors, "Should have validation errors")
}

func TestSchemaValidator_OpenAPIModeDisabled(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  nullable: true`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	body := map[string]interface{}{
		"name": nil,
	}
	bodyBytes, _ := json.Marshal(body)

	// Test with OpenAPI mode disabled - should ignore nullable keyword entirely
	v := NewSchemaValidator(config.WithoutOpenAPIMode())
	valid, errors := v.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.False(t, valid, "Should fail without OpenAPI vocabulary (nullable ignored)")
	assert.NotEmpty(t, errors, "Should have validation errors (null vs string type)")
}

func TestSchemaValidator_DiscriminatorKeyword_Valid(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              discriminator:
                propertyName: type
                mapping:
                  dog: "#/components/schemas/Dog"
                  cat: "#/components/schemas/Cat"`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	body := map[string]interface{}{
		"type": "dog",
		"name": "Buddy",
	}
	bodyBytes, _ := json.Marshal(body)

	// Test with discriminator in OpenAPI 3.0 - should pass
	v := NewSchemaValidator()
	valid, errors := v.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.True(t, valid, "Validation should pass with valid discriminator")
	assert.Empty(t, errors, "Should have no validation errors")
}

func TestSchemaValidator_MultipleOpenAPIKeywords(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  nullable: true
                  example: "John Doe"
                  deprecated: true`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	body := map[string]interface{}{
		"name": nil,
	}
	bodyBytes, _ := json.Marshal(body)

	// Test with multiple OpenAPI keywords in OpenAPI 3.0 - should pass
	v := NewSchemaValidator()
	valid, errors := v.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.True(t, valid, "Validation should pass with multiple OpenAPI keywords")
	assert.Empty(t, errors, "Should have no validation errors")
}

func TestSchemaValidator_NullableEnum_OriginalCommentedTest(t *testing.T) {
	// This is the original test case that was commented out
	spec := `openapi: 3.0.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                  enum: [mcbird, mcbeef, veggie, null]
                  nullable: true
                patties:
                  type: integer
                vegetarian:
                  type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	body := map[string]interface{}{
		"name":       nil,
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)
	sch := m.Model.Paths.PathItems.GetOrZero("/burgers/createBurger").Post.RequestBody.Content.GetOrZero("application/json").Schema

	// create a schema validator
	v := NewSchemaValidator()

	// validate with OpenAPI 3.0 version - should now pass!
	valid, errors := v.ValidateSchemaStringWithVersion(sch.Schema(), string(bodyBytes), 3.0)

	assert.True(t, valid, "Should pass with nullable enum in OpenAPI 3.0")
	assert.Empty(t, errors, "Should have no validation errors")
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 1; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
