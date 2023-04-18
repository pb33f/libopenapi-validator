// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
    "github.com/pb33f/libopenapi-validator/errors"
    v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

// ParameterValidator is an interface that defines the methods for validating parameters
// There are 4 types of parameters: query, header, cookie and path.
//
//  ValidateQueryParams will validate the query parameters for the request
//  ValidateHeaderParams will validate the header parameters for the request
//  ValidateCookieParams will validate the cookie parameters for the request
//  ValidatePathParams will validate the path parameters for the request
//
// Each method accepts an *http.Request and returns true if validation passed,
// false if validation failed and a slice of ValidationError pointers.
type ParameterValidator interface {
    ValidateQueryParams(request *http.Request) (bool, []*errors.ValidationError)
    ValidateHeaderParams(request *http.Request) (bool, []*errors.ValidationError)
    ValidateCookieParams(request *http.Request) (bool, []*errors.ValidationError)
    ValidatePathParams(request *http.Request) (bool, []*errors.ValidationError)
}

// NewParameterValidator will create a new ParameterValidator from an OpenAPI 3+ document
func NewParameterValidator(document *v3.Document) ParameterValidator {
    return &paramValidator{document: document}
}

type paramValidator struct {
    document *v3.Document
    errors   []*errors.ValidationError
}
