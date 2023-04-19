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
//	ValidateRequestBody method accepts an *http.Request and returns true if validation passed,
//	                    false if validation failed and a slice of ValidationError pointers.
type RequestBodyValidator interface {
	ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError)
	SetPathItem(path *v3.PathItem, pathValue string)
}

// NewRequestBodyValidator will create a new RequestBodyValidator from an OpenAPI 3+ document
func NewRequestBodyValidator(document *v3.Document) RequestBodyValidator {
	return &requestBodyValidator{document: document}
}

// SetPathItem will set the pathItem for the RequestBodyValidator, all validations will be performed
// against this pathItem otherwise if not set, each validation will perform a lookup for the pathItem
// based on the *http.Request
func (v *requestBodyValidator) SetPathItem(path *v3.PathItem, pathValue string) {
	v.pathItem = path
	v.pathValue = pathValue
}

type requestBodyValidator struct {
	document  *v3.Document
	pathItem  *v3.PathItem
	pathValue string
	errors    []*errors.ValidationError
}
