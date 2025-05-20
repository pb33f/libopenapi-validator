// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package helpers

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ExtractJSONPathFromValidationError traverses and processes a ValidationError to construct a JSONPath string representation of its instance location.
func ExtractJSONPathFromValidationError(e *jsonschema.ValidationError) string {
	if len(e.Causes) > 0 {
		for _, cause := range e.Causes {
			ExtractJSONPathFromValidationError(cause)
		}
	}

	if len(e.InstanceLocation) > 0 {

		var b strings.Builder
		b.WriteString("$")

		for _, seg := range e.InstanceLocation {
			switch {
			case isNumeric(seg):
				b.WriteString(fmt.Sprintf("[%s]", seg))

			case isSimpleIdentifier(seg):
				b.WriteByte('.')
				b.WriteString(seg)

			default:
				esc := escapeBracketString(seg)
				b.WriteString("['")
				b.WriteString(esc)
				b.WriteString("']")
			}
		}
		return b.String()
	}
	return ""
}

// isNumeric returns true if s is a non‐empty string of digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isSimpleIdentifier returns true if s matches [A-Za-z_][A-Za-z0-9_]*.
func isSimpleIdentifier(s string) bool {
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	return len(s) > 0
}

// escapeBracketString escapes backslashes and single‐quotes for inside ['...']
func escapeBracketString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

// ExtractJSONPathsFromValidationErrors takes a slice of ValidationError pointers and returns a slice of JSONPath strings
func ExtractJSONPathsFromValidationErrors(errors []*jsonschema.ValidationError) []string {
	var paths []string
	for _, err := range errors {
		path := ExtractJSONPathFromValidationError(err)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
