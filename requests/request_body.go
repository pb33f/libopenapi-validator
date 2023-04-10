// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

type RequestBodyValidator interface {
    ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError)
}

type requestBodyValidator struct {
    document *v3.Document
    errors   []*errors.ValidationError
}

func NewRequestBodyValidator(document *v3.Document) RequestBodyValidator {
    return &requestBodyValidator{document: document}
}
