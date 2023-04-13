// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package errors

import (
    "fmt"
    "github.com/santhosh-tekuri/jsonschema/v5"
)

type SchemaValidationFailure struct {
    Reason        string                      `json:"reason,omitempty" yaml:"reason,omitempty"`
    Location      string                      `json:"location,omitempty" yaml:"location,omitempty"`
    Line          int                         `json:"line,omitempty" yaml:"line,omitempty"`
    Column        int                         `json:"column,omitempty" yaml:"column,omitempty"`
    OriginalError *jsonschema.ValidationError `json:"-" yaml:"-"`
}

func (s *SchemaValidationFailure) Error() string {
    return fmt.Sprintf("Reason: %s, Location: %s", s.Reason, s.Location)
}

type ValidationError struct {
    Message                string                     `json:"message" yaml:"message"`
    ValidationType         string                     `json:"validationType" yaml:"validationType"`
    ValidationSubType      string                     `json:"validationSubType" yaml:"validationSubType"`
    Reason                 string                     `json:"reason" yaml:"reason"`
    SpecLine               int                        `json:"specLine" yaml:"specLine"`
    SpecCol                int                        `json:"specColumn" yaml:"specColumn"`
    HowToFix               string                     `json:"howToFix" yaml:"howToFix"`
    SchemaValidationErrors []*SchemaValidationFailure `json:"validationErrors,omitempty" yaml:"validationErrors,omitempty"`
    Context                interface{}                `json:"-" yaml:"-"`
}

func (v *ValidationError) Error() string {
    if v.SchemaValidationErrors != nil {
        return fmt.Sprintf("Error: %s, Reason: %s, Validation Errors: %s, Line: %d, Column: %d",
            v.Message, v.Reason, v.SchemaValidationErrors, v.SpecLine, v.SpecCol)
    } else {
        return fmt.Sprintf("Error: %s, Reason: %s, Line: %d, Column: %d",
            v.Message, v.Reason, v.SpecLine, v.SpecCol)
    }
}
