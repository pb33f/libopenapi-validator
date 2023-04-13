// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

type ResponseBodyValidator interface {
    ValidateResponseBody(request *http.Request, response *http.Response) (bool, []*errors.ValidationError)
}

type responseBodyValidator struct {
    document *v3.Document
    request  *http.Request
    errors   []*errors.ValidationError
}

func NewResponseBodyValidator(document *v3.Document) ResponseBodyValidator {
    return &responseBodyValidator{document: document}
}
