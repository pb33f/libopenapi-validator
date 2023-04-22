// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package errors

import (
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// SchemaValidationFailure is a wrapper around the jsonschema.ValidationError object, to provide a more
// user-friendly way to break down what went wrong.
type SchemaValidationFailure struct {
	// Reason is a human-readable message describing the reason for the error.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`

	// Location is the XPath-like location of the validation failure
	Location string `json:"location,omitempty" yaml:"location,omitempty"`

	// Line is the line number where the violation occurred. This may a local line number
	// if the validation is a schema (only schemas are validated locally, so the line number will be relative to
	// the Context object held by the ValidationError object).
	Line int `json:"line,omitempty" yaml:"line,omitempty"`

	// Column is the column number where the violation occurred. This may a local column number
	// if the validation is a schema (only schemas are validated locally, so the column number will be relative to
	// the Context object held by the ValidationError object).
	Column int `json:"column,omitempty" yaml:"column,omitempty"`

	// The original error object, which is a jsonschema.ValidationError object.
	OriginalError *jsonschema.ValidationError `json:"-" yaml:"-"`
}

// Error returns a string representation of the error
func (s *SchemaValidationFailure) Error() string {
	return fmt.Sprintf("Reason: %s, Location: %s", s.Reason, s.Location)
}

// ValidationError is a struct that contains all the information about a validation error.
type ValidationError struct {

	// Message is a human-readable message describing the error.
	Message string `json:"message" yaml:"message"`

	// Reason is a human-readable message describing the reason for the error.
	Reason string `json:"reason" yaml:"reason"`

	// ValidationType is a string that describes the type of validation that failed.
	ValidationType string `json:"validationType" yaml:"validationType"`

	// ValidationSubType is a string that describes the subtype of validation that failed.
	ValidationSubType string `json:"validationSubType" yaml:"validationSubType"`

	// SpecLine is the line number in the spec where the error occurred.
	SpecLine int `json:"specLine" yaml:"specLine"`

	// SpecCol is the column number in the spec where the error occurred.
	SpecCol int `json:"specColumn" yaml:"specColumn"`

	// HowToFix is a human-readable message describing how to fix the error.
	HowToFix string `json:"howToFix" yaml:"howToFix"`

	// SchemaValidationErrors is a slice of SchemaValidationFailure objects that describe the validation errors
	// This is only populated whe the validation type is against a schema.
	SchemaValidationErrors []*SchemaValidationFailure `json:"validationErrors,omitempty" yaml:"validationErrors,omitempty"`

	// Context is the object that the validation error occurred on. This is usually a pointer to a schema
	// or a parameter object.
	Context interface{} `json:"-" yaml:"-"`
}

// Error returns a string representation of the error
func (v *ValidationError) Error() string {
	if v.SchemaValidationErrors != nil {
		return fmt.Sprintf("Error: %s, Reason: %s, Validation Errors: %s, Line: %d, Column: %d",
			v.Message, v.Reason, v.SchemaValidationErrors, v.SpecLine, v.SpecCol)
	} else {
		return fmt.Sprintf("Error: %s, Reason: %s, Line: %d, Column: %d",
			v.Message, v.Reason, v.SpecLine, v.SpecCol)
	}
}