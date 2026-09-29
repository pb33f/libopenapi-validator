// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/internal/bodycodec"
	"github.com/pb33f/libopenapi-validator/paths"
)

func (v *responseBodyValidator) ValidateResponseBody(
	request *http.Request,
	response *http.Response,
) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.document, v.options)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateResponseBodyWithPathItem(request, response, pathItem, foundPath)
}

func (v *responseBodyValidator) ValidateResponseBodyWithPathItem(request *http.Request, response *http.Response, pathItem *v3.PathItem, pathFound string) (bool, []*errors.ValidationError) {
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
	var validationErrors []*errors.ValidationError
	operation := helpers.ExtractOperation(request, pathItem)
	if operation == nil {
		return false, []*errors.ValidationError{errors.OperationNotFound(pathItem, request, request.Method, pathFound)}
	}
	// extract the response code from the response
	httpCode := response.StatusCode
	contentType := response.Header.Get(helpers.ContentTypeHeader)
	codeStr := strconv.Itoa(httpCode)

	// check if operation has responses defined
	if operation.Responses == nil || operation.Responses.Codes == nil {
		return true, nil
	}

	// check if the response code is in the contract
	foundResponse := operation.Responses.Codes.GetOrZero(codeStr)
	if foundResponse == nil {
		// check range definition for response codes
		foundResponse = operation.Responses.Codes.GetOrZero(fmt.Sprintf("%dXX", httpCode/100))
		if foundResponse != nil {
			codeStr = fmt.Sprintf("%dXX", httpCode/100)
		}
	}

	// the default response covers every code without a match, whether or not it declares content.
	isDefault := false
	if foundResponse == nil && operation.Responses.Default != nil {
		foundResponse = operation.Responses.Default
		isDefault = true
	}

	if foundResponse == nil {
		if v.options.ValidateResponseStatus {
			// no default, no code match, nothing!
			validationErrors = append(validationErrors,
				errors.ResponseCodeNotFound(operation, request, httpCode))
		}
	} else {
		// only validate if the response declares content (a 204 response does not).
		if v.options.ValidateResponseBody && orderedmap.Len(foundResponse.Content) > 0 {
			// check content type has been defined in the contract
			if mediaType, ok := helpers.FindMediaType(foundResponse.Content, contentType); ok {
				validationErrors = append(validationErrors,
					v.checkResponseSchema(request, response, contentType, mediaType, operation)...)
			} else {
				validationErrors = append(validationErrors,
					errors.ResponseContentTypeNotFound(operation, request, response, codeStr, isDefault))
			}
		}

		// check for headers in the response
		if foundResponse.Headers != nil {
			headerCode := codeStr
			if isDefault {
				headerCode = lowv3.DefaultLabel
			}
			if ok, hErrs := ValidateResponseHeaders(request, response, foundResponse.Headers, pathFound, headerCode, config.WithExistingOpts(v.options)); !ok {
				validationErrors = append(validationErrors, hErrs...)
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
	operation *v3.Operation,
) []*errors.ValidationError {
	var validationErrors []*errors.ValidationError

	if mediaType.Schema == nil {
		return validationErrors
	}

	isJson := strings.Contains(strings.ToLower(contentType), helpers.JSONType)

	schema := mediaType.Schema.Schema()
	decoder, normalized, parameters := v.options.BodyRegistry.Decoder(contentType)
	if decoder == nil && isJson {
		decoder = content.JSONDecoder()
	}
	if decoder == nil {
		if !v.options.RejectUnsupportedBodyContent {
			return validationErrors
		}
		return []*errors.ValidationError{{
			ValidationType: helpers.ResponseBodyValidation, ValidationSubType: helpers.Schema,
			Message: fmt.Sprintf("%d response body has no registered decoder", response.StatusCode),
			Reason:  fmt.Sprintf("no body decoder is registered for %s", contentType),
			Context: &content.FailureContext{Request: request, Response: response, Operation: operation, MediaType: mediaType, Schema: schema},
		}}
	}
	if response == nil || response.Body == nil || response.Body == http.NoBody {
		validationResponse := response
		if response != nil && response.Body == nil {
			copyResponse := *response
			copyResponse.Body = http.NoBody
			validationResponse = &copyResponse
		}
		_, bodyErrors := ValidateResponseSchema(&ValidateResponseSchemaInput{
			Request: request, Response: validationResponse, Schema: schema,
			Version: helpers.VersionToFloat(v.document.Version), Options: []config.Option{config.WithExistingOpts(v.options)},
		})
		return bodyErrors
	}
	responseBody, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(responseBody))
	if readErr != nil {
		return []*errors.ValidationError{{
			ValidationType: helpers.ResponseBodyValidation, ValidationSubType: helpers.Schema,
			Message: "response body could not be read", Reason: "The response body cannot be decoded: " + readErr.Error(), Context: schema,
		}}
	}

	var decodedValue any
	decoded := false
	var decodeErr error
	decodedValue, decodeErr = decoder.Decode(&content.DecodeInput{
		Context: request.Context(), Body: bytes.NewReader(responseBody), Header: response.Header,
		MediaType: normalized, Parameters: parameters, Schema: schema, Encoding: mediaType.Encoding, Direction: content.Response,
	})
	if decodeErr == nil {
		decodedValue, decodeErr = content.Canonicalize(decodedValue)
	}
	if decodeErr != nil {
		var transformErrors *bodycodec.ValidationErrors
		if stderrors.As(decodeErr, &transformErrors) {
			return transformErrors.Errors
		}
		structured := &content.DecodingError{MediaType: normalized, Direction: content.Response, Err: decodeErr}
		if isJson {
			return []*errors.ValidationError{{
				ValidationType: helpers.ResponseBodyValidation, ValidationSubType: helpers.Schema,
				Message: fmt.Sprintf("%s response body for '%s' failed to validate schema", request.Method, request.URL.Path),
				Reason:  "The response body cannot be decoded: " + structured.Error(),
				Context: &content.FailureContext{Request: request, Response: response, Operation: operation, MediaType: mediaType, Schema: schema},
			}}
		}
		return []*errors.ValidationError{{
			ValidationType: helpers.ResponseBodyValidation, ValidationSubType: helpers.Schema,
			Message: fmt.Sprintf("%d response body could not be decoded", response.StatusCode), Reason: structured.Error(),
			Context: &content.FailureContext{Request: request, Response: response, Operation: operation, MediaType: mediaType, Schema: schema},
		}}
	}
	decoded = true

	// Validate response schema
	valid, vErrs := ValidateResponseSchema(&ValidateResponseSchemaInput{
		Request:      request,
		Response:     response,
		Schema:       schema,
		Version:      helpers.VersionToFloat(v.document.Version),
		Options:      []config.Option{config.WithExistingOpts(v.options)},
		DecodedValue: decodedValue,
		RawBody:      responseBody,
		ValueDecoded: decoded,
	})

	if !valid {
		validationErrors = append(validationErrors, vErrs...)
	}

	return validationErrors
}
