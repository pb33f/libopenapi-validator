// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package errors

import (
    "fmt"
    "github.com/pb33f/libopenapi-validator/helpers"
    v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "strings"
)

func RequestContentTypeNotFound(op *v3.Operation, request *http.Request) *ValidationError {
    ct := request.Header.Get(helpers.ContentTypeHeader)
    var ctypes []string
    for k := range op.RequestBody.Content {
        ctypes = append(ctypes, k)
    }
    return &ValidationError{
        ValidationType:    helpers.RequestBodyValidation,
        ValidationSubType: helpers.RequestBodyContentType,
        Message: fmt.Sprintf("%s operation request content type '%s' does not exist",
            request.Method, ct),
        Reason: fmt.Sprintf("The content type '%s' of the %s request submitted has not "+
            "been defined, it's an unknown type", ct, request.Method),
        SpecLine: op.RequestBody.GoLow().Content.KeyNode.Line,
        SpecCol:  op.RequestBody.GoLow().Content.KeyNode.Column,
        Context:  op,
        HowToFix: fmt.Sprintf(HowToFixInvalidContentType, len(op.RequestBody.Content), strings.Join(ctypes, ", ")),
    }
}
