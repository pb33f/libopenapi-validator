// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"fmt"
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

// BenchmarkGetRegexForPath measures a repeated call for the same template.
// Without a cache this recompiles the regex every call; with the cache it
// measures a cache hit.
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

// coldCounter hands every cold-benchmark iteration a template that no earlier
// iteration in this process used, so each call is a cache miss.
var coldCounter atomic.Int64

// BenchmarkGetRegexForPathCold measures a cache miss on every iteration: the
// template counter never repeats inside the process, so each call compiles.
// It shows the cost the cache removes from the repeat path, and the fixed
// overhead that a miss keeps.
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
// many goroutines, which is what a request-validation hot path looks like.
func BenchmarkGetRegexForPathParallel(b *testing.B) {
	templates := []string{
		"/users/{id}",
		"/users/{id:[0-9a-f-]{36}}",
		"/orgs/{org}/repos/{repo}",
		"/health/live",
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tpl := templates[i%len(templates)]
			if _, err := GetRegexForPath(tpl); err != nil {
				b.Fatalf("unexpected error for %q: %v", tpl, err)
			}
			i++
		}
	})
}
