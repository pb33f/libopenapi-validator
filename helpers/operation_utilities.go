// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"mime"
	"net/http"

	"github.com/pb33f/libopenapi/datamodel/high/v3"
)

// OperationForMethod returns the operation from the PathItem for the given HTTP method string.
// Returns nil if the method doesn't exist on the PathItem.
func OperationForMethod(method string, pathItem *v3.PathItem) *v3.Operation {
	switch method {
	case http.MethodGet:
		return pathItem.Get
	case http.MethodPost:
		return pathItem.Post
	case http.MethodPut:
		return pathItem.Put
	case http.MethodDelete:
		return pathItem.Delete
	case http.MethodOptions:
		return pathItem.Options
	case http.MethodHead:
		return pathItem.Head
	case http.MethodPatch:
		return pathItem.Patch
	case http.MethodTrace:
		return pathItem.Trace
	}
	return nil
}

// ExtractOperation extracts the operation from the path item based on the request method.
func ExtractOperation(request *http.Request, item *v3.PathItem) *v3.Operation {
	return OperationForMethod(request.Method, item)
}

// ExtractContentType extracts the content type from the request header. First return argument is the content type
// of the request.The second (optional) argument is the charset of the request. The third (optional)
// argument is the boundary of the type (only used with forms really).
func ExtractContentType(contentType string) (string, string, string) {
	// mime.ParseMediaType: "If there is an error parsing the optional parameter,
	// the media type will be returned along with the error ErrInvalidMediaParameter."
	ct, params, _ := mime.ParseMediaType(contentType)
	return ct, params["charset"], params["boundary"]
}
