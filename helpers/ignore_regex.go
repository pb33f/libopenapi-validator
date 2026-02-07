// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package helpers

import (
	"regexp"
	"strings"
)

var (
	// Ignore generic poly errors that just say "none matched" since we get specific errors
	// But keep errors that say which subschemas matched (for multiple match scenarios)
	IgnorePattern     = `^'?(anyOf|allOf|oneOf|validation)'? failed(, none matched)?$`
	IgnorePolyPattern = `^'?(anyOf|allOf|oneOf)'? failed(, none matched)?$`
)

// IgnoreRegex is a regular expression that matches the IgnorePattern
//
// Deprecated: Use ShouldIgnoreError instead.
var IgnoreRegex = regexp.MustCompile(IgnorePattern)

// IgnorePolyRegex is a regular expression that matches the IgnorePattern
//
// Deprecated: Use ShouldIgnorePolyError instead.
var IgnorePolyRegex = regexp.MustCompile(IgnorePolyPattern)

// ShouldIgnoreError checks if an error message should be ignored.
// Replaces the previous IgnoreRegex for better performance.
// Matches messages like: "anyOf failed", "'allOf' failed, none matched", "validation failed"
func ShouldIgnoreError(msg string) bool {
	return isIgnoredValidationError(msg, true)
}

// ShouldIgnorePolyError checks if a polymorphic error message should be ignored.
// Replaces the previous IgnorePolyRegex.
// Like ShouldIgnoreError but does NOT match "validation failed".
func ShouldIgnorePolyError(msg string) bool {
	return isIgnoredValidationError(msg, false)
}

func isIgnoredValidationError(msg string, includeValidation bool) bool {
	// Strip optional quotes
	s := msg
	if len(s) > 0 && s[0] == '\'' {
		s = s[1:]
	}

	// Check prefix
	var rest string
	switch {
	case strings.HasPrefix(s, "anyOf"):
		rest = s[5:]
	case strings.HasPrefix(s, "allOf"):
		rest = s[5:]
	case strings.HasPrefix(s, "oneOf"):
		rest = s[5:]
	case includeValidation && strings.HasPrefix(s, "validation"):
		rest = s[10:]
	default:
		return false
	}

	// Strip optional closing quote
	if len(rest) > 0 && rest[0] == '\'' {
		rest = rest[1:]
	}

	// Must be followed by " failed" with optional ", none matched"
	return rest == " failed" || rest == " failed, none matched"
}
