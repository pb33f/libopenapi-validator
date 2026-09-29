// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"strings"

	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// FindMediaType returns the content entry that applies to contentType, a Content-Type header value.
//
// Content keys may be media ranges, and OpenAPI applies only the most specific key that matches:
// an exact media type, then a structured syntax range (application/*+json), then a type range
// (application/*), then a subtype range (*/json), then */*. Keys are compared without their
// parameters, and case is ignored. Keys of equal specificity keep document order.
func FindMediaType(content *orderedmap.Map[string, *v3.MediaType], contentType string) (*v3.MediaType, bool) {
	if content == nil {
		return nil, false
	}
	mediaType, _, _ := ExtractContentType(contentType)
	if found, ok := content.Get(mediaType); ok {
		return found, true
	}

	typ, subtype, _ := strings.Cut(mediaType, "/")
	suffix := ""
	if plus := strings.LastIndexByte(subtype, '+'); plus >= 0 {
		suffix = subtype[plus:]
	}

	var found *v3.MediaType
	foundRank := 0
	for pair := content.First(); pair != nil; pair = pair.Next() {
		var rank int
		switch key := normalizeMediaRange(pair.Key()); key {
		case mediaType:
			rank = 5
		case typ + "/*" + suffix:
			rank = 4 // the same key as the type range below when there is no suffix
		case typ + "/*":
			rank = 3
		case "*/" + subtype:
			rank = 2
		case "*/*":
			rank = 1
		}
		if rank > foundRank {
			found, foundRank = pair.Value(), rank
		}
	}
	return found, foundRank > 0
}

// normalizeMediaRange lowercases a media type or range and drops its parameters.
func normalizeMediaRange(value string) string {
	base, _, _ := strings.Cut(value, ";")
	return strings.ToLower(strings.TrimSpace(base))
}
