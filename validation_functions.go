// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

type Validator interface {
    ValidateHttpRequest(request *http.Request) (bool, []*errors.ValidationError)
    AllValidationErrors() []*errors.ValidationError
}

type validator struct {
    document *v3.Document
    errors   []*errors.ValidationError
}

func NewValidator(document *v3.Document) Validator {
    return &validator{document: document}
}

func (v *validator) ValidateHttpRequest(request *http.Request) (bool, []*errors.ValidationError) {

    // find path
    pathItem, errs, _ := paths.FindPath(request, v.document)
    if pathItem == nil || errs != nil {
        v.errors = errs
        return false, errs
    }

    // validate query params
    //if !v.validateQueryParams(request) {
    //    return false, v.errors
    //}
    return false, nil
}

func (v *validator) AllValidationErrors() []*errors.ValidationError {
    return v.errors
}
