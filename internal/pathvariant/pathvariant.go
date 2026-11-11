// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package pathvariant lists the other ways a server or router may read an escaped request path.
//
// Routers disagree on percent-encoding and path cleaning. net/http ServeMux decodes each segment
// before comparing it, gin and gorilla/mux route on the fully decoded path, chi and echo route on
// the escaped path, and some routers remove empty and dot segments. A validator that reads a path
// one way, in front of a router that reads it another, can validate one operation while the router
// runs a different one. Comparing the operation selected by each reading closes that gap.
package pathvariant

import (
	"path"
	"slices"
	"strings"
)

// Variants returns the readings of an escaped request path that differ from the path as sent:
// with percent-encoded path characters decoded, with encoded slashes also decoded, and each of
// those with empty and dot segments removed. It returns nil when every reading is the path itself.
func Variants(escapedPath string) []string {
	if !mayVary(escapedPath) {
		return nil
	}
	decoded := decode(escapedPath, false)
	split := decode(escapedPath, true)
	var variants []string
	for _, variant := range []string{decoded, split, clean(escapedPath), clean(decoded), clean(split)} {
		if variant != escapedPath && !slices.Contains(variants, variant) {
			variants = append(variants, variant)
		}
	}
	return variants
}

// CollapseSlashes replaces each run of slashes in a path with a single slash.
func CollapseSlashes(p string) string {
	var b strings.Builder
	b.Grow(len(p))
	for i := 0; i < len(p); i++ {
		if p[i] != '/' || i == 0 || p[i-1] != '/' {
			b.WriteByte(p[i])
		}
	}
	return b.String()
}

// mayVary reports whether a path contains a percent-encoding, an empty segment, or a dot segment.
func mayVary(p string) bool {
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '%':
			return true
		case '/':
			if i+1 < len(p) && (p[i+1] == '/' || p[i+1] == '.') {
				return true
			}
		}
	}
	return false
}

// decode decodes each percent-encoded octet that may appear unencoded in a path segment
// (RFC 3986 pchar: unreserved, sub-delims, ":" and "@") or that is part of a UTF-8 sequence.
// With slashes set it also decodes "%2F". Every other octet, including "%25", stays encoded, so
// the result is still an escaped path and is never decoded twice.
func decode(p string, slashes bool) string {
	var b strings.Builder
	changed := false
	for i := 0; i < len(p); i++ {
		if p[i] == '%' && i+2 < len(p) {
			if c, ok := unhex(p[i+1], p[i+2]); ok && (pathChar(c) || (slashes && c == '/')) {
				if !changed {
					changed = true
					b.Grow(len(p))
					b.WriteString(p[:i])
				}
				b.WriteByte(c)
				i += 2
				continue
			}
		}
		if changed {
			b.WriteByte(p[i])
		}
	}
	if !changed {
		return p
	}
	return b.String()
}

// clean removes empty and dot segments, keeping a trailing slash, as net/http ServeMux does.
func clean(p string) string {
	cleaned := path.Clean(p)
	if strings.HasSuffix(p, "/") && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned
}

func pathChar(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c >= 0x80:
		return true
	}
	return strings.IndexByte("-._~!$&'()*+,;=:@", c) >= 0
}

func unhex(hi, lo byte) (byte, bool) {
	h, ok := hexValue(hi)
	if !ok {
		return 0, false
	}
	l, ok := hexValue(lo)
	if !ok {
		return 0, false
	}
	return h<<4 | l, true
}

func hexValue(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
