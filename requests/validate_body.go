// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/utils"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
)

func (v *requestBodyValidator) ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.document)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateRequestBodyWithPathItem(request, pathItem, foundPath)
}

func (v *requestBodyValidator) ValidateRequestBodyWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
	if pathItem == nil {
		return false, []*errors.ValidationError{{
			ValidationType:    helpers.ParameterValidationPath,
			ValidationSubType: "missing",
			Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
			Reason: fmt.Sprintf("The %s request contains a path of '%s' "+
				"however that path, or the %s method for that path does not exist in the specification",
				request.Method, request.URL.Path, request.Method),
			SpecLine: -1,
			SpecCol:  -1,
			HowToFix: errors.HowToFixPath,
		}}
	}
	operation := helpers.ExtractOperation(request, pathItem)
	if operation == nil {
		return false, []*errors.ValidationError{errors.OperationNotFound(pathItem, request, request.Method, pathValue)}
	}
	if operation.RequestBody == nil {
		return true, nil
	}

	// extract the content type from the request
	contentType := request.Header.Get(helpers.ContentTypeHeader)
	required := false
	if operation.RequestBody.Required != nil {
		required = *operation.RequestBody.Required
	}
	if contentType == "" {
		if !required {
			// request body is not required, the validation stop there.
			return true, nil
		}
		return false, []*errors.ValidationError{errors.RequestContentTypeNotFound(operation, request, pathValue)}
	}

	// extract the media type from the content type header.
	ct, _, _ := helpers.ExtractContentType(contentType)
	mediaType, ok := operation.RequestBody.Content.Get(ct)
	if !ok {
		return false, []*errors.ValidationError{errors.RequestContentTypeNotFound(operation, request, pathValue)}
	}

	// we currently only support JSON validation for request bodies
	// this will capture *everything* that contains some form of 'json' in the content type
	if !strings.Contains(strings.ToLower(contentType), helpers.JSONType) {
		return true, nil
	}

	// Nothing to validate
	if mediaType.Schema == nil {
		return true, nil
	}

	// extract schema from media type
	var schema *base.Schema
	var renderedInline, renderedJSON []byte

	// have we seen this schema before? let's hash it and check the cache.
	hash := mediaType.GoLow().Schema.Value.Hash()

	// perform work only once and cache the result in the validator.
	if cacheHit, ch := v.schemaCache.Load(hash); ch {
		// got a hit, use cached values
		schema = cacheHit.(*schemaCache).schema
		renderedInline = cacheHit.(*schemaCache).renderedInline
		renderedJSON = cacheHit.(*schemaCache).renderedJSON

	} else {

		// render the schema inline and perform the intensive work of rendering and converting
		// this is only performed once per schema and cached in the validator.
		schema = mediaType.Schema.Schema()
		renderedInline, _ = schema.RenderInline()
		renderedJSON, _ = utils.ConvertYAMLtoJSON(renderedInline)
		v.schemaCache.Store(hash, &schemaCache{
			schema:         schema,
			renderedInline: renderedInline,
			renderedJSON:   renderedJSON,
		})
	}

	// render the schema, to be used for validation
	validationSucceeded, validationErrors := ValidateRequestSchema(request, schema, renderedInline, renderedJSON)

	errors.PopulateValidationErrors(validationErrors, request, pathValue)

	return validationSucceeded, validationErrors
}
