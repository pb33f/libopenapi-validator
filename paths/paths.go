// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/internal/pathvariant"
	"github.com/pb33f/libopenapi-validator/internal/requeststate"
	"github.com/pb33f/libopenapi-validator/internal/serverurl"
	"github.com/pb33f/libopenapi-validator/router"
)

// FindPath will find the path in the document that matches the request path. If a successful match was found, then
// the first return value will be a pointer to the PathItem. The second return value will contain any validation errors
// that were picked up when locating the path.
// The third return value will be the path that was found in the document, as it pertains to the contract, so all path
// parameters will not have been replaced with their values from the request - allowing model lookups.
//
// This function first tries a fast O(k) radix tree lookup (where k is path depth). If the radix tree
// doesn't find a match, it falls back to regex-based matching which handles complex path patterns
// like matrix-style ({;param}), label-style ({.param}), and OData-style (entities('{Entity}')).
//
// Path matching follows the OpenAPI specification: literal (concrete) paths take precedence over
// parameterized paths, regardless of definition order in the specification.
//
// A request path that selects a different operation when its percent-encoding, encoded slashes, or
// empty and dot segments are normalized is rejected as ambiguous, because a router could run an
// operation other than the one validated.
func FindPath(request *http.Request, document *v3.Document, options *config.ValidationOptions) (*v3.PathItem, []*errors.ValidationError, string) {
	if options != nil && options.Router != nil {
		route, validationErrors := ResolveRoute(request, document, options)
		if route == nil {
			return nil, validationErrors, ""
		}
		return route.PathItem, validationErrors, route.Path
	}
	pathItem, validationErrors, foundPath := findPath(request, document, options, StripRequestPath(request, document))
	if requeststate.Route(request) != nil {
		return pathItem, validationErrors, foundPath
	}
	escapedPath := request.URL.EscapedPath()
	if pathItem == nil && strings.Contains(escapedPath, "//") {
		// doubled slashes match as single ones when nothing matches them as sent
		collapsed := stripPath(pathvariant.CollapseSlashes(escapedPath), request.URL.Fragment, getBasePaths(document))
		pathItem, validationErrors, foundPath = findPath(request, document, options, collapsed)
	}
	if len(validationErrors) > 0 {
		return pathItem, validationErrors, foundPath
	}
	// a router that reads the path another way must run the same operation, or the request is rejected
	if variants := pathvariant.Variants(escapedPath); len(variants) > 0 {
		operation := helpers.ExtractOperation(request, pathItem)
		basePaths := getBasePaths(document)
		for _, variant := range variants {
			other, otherErrors, _ := findPath(request, document, options, stripPath(variant, request.URL.Fragment, basePaths))
			if len(otherErrors) == 0 && helpers.ExtractOperation(request, other) != operation {
				return nil, ambiguousPathError(request), ""
			}
		}
	}
	return pathItem, nil, foundPath
}

// findPath matches stripped, one reading of the request path without its base path.
func findPath(request *http.Request, document *v3.Document, options *config.ValidationOptions, stripped string) (*v3.PathItem, []*errors.ValidationError, string) {
	// Fast path: try radix tree first (O(k) where k = path depth)
	// If no path lookup is provided, we will fall back to regex-based matching.
	if options != nil && options.PathTree != nil {
		if pathItem, matchedPath, found := options.PathTree.Lookup(stripped); found {
			if pathHasMethod(pathItem, request.Method) {
				return pathItem, nil, matchedPath
			}
			return pathItem, missingOperationError(request, matchedPath), matchedPath
		}
	}

	// Slow path: fall back to regex matching for complex paths (matrix, label, OData, etc.)
	basePaths := getBasePaths(document)

	reqPathSegments := strings.Split(stripped, "/")
	if reqPathSegments[0] == "" {
		reqPathSegments = reqPathSegments[1:]
	}

	var regexCache config.RegexCache
	if options != nil {
		regexCache = options.RegexCache
	}

	candidates := make([]pathCandidate, 0, document.Paths.PathItems.Len())

	for pair := orderedmap.First(document.Paths.PathItems); pair != nil; pair = pair.Next() {
		path := pair.Key()
		pathItem := pair.Value()

		pathForMatching := normalizePathForMatching(path, stripped)

		segs := strings.Split(pathForMatching, "/")
		if segs[0] == "" {
			segs = segs[1:]
		}

		ok := comparePaths(segs, reqPathSegments, basePaths, regexCache)
		if !ok {
			continue
		}

		// Compute specificity score and check if method exists
		score := computeSpecificityScore(path)
		hasMethod := pathHasMethod(pathItem, request.Method)

		candidates = append(candidates, pathCandidate{
			pathItem:  pathItem,
			path:      path,
			score:     score,
			hasMethod: hasMethod,
		})
	}

	if len(candidates) == 0 {
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
		return nil, validationErrors, ""
	}

	bestWithMethod, bestOverall := selectMatches(candidates)

	if bestWithMethod != nil {
		return bestWithMethod.pathItem, nil, bestWithMethod.path
	}

	// path matches exist but none have the required method
	return bestOverall.pathItem, missingOperationError(request, bestOverall.path), bestOverall.path
}

