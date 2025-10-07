// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/libopenapi/utils"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
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
				foundResponse = operation.Responses.Default
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

	if foundResponse != nil {
		// check for headers in the response
		if foundResponse.Headers != nil {
			if ok, herrs := ValidateResponseHeaders(request, response, foundResponse.Headers); !ok {
				validationErrors = append(validationErrors, herrs...)
			}
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

			// Check cache for pre-rendered and pre-compiled schema
			var compiledSchema *jsonschema.Schema
			if cacheHit, ch := v.schemaCache.Load(hash); ch {
				// got a hit, use cached values
				if cached, ok := cacheHit.(*helpers.SchemaCache); ok {
					schema = cached.Schema
					renderedInline = cached.RenderedInline
					renderedJSON = cached.RenderedJSON
					compiledSchema = cached.CompiledSchema
				}
			} else {
				// render the schema inline and perform the intensive work of rendering and converting
				// this is only performed once per schema and cached in the validator.
				schemaP := mediaType.Schema
				marshalled, mErr := schemaP.MarshalYAMLInline()

				if mErr != nil {
					validationErrors = append(validationErrors, &errors.ValidationError{
						Reason:            mErr.Error(),
						Message:           fmt.Sprintf("unable to marshal schema for %s", contentType),
						ValidationType:    helpers.ResponseBodyValidation,
						ValidationSubType: helpers.Schema,
						SpecLine:          mediaType.Schema.GetSchemaKeyNode().Line,
						SpecCol:           mediaType.Schema.GetSchemaKeyNode().Column,
						RequestPath:       request.URL.Path,
						RequestMethod:     request.Method,
						HowToFix:          "ensure schema is valid and does not contain circular references",
					})
				} else {
					schema = schemaP.Schema()
					renderedInline, _ = yaml.Marshal(marshalled)
					renderedJSON, _ = utils.ConvertYAMLtoJSON(renderedInline)

					// Compile the schema and cache it (so future requests don't need to compile)
					var err error
					compiledSchema, err = helpers.NewCompiledSchema(fmt.Sprintf("%x", hash), renderedJSON, v.options)
					if err != nil {
						// Compilation failed - cache with nil compiledSchema so we don't re-render
						// ValidateResponseSchema will handle nil and report the compilation error
						compiledSchema = nil
					}

					// Always cache (even if compilation failed) to avoid re-rendering on every request
					v.schemaCache.Store(hash, &helpers.SchemaCache{
						Schema:         schema,
						RenderedInline: renderedInline,
						RenderedJSON:   renderedJSON,
						CompiledSchema: compiledSchema, // may be nil if compilation failed
					})
				}
			}

			// Validate if we have valid schema data
			// ValidateResponseSchema will handle nil compiledSchema and report all validation errors
			if len(renderedInline) > 0 && len(renderedJSON) > 0 && schema != nil {
				// render the schema, to be used for validation
				valid, vErrs := ValidateResponseSchema(request, response, schema, renderedInline, renderedJSON, helpers.VersionToFloat(v.document.Version), compiledSchema, config.WithRegexEngine(v.options.RegexEngine))
				if !valid {
					validationErrors = append(validationErrors, vErrs...)
				}
			}
		}
	}
	return validationErrors
}
