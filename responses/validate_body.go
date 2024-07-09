// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/libopenapi/utils"
)

func (v *responseBodyValidator) ValidateResponseBody(
	request *http.Request,
	response *http.Response,
) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.document)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateResponseBodyWithPathItem(request, response, pathItem, foundPath)
}

func (v *responseBodyValidator) ValidateResponseBodyWithPathItem(request *http.Request, response *http.Response, pathItem *v3.PathItem, pathFound string) (bool, []*errors.ValidationError) {
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
	var validationErrors []*errors.ValidationError
	operation := helpers.ExtractOperation(request, pathItem)
	if operation == nil {
		return false, []*errors.ValidationError{errors.OperationNotFound(pathItem, request, request.Method, pathFound)}
	}
	// extract the response code from the response
	httpCode := response.StatusCode
	contentType := response.Header.Get(helpers.ContentTypeHeader)
	codeStr := strconv.Itoa(httpCode)

	// extract the media type from the content type header.
	mediaTypeSting, _, _ := helpers.ExtractContentType(contentType)

	// check if the response code is in the contract
	foundResponse := operation.Responses.Codes.GetOrZero(codeStr)
	if foundResponse == nil {
		// check range definition for response codes
		foundResponse = operation.Responses.Codes.GetOrZero(fmt.Sprintf("%dXX", httpCode/100))
		if foundResponse != nil {
			codeStr = fmt.Sprintf("%dXX", httpCode/100)
		}
	}

	if foundResponse != nil {
		if foundResponse.Content != nil { // only validate if we have content types.
			// check content type has been defined in the contract
			if mediaType, ok := foundResponse.Content.Get(mediaTypeSting); ok {
				validationErrors = append(validationErrors,
					v.checkResponseSchema(request, response, mediaTypeSting, mediaType)...)
			} else {
				// check that the operation *actually* returns a body. (i.e. a 204 response)
				if foundResponse.Content != nil && orderedmap.Len(foundResponse.Content) > 0 {

					// content type not found in the contract
					validationErrors = append(validationErrors,
						errors.ResponseContentTypeNotFound(operation, request, response, codeStr, false))

				}
			}
		}
	} else {
		// no code match, check for default response
		if operation.Responses.Default != nil && operation.Responses.Default.Content != nil {
			// check content type has been defined in the contract
			if mediaType, ok := operation.Responses.Default.Content.Get(mediaTypeSting); ok {
				validationErrors = append(validationErrors,
					v.checkResponseSchema(request, response, contentType, mediaType)...)
			} else {
				// check that the operation *actually* returns a body. (i.e. a 204 response)
				if operation.Responses.Default.Content != nil && orderedmap.Len(operation.Responses.Default.Content) > 0 {

					// content type not found in the contract
					validationErrors = append(validationErrors,
						errors.ResponseContentTypeNotFound(operation, request, response, codeStr, true))
				}
			}
		} else {
			// TODO: add support for '2XX' and '3XX' responses in the contract
			// no default, no code match, nothing!
			validationErrors = append(validationErrors,
				errors.ResponseCodeNotFound(operation, request, httpCode))
		}
	}

	errors.PopulateValidationErrors(validationErrors, request, pathFound)

	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}

func (v *responseBodyValidator) checkResponseSchema(
	request *http.Request,
	response *http.Response,
	contentType string,
	mediaType *v3.MediaType,
) []*errors.ValidationError {
	var validationErrors []*errors.ValidationError

	// currently, we can only validate JSON based responses, so check for the presence
	// of 'json' in the content type (what ever it may be) so we can perform a schema check on it.
	// anything other than JSON, will be ignored.
	if strings.Contains(strings.ToLower(contentType), helpers.JSONType) {
		// extract schema from media type
		if mediaType.Schema != nil {

			var schema *base.Schema
			var renderedInline, renderedJSON []byte

			// have we seen this schema before? let's hash it and check the cache.
			hash := mediaType.GoLow().Schema.Value.Hash()

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
			valid, vErrs := ValidateResponseSchema(request, response, schema, renderedInline, renderedJSON)
			if !valid {
				validationErrors = append(validationErrors, vErrs...)
			}
		}
	}
	return validationErrors
}
