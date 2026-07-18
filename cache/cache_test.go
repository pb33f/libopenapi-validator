// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package cache

import (
	"sync"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestNewDefaultCache(t *testing.T) {
	cache := NewDefaultCache()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.m)
}

func TestDefaultCache_StoreAndLoad(t *testing.T) {
	cache := NewDefaultCache()

	// Create a test schema cache entry
	testSchema := &SchemaCacheEntry{
		Schema:         &base.Schema{},
		RenderedInline: []byte("rendered"),
		RenderedJSON:   []byte(`{"type":"object"}`),
		CompiledSchema: &jsonschema.Schema{},
	}

	// Create a test key (uint64 hash)
	key := uint64(0x123456789abcdef0)

	// Store the schema
	cache.Store(key, testSchema)

	// Load the schema back
	loaded, ok := cache.Load(key)
	assert.True(t, ok, "Should find the cached schema")
	require.NotNil(t, loaded)
	assert.Equal(t, testSchema.RenderedInline, loaded.RenderedInline)
	assert.Equal(t, testSchema.RenderedJSON, loaded.RenderedJSON)
	assert.NotNil(t, loaded.CompiledSchema)
}

func TestDefaultCache_LoadMissing(t *testing.T) {
	cache := NewDefaultCache()

	// Try to load a key that doesn't exist
	key := uint64(0xdeadbeef)

	loaded, ok := cache.Load(key)
	assert.False(t, ok, "Should not find non-existent key")
	assert.Nil(t, loaded)
}

func TestDefaultCache_LoadNilCache(t *testing.T) {
	var cache *DefaultCache

	key := uint64(0)
	loaded, ok := cache.Load(key)

	assert.False(t, ok)
	assert.Nil(t, loaded)
}

func TestDefaultCache_StoreNilCache(t *testing.T) {
	var cache *DefaultCache

	// Should not panic
	key := uint64(0)
	cache.Store(key, &SchemaCacheEntry{})

	// Verify nothing was stored (cache is nil)
	assert.Nil(t, cache)
}

func TestDefaultCache_Release(t *testing.T) {
	cache := NewDefaultCache()
	cache.Store(1, &SchemaCacheEntry{RenderedInline: []byte("one")})
	cache.Store(2, &SchemaCacheEntry{RenderedInline: []byte("two")})

	cache.Release()

	loaded, ok := cache.Load(1)
	assert.False(t, ok)
	assert.Nil(t, loaded)

	count := 0
	cache.Range(func(key uint64, value *SchemaCacheEntry) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count)

	cache.Release()

	var nilCache *DefaultCache
	nilCache.Release()
}

func TestDefaultCache_Range(t *testing.T) {
	cache := NewDefaultCache()

	// Store multiple entries
	entries := make(map[uint64]*SchemaCacheEntry)
	for i := 0; i < 5; i++ {
		key := uint64(i)

		entry := &SchemaCacheEntry{
			RenderedInline: []byte{byte(i)},
			RenderedJSON:   []byte{byte(i)},
		}
		entries[key] = entry
		cache.Store(key, entry)
	}

	// Range over all entries
	count := 0
	foundKeys := make(map[uint64]bool)
	cache.Range(func(key uint64, value *SchemaCacheEntry) bool {
		count++
		foundKeys[key] = true

		// Verify the value matches what we stored
		expected, exists := entries[key]
		assert.True(t, exists, "Key should exist in original entries")
		assert.Equal(t, expected.RenderedInline, value.RenderedInline)
		return true
	})

	assert.Equal(t, 5, count, "Should iterate over all 5 entries")
	assert.Equal(t, 5, len(foundKeys), "Should find all 5 unique keys")
}

func TestDefaultCache_RangeEarlyTermination(t *testing.T) {
	cache := NewDefaultCache()

	// Store multiple entries
	for i := 0; i < 10; i++ {
		key := uint64(i)
		cache.Store(key, &SchemaCacheEntry{})
	}

	// Range but stop after 3 iterations
	count := 0
	cache.Range(func(key uint64, value *SchemaCacheEntry) bool {
		count++
		return count < 3 // Stop after 3
	})

	assert.Equal(t, 3, count, "Should stop after 3 iterations")
}

