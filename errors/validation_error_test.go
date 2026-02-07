// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/helpers"
)

func TestSchemaValidationFailure_Error(t *testing.T) {
	// Test the Error method of SchemaValidationFailure
	s := &SchemaValidationFailure{
		Reason:   "Invalid type",
		Location: "/path/to/property",
	}

	expectedError := "Reason: Invalid type, Location: /path/to/property"
	require.Equal(t, expectedError, s.Error())
}

func TestValidationError_Error_NoSchemaValidationErrors(t *testing.T) {
	// Test the Error method of ValidationError with no SchemaValidationErrors and no line/column info
	v := &ValidationError{
		Message: "Missing required field",
		Reason:  "The field 'id' is required but missing",
	}

	expectedError := "Error: Missing required field, Reason: The field 'id' is required but missing"
	require.Equal(t, expectedError, v.Error())
}

func TestValidationError_Error_WithSpecLineAndColumn(t *testing.T) {
	// Test the Error method of ValidationError with spec line and column
	v := &ValidationError{
		Message:  "Invalid data type",
		Reason:   "Expected 'string', got 'integer'",
		SpecLine: 10,
		SpecCol:  15,
	}

	expectedError := "Error: Invalid data type, Reason: Expected 'string', got 'integer', Line: 10, Column: 15"
	require.Equal(t, expectedError, v.Error())
}

func TestValidationError_Error_WithSchemaValidationErrors(t *testing.T) {
	// Test the Error method of ValidationError with SchemaValidationErrors
	schemaError := &SchemaValidationFailure{
		Reason:   "Invalid enum value",
		Location: "/path/to/enum",
	}
	v := &ValidationError{
		Message:                "Enum validation failed",
		Reason:                 "Invalid enum value",
		SchemaValidationErrors: []*SchemaValidationFailure{schemaError},
	}

	expectedError := fmt.Sprintf("Error: Enum validation failed, Reason: Invalid enum value, Validation Errors: %s", []*SchemaValidationFailure{schemaError})
	require.Equal(t, expectedError, v.Error())
}

func TestValidationError_Error_WithSchemaValidationErrors_AndSpecLineColumn(t *testing.T) {
	// Test the Error method of ValidationError with SchemaValidationErrors and SpecLine and SpecCol
	schemaError := &SchemaValidationFailure{
		Reason:   "Invalid enum value",
		Location: "/path/to/enum",
	}
	v := &ValidationError{
		Message:                "Enum validation failed",
		Reason:                 "Invalid enum value",
		SchemaValidationErrors: []*SchemaValidationFailure{schemaError},
		SpecLine:               12,
		SpecCol:                5,
	}

	expectedError := fmt.Sprintf("Error: Enum validation failed, Reason: Invalid enum value, Validation Errors: %s, Line: 12, Column: 5", []*SchemaValidationFailure{schemaError})
	require.Equal(t, expectedError, v.Error())
}

func TestValidationError_IsPathMissingError(t *testing.T) {
	// Test the IsPathMissingError method
	v := &ValidationError{
		ValidationType:    helpers.PathValidation,
		ValidationSubType: helpers.ValidationMissing,
	}

	require.True(t, v.IsPathMissingError())

	// Test with different ValidationSubType
	v.ValidationSubType = "wrongType"
	require.False(t, v.IsPathMissingError())

	// Test with different ValidationType
	v.ValidationType = helpers.RequestValidation
	v.ValidationSubType = helpers.ValidationMissing
	require.False(t, v.IsPathMissingError())
}

func TestValidationError_IsOperationMissingError(t *testing.T) {
	// Test the IsOperationMissingError method
	v := &ValidationError{
		ValidationType:    helpers.PathValidation,
		ValidationSubType: helpers.ValidationMissingOperation,
	}

	require.True(t, v.IsOperationMissingError())

	// Test with different ValidationSubType
	v.ValidationSubType = "wrongOperation"
	require.False(t, v.IsOperationMissingError())

	// Test with different ValidationType
	v.ValidationType = helpers.RequestValidation
	v.ValidationSubType = helpers.ValidationMissingOperation
	require.False(t, v.IsOperationMissingError())
}
