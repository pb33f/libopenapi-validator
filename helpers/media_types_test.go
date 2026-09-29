// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"testing"

	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/testify/assert"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func mediaTypeContent(keys ...string) *orderedmap.Map[string, *v3.MediaType] {
	content := orderedmap.New[string, *v3.MediaType]()
	for _, key := range keys {
		content.Set(key, &v3.MediaType{})
	}
	return content
}

func TestFindMediaType(t *testing.T) {
	all := mediaTypeContent("*/*", "*/json", "application/*", "application/*+json", "application/json")

	for _, test := range []struct {
		name        string
		content     *orderedmap.Map[string, *v3.MediaType]
		contentType string
		expected    string
	}{
		{"exact beats every range", all, "application/json; charset=utf-8", "application/json"},
		{"structured syntax range beats type range", all, "application/problem+json", "application/*+json"},
		{"type range beats subtype range", all, "application/xml", "application/*"},
		{"subtype range beats */*", all, "text/json", "*/json"},
		{"*/* matches anything", all, "image/png", "*/*"},
		{"type range without suffix range", mediaTypeContent("*/*", "application/*"), "application/problem+json", "application/*"},
		{"keys ignore case and parameters", mediaTypeContent("Text/Plain; charset=utf-8"), "text/plain", "Text/Plain; charset=utf-8"},
		{"content type ignores case", mediaTypeContent("application/json"), "Application/JSON", "application/json"},
		{"equal specificity keeps document order", mediaTypeContent("text/*", "TEXT/*"), "text/csv", "text/*"},
		{"no slash only matches ranges it fits", mediaTypeContent("application/json", "application/*"), "application", "application/*"},
		{"unparseable content type only matches */*", mediaTypeContent("application/json", "*/*"), "application/", "*/*"},
	} {
		t.Run(test.name, func(t *testing.T) {
			found, ok := FindMediaType(test.content, test.contentType)
			assert.True(t, ok)
			assert.Same(t, test.content.GetOrZero(test.expected), found)
		})
	}

	for _, test := range []struct {
		name        string
		content     *orderedmap.Map[string, *v3.MediaType]
		contentType string
	}{
		{"nil content", nil, "application/json"},
		{"no matching key", mediaTypeContent("application/json", "text/*"), "image/png"},
		{"no slash and no range", mediaTypeContent("application/json"), "application"},
		{"ranges are not matched in reverse", mediaTypeContent("application/json"), "application/*"},
	} {
		t.Run(test.name, func(t *testing.T) {
			found, ok := FindMediaType(test.content, test.contentType)
			assert.False(t, ok)
			assert.Nil(t, found)
		})
	}
}
