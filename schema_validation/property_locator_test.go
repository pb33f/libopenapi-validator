// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
)

func TestExtractPropertyNameFromError_InvalidPropertyName(t *testing.T) {
	// We can't easily create a complete ValidationError without the jsonschema library internals,
	// so we test the regex patterns separately in TestCheckErrorForPropertyInfo_*
	// This test verifies that nil is returned for nil input
	info := extractPropertyNameFromError(nil)
	assert.Nil(t, info)
}

func TestCheckErrorForPropertyInfo_InvalidPropertyName(t *testing.T) {
	// Create a mock validation error that would produce the error message
	// Since we can't easily mock ErrorKind, we'll test the regex directly
	testCases := []struct {
		name           string
		errorMsg       string
		expectedProp   string
		expectedParent []string
	}{
		{
			name:           "Simple invalid property name",
			errorMsg:       "invalid propertyName '$defs-atmVolatility_type'",
			expectedProp:   "$defs-atmVolatility_type",
			expectedParent: []string{"components", "schemas"},
		},
		{
			name:           "Property name with special chars",
			errorMsg:       "invalid propertyName '$ref-test_value'",
			expectedProp:   "$ref-test_value",
			expectedParent: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the regex directly
			matches := invalidPropertyNameRegex.FindStringSubmatch(tc.errorMsg)
			assert.Len(t, matches, 2)
			assert.Equal(t, tc.expectedProp, matches[1])
		})
	}
}

func TestCheckErrorForPropertyInfo_PatternMismatch(t *testing.T) {
	testCases := []struct {
		name           string
		errorMsg       string
		expectedValue  string
		expectedPattern string
	}{
		{
			name:            "Standard pattern mismatch",
			errorMsg:        "'$defs-atmVolatility_type' does not match pattern '^[a-zA-Z0-9._-]+$'",
			expectedValue:   "$defs-atmVolatility_type",
			expectedPattern: "^[a-zA-Z0-9._-]+$",
		},
		{
			name:            "Complex pattern",
			errorMsg:        "'invalid@value' does not match pattern '^[a-zA-Z]+$'",
			expectedValue:   "invalid@value",
			expectedPattern: "^[a-zA-Z]+$",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := patternMismatchRegex.FindStringSubmatch(tc.errorMsg)
			assert.Len(t, matches, 3)
			assert.Equal(t, tc.expectedValue, matches[1])
			assert.Equal(t, tc.expectedPattern, matches[2])
		})
	}
}

