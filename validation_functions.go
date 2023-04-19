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
	//ValidateHttpResponse(request *http.Request, response *http.Response) (bool, []*errors.ValidationError)
	//ValidateDocument() (bool, []*errors.ValidationError)
	//GetParameterValidator() parameters.ParameterValidator
	//GetRequestBodyValidator() requests.RequestBodyValidator
	//GetResponseBodyValidator() responses.ResponseBodyValidator
}

type validator struct {
	document *v3.Document
	errors   []*errors.ValidationError
}

// NewValidator will create a new Validator from an OpenAPI 3+ document
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
	//if !v.validateQueryParams(requests) {
	//    return false, v.errors
	//}
	return false, nil
}
