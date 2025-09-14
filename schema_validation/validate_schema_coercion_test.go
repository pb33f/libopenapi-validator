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

func TestSchemaValidator_ScalarCoercion_Boolean(t *testing.T) {
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
                active:
                  type: boolean
                count:
                  type: integer`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	// Test data with string values that should coerce
	body := map[string]interface{}{
		"active": "true", // String that should coerce to boolean
		"count":  "42",   // String that should coerce to integer
	}
	bodyBytes, _ := json.Marshal(body)

	// Test with coercion enabled
	vWithCoercion := NewSchemaValidator(config.WithScalarCoercion())
	valid, errors := vWithCoercion.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.True(t, valid, "Validation should pass with scalar coercion enabled")
	assert.Empty(t, errors, "Should have no validation errors")

	// Test with coercion disabled (default)
	vWithoutCoercion := NewSchemaValidator()
	valid, errors = vWithoutCoercion.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.False(t, valid, "Validation should fail with scalar coercion disabled")
	assert.NotEmpty(t, errors, "Should have validation errors")
}

func TestSchemaValidator_ScalarCoercion_InvalidStrings(t *testing.T) {
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
                active:
                  type: boolean
                count:
                  type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	// Test data with invalid coercion strings
	body := map[string]interface{}{
		"active": "yes", // Invalid boolean string
		"count":  "abc", // Invalid number string
	}
	bodyBytes, _ := json.Marshal(body)

	// Even with coercion enabled, invalid strings should fail
	vWithCoercion := NewSchemaValidator(config.WithScalarCoercion())
	valid, errors := vWithCoercion.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.False(t, valid, "Validation should fail with invalid coercion strings")
	assert.NotEmpty(t, errors, "Should have validation errors for invalid strings")
}

func TestSchemaValidator_ScalarCoercion_MixedTypes(t *testing.T) {
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
                flag:
                  type: boolean
                score:
                  type: number
                rank:
                  type: integer
                name:
                  type: string`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	// Test mixed coercion and normal values
	body := map[string]interface{}{
		"flag":  "false", // Boolean coercion
		"score": "95.5",  // Number coercion
		"rank":  "1",     // Integer coercion
		"name":  "Alice", // Normal string (no coercion)
	}
	bodyBytes, _ := json.Marshal(body)

	vWithCoercion := NewSchemaValidator(config.WithScalarCoercion())
	valid, errors := vWithCoercion.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
	assert.True(t, valid, "Validation should pass with mixed coercion")
	assert.Empty(t, errors, "Should have no validation errors")
}

func TestSchemaValidator_ScalarCoercion_WithNullable(t *testing.T) {
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
                active:
                  type: boolean
                  nullable: true`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	vWithCoercion := NewSchemaValidator(config.WithScalarCoercion())

	// Test coercion with nullable
	testCases := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"String true", "true", true},
		{"String false", "false", true},
		{"Boolean true", true, true},
		{"Boolean false", false, true},
		{"Null value", nil, true},
		{"Invalid string", "yes", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]interface{}{
				"active": tc.value,
			}
			bodyBytes, _ := json.Marshal(body)

			valid, errors := vWithCoercion.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
			if tc.expected {
				assert.True(t, valid, "Should pass: %s", tc.name)
				assert.Empty(t, errors, "Should have no errors: %s", tc.name)
			} else {
				assert.False(t, valid, "Should fail: %s", tc.name)
				assert.NotEmpty(t, errors, "Should have errors: %s", tc.name)
			}
		})
	}
}

func TestSchemaValidator_ScalarCoercion_EdgeCases(t *testing.T) {
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
                scientific:
                  type: number
                leadingZero:
                  type: integer
                negative:
                  type: number`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, errs := doc.BuildV3Model()
	assert.Empty(t, errs)

	schema := m.Model.Paths.PathItems.GetOrZero("/test").Post.RequestBody.Content.GetOrZero("application/json").Schema

	vWithCoercion := NewSchemaValidator(config.WithScalarCoercion())

	// Test edge cases
	testCases := []struct {
		body     map[string]interface{}
		expected bool
		desc     string
	}{
		{map[string]interface{}{"scientific": "1.23e-10"}, true, "Scientific notation"},
		{map[string]interface{}{"leadingZero": "007"}, false, "Leading zeros not allowed for integers"},
		{map[string]interface{}{"negative": "-0"}, true, "Negative zero"},
		{map[string]interface{}{"scientific": "1.23E+5"}, true, "Uppercase E notation"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tc.body)

			valid, errors := vWithCoercion.ValidateSchemaStringWithVersion(schema.Schema(), string(bodyBytes), 3.0)
			if tc.expected {
				assert.True(t, valid, "Should pass: %s", tc.desc)
				assert.Empty(t, errors, "Should have no errors: %s", tc.desc)
			} else {
				assert.False(t, valid, "Should fail: %s", tc.desc)
				assert.NotEmpty(t, errors, "Should have errors: %s", tc.desc)
			}
		})
	}
}
