// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"testing"

	"github.com/pb33f/testify/assert"
)

func TestParseInteger(t *testing.T) {
	for value, expected := range map[string]int64{
		"1":                   1,
		"-7":                  -7,
		"+3":                  3,
		"007":                 7,
		"1.0":                 1,
		"-2.000":              -2,
		"1e3":                 1000,
		"2.5E1":               25,
		"-0.0":                0,
		"9223372036854775807": 9223372036854775807,
	} {
		parsed, err := ParseInteger(value)
		assert.NoError(t, err, value)
		assert.Equal(t, expected, parsed, value)
	}

	for _, value := range []string{
		"", "abc", "1.5", "1e-3", "0x10", "0x1p3", "1_000", "NaN", "Inf", "-infinity",
		"9223372036854775808", "-9223372036854775809", "1e19", "-1e19", "1e400",
	} {
		_, err := ParseInteger(value)
		assert.Error(t, err, value)
	}
}

func TestParseNumber(t *testing.T) {
	for value, expected := range map[string]float64{
		"1": 1, "-2.5": -2.5, "+3": 3, "1e3": 1000, "2.5E-1": 0.25, "1e-400": 0,
	} {
		parsed, err := ParseNumber(value)
		assert.NoError(t, err, value)
		assert.Equal(t, expected, parsed, value)
	}

	for _, value := range []string{"", "abc", "NaN", "Inf", "+Inf", "-infinity", "0x1p3", "1_000", "1e400", "-1e400", "1.2.3"} {
		_, err := ParseNumber(value)
		assert.Error(t, err, value)
	}
}
