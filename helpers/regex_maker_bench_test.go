// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
)

// benchTemplates covers the path template shapes this library sees in real specs.
var benchTemplates = []struct {
	name string
	tpl  string
}{
	{"no_params", "/health/live"},
	{"one_default_param", "/users/{id}"},
	{"one_custom_pattern", "/users/{id:[0-9a-f-]{36}}"},
	{"many_params", "/orgs/{org}/repos/{repo}/pulls/{pull}/files/{file}"},
	{"odata_style", "/entities('{id}')"},
}

// BenchmarkGetRegexForPath measures the stateless compile: every call
// compiles the template again. Validators avoid this with the per-validator
// regex cache, so this benchmark shows the cost the cache removes.
func BenchmarkGetRegexForPath(b *testing.B) {
	for _, bc := range benchTemplates {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				r, err := GetRegexForPath(bc.tpl)
				if err != nil {
					b.Fatalf("unexpected error for %q: %v", bc.tpl, err)
				}
				if r == nil {
					b.Fatalf("nil regex for %q", bc.tpl)
				}
			}
		})
	}
}

// BenchmarkGetRegexForPathCached measures the cached path with the same
// sync.Map shape the validator defaults to: a hit returns the stored regex.
func BenchmarkGetRegexForPathCached(b *testing.B) {
	for _, bc := range benchTemplates {
		b.Run(bc.name, func(b *testing.B) {
			var cache sync.Map
			seed, err := GetRegexForPath(bc.tpl)
			if err != nil {
				b.Fatalf("unexpected error for %q: %v", bc.tpl, err)
			}
			cache.Store(bc.tpl, seed)

			b.ReportAllocs()
			for b.Loop() {
				cached, found := cache.Load(bc.tpl)
				if !found {
					b.Fatalf("expected a cached regex for %q", bc.tpl)
				}
				if cached.(*regexp.Regexp) == nil {
					b.Fatalf("nil regex for %q", bc.tpl)
				}
			}
		})
	}
}

// coldCounter hands every cold-benchmark iteration a template that no earlier
// iteration in this process used, so each call is a cache miss.
var coldCounter atomic.Int64

// BenchmarkGetRegexForPathCold measures a compile of a unique template on
// every iteration. It shows the cost the per-validator cache removes from
// the repeat path, and the fixed overhead a miss keeps.
func BenchmarkGetRegexForPathCold(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		tpl := fmt.Sprintf("/users/%d/{id}", coldCounter.Add(1))
		if _, err := GetRegexForPath(tpl); err != nil {
			b.Fatalf("unexpected error for %q: %v", tpl, err)
		}
	}
}

// BenchmarkGetRegexForPathParallel hammers a small, stable template set from
// many goroutines through a shared per-validator-shaped cache, which is what
// a request-validation hot path looks like.
func BenchmarkGetRegexForPathParallel(b *testing.B) {
	templates := []string{
		"/users/{id}",
		"/users/{id:[0-9a-f-]{36}}",
		"/orgs/{org}/repos/{repo}",
		"/health/live",
	}
	var cache sync.Map
	for _, tpl := range templates {
		seed, err := GetRegexForPath(tpl)
		if err != nil {
			b.Fatalf("unexpected error for %q: %v", tpl, err)
		}
		cache.Store(tpl, seed)
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tpl := templates[i%len(templates)]
			if cached, found := cache.Load(tpl); found {
				_ = cached.(*regexp.Regexp)
			} else {
				if _, err := GetRegexForPath(tpl); err != nil {
					b.Fatalf("unexpected error for %q: %v", tpl, err)
				}
			}
			i++
		}
	})
}
