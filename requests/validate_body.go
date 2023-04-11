// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi-validator/schemas"
    "net/http"
    "strings"
)

func (v *requestBodyValidator) ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError) {

    // find path
    pathItem, errs, _ := paths.FindPath(request, v.document)
    if pathItem == nil || errs != nil {
        v.errors = errs
        return false, errs
    }

    var validationErrors []*errors.ValidationError
    operation := helpers.ExtractOperation(request, pathItem)

    var contentType string
    // extract the content type from the request

    if contentType = request.Header.Get("Content-Type"); contentType != "" {
        if mediaType, ok := operation.RequestBody.Content[contentType]; ok {

            // we currently only support JSON validation for request bodies
            if strings.ToLower(contentType) == helpers.JSONContentType {

                // extract schema from media type
                if mediaType.Schema != nil {
                    schema := mediaType.Schema.Schema()

                    // render the schema, to be used for validation
                    valid, verrs := schemas.ValidateRequestSchema(request, schema)
                    if !valid {
                        validationErrors = append(validationErrors, verrs...)
                    }
                }
            }
        } else {

            // TODO: content type not found in operation request
        }
    }
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