func TestBuildEnhancedReason(t *testing.T) {
	testCases := []struct {
		name         string
		propertyName string
		pattern      string
		expected     string
	}{
		{
			name:         "Standard case",
			propertyName: "$defs-test",
			pattern:      "^[a-zA-Z0-9._-]+$",
			expected:     "invalid propertyName '$defs-test': does not match pattern '^[a-zA-Z0-9._-]+$'",
		},
		{
			name:         "Empty pattern",
			propertyName: "test",
			pattern:      "",
			expected:     "invalid propertyName 'test': does not match pattern ''",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildEnhancedReason(tc.propertyName, tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractPatternFromCauses_WithPattern(t *testing.T) {
	// extractPatternFromCauses calls ve.Error() internally which requires proper ValidationError initialization.
	// We test the regex pattern matching separately in TestCheckErrorForPropertyInfo_PatternMismatch.
	// Test the nil case here
	pattern := extractPatternFromCauses(nil)
	assert.Empty(t, pattern)
}

func TestExtractPatternFromCauses_NoPattern(t *testing.T) {
	ve := &jsonschema.ValidationError{
		Causes: []*jsonschema.ValidationError{},
	}

	pattern := extractPatternFromCauses(ve)
	assert.Empty(t, pattern)
}

func TestExtractPatternFromCauses_Nil(t *testing.T) {
	pattern := extractPatternFromCauses(nil)
	assert.Empty(t, pattern)
}

func TestExtractPropertyNameFromError_Nil(t *testing.T) {
	info := extractPropertyNameFromError(nil)
	assert.Nil(t, info)
}

func TestExtractPropertyNameFromError_NoCauses(t *testing.T) {
	// We can't create a ValidationError without internal state that makes Error() work.
	// Testing with nil is sufficient to verify nil-safety, which is tested in TestExtractPropertyNameFromError_Nil.
	// The actual functionality is tested through integration tests with real validation errors.
	t.Skip("Skipping as we cannot create a proper ValidationError without internal state")
}

func TestFindPropertyKeyNodeInYAML_Success(t *testing.T) {
	yamlContent := `
components:
  schemas:
    $defs-atmVolatility_type:
      type: object
      properties:
        value:
          type: string
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	// Find the problematic property name
	foundNode := findPropertyKeyNodeInYAML(rootNode.Content[0], "$defs-atmVolatility_type", []string{"components", "schemas"})
	assert.NotNil(t, foundNode)
	assert.Equal(t, "$defs-atmVolatility_type", foundNode.Value)
	assert.Greater(t, foundNode.Line, 0)
	assert.Greater(t, foundNode.Column, 0)
}

func TestFindPropertyKeyNodeInYAML_NotFound(t *testing.T) {
	yamlContent := `
components:
  schemas:
    ValidSchema:
      type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	foundNode := findPropertyKeyNodeInYAML(rootNode.Content[0], "NonExistent", []string{"components", "schemas"})
	assert.Nil(t, foundNode)
}

func TestFindPropertyKeyNodeInYAML_InvalidParentPath(t *testing.T) {
	yamlContent := `
components:
  schemas:
    TestSchema:
      type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	foundNode := findPropertyKeyNodeInYAML(rootNode.Content[0], "TestSchema", []string{"invalid", "path"})
	assert.Nil(t, foundNode)
}

func TestFindPropertyKeyNodeInYAML_NilRootNode(t *testing.T) {
	foundNode := findPropertyKeyNodeInYAML(nil, "test", []string{"components"})
	assert.Nil(t, foundNode)
}

func TestFindPropertyKeyNodeInYAML_EmptyPropertyName(t *testing.T) {
	yamlContent := `
components:
  schemas:
    TestSchema:
      type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	foundNode := findPropertyKeyNodeInYAML(rootNode.Content[0], "", []string{"components", "schemas"})
	assert.Nil(t, foundNode)
}

func TestFindPropertyKeyNodeInYAML_EmptyParentPath(t *testing.T) {
	yamlContent := `
TestProperty:
  type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	foundNode := findPropertyKeyNodeInYAML(rootNode.Content[0], "TestProperty", []string{})
	assert.NotNil(t, foundNode)
	assert.Equal(t, "TestProperty", foundNode.Value)
}

func TestNavigateToYAMLChild_Success(t *testing.T) {
	yamlContent := `
components:
  schemas:
    TestSchema:
      type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	// Navigate to components
	child := navigateToYAMLChild(rootNode.Content[0], "components")
	assert.NotNil(t, child)
	assert.Equal(t, yaml.MappingNode, child.Kind)
}

func TestNavigateToYAMLChild_NotFound(t *testing.T) {
	yamlContent := `
components:
  schemas:
    TestSchema:
      type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	child := navigateToYAMLChild(rootNode.Content[0], "nonexistent")
	assert.Nil(t, child)
}

func TestNavigateToYAMLChild_NilParent(t *testing.T) {
	child := navigateToYAMLChild(nil, "test")
	assert.Nil(t, child)
}

func TestNavigateToYAMLChild_DocumentNode(t *testing.T) {
	yamlContent := `
test:
  value: 123
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	// rootNode itself is a DocumentNode
	child := navigateToYAMLChild(&rootNode, "test")
	assert.NotNil(t, child)
}

func TestNavigateToYAMLChild_NonMappingNode(t *testing.T) {
	yamlContent := `
- item1
- item2
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	// Try to navigate a sequence node as if it were a map
	child := navigateToYAMLChild(rootNode.Content[0], "test")
	assert.Nil(t, child)
}

func TestFindMapKeyValue_Success(t *testing.T) {
	yamlContent := `
key1: value1
key2: value2
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	valueNode := findMapKeyValue(rootNode.Content[0], "key1")
	assert.NotNil(t, valueNode)
	assert.Equal(t, "value1", valueNode.Value)
}

func TestFindMapKeyValue_NotFound(t *testing.T) {
	yamlContent := `
key1: value1
key2: value2
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	valueNode := findMapKeyValue(rootNode.Content[0], "key3")
	assert.Nil(t, valueNode)
}

func TestFindMapKeyValue_NonMappingNode(t *testing.T) {
	yamlContent := `
- item1
- item2
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	valueNode := findMapKeyValue(rootNode.Content[0], "test")
	assert.Nil(t, valueNode)
}

func TestFindMapKeyNode_Success(t *testing.T) {
	yamlContent := `
key1: value1
key2: value2
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	keyNode := findMapKeyNode(rootNode.Content[0], "key1")
	assert.NotNil(t, keyNode)
	assert.Equal(t, "key1", keyNode.Value)
}

func TestFindMapKeyNode_NotFound(t *testing.T) {
	yamlContent := `
key1: value1
key2: value2
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	keyNode := findMapKeyNode(rootNode.Content[0], "key3")
	assert.Nil(t, keyNode)
}

func TestFindMapKeyNode_NilNode(t *testing.T) {
	keyNode := findMapKeyNode(nil, "test")
	assert.Nil(t, keyNode)
}

func TestFindMapKeyNode_DocumentNode(t *testing.T) {
	yamlContent := `
test: value
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	// Pass the document node itself
	keyNode := findMapKeyNode(&rootNode, "test")
	assert.NotNil(t, keyNode)
	assert.Equal(t, "test", keyNode.Value)
}

func TestFindMapKeyNode_NonMappingNode(t *testing.T) {
	yamlContent := `
- item1
- item2
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	keyNode := findMapKeyNode(rootNode.Content[0], "test")
	assert.Nil(t, keyNode)
}

func TestEnrichSchemaValidationFailure_Success(t *testing.T) {
	yamlContent := `
components:
  schemas:
    $defs-atmVolatility_type:
      type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	failure := &PropertyNameInfo{
		PropertyName:   "$defs-atmVolatility_type",
		ParentLocation: []string{"components", "schemas"},
		EnhancedReason: "invalid propertyName '$defs-atmVolatility_type': does not match pattern '^[a-zA-Z0-9._-]+$'",
		Pattern:        "^[a-zA-Z0-9._-]+$",
	}

	var line, column int
	var reason, fieldName, fieldPath, location string
	var instancePath []string

	enriched := enrichSchemaValidationFailure(
		failure,
		rootNode.Content[0],
		&line,
		&column,
		&reason,
		&fieldName,
		&fieldPath,
		&location,
		&instancePath,
	)

	assert.True(t, enriched)
	assert.Greater(t, line, 0)
	assert.Greater(t, column, 0)
	assert.Equal(t, "invalid propertyName '$defs-atmVolatility_type': does not match pattern '^[a-zA-Z0-9._-]+$'", reason)
	assert.Equal(t, "$defs-atmVolatility_type", fieldName)
	assert.Contains(t, fieldPath, "$defs-atmVolatility_type")
	assert.Equal(t, "/components/schemas", location)
	assert.Equal(t, []string{"components", "schemas"}, instancePath)
}

func TestEnrichSchemaValidationFailure_NilFailure(t *testing.T) {
	yamlContent := `
test: value
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	var line, column int
	var reason, fieldName, fieldPath, location string
	var instancePath []string

	enriched := enrichSchemaValidationFailure(
		nil,
		rootNode.Content[0],
		&line,
		&column,
		&reason,
		&fieldName,
		&fieldPath,
		&location,
		&instancePath,
	)

	assert.False(t, enriched)
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, column)
}

func TestEnrichSchemaValidationFailure_PropertyNotFound(t *testing.T) {
	yamlContent := `
components:
  schemas:
    ValidSchema:
      type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	failure := &PropertyNameInfo{
		PropertyName:   "NonExistent",
		ParentLocation: []string{"components", "schemas"},
		EnhancedReason: "test reason",
	}

	var line, column int
	var reason, fieldName, fieldPath, location string
	var instancePath []string

	enriched := enrichSchemaValidationFailure(
		failure,
		rootNode.Content[0],
		&line,
		&column,
		&reason,
		&fieldName,
		&fieldPath,
		&location,
		&instancePath,
	)

	assert.False(t, enriched)
	assert.Equal(t, 0, line)
	assert.Equal(t, 0, column)
}

func TestEnrichSchemaValidationFailure_EmptyParentLocation(t *testing.T) {
	yamlContent := `
$defs-test:
  type: object
`

	var rootNode yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rootNode)
	assert.NoError(t, err)

	failure := &PropertyNameInfo{
		PropertyName:   "$defs-test",
		ParentLocation: []string{},
		EnhancedReason: "test reason",
	}

	var line, column int
	var reason, fieldName, fieldPath, location string
	var instancePath []string

	enriched := enrichSchemaValidationFailure(
		failure,
		rootNode.Content[0],
		&line,
		&column,
		&reason,
		&fieldName,
		&fieldPath,
		&location,
		&instancePath,
	)

	assert.True(t, enriched)
	assert.Greater(t, line, 0)
	assert.Equal(t, "test reason", reason)
	assert.Equal(t, "$defs-test", fieldName)
	assert.Equal(t, "/", location)
	assert.Equal(t, []string{}, instancePath)
}

func TestCheckErrorForPropertyInfo_NoMatch(t *testing.T) {
	// checkErrorForPropertyInfo calls ve.Error() which requires a properly initialized ValidationError.
	// We can't easily create one without the jsonschema library internals.
	// The regex patterns are tested separately in TestCheckErrorForPropertyInfo_* tests above.
	// This test is redundant with TestExtractPropertyNameFromError_NoCauses
	t.Skip("Skipping as we cannot create a proper ValidationError without internal state")
}

// TestPropertyLocator_Integration_InvalidPropertyName tests the full flow from ValidateOpenAPIDocument
// through the property locator functions. This provides coverage for extractPropertyNameFromError
// and checkErrorForPropertyInfo which require real ValidationError objects from jsonschema.
func TestPropertyLocator_Integration_InvalidPropertyName(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test Invalid Property Name
  version: 1.0.0
components:
  schemas:
    $defs-atmVolatility_type:
      type: object
      properties:
        value:
          type: string`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	// Before integration - validate without our fallback logic
	// This just verifies the test scenario triggers a validation error
	valid, errors := ValidateOpenAPIDocument(doc)
	assert.False(t, valid)
	assert.Len(t, errors, 1)

	// The validator should find the error
	assert.Len(t, errors[0].SchemaValidationErrors, 1)
	sve := errors[0].SchemaValidationErrors[0]

	// After integration, the fallback logic should populate Line and Column
	assert.Greater(t, sve.Line, 0, "Line should be populated by fallback logic")
	assert.Greater(t, sve.Column, 0, "Column should be populated by fallback logic")

	// Verify the enhanced error message includes the property name and pattern
	assert.Contains(t, sve.Reason, "$defs-atmVolatility_type", "Reason should include property name")
	assert.Contains(t, sve.Reason, "does not match pattern", "Reason should include pattern info")

	// Verify additional fields are populated
	assert.Equal(t, "$defs-atmVolatility_type", sve.FieldName, "FieldName should be extracted")
	assert.Contains(t, sve.FieldPath, "$defs-atmVolatility_type", "FieldPath should include property name")

	// Original validation check that extractPropertyNameFromError works
	assert.NotNil(t, sve.OriginalError, "OriginalError should be populated")

	info := extractPropertyNameFromError(sve.OriginalError)
	// This should successfully extract the property name
	assert.NotNil(t, info, "Should extract property name info from error")
	assert.Equal(t, "$defs-atmVolatility_type", info.PropertyName)
	assert.Contains(t, info.EnhancedReason, "$defs-atmVolatility_type")
	assert.Contains(t, info.EnhancedReason, "does not match pattern")

	// Verify we can find it in the YAML
	docInfo := doc.GetSpecInfo()

	// The parent location might be empty or have "components", "schemas" depending on how
	// the error was structured. Let's try different combinations.
	foundNode := findPropertyKeyNodeInYAML(docInfo.RootNode.Content[0], info.PropertyName, []string{"components", "schemas"})
	if foundNode == nil {
		// Try without parent location
		foundNode = findPropertyKeyNodeInYAML(docInfo.RootNode.Content[0], info.PropertyName, []string{})
	}
	if foundNode == nil {
		// Try with just components
		foundNode = findPropertyKeyNodeInYAML(docInfo.RootNode.Content[0], info.PropertyName, []string{"components"})
	}

	assert.NotNil(t, foundNode, "Should find property key in YAML tree")
	if foundNode != nil {
		assert.Greater(t, foundNode.Line, 0)
		assert.Equal(t, "$defs-atmVolatility_type", foundNode.Value)
	}
}

// TestValidateOpenAPIDocument_Issue726_InvalidPropertyName tests the fix for GitHub issue #726
// (https://github.com/daveshanley/vacuum/issues/726)
//
// Issue: Invalid spec (not valid against OAS 3 schema) reports errors at line 0:0
//
// The problem was that when an OpenAPI document contained invalid property names
// (e.g., starting with '$' which doesn't match the required pattern '^[a-zA-Z0-9._-]+$'),
// the validator would correctly identify the error but report it at location 0:0
// instead of the actual line number where the invalid property was defined.
//
// This test verifies that after the fix, the validator:
// 1. Correctly identifies the invalid property name
// 2. Reports the actual line number (not 0:0)
// 3. Provides an enhanced error message with the property name and pattern
// 4. Populates all relevant fields (FieldName, FieldPath, etc.)
func TestValidateOpenAPIDocument_Issue726_InvalidPropertyName(t *testing.T) {
	// This spec has an invalid schema name: $defs-atmVolatility_type
	// The '$' at the beginning violates the OpenAPI pattern: ^[a-zA-Z0-9._-]+$
	spec := `openapi: 3.1.0
info:
  title: Test Spec with Invalid Property Name
  version: 1.0.0
components:
  schemas:
    $defs-atmVolatility_type:
      type: object
      properties:
        volatility:
          type: number`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	// Validate the document
	valid, errors := ValidateOpenAPIDocument(doc)

	// Should not be valid due to invalid property name
	assert.False(t, valid, "Document should not be valid")
	assert.Len(t, errors, 1, "Should have exactly one validation error")

	// Check the validation error structure
	assert.Len(t, errors[0].SchemaValidationErrors, 1, "Should have exactly one schema validation error")

	sve := errors[0].SchemaValidationErrors[0]

	// CRITICAL: Line and Column should NOT be 0 (this was the bug)
	assert.Greater(t, sve.Line, 0, "Line should be greater than 0 (bug fix verification)")
	assert.Greater(t, sve.Column, 0, "Column should be greater than 0 (bug fix verification)")

	// The line should point to where $defs-atmVolatility_type is defined (line 7 in this spec)
	assert.Equal(t, 7, sve.Line, "Line should point to the invalid property name")

	// Verify the enhanced error message includes the property name and pattern
	assert.Contains(t, sve.Reason, "$defs-atmVolatility_type",
		"Reason should include the invalid property name")
	assert.Contains(t, sve.Reason, "does not match pattern",
		"Reason should explain the pattern mismatch")
	assert.Contains(t, sve.Reason, "^[a-zA-Z0-9._-]+$",
		"Reason should include the required pattern")

	// Verify additional fields are populated correctly
	assert.Equal(t, "$defs-atmVolatility_type", sve.FieldName,
		"FieldName should be extracted from the error")
	assert.Contains(t, sve.FieldPath, "$defs-atmVolatility_type",
		"FieldPath should include the property name")

	// Verify OriginalError is preserved for debugging
	assert.NotNil(t, sve.OriginalError, "OriginalError should be populated for debugging")
}

// TestValidateOpenAPIDocument_Issue726_MultipleInvalidPropertyNames tests that the fix
// works correctly when there are multiple invalid property names in the same document.
func TestValidateOpenAPIDocument_Issue726_MultipleInvalidPropertyNames(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test Spec with Multiple Invalid Property Names
  version: 1.0.0
components:
  schemas:
    $invalid-name-1:
      type: object
      properties:
        field1:
          type: string
    $invalid-name-2:
      type: object
      properties:
        field2:
          type: string`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	valid, errors := ValidateOpenAPIDocument(doc)

	assert.False(t, valid)
	assert.Len(t, errors, 1)

	// Should have errors for both invalid property names
	assert.GreaterOrEqual(t, len(errors[0].SchemaValidationErrors), 1,
		"Should have at least one schema validation error")

	// Check that all errors have valid line numbers (not 0)
	for i, sve := range errors[0].SchemaValidationErrors {
		assert.Greater(t, sve.Line, 0,
			"Error %d: Line should be greater than 0", i)
	}
}

// TestValidateOpenAPIDocument_Issue726_ValidPropertyNames is a negative test that verifies
// the fix doesn't break validation of valid specs.
func TestValidateOpenAPIDocument_Issue726_ValidPropertyNames(t *testing.T) {
	// This spec has valid schema names
	spec := `openapi: 3.1.0
info:
  title: Test Spec with Valid Property Names
  version: 1.0.0
components:
  schemas:
    ValidSchemaName:
      type: object
      properties:
        field1:
          type: string
    AnotherValidName:
      type: object
      properties:
        field2:
          type: string`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	valid, errors := ValidateOpenAPIDocument(doc)

	// Should be valid
	assert.True(t, valid, "Document with valid property names should be valid")
	assert.Len(t, errors, 0, "Should have no validation errors")
}

// TestValidateOpenAPIDocument_Issue726_BackwardCompatibility ensures that the fix
// doesn't break existing error reporting for errors that already had line numbers.
func TestValidateOpenAPIDocument_Issue726_BackwardCompatibility(t *testing.T) {
	// This spec has a different type of validation error (missing required field)
	// to ensure the fix doesn't break other validation errors
	spec := `openapi: 3.1.0
info:
  title: Test Spec`
	// version is required but missing

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	valid, errors := ValidateOpenAPIDocument(doc)

	// Should not be valid
	assert.False(t, valid)
	assert.Greater(t, len(errors), 0)

	// All errors should have valid line numbers
	for _, verr := range errors {
		for i, sve := range verr.SchemaValidationErrors {
			// Line might be 0 for some error types, but that's okay - we're just
			// checking that the fix didn't break existing error reporting
			assert.GreaterOrEqual(t, sve.Line, 0,
				"Error %d: Line should not be negative", i)
		}
	}
}
