// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package serverurl

import (
	"testing"

	"github.com/pb33f/testify/assert"
)

func TestBasePath(t *testing.T) {
	for serverURL, expected := range map[string]string{
		"https://api.example.com/api/v1":         "/api/v1",
		"https://api.example.com":                "",
		"https://api.example.com/":               "/",
		"/api/v1":                                "/api/v1",
		"":                                       "",
		"https://{host}/api/v1":                  "/api/v1",
		"https://{host}":                         "",
		"https://{host}/":                        "/",
		"https://{host}//more//paths":            "/more//paths",
		"https://api-{env}.example.com/v2":       "/v2",
		"http://localhost:{port}/v1":             "/v1",
		"{scheme}://api.example.com/v1":          "/v1",
		"https://api.example.com/{version}/api":  "/{version}/api",
		"https://{host}/bad%zzpath":              "",
		"https://{host}/encoded%20path/resource": "/encoded path/resource",
	} {
		assert.Equal(t, expected, BasePath(serverURL), serverURL)
	}
}

func TestTrimBasePath(t *testing.T) {
	for _, tc := range []struct {
		path, base, rest string
		ok               bool
	}{
		{"/api", "/api", "", true},
		{"/api/", "/api", "/", true},
		{"/api/status", "/api", "/status", true},
		{"/apistatus", "/api", "/apistatus", false},
		{"/api-v2/status", "/api", "/api-v2/status", false},
		{"/other", "/api", "/other", false},
		{"/api/status", "/api/", "status", true},
		{"/api", "/api/", "/api", false},
		{"/status", "/", "status", true},
	} {
		rest, ok := TrimBasePath(tc.path, tc.base)
		assert.Equal(t, tc.rest, rest, "%s - %s", tc.path, tc.base)
		assert.Equal(t, tc.ok, ok, "%s - %s", tc.path, tc.base)
	}
}