func TestDefaultCache_RangeNilCache(t *testing.T) {
	var cache *DefaultCache

	// Should not panic
	called := false
	cache.Range(func(key uint64, value *SchemaCacheEntry) bool {
		called = true
		return true
	})

	assert.False(t, called, "Callback should not be called on nil cache")
}

func TestDefaultCache_RangeEmpty(t *testing.T) {
	cache := NewDefaultCache()

	// Range over empty cache
	count := 0
	cache.Range(func(key uint64, value *SchemaCacheEntry) bool {
		count++
		return true
	})

	assert.Equal(t, 0, count, "Should not iterate over empty cache")
}

func TestDefaultCache_Overwrite(t *testing.T) {
	cache := NewDefaultCache()

	key := uint64(0x12345678)

	// Store first value
	first := &SchemaCacheEntry{
		RenderedInline: []byte("first"),
	}
	cache.Store(key, first)

	// Store second value with same key
	second := &SchemaCacheEntry{
		RenderedInline: []byte("second"),
	}
	cache.Store(key, second)

	// Load should return the second value
	loaded, ok := cache.Load(key)
	assert.True(t, ok)
	require.NotNil(t, loaded)
	assert.Equal(t, []byte("second"), loaded.RenderedInline)
}

func TestDefaultCache_MultipleKeys(t *testing.T) {
	cache := NewDefaultCache()

	// Store with different keys
	key1 := uint64(1)
	key2 := uint64(2)
	key3 := uint64(3)

	cache.Store(key1, &SchemaCacheEntry{RenderedInline: []byte("value1")})
	cache.Store(key2, &SchemaCacheEntry{RenderedInline: []byte("value2")})
	cache.Store(key3, &SchemaCacheEntry{RenderedInline: []byte("value3")})

	// Load each one
	val1, ok1 := cache.Load(key1)
	val2, ok2 := cache.Load(key2)
	val3, ok3 := cache.Load(key3)

	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.True(t, ok3)

	assert.Equal(t, []byte("value1"), val1.RenderedInline)
	assert.Equal(t, []byte("value2"), val2.RenderedInline)
	assert.Equal(t, []byte("value3"), val3.RenderedInline)
}

func TestNewDefaultSchemaResourceCache(t *testing.T) {
	cache := NewDefaultSchemaResourceCache()

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.m)
}

func TestDefaultSchemaResourceCache_StoreLoadRangeAndOverwrite(t *testing.T) {
	cache := NewDefaultSchemaResourceCache()
	first := &SchemaResourceCacheEntry{
		RenderedInline:  []byte("first"),
		ReferenceSchema: "first",
		RenderedJSON:    []byte(`{"type":"object"}`),
	}
	second := &SchemaResourceCacheEntry{
		RenderedInline: []byte("second"),
		RenderedJSON:   []byte(`{"type":"string"}`),
	}

	cache.Store("resource", first)
	loaded, ok := cache.Load("resource")
	require.True(t, ok)
	assert.Equal(t, first.RenderedInline, loaded.RenderedInline)
	assert.Equal(t, first.ReferenceSchema, loaded.ReferenceSchema)

	cache.Store("resource", second)
	loaded, ok = cache.Load("resource")
	require.True(t, ok)
	assert.Equal(t, second.RenderedInline, loaded.RenderedInline)

	seen := 0
	cache.Range(func(key string, value *SchemaResourceCacheEntry) bool {
		seen++
		assert.Equal(t, "resource", key)
		assert.Equal(t, second.RenderedJSON, value.RenderedJSON)
		return false
	})
	assert.Equal(t, 1, seen)
}

func TestDefaultSchemaResourceCache_Release(t *testing.T) {
	cache := NewDefaultSchemaResourceCache()
	cache.Store("one", &SchemaResourceCacheEntry{RenderedInline: []byte("one")})
	cache.Store("two", &SchemaResourceCacheEntry{RenderedInline: []byte("two")})

	cache.Release()

	loaded, ok := cache.Load("one")
	assert.False(t, ok)
	assert.Nil(t, loaded)

	count := 0
	cache.Range(func(key string, value *SchemaResourceCacheEntry) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count)

	cache.Release()

	var nilCache *DefaultSchemaResourceCache
	nilCache.Release()
}

