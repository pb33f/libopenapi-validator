// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package helpers

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
)

func TestDiveIntoValidationError(t *testing.T) {
	tests := []struct {
		name     string
		error    *jsonschema.ValidationError
		expected string
	}{
		{
			name: "empty instance location",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{},
			},
			expected: "",
		},
		{
			name: "numeric path segments",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"root", "array", "0", "1"},
			},
			expected: "$.root.array[0][1]",
		},
		{
			name: "simple identifier path segments",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"user", "name", "first"},
			},
			expected: "$.user.name.first",
		},
		{
			name: "complex path segments requiring escaping",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"user", "name-with-dash", "special'quote", "back\\slash"},
			},
			expected: "$.user['name-with-dash']['special\\'quote']['back\\\\slash']",
		},
		{
			name: "mixed path segments",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"users", "0", "address", "street-name", "123"},
			},
			expected: "$.users[0].address['street-name'][123]",
		},
		{
			name: "with nested causes",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"root"},
				Causes: []*jsonschema.ValidationError{
					{
						InstanceLocation: []string{"nested", "error"},
					},
				},
			},
			expected: "$.root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSONPathFromValidationError(tt.error)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"01", true},
		{"", false},
		{"abc", false},
		{"123abc", false},
		{"12.3", false},
		{"-123", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSimpleIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc", true},
		{"a123", true},
		{"_abc", true},
		{"_123", true},
		{"abc_123", true},
		{"", false},
		{"123abc", false},
		{"abc-def", false},
		{"abc.def", false},
		{"abc def", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isSimpleIdentifier(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEscapeBracketString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"with'quote", "with\\'quote"},
		{"with\\backslash", "with\\\\backslash"},
		{"with'quote\\and\\backslash", "with\\'quote\\\\and\\\\backslash"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeBracketString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDiveIntoValidationErrorRecursion tests that the function properly handles
// recursive traversal through nested validation errors.
func TestDiveIntoValidationErrorRecursion(t *testing.T) {
	childError1 := &jsonschema.ValidationError{
		InstanceLocation: []string{"child1", "prop"},
	}

	childError2 := &jsonschema.ValidationError{
		InstanceLocation: []string{"child2", "0", "name"},
	}

	parentError := &jsonschema.ValidationError{
		InstanceLocation: []string{"parent"},
		Causes:           []*jsonschema.ValidationError{childError1, childError2},
	}

	// The parent error should return its own path
	result := ExtractJSONPathFromValidationError(parentError)
	assert.Equal(t, "$.parent", result)

	// Verify the child errors return their paths correctly when called directly
	assert.Equal(t, "$.child1.prop", ExtractJSONPathFromValidationError(childError1))
	assert.Equal(t, "$.child2[0].name", ExtractJSONPathFromValidationError(childError2))
}

// TestDiveIntoValidationErrorEdgeCases tests edge cases including empty strings and unusual characters
func TestDiveIntoValidationErrorEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		error    *jsonschema.ValidationError
		expected string
	}{
		{
			name: "empty strings as elements",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"", "property"},
			},
			expected: "$[''].property",
		},
		{
			name: "Unicode characters",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"🙂", "unicode_property"},
			},
			expected: "$['🙂'].unicode_property",
		},
		{
			name: "null causes",
			error: &jsonschema.ValidationError{
				InstanceLocation: []string{"root"},
				Causes:           nil,
			},
			expected: "$.root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSONPathFromValidationError(tt.error)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractJSONPathsFromValidationErrors tests the ExtractJSONPathsFromValidationErrors function
func TestExtractJSONPathsFromValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []*jsonschema.ValidationError
		expected []string
	}{
		{
			name:     "nil errors",
			errors:   nil,
			expected: nil,
		},
		{
			name:     "empty errors",
			errors:   []*jsonschema.ValidationError{},
			expected: nil,
		},
		{
			name: "single error with empty path",
			errors: []*jsonschema.ValidationError{
				{
					InstanceLocation: []string{},
				},
			},
			expected: nil,
		},
		{
			name: "single error with path",
			errors: []*jsonschema.ValidationError{
				{
					InstanceLocation: []string{"root", "property"},
				},
			},
			expected: []string{"$.root.property"},
		},
		{
			name: "multiple errors with paths",
			errors: []*jsonschema.ValidationError{
				{
					InstanceLocation: []string{"users", "0", "name"},
				},
				{
					InstanceLocation: []string{"users", "1", "address", "street"},
				},
			},
			expected: []string{"$.users[0].name", "$.users[1].address.street"},
		},
		{
			name: "mixed errors - some with empty paths",
			errors: []*jsonschema.ValidationError{
				{
					InstanceLocation: []string{},
				},
				{
					InstanceLocation: []string{"users", "0", "name"},
				},
				{
					InstanceLocation: []string{},
				},
			},
			expected: []string{"$.users[0].name"},
		},
		{
			name: "complex paths with special characters",
			errors: []*jsonschema.ValidationError{
				{
					InstanceLocation: []string{"data", "special-field", "nested"},
				},
				{
					InstanceLocation: []string{"data", "array", "0", "item's", "property"},
				},
			},
			expected: []string{"$.data['special-field'].nested", "$.data.array[0]['item\\'s'].property"},
		},
		{
			name: "with nested causes",
			errors: []*jsonschema.ValidationError{
				{
					InstanceLocation: []string{"parent"},
					Causes: []*jsonschema.ValidationError{
						{
							InstanceLocation: []string{"child", "property"},
						},
					},
				},
			},
			expected: []string{"$.parent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSONPathsFromValidationErrors(tt.errors)
			assert.Equal(t, tt.expected, result)
		})
	}
}
