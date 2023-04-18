// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "strconv"
    "strings"
)

// ValidateResponseBody will validate the response body for a http.Response pointer. The request is used to
// locate the operation in the specification, the response is used to ensure the response code, media type and the
// schema of the response body are valid.
func (v *responseBodyValidator) ValidateResponseBody(
    request *http.Request,
    response *http.Response) (bool, []*errors.ValidationError) {

    // find path
    pathItem, errs, _ := paths.FindPath(request, v.document)
    if pathItem == nil || errs != nil {
        v.errors = errs
        return false, errs
    }

    var validationErrors []*errors.ValidationError
    operation := helpers.ExtractOperation(request, pathItem)

    // extract the response code from the response
    httpCode := response.StatusCode
    contentType := response.Header.Get(helpers.ContentTypeHeader)

    // extract the media type from the content type header.
    mediaTypeSting, _, _ := helpers.ExtractContentType(contentType)

    // check if the response code is in the contract
    foundResponse := operation.Responses.FindResponseByCode(httpCode)
    if foundResponse != nil {

        // check content type has been defined in the contract
        if mediaType, ok := foundResponse.Content[mediaTypeSting]; ok {

            validationErrors = append(validationErrors,
                v.checkResponseSchema(request, response, mediaTypeSting, mediaType)...)

        } else {

            // check that the operation *actually* returns a body. (i.e. a 204 response)
            if foundResponse.Content != nil {

                // content type not found in the contract
                codeStr := strconv.Itoa(httpCode)
                validationErrors = append(validationErrors,
                    errors.ResponseContentTypeNotFound(operation, request, response, codeStr, false))

            }
        }
    } else {

        // no code match, check for default response
        if operation.Responses.Default != nil {

            // check content type has been defined in the contract
            if mediaType, ok := operation.Responses.Default.Content[mediaTypeSting]; ok {

                validationErrors = append(validationErrors,
                    v.checkResponseSchema(request, response, contentType, mediaType)...)

            } else {

                // check that the operation *actually* returns a body. (i.e. a 204 response)
                if operation.Responses.Default.Content != nil {

                    // content type not found in the contract
                    codeStr := strconv.Itoa(httpCode)
                    validationErrors = append(validationErrors,
                        errors.ResponseContentTypeNotFound(operation, request, response, codeStr, true))
                }
            }

        } else {
            // no default, no code match, nothing!
            validationErrors = append(validationErrors,
                errors.ResponseCodeNotFound(operation, request, httpCode))
        }
    }
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}

func (v *responseBodyValidator) checkResponseSchema(
    request *http.Request,
    response *http.Response,
    contentType string,
    mediaType *v3.MediaType) []*errors.ValidationError {

    var validationErrors []*errors.ValidationError

    // currently, we can only validate JSON based responses, so check for the presence
    // of 'json' in the content type (what ever it may be) so we can perform a schema check on it.
    // anything other than JSON, will be ignored.
    if strings.Contains(strings.ToLower(contentType), helpers.JSONType) {

        // extract schema from media type
        if mediaType.Schema != nil {
            schema := mediaType.Schema.Schema()

            // render the schema, to be used for validation
            valid, vErrs := ValidateResponseSchema(request, response, schema)
            if !valid {
                validationErrors = append(validationErrors, vErrs...)
            }
        }
    }
    return validationErrors
}