// ResolveRoute returns the complete route used by high-level validation.
// It preserves existing ValidationError shapes while exposing the router's server,
// operation, path-parameter, and server-variable context to internal callers.
func ResolveRoute(request *http.Request, document *v3.Document, options *config.ValidationOptions) (*router.Route, []*errors.ValidationError) {
	if options == nil || options.Router == nil {
		pathItem, validationErrors, path := FindPath(request, document, options)
		if pathItem == nil {
			return nil, validationErrors
		}
		return &router.Route{
			Document: document, Path: path, PathItem: pathItem, Method: request.Method,
			Operation: helpers.ExtractOperation(request, pathItem),
		}, validationErrors
	}
	route, err := options.Router.FindRoute(request)
	if err == nil && route != nil {
		return route, nil
	}
	if stderrors.Is(err, router.ErrMethodNotAllowed) && route != nil {
		return route, missingOperationError(request, route.Path)
	}
	if stderrors.Is(err, router.ErrAmbiguousPath) {
		return nil, ambiguousPathError(request)
	}
	validationErrors := []*errors.ValidationError{{
		ValidationType:    helpers.PathValidation,
		ValidationSubType: helpers.ValidationMissing,
		Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
		Reason: fmt.Sprintf("The %s request contains a path of '%s' however that path, or the %s method for that path does not exist in the specification",
			request.Method, request.URL.Path, request.Method),
		SpecLine: -1,
		SpecCol:  -1,
		HowToFix: errors.HowToFixPath,
	}}
	errors.PopulateValidationErrors(validationErrors, request, "")
	return nil, validationErrors
}

// normalizePathForMatching removes the fragment from a path template unless
// the request path itself contains a fragment.
func normalizePathForMatching(path, requestPath string) string {
	if strings.Contains(requestPath, "#") {
		return path
	}
	if idx := strings.IndexByte(path, '#'); idx >= 0 {
		return path[:idx]
	}
	return path
}

func getBasePaths(document *v3.Document) []string {
	// extract base path from document to check against paths.
	var basePaths []string
	for _, s := range document.Servers {
		if basePath := serverurl.BasePath(s.URL); basePath != "" {
			basePaths = append(basePaths, basePath)
		}
	}

	return basePaths
}

// StripRequestPath strips the base path from the request path, based on the server paths provided in the specification.
// During high-level validation it returns the path the router matched, which accounts for server variables.
func StripRequestPath(request *http.Request, document *v3.Document) string {
	if route := requeststate.Route(request); route != nil && route.Document == document && route.RequestPath != "" {
		// strict server matching ignores the fragment, path-only matching keeps it; always keep it here
		stripped, _, _ := strings.Cut(route.RequestPath, "#")
		if request.URL.Fragment != "" {
			stripped += "#" + request.URL.Fragment
		}
		return stripped
	}
	return stripPath(request.URL.EscapedPath(), request.URL.Fragment, getBasePaths(document))
}

// stripPath removes the first matching base path from an escaped request path, appends the
// fragment, and makes the result absolute.
func stripPath(escapedPath, fragment string, basePaths []string) string {
	stripped := stripBaseFromPath(escapedPath, basePaths)
	if fragment != "" {
		stripped = fmt.Sprintf("%s#%s", stripped, fragment)
	}
	if !strings.HasPrefix(stripped, "/") {
		stripped = "/" + stripped
	}
	return stripped
}

func checkPathAgainstBase(docPath, urlPath string, basePaths []string) bool {
	if docPath == urlPath {
		return true
	}
	for _, basePath := range basePaths {
		if len(basePath) > 1 && basePath[len(basePath)-1] == '/' {
			basePath = basePath[:len(basePath)-1]
		}
		merged := fmt.Sprintf("%s%s", basePath, urlPath)
		if docPath == merged {
			return true
		}
	}
	return false
}

func stripBaseFromPath(path string, basePaths []string) string {
	for i := range basePaths {
		if rest, ok := serverurl.TrimBasePath(path, basePaths[i]); ok {
			return rest
		}
	}
	return path
}

func comparePaths(mapped, requested, basePaths []string, regexCache config.RegexCache) bool {
	if len(mapped) != len(requested) {
		return false // short circuit out
	}
	var imploded []string
	for i, seg := range mapped {
		s := seg
		var rgx *regexp.Regexp

		if regexCache != nil {
			if cachedRegex, found := regexCache.Load(s); found {
				rgx = cachedRegex.(*regexp.Regexp)
			}
		}

		if rgx == nil {
			r, err := helpers.GetRegexForPath(seg)
			if err != nil {
				return false
			}

			rgx = r

			if regexCache != nil {
				regexCache.Store(seg, r)
			}
		}

		if rgx.MatchString(requested[i]) {
			s = requested[i]
		}
		imploded = append(imploded, s)
	}
	l := filepath.Join(imploded...)
	r := filepath.Join(requested...)
	return checkPathAgainstBase(l, r, basePaths)
}

// ambiguousPathError returns a validation error for a request path whose normalized readings select
// different operations.
func ambiguousPathError(request *http.Request) []*errors.ValidationError {
	validationErrors := []*errors.ValidationError{{
		ValidationType:    helpers.PathValidation,
		ValidationSubType: helpers.ValidationAmbiguousPath,
		Message:           fmt.Sprintf("%s Path '%s' is ambiguous", request.Method, request.URL.EscapedPath()),
		Reason: fmt.Sprintf("The %s request path '%s' selects a different operation when its percent-encoding, "+
			"encoded slashes, or empty and dot segments are normalized, so a router could run an operation "+
			"other than the one validated", request.Method, request.URL.EscapedPath()),
		SpecLine: -1,
		SpecCol:  -1,
		HowToFix: errors.HowToFixAmbiguousPath,
	}}
	errors.PopulateValidationErrors(validationErrors, request, "")
	return validationErrors
}

// missingOperationError returns a validation error for when a path was found but the HTTP method doesn't exist.
func missingOperationError(request *http.Request, matchedPath string) []*errors.ValidationError {
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
	errors.PopulateValidationErrors(validationErrors, request, matchedPath)
	return validationErrors
}
