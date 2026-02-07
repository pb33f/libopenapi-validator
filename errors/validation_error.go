// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package errors

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/pb33f/libopenapi-validator/helpers"
)

var instanceLocationRegex = regexp.MustCompile(`^/(\d+)`)

// SchemaValidationFailure is a wrapper around the jsonschema.ValidationError object, to provide a more
// user-friendly way to break down what went wrong.
type SchemaValidationFailure struct {
	// Reason is a human-readable message describing the reason for the error.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`

	// Location is the XPath-like location of the validation failure
	Location string `json:"location,omitempty" yaml:"location,omitempty"`

	// FieldName is the name of the specific field that failed validation (last segment of the path)
	FieldName string `json:"fieldName,omitempty" yaml:"fieldName,omitempty"`

	// FieldPath is the JSONPath representation of the field location (e.g., "$.user.email")
	FieldPath string `json:"fieldPath,omitempty" yaml:"fieldPath,omitempty"`

	// InstancePath is the raw path segments from the root to the failing field
	InstancePath []string `json:"instancePath,omitempty" yaml:"instancePath,omitempty"`

	// DeepLocation is the path to the validation failure as exposed by the jsonschema library.
	DeepLocation string `json:"deepLocation,omitempty" yaml:"deepLocation,omitempty"`

	// AbsoluteLocation is the absolute path to the validation failure as exposed by the jsonschema library.
	AbsoluteLocation string `json:"absoluteLocation,omitempty" yaml:"absoluteLocation,omitempty"`

	// Line is the line number where the violation occurred. This may a local line number
	// if the validation is a schema (only schemas are validated locally, so the line number will be relative to
	// the Context object held by the ValidationError object).
	Line int `json:"line,omitempty" yaml:"line,omitempty"`

	// Column is the column number where the violation occurred. This may a local column number
	// if the validation is a schema (only schemas are validated locally, so the column number will be relative to
	// the Context object held by the ValidationError object).
	Column int `json:"column,omitempty" yaml:"column,omitempty"`

	// Deprecated: Use GetReferenceSchema() instead for forward-compatible access.
	ReferenceSchema string `json:"referenceSchema,omitempty" yaml:"referenceSchema,omitempty"`

	// Deprecated: Use GetReferenceObject() instead for forward-compatible access.
	ReferenceObject string `json:"referenceObject,omitempty" yaml:"referenceObject,omitempty"`

	// lazySrc holds state for deferred resolution of ReferenceSchema and ReferenceObject.
	// This is only set when WithLazyErrors is enabled.
	lazySrc *lazySchemaSource

	// ReferenceExample is an example object generated from the schema that was referenced in the validation failure.
	ReferenceExample string `json:"referenceExample,omitempty" yaml:"referenceExample,omitempty"`

	// The original error object, which is a jsonschema.ValidationError object.
	OriginalError *jsonschema.ValidationError `json:"-" yaml:"-"`
}

// lazySchemaSource holds the data needed to resolve ReferenceSchema and ReferenceObject
// on demand when WithLazyErrors is enabled.
type lazySchemaSource struct {
	renderedInline []byte    // raw rendered schema bytes (for ReferenceSchema)
	decodedObj     any       // decoded request/response body
	bodyBytes      []byte    // raw request/response body bytes
	instanceLoc    string    // instance location from validation error (e.g. "/0")
	schemaOnce     sync.Once // ensures ReferenceSchema is resolved exactly once
	objectOnce     sync.Once // ensures ReferenceObject is resolved exactly once
}

// Error returns a string representation of the error
func (s *SchemaValidationFailure) Error() string {
	return fmt.Sprintf("Reason: %s, Location: %s", s.Reason, s.Location)
}

// GetReferenceSchema returns the reference schema string. In eager mode (default),
// this returns the pre-populated ReferenceSchema field. In lazy mode (WithLazyErrors),
// it resolves from the cached schema data on first call. Thread-safe via sync.Once.
func (s *SchemaValidationFailure) GetReferenceSchema() string {
	if s.ReferenceSchema != "" {
		return s.ReferenceSchema
	}
	if s.lazySrc != nil {
		s.lazySrc.schemaOnce.Do(func() {
			if s.lazySrc.renderedInline != nil {
				s.ReferenceSchema = string(s.lazySrc.renderedInline)
			}
		})
	}
	return s.ReferenceSchema
}

// GetReferenceObject returns the reference object string. In eager mode (default),
// this returns the pre-populated ReferenceObject field. In lazy mode (WithLazyErrors),
// it resolves from the decoded object data on first call.
func (s *SchemaValidationFailure) GetReferenceObject() string {
	if s.ReferenceObject != "" {
		return s.ReferenceObject
	}
	if s.lazySrc == nil {
		return s.ReferenceObject
	}
	s.resolveReferenceObject()
	return s.ReferenceObject
}

func (s *SchemaValidationFailure) resolveReferenceObject() {
	s.lazySrc.objectOnce.Do(func() {
		if s.lazySrc.decodedObj != nil && s.lazySrc.instanceLoc != "" {
			val := instanceLocationRegex.FindStringSubmatch(s.lazySrc.instanceLoc)
			if len(val) > 0 {
				referenceIndex, _ := strconv.Atoi(val[1])
				if reflect.ValueOf(s.lazySrc.decodedObj).Type().Kind() == reflect.Slice {
					found := s.lazySrc.decodedObj.([]any)[referenceIndex]
					recoded, _ := json.Marshal(found)
					s.ReferenceObject = string(recoded)
					return
				}
			}
		}
		if s.lazySrc.bodyBytes != nil {
			s.ReferenceObject = string(s.lazySrc.bodyBytes)
		}
	})
}

// SetLazySource configures the lazy resolution source for this failure.
// This is called by the validation code when WithLazyErrors is enabled.
func (s *SchemaValidationFailure) SetLazySource(renderedInline []byte, decodedObj any, bodyBytes []byte, instanceLoc string) {
	s.lazySrc = &lazySchemaSource{
		renderedInline: renderedInline,
		decodedObj:     decodedObj,
		bodyBytes:      bodyBytes,
		instanceLoc:    instanceLoc,
	}
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
	// This is only populated whe the validation type is against a schema.
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
	return v.ValidationType == helpers.PathValidation && v.ValidationSubType == helpers.ValidationMissing
}

// IsOperationMissingError returns true if the error has a ValidationType of "request" and a ValidationSubType of "missingOperation"
func (v *ValidationError) IsOperationMissingError() bool {
	return v.ValidationType == helpers.PathValidation && v.ValidationSubType == helpers.ValidationMissingOperation
}
