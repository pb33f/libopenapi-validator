// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package errors

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

// Helper function to create a mock ValidationError
func createMockValidationError() *ValidationError {
	return &ValidationError{
		Message: "Test validation error",
	}
}

func TestPopulateValidationErrors(t *testing.T) {
	// Create a mock request
	req, _ := http.NewRequest(http.MethodGet, "/test/path", nil)

	// Create mock validation errors
	validationErrors := []*ValidationError{
		createMockValidationError(),
		createMockValidationError(),
	}

	// Call the function
	PopulateValidationErrors(validationErrors, req, "/spec/path")

	// Validate the results
	for _, validationError := range validationErrors {
		require.Equal(t, "/spec/path", validationError.SpecPath)
		require.Equal(t, http.MethodGet, validationError.RequestMethod)
		require.Equal(t, "/test/path", validationError.RequestPath)
	}
}
