// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "fmt"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

type ValidationError struct {
    Message  string `json:"message" yaml:"message"`
    Reason   string `json:"reason" yaml:"reason"`
    SpecLine int    `json:"specLine" yaml:"specLine"`
    SpecCol  int    `json:"specColumn" yaml:"specColumn"`
}

func (v *ValidationError) Error() string {
    return fmt.Sprintf("Error: %s, Reason: %s, Line: %d, Column: %d",
        v.Message, v.Reason, v.SpecLine, v.SpecCol)
}

type Validator interface {
    ValidateHttpRequest(request *http.Request) (bool, []*ValidationError)
    ValidateQueryParams(request *http.Request) (bool, []*ValidationError)
    FindPath(request *http.Request) (*v3.PathItem, []*ValidationError)
    AllValidationErrors() []*ValidationError
}

type validator struct {
    document *v3.Document
    errors   []*ValidationError
}

func NewValidator(document *v3.Document) Validator {
    return &validator{document: document}
}

func (v *validator) ValidateHttpRequest(request *http.Request) (bool, []*ValidationError) {

    // find path
    pathItem, errs := v.FindPath(request)
    if pathItem == nil || errs != nil {
        return false, errs
    }

    // validate query params
    //if !v.validateQueryParams(request) {
    //    return false, v.errors
    //}
    return false, nil
}

func (v *validator) AllValidationErrors() []*ValidationError {
    return v.errors
}
