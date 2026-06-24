// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package cache

import (
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"
)

// SchemaCacheEntry holds a compiled schema and its intermediate representations.
// This is stored in the cache to avoid re-rendering and re-compiling schemas on each request.
type SchemaCacheEntry struct {
	Schema          *base.Schema
	RenderedInline  []byte
	ReferenceSchema string // String version of RenderedInline
	RenderedJSON    []byte
	CompiledSchema  *jsonschema.Schema
	RenderedNode    *yaml.Node
	ResourceNodes   map[string]*yaml.Node
}

// SchemaResourceCacheEntry holds one rendered document-level JSON Schema resource.
// Resource entries are shared by many compiled schema entry points from the same parsed document.
// RenderedNode may point at source YAML for generic validation; consumers must treat it as read-only.
type SchemaResourceCacheEntry struct {
	RenderedInline  []byte
	ReferenceSchema string
	RenderedJSON    []byte
	RenderedNode    *yaml.Node
	SourceRootNode  *yaml.Node // Keeps pointer-identity cache keys live for the cache entry lifetime.
}

// SchemaCache defines the interface for schema caching implementations.
// The key is a uint64 hash of the schema (from schema.GoLow().Hash()).
type SchemaCache interface {
	Load(key uint64) (*SchemaCacheEntry, bool)
	Store(key uint64, value *SchemaCacheEntry)
	Range(f func(key uint64, value *SchemaCacheEntry) bool)
}

// SchemaResourceCache caches rendered document resources by parsed document identity.
// Entries are immutable once stored; implementations must be safe for concurrent use.
type SchemaResourceCache interface {
	Load(key string) (*SchemaResourceCacheEntry, bool)
	Store(key string, value *SchemaResourceCacheEntry)
	Range(f func(key string, value *SchemaResourceCacheEntry) bool)
}