func TestDefaultSchemaResourceCache_EdgeCases(t *testing.T) {
	cache := NewDefaultSchemaResourceCache()
	loaded, ok := cache.Load("missing")
	assert.False(t, ok)
	assert.Nil(t, loaded)

	var nilCache *DefaultSchemaResourceCache
	loaded, ok = nilCache.Load("missing")
	assert.False(t, ok)
	assert.Nil(t, loaded)
	nilCache.Store("resource", &SchemaResourceCacheEntry{})

	called := false
	nilCache.Range(func(key string, value *SchemaResourceCacheEntry) bool {
		called = true
		return true
	})
	assert.False(t, called)

	count := 0
	cache.Range(func(key string, value *SchemaResourceCacheEntry) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count)

	cache.m.Store(42, &SchemaResourceCacheEntry{})
	cache.m.Store("invalid", "not-a-resource-entry")
	cache.Store("valid", &SchemaResourceCacheEntry{RenderedInline: []byte("ok")})
	var keys []string
	cache.Range(func(key string, value *SchemaResourceCacheEntry) bool {
		keys = append(keys, key)
		return true
	})
	assert.Equal(t, []string{"valid"}, keys)
}

func TestDefaultSchemaResourceCache_ThreadSafety(t *testing.T) {
	cache := NewDefaultSchemaResourceCache()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			cache.Store(string(rune('a'+val)), &SchemaResourceCacheEntry{
				RenderedInline: []byte{byte(val)},
				RenderedJSON:   []byte{byte(val)},
			})
		}(i)
	}
	wg.Wait()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			loaded, ok := cache.Load(string(rune('a' + val)))
			assert.True(t, ok)
			assert.NotNil(t, loaded)
		}(i)
	}
	wg.Wait()

	count := 0
	cache.Range(func(key string, value *SchemaResourceCacheEntry) bool {
		count++
		return true
	})
	assert.Equal(t, 20, count)
}

func TestDefaultCache_ThreadSafety(t *testing.T) {
	cache := NewDefaultCache()

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(val int) {
			key := uint64(val)
			cache.Store(key, &SchemaCacheEntry{
				RenderedInline: []byte{byte(val)},
			})
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(val int) {
			key := uint64(val)
			loaded, ok := cache.Load(key)
			assert.True(t, ok)
			assert.NotNil(t, loaded)
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all entries exist
	count := 0
	cache.Range(func(key uint64, value *SchemaCacheEntry) bool {
		count++
		return true
	})
	assert.Equal(t, 10, count, "All entries should be present")
}

func TestSchemaCache_Fields(t *testing.T) {
	// Test that SchemaCache properly holds all fields
	schema := &base.Schema{}
	compiled := &jsonschema.Schema{}

	sc := &SchemaCacheEntry{
		Schema:         schema,
		RenderedInline: []byte("rendered"),
		RenderedJSON:   []byte(`{"type":"object"}`),
		CompiledSchema: compiled,
	}

	assert.Equal(t, schema, sc.Schema)
	assert.Equal(t, []byte("rendered"), sc.RenderedInline)
	assert.Equal(t, []byte(`{"type":"object"}`), sc.RenderedJSON)
	assert.Equal(t, compiled, sc.CompiledSchema)
}

func TestDefaultCache_RangeWithInvalidTypes(t *testing.T) {
	cache := NewDefaultCache()

	// Manually insert invalid types into the underlying sync.Map to test defensive programming
	// Store an entry with wrong key type
	cache.m.Store("invalid-key-type", &SchemaCacheEntry{})

	// Store an entry with wrong value type
	validKey := uint64(1)
	cache.m.Store(validKey, "invalid-value-type")

	// Store a valid entry
	validKey2 := uint64(2)
	validEntry := &SchemaCacheEntry{RenderedInline: []byte("valid")}
	cache.Store(validKey2, validEntry)

	// Range should skip invalid entries and only process valid ones
	count := 0
	var seenEntry *SchemaCacheEntry
	cache.Range(func(key uint64, value *SchemaCacheEntry) bool {
		count++
		seenEntry = value
		return true
	})

	assert.Equal(t, 1, count, "Should only process valid entry")
	assert.Equal(t, validEntry, seenEntry, "Should see the valid entry")
}
