// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package helpers

import "regexp"

var (
	IgnorePattern     = `^'?(anyOf|allOf|oneOf|validation)'? failed\b`
	IgnorePolyPattern = `^'?(anyOf|allOf|oneOf)'? failed\b`
)

// IgnoreRegex is a regular expression that matches the IgnorePattern
var IgnoreRegex = regexp.MustCompile(IgnorePattern)

// IgnorePolyRegex is a regular expression that matches the IgnorePattern
var IgnorePolyRegex = regexp.MustCompile(IgnorePolyPattern)
