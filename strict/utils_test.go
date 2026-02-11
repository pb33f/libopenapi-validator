// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package strict

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompilePattern_EscapedDoubleAsterisk(t *testing.T) {
	// Pattern \*\* should match literal "**" in path (lines 78-80)
	// This escapes the glob ** so it matches the literal string **
	re := compilePattern(`$.body.\*\*`)
	assert.NotNil(t, re)

	// Should match literal ** in path
	assert.True(t, re.MatchString("$.body.**"))

	// Should NOT match arbitrary depth (that's what unescaped ** does)
	assert.False(t, re.MatchString("$.body.foo.bar"))
}

func TestCompilePattern_EscapedNonAsterisk(t *testing.T) {
	// Pattern with escaped character that's not * (lines 88-90)
	// \n should match literal 'n', \. should match literal '.'
	re := compilePattern(`$.body\nvalue`)
	assert.NotNil(t, re)

	// Should match with literal 'n' (the escape just includes the next char)
	assert.True(t, re.MatchString("$.bodynvalue"))
}

func TestCompilePattern_EscapedDot(t *testing.T) {
	// Escaped dot should be literal dot
	re := compilePattern(`$.body\.name`)
	assert.NotNil(t, re)

	// Should match path with literal dot
	assert.True(t, re.MatchString("$.body.name"))
}

func TestCompilePattern_Empty(t *testing.T) {
	// Empty pattern returns nil
	re := compilePattern("")
	assert.Nil(t, re)
}

func TestBuildPath_WithDot(t *testing.T) {
	// Property with dot uses bracket notation
	result := buildPath("$.body", "a.b")
	assert.Equal(t, "$.body['a.b']", result)
}

func TestBuildPath_WithBrackets(t *testing.T) {
	// Property with brackets uses bracket notation
	result := buildPath("$.body", "x[0]")
	assert.Equal(t, "$.body['x[0]']", result)
}

func TestBuildPath_Simple(t *testing.T) {
	// Simple property uses dot notation
	result := buildPath("$.body", "name")
	assert.Equal(t, "$.body.name", result)
}

func TestBuildArrayPath(t *testing.T) {
	result := buildArrayPath("$.body.items", 5)
	assert.Equal(t, "$.body.items[5]", result)
}

func TestCompileIgnorePaths_Empty(t *testing.T) {
	result := compileIgnorePaths(nil)
	assert.Nil(t, result)

	result = compileIgnorePaths([]string{})
	assert.Nil(t, result)
}

func TestCompileIgnorePaths_WithPatterns(t *testing.T) {
	patterns := []string{
		"$.body.metadata",
		"$.body.items[*].internal",
	}
	result := compileIgnorePaths(patterns)
	assert.Len(t, result, 2)
}

func TestTruncateValue_LongString(t *testing.T) {
	// String > 50 chars gets truncated
	longStr := "this is a very long string that exceeds fifty characters in length"
	result := TruncateValue(longStr)
	assert.Equal(t, "this is a very long string that exceeds fifty c...", result)
}

func TestTruncateValue_ShortString(t *testing.T) {
	shortStr := "short"
	result := TruncateValue(shortStr)
	assert.Equal(t, "short", result)
}

func TestTruncateValue_LargeMap(t *testing.T) {
	// Map with > 3 keys shows {...}
	m := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}
	result := TruncateValue(m)
	assert.Equal(t, "{...}", result)
}

func TestTruncateValue_SmallMap(t *testing.T) {
	m := map[string]any{"a": 1, "b": 2}
	result := TruncateValue(m)
	assert.Equal(t, m, result)
}

func TestTruncateValue_LargeSlice(t *testing.T) {
	// Slice with > 3 elements shows [...]
	s := []any{1, 2, 3, 4}
	result := TruncateValue(s)
	assert.Equal(t, "[...]", result)
}

func TestTruncateValue_SmallSlice(t *testing.T) {
	s := []any{1, 2}
	result := TruncateValue(s)
	assert.Equal(t, s, result)
}

func TestTruncateValue_OtherTypes(t *testing.T) {
	// Other types returned as-is
	assert.Equal(t, 42, TruncateValue(42))
	assert.Equal(t, true, TruncateValue(true))
	assert.Equal(t, 3.14, TruncateValue(3.14))
}
