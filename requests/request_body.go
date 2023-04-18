// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

// RequestBodyValidator is an interface that defines the methods for validating request bodies for Operations.
//
//  ValidateRequestBody method accepts an *http.Request and returns true if validation passed,
//                      false if validation failed and a slice of ValidationError pointers.
type RequestBodyValidator interface {
    ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError)
}

// NewRequestBodyValidator will create a new RequestBodyValidator from an OpenAPI 3+ document
func NewRequestBodyValidator(document *v3.Document) RequestBodyValidator {
    return &requestBodyValidator{document: document}
}

type requestBodyValidator struct {
    document *v3.Document
    errors   []*errors.ValidationError
}
