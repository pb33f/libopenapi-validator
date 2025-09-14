// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
)

func TestNewValidationOptions_Defaults(t *testing.T) {
	opts := NewValidationOptions()

	assert.NotNil(t, opts)
	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestNewValidationOptions_WithNilOption(t *testing.T) {
	opts := NewValidationOptions(nil)

	assert.NotNil(t, opts)
	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestWithFormatAssertions(t *testing.T) {
	opts := NewValidationOptions(WithFormatAssertions())

	assert.True(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestWithContentAssertions(t *testing.T) {
	opts := NewValidationOptions(WithContentAssertions())

	assert.False(t, opts.FormatAssertions)
	assert.True(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestWithoutSecurityValidation(t *testing.T) {
	opts := NewValidationOptions(WithoutSecurityValidation())

	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.False(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestWithRegexEngine(t *testing.T) {
	// Test with nil regex engine (valid)
	var mockEngine jsonschema.RegexpEngine = nil

	opts := NewValidationOptions(WithRegexEngine(mockEngine))

	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestWithExistingOpts(t *testing.T) {
	// Create original options with all settings enabled
	var testEngine jsonschema.RegexpEngine = nil
	original := &ValidationOptions{
		RegexEngine:        testEngine,
		FormatAssertions:   true,
		ContentAssertions:  true,
		SecurityValidation: false,
	}

	// Create new options using existing options
	opts := NewValidationOptions(WithExistingOpts(original))

	assert.Nil(t, opts.RegexEngine) // Both should be nil
	assert.Equal(t, original.FormatAssertions, opts.FormatAssertions)
	assert.Equal(t, original.ContentAssertions, opts.ContentAssertions)
	assert.Equal(t, original.SecurityValidation, opts.SecurityValidation)
}

func TestWithExistingOpts_NilSource(t *testing.T) {
	// Test with nil source options
	opts := NewValidationOptions(WithExistingOpts(nil))

	assert.NotNil(t, opts)
	// Should not panic and should have default values
	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestMultipleOptions(t *testing.T) {
	opts := NewValidationOptions(
		WithFormatAssertions(),
		WithContentAssertions(),
	)

	assert.True(t, opts.FormatAssertions)
	assert.True(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestOptionOverride(t *testing.T) {
	// Test that later options override earlier ones
	// First set format assertions, then turn them off by not setting them again
	opts := NewValidationOptions(
		WithFormatAssertions(),
		WithContentAssertions(),
	)

	assert.True(t, opts.FormatAssertions)
	assert.True(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
	assert.True(t, opts.OpenAPIMode)          // Default is true
	assert.False(t, opts.AllowScalarCoercion) // Default is false
	assert.Nil(t, opts.RegexEngine)
}

func TestWithExistingOpts_PartialOverride(t *testing.T) {
	// Create original options
	var testEngine jsonschema.RegexpEngine = nil
	original := &ValidationOptions{
		RegexEngine:        testEngine,
		FormatAssertions:   true,
		ContentAssertions:  true,
		SecurityValidation: false,
	}

	// Create new options using existing options, then override one setting
	opts := NewValidationOptions(
		WithExistingOpts(original),
		WithContentAssertions(), // This should still be true (no change)
	)

	assert.Nil(t, opts.RegexEngine)          // Both should be nil
	assert.True(t, opts.FormatAssertions)    // From original
	assert.True(t, opts.ContentAssertions)   // Reapplied, but same value
	assert.False(t, opts.SecurityValidation) // From original
}

func TestComplexScenario(t *testing.T) {
	// Test a complex real-world scenario
	var mockEngine jsonschema.RegexpEngine = nil

	// Start with some base options
	baseOpts := &ValidationOptions{
		FormatAssertions:   true,
		SecurityValidation: false,
		// RegexEngine and ContentAssertions are defaults (nil/false)
	}

	// Create new options building on the base
	opts := NewValidationOptions(
		WithExistingOpts(baseOpts),
		WithContentAssertions(),
		WithRegexEngine(mockEngine),
	)

	// Verify all settings are as expected
	assert.True(t, opts.FormatAssertions)    // From base
	assert.True(t, opts.ContentAssertions)   // Added
	assert.False(t, opts.SecurityValidation) // From base
	assert.Nil(t, opts.RegexEngine)          // Should be nil
}

func TestMultipleOptionsWithSecurityDisabled(t *testing.T) {
	opts := NewValidationOptions(
		WithFormatAssertions(),
		WithContentAssertions(),
		WithoutSecurityValidation(),
	)

	assert.True(t, opts.FormatAssertions)
	assert.True(t, opts.ContentAssertions)
	assert.False(t, opts.SecurityValidation)
	assert.Nil(t, opts.RegexEngine)
}

func TestWithExistingOpts_SecurityValidationCopied(t *testing.T) {
	// Test that SecurityValidation is properly copied
	original := &ValidationOptions{
		SecurityValidation: false,
	}

	opts := NewValidationOptions(WithExistingOpts(original))

	assert.False(t, opts.SecurityValidation)

	// Test the opposite
	original2 := &ValidationOptions{
		SecurityValidation: true,
	}

	opts2 := NewValidationOptions(WithExistingOpts(original2))

	assert.True(t, opts2.SecurityValidation)
}

// Tests for new OpenAPI and scalar coercion configuration options

func TestWithOpenAPIMode(t *testing.T) {
	opts := NewValidationOptions(WithOpenAPIMode())

	assert.True(t, opts.OpenAPIMode)
	assert.False(t, opts.AllowScalarCoercion) // Should be default false
	assert.False(t, opts.FormatAssertions)    // Should be default false
	assert.False(t, opts.ContentAssertions)   // Should be default false
	assert.True(t, opts.SecurityValidation)   // Should be default true
}

func TestWithoutOpenAPIMode(t *testing.T) {
	opts := NewValidationOptions(WithoutOpenAPIMode())

	assert.False(t, opts.OpenAPIMode)
	assert.False(t, opts.AllowScalarCoercion) // Should be default false
	assert.False(t, opts.FormatAssertions)    // Should be default false
	assert.False(t, opts.ContentAssertions)   // Should be default false
	assert.True(t, opts.SecurityValidation)   // Should be default true
}

func TestWithScalarCoercion(t *testing.T) {
	opts := NewValidationOptions(WithScalarCoercion())

	assert.True(t, opts.AllowScalarCoercion)
	assert.True(t, opts.OpenAPIMode)        // Should be default true
	assert.False(t, opts.FormatAssertions)  // Should be default false
	assert.False(t, opts.ContentAssertions) // Should be default false
	assert.True(t, opts.SecurityValidation) // Should be default true
}

func TestWithOpenAPIModeAndScalarCoercion(t *testing.T) {
	opts := NewValidationOptions(
		WithOpenAPIMode(),
		WithScalarCoercion(),
	)

	assert.True(t, opts.OpenAPIMode)
	assert.True(t, opts.AllowScalarCoercion)
	assert.False(t, opts.FormatAssertions)  // Should be default false
	assert.False(t, opts.ContentAssertions) // Should be default false
	assert.True(t, opts.SecurityValidation) // Should be default true
}

func TestWithOpenAPIModeOverride(t *testing.T) {
	// Test that WithoutOpenAPIMode can override WithOpenAPIMode
	opts := NewValidationOptions(
		WithOpenAPIMode(),
		WithoutOpenAPIMode(),
	)

	assert.False(t, opts.OpenAPIMode) // Should be false (last option wins)
	assert.False(t, opts.AllowScalarCoercion)
}

func TestComplexOpenAPIScenario(t *testing.T) {
	// Test a complex scenario with OpenAPI mode and other options
	opts := NewValidationOptions(
		WithFormatAssertions(),
		WithOpenAPIMode(),
		WithScalarCoercion(),
		WithContentAssertions(),
		WithoutSecurityValidation(),
	)

	assert.True(t, opts.OpenAPIMode)
	assert.True(t, opts.AllowScalarCoercion)
	assert.True(t, opts.FormatAssertions)
	assert.True(t, opts.ContentAssertions)
	assert.False(t, opts.SecurityValidation)
	assert.Nil(t, opts.RegexEngine)
}

func TestWithExistingOpts_OpenAPIFields(t *testing.T) {
	// Test that OpenAPI fields are properly copied from existing options
	original := &ValidationOptions{
		OpenAPIMode:         true,
		AllowScalarCoercion: true,
		FormatAssertions:    false,
		ContentAssertions:   false,
		SecurityValidation:  true,
	}

	opts := NewValidationOptions(WithExistingOpts(original))

	assert.True(t, opts.OpenAPIMode)
	assert.True(t, opts.AllowScalarCoercion)
	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.True(t, opts.SecurityValidation)
}

func TestWithCustomFormat(t *testing.T) {
	// Test WithCustomFormat option
	testFormatFunc := func(v any) error {
		return nil // Simple test format function
	}

	opts := NewValidationOptions(WithCustomFormat("test-format", testFormatFunc))

	assert.NotNil(t, opts.Formats)
	assert.Contains(t, opts.Formats, "test-format")
	assert.NotNil(t, opts.Formats["test-format"])
}
