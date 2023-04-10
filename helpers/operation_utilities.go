// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
)

func ExtractOperation(request *http.Request, item *v3.PathItem) *v3.Operation {
    switch request.Method {
    case http.MethodGet:
        return item.Get
    case http.MethodPost:
        return item.Post
    case http.MethodPut:
        return item.Put
    case http.MethodDelete:
        return item.Delete
    case http.MethodOptions:
        return item.Options
    case http.MethodHead:
        return item.Head
    case http.MethodPatch:
        return item.Patch
    case http.MethodTrace:
        return item.Trace
    }
    return nil
}
