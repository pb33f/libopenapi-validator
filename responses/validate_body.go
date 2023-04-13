// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi-validator/schemas"
    "net/http"
    "strconv"
    "strings"
)

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

    // check if the response code is in the contract
    foundResponse := operation.Responses.FindResponseByCode(httpCode)
    if foundResponse != nil {

        // check content type has been defined in the contract
        if mediaType, ok := foundResponse.Content[contentType]; ok {

            // currently, we can only validate JSON based responses, so check for the presence
            // of 'json' in the content type (what ever it may be) so we can perform a schema check on it.
            // anything other than JSON, will be ignored.

            if strings.Contains(strings.ToLower(contentType), helpers.JSONType) {

                // extract schema from media type
                if mediaType.Schema != nil {
                    schema := mediaType.Schema.Schema()

                    // render the schema, to be used for validation
                    valid, vErrs := schemas.ValidateResponseSchema(request, response, schema)
                    if !valid {
                        validationErrors = append(validationErrors, vErrs...)
                    }
                }
            }

        } else {
            // content type not found in the contract
            codeStr := strconv.Itoa(httpCode)
            validationErrors = append(validationErrors,
                errors.ResponseContentTypeNotFound(operation, request, response, codeStr))
        }
    } else {

        // TODO: response code not defined, check for default response, or fail.
    }
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
