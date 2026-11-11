// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package pathvariant

import (
	"testing"

	"github.com/pb33f/testify/assert"
)

func TestVariants(t *testing.T) {
	for _, tc := range []struct {
		path     string
		expected []string
	}{
		// nothing to normalize
		{"/users/me", nil},
		{"", nil},
		{"/", nil},
		{"/users/me.json", nil},
		// percent-encodings that decode to the same reading
		{"/users/a%20b", nil},
		{"/users/%25", nil},
		{"/users/%zz", nil},
		{"/users/%6", nil},
		{"/users/%6z", nil},
		{"/users/%", nil},
		// a dot segment-looking name that is not one
		{"/.well-known/openid", nil},
		// over-encoded path characters, upper and lower case hex
		{"/users/m%65", []string{"/users/me"}},
		{"/users/%6D%65", []string{"/users/me"}},
		{"/users/%6d%65", []string{"/users/me"}},
		{"/v1/items%3Abatch", []string{"/v1/items:batch"}},
		{"/a/%7E%21%24%26%27%28%29%2A%2B%2C%3B%3D%40%2D%2E%5F", []string{"/a/~!$&'()*+,;=@-._"}},
		{"/caf%C3%A9", []string{"/caf\xc3\xa9"}},
		// encoded characters that are not path characters stay encoded
		{"/a%23b%3Fc%25d%2Fe", []string{"/a%23b%3Fc%25d/e"}},
		// "%2541" is a literal "%41", never a second "A"
		{"/m%2541", nil},
		// encoded slashes split only in the split reading
		{"/users/5%2Fsecrets", []string{"/users/5/secrets"}},
		{"/users/5%2fsecrets", []string{"/users/5/secrets"}},
		{"/users/m%65%2Fx", []string{"/users/me%2Fx", "/users/me/x"}},
		// empty and dot segments, trailing slash kept
		{"/items//5", []string{"/items/5"}},
		{"/items//", []string{"/items/"}},
		{"/users/x/../me", []string{"/users/me"}},
		{"/users/./me/", []string{"/users/me/"}},
		{"//", []string{"/"}},
		{"/..", []string{"/"}},
		// encoded dots and slashes become dot segments only once decoded
		{"/users/%2E%2E/admin", []string{"/users/../admin", "/admin"}},
		{"/a/b%2F..%2Fc", []string{"/a/b/../c", "/a/c"}},
		{"/a%2F%2Fb", []string{"/a//b", "/a/b"}},
	} {
		assert.Equal(t, tc.expected, Variants(tc.path), tc.path)
	}
}

func TestCollapseSlashes(t *testing.T) {
	for path, expected := range map[string]string{
		"":                "",
		"/":               "/",
		"//":              "/",
		"/items//5":       "/items/5",
		"///items///5///": "/items/5/",
		"/users/a%2F%2Fb": "/users/a%2F%2Fb",
		"items//5":        "items/5",
	} {
		assert.Equal(t, expected, CollapseSlashes(path), path)
	}
}

func TestDecodeKeepsPathWithoutDecodableOctets(t *testing.T) {
	path := "/users/a%20b"
	assert.Equal(t, path, decode(path, true))
	assert.Equal(t, "/users/me", decode("/users/m%65", false))
	assert.Equal(t, "e/x", decode("%65/x", false))
}
