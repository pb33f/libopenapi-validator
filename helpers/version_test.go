package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionToFloat(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected float32
	}{
		{
			name:     "OpenAPI 3.0",
			version:  "3.0",
			expected: 3.0,
		},
		{
			name:     "OpenAPI 3.0.0",
			version:  "3.0.0",
			expected: 3.0,
		},
		{
			name:     "OpenAPI 3.0.3",
			version:  "3.0.3",
			expected: 3.0,
		},
		{
			name:     "OpenAPI 3.1",
			version:  "3.1",
			expected: 3.1,
		},
		{
			name:     "OpenAPI 3.1.0",
			version:  "3.1.0",
			expected: 3.1,
		},
		{
			name:     "OpenAPI 3.1.1",
			version:  "3.1.1",
			expected: 3.1,
		},
		{
			name:     "default to 3.1 for unknown version",
			version:  "4.0",
			expected: 3.1,
		},
		{
			name:     "default to 3.1 for empty string",
			version:  "",
			expected: 3.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VersionToFloat(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}
