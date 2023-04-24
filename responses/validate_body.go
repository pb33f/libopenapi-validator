// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "github.com/pb33f/libopenapi/utils"
    "net/http"
    "strconv"
    "strings"
)

func (v *responseBodyValidator) ValidateResponseBody(
    request *http.Request,
    response *http.Response) (bool, []*errors.ValidationError) {

    // find path
    var pathItem *v3.PathItem
    var errs []*errors.ValidationError
    if v.pathItem == nil {
        pathItem, errs, _ = paths.FindPath(request, v.document)
        if pathItem == nil || errs != nil {
            v.errors = errs
            return false, errs
        }
    } else {
        pathItem = v.pathItem
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
            // TODO: add support for '2XX' and '3XX' responses in the contract
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

            var schema *base.Schema
            var renderedInline, renderedJSON []byte

            // have we seen this schema before? let's hash it and check the cache.
            hash := mediaType.GoLow().Schema.Value.Hash()

            if cacheHit, ch := v.schemaCache[hash]; ch {

                // got a hit, use cached values
                schema = cacheHit.schema
                renderedInline = cacheHit.renderedInline
                renderedJSON = cacheHit.renderedJSON

            } else {

                // render the schema inline and perform the intensive work of rendering and converting
                // this is only performed once per schema and cached in the validator.
                schema = mediaType.Schema.Schema()
                renderedInline, _ = schema.RenderInline()
                renderedJSON, _ = utils.ConvertYAMLtoJSON(renderedInline)
                v.schemaCache[hash] = &schemaCache{
                    schema:         schema,
                    renderedInline: renderedInline,
                    renderedJSON:   renderedJSON,
                }
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
