// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
)

// requestContext is per-request shared state that flows through the entire
// validation pipeline. Created once per request, shared by all validators.
type requestContext struct {
	request    *http.Request
	route      *resolvedRoute
	operation  *v3.Operation
	parameters []*v3.Parameter // path + operation params, extracted once
	security   []*base.SecurityRequirement
	stripped   string   // request path with base path removed
	segments   []string // pre-split path segments
	version    float32  // cached OAS version (3.0 or 3.1)
}

// buildRequestContext creates a requestContext from an incoming request.
// It strips the path, matches it against the spec, resolves the operation,
// and extracts parameters and security requirements — all exactly once.
//
// Returns (*requestContext, nil) on success, or (nil, errors) on failure
// (path not found, method not found).
func (v *validator) buildRequestContext(request *http.Request) (*requestContext, []*errors.ValidationError) {
	stripped := paths.StripRequestPath(request, v.v3Model)

	// Split path into segments for future use (filter leading empty string)
	segments := strings.Split(stripped, "/")
	if len(segments) > 0 && segments[0] == "" {
		segments = segments[1:]
	}

	// Match path using the matcher chain
	route := v.matchers.Match(stripped, v.v3Model)
	if route == nil {
		validationErrors := []*errors.ValidationError{
			{
				ValidationType:    helpers.PathValidation,
				ValidationSubType: helpers.ValidationMissing,
				Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
				Reason: fmt.Sprintf("The %s request contains a path of '%s' "+
					"however that path, or the %s method for that path does not exist in the specification",
					request.Method, request.URL.Path, request.Method),
				SpecLine: -1,
				SpecCol:  -1,
				HowToFix: errors.HowToFixPath,
			},
		}
		errors.PopulateValidationErrors(validationErrors, request, "")
		return nil, validationErrors
	}

	// Resolve operation for the HTTP method
	operation := helpers.OperationForMethod(request.Method, route.pathItem)
	if operation == nil {
		validationErrors := []*errors.ValidationError{{
			ValidationType:    helpers.PathValidation,
			ValidationSubType: helpers.ValidationMissingOperation,
			Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
			Reason: fmt.Sprintf("The %s method for that path does not exist in the specification",
				request.Method),
			SpecLine: -1,
			SpecCol:  -1,
			HowToFix: errors.HowToFixPath,
		}}
		errors.PopulateValidationErrors(validationErrors, request, route.matchedPath)
		return nil, validationErrors
	}

	// Extract parameters and security once
	params := helpers.ExtractParamsForOperation(request, route.pathItem)
	security := helpers.ExtractSecurityForOperation(request, route.pathItem)

	return &requestContext{
		request:    request,
		route:      route,
		operation:  operation,
		parameters: params,
		security:   security,
		stripped:   stripped,
		segments:   segments,
		version:    v.version,
	}, nil
}
