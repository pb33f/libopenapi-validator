// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
    "github.com/pb33f/libopenapi-validator/errors"
    v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

type ParameterValidator interface {
    ValidateQueryParams(request *http.Request) (bool, []*errors.ValidationError)
    ValidateHeaderParams(request *http.Request) (bool, []*errors.ValidationError)
    ValidateCookieParams(request *http.Request) (bool, []*errors.ValidationError)
    ValidatePathParams(request *http.Request) (bool, []*errors.ValidationError)
}

type paramValidator struct {
    document *v3.Document
    errors   []*errors.ValidationError
}

func NewParameterValidator(document *v3.Document) ParameterValidator {
    return &paramValidator{document: document}
}
