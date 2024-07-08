// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"net/http"
	"sync"
)

// RequestBodyValidator is an interface that defines the methods for validating request bodies for Operations.
//
//	ValidateRequestBodyWithPathItem method accepts an *http.Request and returns true if validation passed,
//	                    false if validation failed and a slice of ValidationError pointers.
type RequestBodyValidator interface {
	// ValidateRequestBody will validate the request body for an operation. The first return value will be true if the
	// request body is valid, false if it is not. The second return value will be a slice of ValidationError pointers if
	// the body is not valid.
	ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError)

	// ValidateRequestBodyWithPathItem will validate the request body for an operation. The first return value will be true if the
	// request body is valid, false if it is not. The second return value will be a slice of ValidationError pointers if
	// the body is not valid.
	ValidateRequestBodyWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError)
}

// NewRequestBodyValidator will create a new RequestBodyValidator from an OpenAPI 3+ document
func NewRequestBodyValidator(document *v3.Document) RequestBodyValidator {
	return &requestBodyValidator{document: document, schemaCache: &sync.Map{}}
}


type schemaCache struct {
	schema         *base.Schema
	renderedInline []byte
	renderedJSON   []byte
}

type requestBodyValidator struct {
	document    *v3.Document
	errors      []*errors.ValidationError
	schemaCache *sync.Map
}
