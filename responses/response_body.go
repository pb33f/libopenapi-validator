// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"net/http"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
)

// ResponseBodyValidator is an interface that defines the methods for validating response bodies for Operations.
//
//	ValidateResponseBody method accepts an *http.Request and returns true if validation passed,
//	                     false if validation failed and a slice of ValidationError pointers.
type ResponseBodyValidator interface {
	// ValidateResponseBody will validate the response body for a http.Response pointer. The request is used to
	// locate the operation in the specification, the response is used to ensure the response code, media type and the
	// schema of the response body are valid.
	ValidateResponseBody(request *http.Request, response *http.Response) (bool, []*errors.ValidationError)

	// ValidateResponseBodyWithPathItem will validate the response body for a http.Response pointer. The request is used to
	// locate the operation in the specification, the response is used to ensure the response code, media type and the
	// schema of the response body are valid.
	ValidateResponseBodyWithPathItem(request *http.Request, response *http.Response, pathItem *v3.PathItem, pathFound string) (bool, []*errors.ValidationError)

	// Release clears validator-owned options and drops the OpenAPI document reference.
	Release()
}

// NewResponseBodyValidator will create a new ResponseBodyValidator from an OpenAPI 3+ document
func NewResponseBodyValidator(document *v3.Document, opts ...config.Option) ResponseBodyValidator {
	options := config.NewValidationOptions(opts...)

	return &responseBodyValidator{options: options, document: document}
}

type responseBodyValidator struct {
	options  *config.ValidationOptions
	document *v3.Document
}

func (r *responseBodyValidator) Release() {
	if r == nil {
		return
	}
	if r.options != nil {
		r.options.Release()
		r.options = nil
	}
	r.document = nil
}
