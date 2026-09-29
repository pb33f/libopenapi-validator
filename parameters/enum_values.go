// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi-validator/helpers"
)

// enumValueMatches reports whether a parameter value matches an enum value. Values are compared
// as text, and an integer parameter (parsed as an int64) is also compared as a number, so that
// "1.0" and "01" both match an enum value of 1.
func enumValueMatches(value string, parsed any, enumValue any) bool {
	enumText := fmt.Sprint(enumValue)
	if strings.TrimSpace(value) == enumText {
		return true
	}
	integer, isInteger := parsed.(int64)
	if !isInteger {
		return false
	}
	enumInteger, err := helpers.ParseInteger(enumText)
	return err == nil && enumInteger == integer
}
