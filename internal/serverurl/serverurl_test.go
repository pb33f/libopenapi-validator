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
