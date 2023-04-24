// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
    "github.com/pb33f/libopenapi/utils"
    "net/http"
    "strings"
)

func (v *requestBodyValidator) ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError) {

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

    var contentType string
    // extract the content type from the request

    if contentType = request.Header.Get(helpers.ContentTypeHeader); contentType != "" {

        // extract the media type from the content type header.
        ct, _, _ := helpers.ExtractContentType(contentType)
        if operation.RequestBody != nil {
            if mediaType, ok := operation.RequestBody.Content[ct]; ok {

                // we currently only support JSON validation for request bodies
                // this will capture *everything* that contains some form of 'json' in the content type
                if strings.Contains(strings.ToLower(contentType), helpers.JSONType) {

                    // extract schema from media type
                    if mediaType.Schema != nil {

                        var schema *base.Schema
                        var renderedInline, renderedJSON []byte

                        // have we seen this schema before? let's hash it and check the cache.
                        hash := mediaType.GoLow().Schema.Value.Hash()

                        // perform work only once and cache the result in the validator.
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

                        //render the schema, to be used for validation
                        valid, vErrs := ValidateRequestSchema(request, schema, renderedInline, renderedJSON)
                        if !valid {
                            validationErrors = append(validationErrors, vErrs...)
                        }
                    }
                }
            } else {
                // content type not found in the contract
                validationErrors = append(validationErrors, errors.RequestContentTypeNotFound(operation, request))
            }
        }
    }
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
