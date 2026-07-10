// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/internal/bodycodec"
	"github.com/pb33f/libopenapi-validator/internal/requeststate"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi-validator/schema_validation"
)

func (v *requestBodyValidator) ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.document, v.options)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateRequestBodyWithPathItem(request, pathItem, foundPath)
}

func (v *requestBodyValidator) ValidateRequestBodyWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
	if pathItem == nil {
		return false, []*errors.ValidationError{{
			ValidationType:    helpers.PathValidation,
			ValidationSubType: helpers.ValidationMissing,
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
		if v.options.RejectUndeclaredRequestBody {
			body, err := requeststate.Snapshot(request)
			if err != nil {
				return false, []*errors.ValidationError{{
					ValidationType: helpers.RequestBodyValidation, ValidationSubType: helpers.Schema,
					Message: "request body could not be inspected", Reason: err.Error(), Context: operation,
				}}
			}
			if len(body) > 0 {
				return false, []*errors.ValidationError{{
					ValidationType: helpers.RequestBodyValidation, ValidationSubType: helpers.Schema,
					Message: fmt.Sprintf("%s request body for '%s' is not declared", request.Method, request.URL.Path),
					Reason:  "the matched operation does not declare a requestBody", Context: operation,
				}}
			}
		}
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
	mediaType, ok := v.extractContentType(contentType, operation)
	if !ok {
		return false, []*errors.ValidationError{errors.RequestContentTypeNotFound(operation, request, pathValue)}
	}

	// Nothing to validate
	if mediaType.Schema == nil {
		return true, nil
	}

	// extract schema from media type
	schema := mediaType.Schema.Schema()

	isJson := strings.Contains(strings.ToLower(contentType), helpers.JSONType)
	isXml := schema_validation.IsXMLContentType(contentType)
	isUrlEncoded := schema_validation.IsURLEncodedContentType(contentType)
	decoder, normalized, parameters := v.options.BodyRegistry.Decoder(contentType)
	if decoder == nil && isJson {
		decoder = content.JSONDecoder()
	}
	if decoder == nil {
		if !v.options.RejectUnsupportedBodyContent {
			return true, nil
		}
		return false, []*errors.ValidationError{{
			ValidationType: helpers.RequestBodyValidation, ValidationSubType: helpers.Schema,
			Message: fmt.Sprintf("%s request body for '%s' has no registered decoder", request.Method, request.URL.Path),
			Reason:  fmt.Sprintf("no body decoder is registered for %s", contentType),
			Context: &content.FailureContext{Request: request, Operation: operation, MediaType: mediaType, Schema: schema},
		}}
	}
	requestBody, readErr := requeststate.Snapshot(request)
	if readErr != nil {
		return false, []*errors.ValidationError{{
			ValidationType: helpers.RequestBodyValidation, ValidationSubType: helpers.Schema,
			Message: "request body could not be read", Reason: readErr.Error(), Context: schema,
		}}
	}
	var decodedValue any
	var decodeErr error
	decoded := false

	decodedValue, decodeErr = decoder.Decode(&content.DecodeInput{
		Context: request.Context(), Body: bytes.NewReader(requestBody), Header: request.Header,
		MediaType: normalized, Parameters: parameters, Schema: schema, Encoding: mediaType.Encoding, Direction: content.Request,
	})
	if decodeErr == nil {
		decodedValue, decodeErr = content.Canonicalize(decodedValue)
	}
	if decodeErr != nil {
		var transformErrors *bodycodec.ValidationErrors
		if stderrors.As(decodeErr, &transformErrors) {
			return false, transformErrors.Errors
		}
		structured := &content.DecodingError{MediaType: normalized, Direction: content.Request, Err: decodeErr}
		if isXml {
			return false, []*errors.ValidationError{errors.InvalidXMLParsing(structured.Error(), string(requestBody))}
		}
		if isUrlEncoded {
			return false, []*errors.ValidationError{errors.InvalidURLEncodedParsing(structured.Error(), string(requestBody))}
		}
		return false, []*errors.ValidationError{{
			ValidationType: helpers.RequestBodyValidation, ValidationSubType: helpers.Schema,
			Message: fmt.Sprintf("%s request body for '%s' could not be decoded", request.Method, request.URL.Path),
			Reason:  "The request body cannot be decoded: " + structured.Error(),
			Context: &content.FailureContext{Request: request, Operation: operation, MediaType: mediaType, Schema: schema},
		}}
	}
	decoded = true

	validationSucceeded, validationErrors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request:      request,
		Schema:       schema,
		Version:      helpers.VersionToFloat(v.document.Version),
		Options:      []config.Option{config.WithExistingOpts(v.options)},
		BodyRequired: required,
		DecodedValue: decodedValue,
		RawBody:      requestBody,
		ValueDecoded: decoded,
	})

	errors.PopulateValidationErrors(validationErrors, request, pathValue)

	return validationSucceeded, validationErrors
}

func (v *requestBodyValidator) extractContentType(contentType string, operation *v3.Operation) (*v3.MediaType, bool) {
	ct, _, _ := helpers.ExtractContentType(contentType)
	mediaType, ok := operation.RequestBody.Content.Get(ct)
	if ok {
		return mediaType, true
	}
	ctMediaRange := strings.SplitN(ct, "/", 2)
	for contentPair := operation.RequestBody.Content.First(); contentPair != nil; contentPair = contentPair.Next() {
		s := contentPair.Key()
		mediaTypeValue := contentPair.Value()
		opMediaRange := strings.SplitN(s, "/", 2)
		if (opMediaRange[0] == "*" || opMediaRange[0] == ctMediaRange[0]) &&
			(opMediaRange[1] == "*" || opMediaRange[1] == ctMediaRange[1]) {
			return mediaTypeValue, true
		}
	}
	return nil, false
}
