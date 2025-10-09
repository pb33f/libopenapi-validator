// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package errors

import (
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaValidationFailure describes any failure that occurs when validating data
// against either an OpenAPI or JSON Schema. It aims to be a more user-friendly
// representation of the error than what is provided by the jsonschema library.
type SchemaValidationFailure struct {
	// Reason is a human-readable message describing the reason for the error.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`

	// InstancePath is the raw path segments from the root to the failing field
	InstancePath []string `json:"instancePath,omitempty" yaml:"instancePath,omitempty"`

	// FieldName is the name of the specific field that failed validation (last segment of the path)
	FieldName string `json:"fieldName,omitempty" yaml:"fieldName,omitempty"`

	// FieldPath is the JSONPath representation of the field location that failed validation (e.g., "$.user.email")
	FieldPath string `json:"fieldPath,omitempty" yaml:"fieldPath,omitempty"`

	// KeywordLocation is the relative path to the JsonSchema keyword that failed validation
	KeywordLocation string `json:"keywordLocation,omitempty" yaml:"keywordLocation,omitempty"`

	// AbsoluteKeywordLocation is the absolute path to the validation failure as exposed by the jsonschema library.
	AbsoluteKeywordLocation string `json:"absoluteKeywordLocation,omitempty" yaml:"absoluteKeywordLocation,omitempty"`

	// Line is the line number where the violation occurred. This may a local line number
	// if the validation is a schema (only schemas are validated locally, so the line number will be relative to
	// the Context object held by the ValidationError object).
	Line int `json:"line,omitempty" yaml:"line,omitempty"`

	// Column is the column number where the violation occurred. This may a local column number
	// if the validation is a schema (only schemas are validated locally, so the column number will be relative to
	// the Context object held by the ValidationError object).
	Column int `json:"column,omitempty" yaml:"column,omitempty"`

	// ReferenceSchema is the schema that was referenced in the validation failure.
	ReferenceSchema string `json:"referenceSchema,omitempty" yaml:"referenceSchema,omitempty"`

	// ReferenceObject is the object that failed schema validation
	ReferenceObject string `json:"referenceObject,omitempty" yaml:"referenceObject,omitempty"`

	// The original jsonschema.ValidationError object, if the schema failure originated from the jsonschema library.
	OriginalJsonSchemaError *jsonschema.ValidationError `json:"-" yaml:"-"`

	// DEPRECATED in favor of explicit use of FieldPath & InstancePath
	// Location is the XPath-like location of the validation failure
	Location string `json:"location,omitempty" yaml:"location,omitempty"`
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

	// RequestPath is the path of the request
	RequestPath string `json:"requestPath" yaml:"requestPath"`

	// SpecPath is the path from the specification that corresponds to the request
	SpecPath string `json:"specPath" yaml:"specPath"`

	// RequestMethod is the HTTP method of the request
	RequestMethod string `json:"requestMethod" yaml:"requestMethod"`

	// ParameterName is the name of the parameter that failed validation (for parameter validation errors)
	ParameterName string `json:"parameterName,omitempty" yaml:"parameterName,omitempty"`

	// SchemaValidationErrors is a slice of SchemaValidationFailure objects that describe the validation errors
	// This is only populated when the validation type is against a schema.
	SchemaValidationErrors []*SchemaValidationFailure `json:"validationErrors,omitempty" yaml:"validationErrors,omitempty"`

	// Context is the object that the validation error occurred on. This is usually a pointer to a schema
	// or a parameter object.
	Context interface{} `json:"-" yaml:"-"`
}

// Error returns a string representation of the error
func (v *ValidationError) Error() string {
	if v.SchemaValidationErrors != nil {
		if v.SpecLine > 0 && v.SpecCol > 0 {
			return fmt.Sprintf("Error: %s, Reason: %s, Validation Errors: %s, Line: %d, Column: %d",
				v.Message, v.Reason, v.SchemaValidationErrors, v.SpecLine, v.SpecCol)
		} else {
			return fmt.Sprintf("Error: %s, Reason: %s, Validation Errors: %s",
				v.Message, v.Reason, v.SchemaValidationErrors)
		}
	} else {
		if v.SpecLine > 0 && v.SpecCol > 0 {
			return fmt.Sprintf("Error: %s, Reason: %s, Line: %d, Column: %d",
				v.Message, v.Reason, v.SpecLine, v.SpecCol)
		} else {
			return fmt.Sprintf("Error: %s, Reason: %s",
				v.Message, v.Reason)
		}
	}
}

// IsPathMissingError returns true if the error has a ValidationType of "path" and a ValidationSubType of "missing"
func (v *ValidationError) IsPathMissingError() bool {
	return v.ValidationType == "path" && v.ValidationSubType == "missing"
}

// IsOperationMissingError returns true if the error has a ValidationType of "request" and a ValidationSubType of "missingOperation"
func (v *ValidationError) IsOperationMissingError() bool {
	return v.ValidationType == "path" && v.ValidationSubType == "missingOperation"
}
