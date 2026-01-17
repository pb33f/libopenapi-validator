// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	"fmt"
	"net/http"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/radix"
)

// PathRadixTree is an alias for radix.PathTree for backwards compatibility.
// Deprecated: Use radix.PathTree directly.
type PathRadixTree = radix.PathTree

// NewPathRadixTree creates a new empty radix tree for path matching.
// Deprecated: Use radix.NewPathTree directly.
func NewPathRadixTree() *radix.PathTree {
	return radix.NewPathTree()
}

// BuildRadixTree creates a PathTree from an OpenAPI document.
// Deprecated: Use radix.BuildPathTree directly.
func BuildRadixTree(doc *v3.Document) *radix.PathTree {
	return radix.BuildPathTree(doc)
}

// FindPathWithRadix uses the radix tree for O(k) path lookup where k is the path depth.
// This replaces the linear scan + regex matching approach with a tree traversal.
// Returns the PathItem, any validation errors, and the matched path template.
func FindPathWithRadix(
	request *http.Request,
	document *v3.Document,
	pathLookup radix.PathLookup,
) (*v3.PathItem, []*errors.ValidationError, string) {
	if pathLookup == nil {
		// Fall back to linear search if no tree
		return FindPath(request, document, nil)
	}

	// Strip the base path from the request URL
	stripped := StripRequestPath(request, document)

	// Look up in the radix tree - O(k) where k = path depth
	pathItem, matchedPath, found := pathLookup.Lookup(stripped)

	if !found {
		validationErrors := []*errors.ValidationError{
			{
				ValidationType:    helpers.ParameterValidationPath,
				ValidationSubType: "missing",
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
		return nil, validationErrors, ""
	}

	// Check if the path has the requested method
	if !pathHasMethod(pathItem, request.Method) {
		validationErrors := []*errors.ValidationError{{
			ValidationType:    helpers.ParameterValidationPath,
			ValidationSubType: "missingOperation",
			Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
			Reason: fmt.Sprintf("The %s method for that path does not exist in the specification",
				request.Method),
			SpecLine: -1,
			SpecCol:  -1,
			HowToFix: errors.HowToFixPath,
		}}
		errors.PopulateValidationErrors(validationErrors, request, matchedPath)
		return pathItem, validationErrors, matchedPath
	}

	return pathItem, nil, matchedPath
}
