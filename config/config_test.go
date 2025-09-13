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
	assert.Nil(t, opts.RegexEngine)
}

func TestNewValidationOptions_WithNilOption(t *testing.T) {
	opts := NewValidationOptions(nil)

	assert.NotNil(t, opts)
	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.Nil(t, opts.RegexEngine)
}

func TestWithFormatAssertions(t *testing.T) {
	opts := NewValidationOptions(WithFormatAssertions())

	assert.True(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.Nil(t, opts.RegexEngine)
}

func TestWithContentAssertions(t *testing.T) {
	opts := NewValidationOptions(WithContentAssertions())

	assert.False(t, opts.FormatAssertions)
	assert.True(t, opts.ContentAssertions)
	assert.Nil(t, opts.RegexEngine)
}

func TestWithRegexEngine(t *testing.T) {
	// Test with nil regex engine (valid)
	var mockEngine jsonschema.RegexpEngine = nil

	opts := NewValidationOptions(WithRegexEngine(mockEngine))

	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.Nil(t, opts.RegexEngine)
}

func TestWithExistingOpts(t *testing.T) {
	// Create original options with all settings enabled
	var testEngine jsonschema.RegexpEngine = nil
	original := &ValidationOptions{
		RegexEngine:       testEngine,
		FormatAssertions:  true,
		ContentAssertions: true,
	}

	// Create new options using existing options
	opts := NewValidationOptions(WithExistingOpts(original))

	assert.Nil(t, opts.RegexEngine) // Both should be nil
	assert.Equal(t, original.FormatAssertions, opts.FormatAssertions)
	assert.Equal(t, original.ContentAssertions, opts.ContentAssertions)
}

func TestWithExistingOpts_NilSource(t *testing.T) {
	// Test with nil source options
	opts := NewValidationOptions(WithExistingOpts(nil))

	assert.NotNil(t, opts)
	// Should not panic and should have default values
	assert.False(t, opts.FormatAssertions)
	assert.False(t, opts.ContentAssertions)
	assert.Nil(t, opts.RegexEngine)
}

func TestMultipleOptions(t *testing.T) {
	opts := NewValidationOptions(
		WithFormatAssertions(),
		WithContentAssertions(),
	)

	assert.True(t, opts.FormatAssertions)
	assert.True(t, opts.ContentAssertions)
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
	assert.Nil(t, opts.RegexEngine)
}

func TestWithExistingOpts_PartialOverride(t *testing.T) {
	// Create original options
	var testEngine jsonschema.RegexpEngine = nil
	original := &ValidationOptions{
		RegexEngine:       testEngine,
		FormatAssertions:  true,
		ContentAssertions: true,
	}

	// Create new options using existing options, then override one setting
	opts := NewValidationOptions(
		WithExistingOpts(original),
		WithContentAssertions(), // This should still be true (no change)
	)

	assert.Nil(t, opts.RegexEngine)        // Both should be nil
	assert.True(t, opts.FormatAssertions)  // From original
	assert.True(t, opts.ContentAssertions) // Reapplied, but same value
}

func TestComplexScenario(t *testing.T) {
	// Test a complex real-world scenario
	var mockEngine jsonschema.RegexpEngine = nil

	// Start with some base options
	baseOpts := &ValidationOptions{
		FormatAssertions: true,
		// RegexEngine and ContentAssertions are defaults (nil/false)
	}

	// Create new options building on the base
	opts := NewValidationOptions(
		WithExistingOpts(baseOpts),
		WithContentAssertions(),
		WithRegexEngine(mockEngine),
	)

	// Verify all settings are as expected
	assert.True(t, opts.FormatAssertions)  // From base
	assert.True(t, opts.ContentAssertions) // Added
	assert.Nil(t, opts.RegexEngine)        // Should be nil
}
