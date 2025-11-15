// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeJSONPointerSegment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "tilde only",
			input:    "some~thing",
			expected: "some~0thing",
		},
		{
			name:     "slash only",
			input:    "path/to/something",
			expected: "path~1to~1something",
		},
		{
			name:     "both tilde and slash",
			input:    "path/with~special/chars~",
			expected: "path~1with~0special~1chars~0",
		},
		{
			name:     "path template",
			input:    "/users/{id}",
			expected: "~1users~1{id}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeJSONPointerSegment(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstructParameterJSONPointer(t *testing.T) {
	tests := []struct {
		name         string
		pathTemplate string
		method       string
		paramName    string
		keyword      string
		expected     string
	}{
		{
			name:         "simple path with query parameter type",
			pathTemplate: "/users",
			method:       "GET",
			paramName:    "limit",
			keyword:      "type",
			expected:     "/paths/users/get/parameters/limit/schema/type",
		},
		{
			name:         "path with parameter and enum keyword",
			pathTemplate: "/users/{id}",
			method:       "POST",
			paramName:    "status",
			keyword:      "enum",
			expected:     "/paths/users~1{id}/post/parameters/status/schema/enum",
		},
		{
			name:         "path with tilde character",
			pathTemplate: "/some~path",
			method:       "PUT",
			paramName:    "value",
			keyword:      "format",
			expected:     "/paths/some~0path/put/parameters/value/schema/format",
		},
		{
			name:         "path with multiple slashes",
			pathTemplate: "/api/v1/users/{userId}/posts/{postId}",
			method:       "DELETE",
			paramName:    "filter",
			keyword:      "required",
			expected:     "/paths/api~1v1~1users~1{userId}~1posts~1{postId}/delete/parameters/filter/schema/required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstructParameterJSONPointer(tt.pathTemplate, tt.method, tt.paramName, tt.keyword)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstructResponseHeaderJSONPointer(t *testing.T) {
	tests := []struct {
		name         string
		pathTemplate string
		method       string
		statusCode   string
		headerName   string
		keyword      string
		expected     string
	}{
		{
			name:         "simple response header",
			pathTemplate: "/health",
			method:       "GET",
			statusCode:   "200",
			headerName:   "X-Request-ID",
			keyword:      "required",
			expected:     "/paths/health/get/responses/200/headers/X-Request-ID/required",
		},
		{
			name:         "path with parameter",
			pathTemplate: "/users/{id}",
			method:       "POST",
			statusCode:   "201",
			headerName:   "Location",
			keyword:      "schema",
			expected:     "/paths/users~1{id}/post/responses/201/headers/Location/schema",
		},
		{
			name:         "path with tilde and slash",
			pathTemplate: "/some~path/to/resource",
			method:       "PUT",
			statusCode:   "204",
			headerName:   "ETag",
			keyword:      "type",
			expected:     "/paths/some~0path~1to~1resource/put/responses/204/headers/ETag/type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstructResponseHeaderJSONPointer(tt.pathTemplate, tt.method, tt.statusCode, tt.headerName, tt.keyword)
			assert.Equal(t, tt.expected, result)
		})
	}
}

