// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"github.com/santhosh-tekuri/jsonschema/v6"
	"net/http"
	"sync"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
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

type Config struct {
	Document    *v3.Document
	RegexEngine jsonschema.RegexpEngine
}

// NewRequestBodyValidator will create a new RequestBodyValidator from an OpenAPI 3+ document
func NewRequestBodyValidator(config Config) RequestBodyValidator {
	return &requestBodyValidator{Config: config, schemaCache: &sync.Map{}}
}

type schemaCache struct {
	schema         *base.Schema
	renderedInline []byte
	renderedJSON   []byte
}

type requestBodyValidator struct {
	Config
	schemaCache *sync.Map
}
