// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package cache

import "sync"

// DefaultCache is the default cache implementation using sync.Map for thread-safe concurrent access.
type DefaultCache struct {
	m *sync.Map
}

// DefaultSchemaResourceCache is the default thread-safe cache for rendered document resources.
type DefaultSchemaResourceCache struct {
	m *sync.Map
}

var (
	_ SchemaCache         = &DefaultCache{}
	_ SchemaResourceCache = &DefaultSchemaResourceCache{}
)

// NewDefaultCache creates a new DefaultCache with an initialized sync.Map.
func NewDefaultCache() *DefaultCache {
	return &DefaultCache{m: &sync.Map{}}
}

// NewDefaultSchemaResourceCache creates a default cache for rendered document resources.
func NewDefaultSchemaResourceCache() *DefaultSchemaResourceCache {
	return &DefaultSchemaResourceCache{m: &sync.Map{}}
}

// Release clears all cached schema entries.
func (c *DefaultCache) Release() {
	if c == nil || c.m == nil {
		return
	}
	c.m.Clear()
}

// Release clears all cached rendered document resources.
func (c *DefaultSchemaResourceCache) Release() {
	if c == nil || c.m == nil {
		return
	}
	c.m.Clear()
}

// Load retrieves a schema from the cache.
func (c *DefaultCache) Load(key uint64) (*SchemaCacheEntry, bool) {
	if c == nil || c.m == nil {
		return nil, false
	}
	val, ok := c.m.Load(key)
	if !ok {
		return nil, false
	}
	schemaCache, ok := val.(*SchemaCacheEntry)
	return schemaCache, ok
}

// Store saves a schema to the cache.
func (c *DefaultCache) Store(key uint64, value *SchemaCacheEntry) {
	if c == nil || c.m == nil {
		return
	}
	c.m.Store(key, value)
}

// Range calls f for each entry in the cache (for testing/inspection).
func (c *DefaultCache) Range(f func(key uint64, value *SchemaCacheEntry) bool) {
	if c == nil || c.m == nil {
		return
	}
	c.m.Range(func(k, v interface{}) bool {
		key, ok := k.(uint64)
		if !ok {
			return true
		}
		val, ok := v.(*SchemaCacheEntry)
		if !ok {
			return true
		}
		return f(key, val)
	})
}

// Load retrieves a rendered document resource from the cache.
func (c *DefaultSchemaResourceCache) Load(key string) (*SchemaResourceCacheEntry, bool) {
	if c == nil || c.m == nil {
		return nil, false
	}
	val, ok := c.m.Load(key)
	if !ok {
		return nil, false
	}
	resourceCache, ok := val.(*SchemaResourceCacheEntry)
	return resourceCache, ok
}

// Store saves a rendered document resource to the cache.
func (c *DefaultSchemaResourceCache) Store(key string, value *SchemaResourceCacheEntry) {
	if c == nil || c.m == nil {
		return
	}
	c.m.Store(key, value)
}

// Range calls f for each rendered document resource cache entry.
func (c *DefaultSchemaResourceCache) Range(f func(key string, value *SchemaResourceCacheEntry) bool) {
	if c == nil || c.m == nil {
		return
	}
	c.m.Range(func(k, v interface{}) bool {
		key, ok := k.(string)
		if !ok {
			return true
		}
		val, ok := v.(*SchemaResourceCacheEntry)
		if !ok {
			return true
		}
		return f(key, val)
	})
}
