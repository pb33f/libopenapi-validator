// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package radix

import (
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// PathLookup defines the interface for radix tree path matching implementations.
// The default implementation provides O(k) lookup where k is the path segment count.
//
// Note: This interface handles URL path matching only. HTTP method validation
// is performed separately after the PathItem is retrieved, since a single path
// (e.g., "/users/{id}") can support multiple HTTP methods (GET, POST, PUT, DELETE).
type PathLookup interface {
	// Lookup finds the PathItem for a given URL path.
	// Returns the matched PathItem, the path template (e.g., "/users/{id}"), and whether found.
	Lookup(urlPath string) (pathItem *v3.PathItem, matchedPath string, found bool)
	// LookupWithParams finds the PathItem for a given URL path and extracts path parameter values.
	// Returns the matched PathItem, path template, extracted parameter map, and whether found.
	// The params map is nil if the matched path has no parameters.
	LookupWithParams(urlPath string) (pathItem *v3.PathItem, matchedPath string, params map[string]string, found bool)
}

// PathTree is a radix tree optimized for OpenAPI path matching.
// It provides O(k) lookup where k is the number of path segments (typically 3-5),
// with minimal allocations during lookup.
//
// This is a thin wrapper around the generic Tree, specialized for
// OpenAPI PathItem values. It implements the PathLookup interface.
type PathTree struct {
	tree *Tree[*v3.PathItem]
}

// Ensure PathTree implements PathLookup at compile time.
var _ PathLookup = (*PathTree)(nil)

// NewPathTree creates a new empty radix tree for path matching.
func NewPathTree() *PathTree {
	return &PathTree{
		tree: New[*v3.PathItem](),
	}
}

// Insert adds a path and its PathItem to the tree.
// Path should be in OpenAPI format, e.g., "/users/{id}/posts"
func (t *PathTree) Insert(path string, pathItem *v3.PathItem) {
	t.tree.Insert(path, pathItem)
}

// Lookup finds the PathItem for a given request path.
// Returns the PathItem, the matched path template, and whether a match was found.
func (t *PathTree) Lookup(urlPath string) (*v3.PathItem, string, bool) {
	return t.tree.Lookup(urlPath)
}

// LookupWithParams finds the PathItem for a given request path and extracts path parameter values.
// Returns the PathItem, matched path template, extracted parameter map, and whether a match was found.
func (t *PathTree) LookupWithParams(urlPath string) (*v3.PathItem, string, map[string]string, bool) {
	return t.tree.LookupWithParams(urlPath)
}

// Size returns the number of paths stored in the tree.
func (t *PathTree) Size() int {
	return t.tree.Size()
}

// Walk calls the given function for each path in the tree.
func (t *PathTree) Walk(fn func(path string, pathItem *v3.PathItem) bool) {
	t.tree.Walk(fn)
}

// BuildPathTree creates a PathTree from an OpenAPI document.
// This should be called once during validator initialization.
func BuildPathTree(doc *v3.Document) *PathTree {
	tree := NewPathTree()

	if doc == nil || doc.Paths == nil || doc.Paths.PathItems == nil {
		return tree
	}

	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		path := pair.Key()
		pathItem := pair.Value()
		tree.Insert(path, pathItem)
	}

	return tree
}
