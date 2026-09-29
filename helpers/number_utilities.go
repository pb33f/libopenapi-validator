// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// ParseInteger parses a parameter value as a JSON Schema integer: any number with a zero
// fractional part, so "1.0" and "1e3" are integers as well as "1". Values outside the int64
// range, and anything that is not a decimal number, return an error. A value written with a
// fraction or exponent is read as a float64, so it is exact up to 2^53.
func ParseInteger(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err == nil || errors.Is(err, strconv.ErrRange) {
		return parsed, err
	}
	f, floatErr := ParseNumber(value)
	if floatErr != nil || f != math.Trunc(f) || math.Abs(f) >= math.MaxInt64 {
		return 0, err
	}
	return int64(f), nil
}

// ParseNumber parses a parameter value as a JSON Schema number, which is always finite.
// strconv.ParseFloat alone also reads NaN, Inf, hex and underscores, none of which are JSON numbers.
func ParseNumber(value string) (float64, error) {
	if strings.ContainsFunc(value, notDecimalNumberRune) {
		return 0, &strconv.NumError{Func: "ParseFloat", Num: value, Err: strconv.ErrSyntax}
	}
	// a decimal number too large for a float64 returns an ErrRange error, never an infinity
	return strconv.ParseFloat(value, 64)
}

// notDecimalNumberRune reports whether r cannot appear in a decimal number such as "-1.5e3".
func notDecimalNumberRune(r rune) bool {
	return (r < '0' || r > '9') && !strings.ContainsRune("+-.eE", r)
}
