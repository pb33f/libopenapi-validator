// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"strings"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi-validator/radix"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// resolvedRoute carries everything learned during path matching.
// This is the single source of truth for "what matched and what was extracted."
type resolvedRoute struct {
	pathItem    *v3.PathItem
	matchedPath string            // path template, e.g. "/users/{id}"
	pathParams  map[string]string // extracted param values, nil if not extracted
}

// pathMatcher finds the matching path for an incoming request path.
// Implementations are composed into a chain — first match wins.
type pathMatcher interface {
	Match(path string, doc *v3.Document) *resolvedRoute
}

// matcherChain tries each matcher in order. First match wins.
type matcherChain []pathMatcher

func (c matcherChain) Match(path string, doc *v3.Document) *resolvedRoute {
	for _, m := range c {
		if result := m.Match(path, doc); result != nil {
			return result
		}
	}
	return nil
}

// radixMatcher uses the radix tree for O(k) path matching with parameter extraction.
type radixMatcher struct {
	pathLookup radix.PathLookup
}

func (m *radixMatcher) Match(path string, _ *v3.Document) *resolvedRoute {
	if m.pathLookup == nil {
		return nil
	}
	pathItem, matchedPath, params, found := m.pathLookup.LookupWithParams(path)
	if !found {
		return nil
	}
	return &resolvedRoute{
		pathItem:    pathItem,
		matchedPath: matchedPath,
		pathParams:  params,
	}
}

// regexMatcher uses regex-based matching for complex paths (matrix, label, OData, etc.).
// This is the fallback when radix matching doesn't find a match.
type regexMatcher struct {
	regexCache config.RegexCache
}

func (m *regexMatcher) Match(path string, doc *v3.Document) *resolvedRoute {
	if doc == nil || doc.Paths == nil || doc.Paths.PathItems == nil {
		return nil
	}
	pathItem, matchedPath, found := paths.FindPathRegex(path, doc, m.regexCache)
	if !found {
		return nil
	}
	return &resolvedRoute{
		pathItem:    pathItem,
		matchedPath: matchedPath,
		pathParams:  extractPathParams(matchedPath, path),
	}
}

// extractPathParams extracts path parameter values by comparing a matched template
// (e.g. "/users/{id}/posts/{postId}") with the actual request path (e.g. "/users/123/posts/456").
// It uses BraceIndices to find parameter names in each template segment and maps them to
// the corresponding request path segment values. Returns nil if no parameters are found.
func extractPathParams(template, requestPath string) map[string]string {
	templateSegs := strings.Split(template, "/")
	requestSegs := strings.Split(requestPath, "/")

	// Strip leading empty segments from the split
	if len(templateSegs) > 0 && templateSegs[0] == "" {
		templateSegs = templateSegs[1:]
	}
	if len(requestSegs) > 0 && requestSegs[0] == "" {
		requestSegs = requestSegs[1:]
	}

	if len(templateSegs) != len(requestSegs) {
		return nil
	}

	var params map[string]string
	for i, seg := range templateSegs {
		idxs, err := helpers.BraceIndices(seg)
		if err != nil || len(idxs) == 0 {
			continue
		}
		// Extract parameter names from brace pairs in this segment.
		for j := 0; j < len(idxs); j += 2 {
			// Content between braces, e.g. "id" or "id:[0-9]+"
			content := seg[idxs[j]+1 : idxs[j+1]-1]
			// Strip optional pattern after ":"
			name, _, _ := strings.Cut(content, ":")
			if name == "" {
				continue
			}
			if params == nil {
				params = make(map[string]string)
			}
			params[name] = requestSegs[i]
		}
	}
	return params
}
