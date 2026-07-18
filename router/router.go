// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package router matches HTTP requests to OpenAPI operations without running validation.
//
// A Router returns the effective server, path template, path item, operation,
// decoded server variables, and raw and decoded path parameters. Standalone
// routers enforce OpenAPI server precedence by default; WithPathOnlyMatching
// provides the validator's backward-compatible path-only behavior.
package router

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/radix"
)

var (
	// ErrPathNotFound identifies requests that do not match an effective server and path.
	ErrPathNotFound = errors.New("openapi path not found")
	// ErrMethodNotAllowed identifies requests whose path exists but method does not.
	// FindRoute returns a partial Route alongside this error.
	ErrMethodNotAllowed = errors.New("openapi operation not found")
)

// RegexCache stores compiled path expressions. config.RegexCache satisfies this contract.
// Values stored by the router are *regexp.Regexp instances.
type RegexCache interface {
	// Load returns a previously cached value for key.
	Load(key any) (value any, ok bool)
	// Store associates value with key.
	Store(key, value any)
}

// PathLookup performs fast standard-template lookup. radix.PathLookup satisfies this contract.
type PathLookup interface {
	// Lookup returns the matched path item, its OpenAPI path template, and whether a match was found.
	Lookup(string) (*v3.PathItem, string, bool)
}

// Router locates an OpenAPI route for a request.
type Router interface {
	// FindRoute returns the route selected for a request.
	//
	// A method mismatch returns a non-nil partial Route and an error matching
	// ErrMethodNotAllowed. A server or path mismatch returns a nil Route and an
	// error matching ErrPathNotFound.
	FindRoute(*http.Request) (*Route, error)
	// Release drops document and router-owned lookup references. Injected lookups
	// and regex caches are borrowed and are never released by the router.
	Release()
}

// Route describes the OpenAPI operation selected for a request.
type Route struct {
	Document  *v3.Document  // Document is the OpenAPI document used for matching.
	Server    *v3.Server    // Server is the matched server, or nil for the implicit relative server.
	Path      string        // Path is the matched OpenAPI path template.
	PathItem  *v3.PathItem  // PathItem is the matched OpenAPI path item.
	Method    string        // Method is the request method, including additional methods.
	Operation *v3.Operation // Operation is nil on a method mismatch.
	// RawPathParams contains escaped parameter values exactly as matched in the URL path.
	RawPathParams map[string]string
	// PathParams contains URL-decoded operation path parameter values.
	PathParams map[string]string
	// ServerParams contains URL-decoded server variable values.
	ServerParams map[string]string
}

// RouteError retains partial match context while supporting errors.Is.
type RouteError struct {
	Kind  error  // Kind is ErrPathNotFound or ErrMethodNotAllowed.
	Route *Route // Route is populated for method mismatches and nil for path misses.
}

// Error returns the stable error message for the route failure kind.
func (e *RouteError) Error() string {
	if e == nil || e.Kind == nil {
		return "openapi route error"
	}
	return e.Kind.Error()
}

// Unwrap exposes the stable route error kind.
func (e *RouteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

type options struct {
	lookup     PathLookup
	regexCache RegexCache
	pathOnly   bool
}

// Option configures Router construction.
type Option func(*options)

// WithPathLookup borrows a path lookup. The router never releases it.
func WithPathLookup(lookup PathLookup) Option {
	return func(o *options) { o.lookup = lookup }
}

// WithRegexCache borrows a compiled-regex cache. The router never releases it.
func WithRegexCache(cache RegexCache) Option {
	return func(o *options) { o.regexCache = cache }
}

// WithPathOnlyMatching preserves validator-compatible document base-path matching.
func WithPathOnlyMatching() Option {
	return func(o *options) { o.pathOnly = true }
}

type routeFinder struct {
	mu          sync.RWMutex
	document    *v3.Document
	lookup      PathLookup
	ownedLookup *radix.PathTree
	regexCache  RegexCache
	pathOnly    bool
	servers     []*v3.Server
	paths       map[string]*compiledPath
	serverMatch map[*v3.Server]*compiledServer
}

type compiledPath struct {
	expression *regexp.Regexp
	names      []string
}

// NewRouter creates an immutable, concurrency-safe router.
// Strict OpenAPI server matching is enabled unless WithPathOnlyMatching is supplied.
func NewRouter(document *v3.Document, opts ...Option) Router {
	o := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	r := &routeFinder{document: document, lookup: o.lookup, regexCache: o.regexCache, pathOnly: o.pathOnly}
	if r.lookup == nil {
		r.ownedLookup = radix.BuildPathTree(document)
		r.lookup = r.ownedLookup
	}
	if !r.pathOnly {
		r.servers = collectServers(document)
	}
	r.paths = compileDocumentPaths(document, r.regexCache)
	r.serverMatch = compileDocumentServers(r.servers)
	return r
}

func (r *routeFinder) FindRoute(request *http.Request) (*Route, error) {
	if r == nil {
		return nil, &RouteError{Kind: ErrPathNotFound}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if request == nil || request.URL == nil || r.document == nil || r.document.Paths == nil || r.document.Paths.PathItems == nil {
		return nil, &RouteError{Kind: ErrPathNotFound}
	}

	if r.pathOnly {
		return r.find(request, compatibilityPath(request, r.document), compatibilityServer(request, r.document), nil)
	}

	if len(r.servers) == 0 {
		return r.find(request, request.URL.EscapedPath(), nil, nil)
	}

	var partial *Route
	for _, server := range r.servers {
		path, params, ok := matchCompiledServer(request, server, r.serverMatch[server])
		if !ok {
			continue
		}
		route, err := r.find(request, path, server, params)
		if route == nil {
			continue
		}
		if !serverIsEffective(server, route.Operation, route.PathItem, r.document) {
			continue
		}
		if err == nil {
			return route, nil
		}
		partial = route
	}
	implicit, implicitErr := r.find(request, request.URL.EscapedPath(), nil, nil)
	if implicit != nil && usesImplicitServer(implicit.Operation, implicit.PathItem, r.document) {
		if implicitErr == nil {
			return implicit, nil
		}
		if partial == nil {
			partial = implicit
		}
	}
	if partial != nil {
		return partial, &RouteError{Kind: ErrMethodNotAllowed, Route: partial}
	}
	return nil, &RouteError{Kind: ErrPathNotFound}
}

func usesImplicitServer(operation *v3.Operation, item *v3.PathItem, document *v3.Document) bool {
	return (operation == nil || len(operation.Servers) == 0) &&
		(item == nil || len(item.Servers) == 0) &&
		(document == nil || len(document.Servers) == 0)
}

func (r *routeFinder) find(request *http.Request, requestPath string, server *v3.Server, serverParams map[string]string) (*Route, error) {
	if requestPath == "" {
		requestPath = "/"
	}
	pathItem, path, found := r.lookup.Lookup(requestPath)
	if found && !simpleRadixTemplate(path) {
		compiled := r.compiledPath(path, requestPath)
		found = compiled != nil && compiled.expression.MatchString(requestPath)
	}
	if found && (!r.pathOnly && !sameTrailingSlash(path, requestPath)) {
		found = false
	}
	if !found {
		pathItem, path = r.regexLookup(requestPath, request.Method)
		found = pathItem != nil
	}
	if !found {
		return nil, &RouteError{Kind: ErrPathNotFound}
	}

	operation := operationForMethod(pathItem, request.Method)
	raw, decoded := r.extractPathParams(path, requestPath)
	route := &Route{
		Document:      r.document,
		Server:        server,
		Path:          path,
		PathItem:      pathItem,
		Method:        request.Method,
		Operation:     operation,
		RawPathParams: raw,
		PathParams:    decoded,
		ServerParams:  serverParams,
	}
	if operation == nil {
		return route, &RouteError{Kind: ErrMethodNotAllowed, Route: route}
	}
	return route, nil
}

type candidate struct {
	path      string
	item      *v3.PathItem
	score     int
	hasMethod bool
}

func (r *routeFinder) regexLookup(requestPath, method string) (*v3.PathItem, string) {
	candidates := make([]candidate, 0)
	for pair := orderedmap.First(r.document.Paths.PathItems); pair != nil; pair = pair.Next() {
		compiled := r.compiledPath(pair.Key(), requestPath)
		if compiled != nil && compiled.expression.MatchString(requestPath) {
			candidates = append(candidates, candidate{pair.Key(), pair.Value(), specificity(pair.Key()), operationForMethod(pair.Value(), method) != nil})
		}
	}
	if len(candidates) == 0 {
		return nil, ""
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].hasMethod != candidates[j].hasMethod {
			return candidates[i].hasMethod
		}
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].path < candidates[j].path
	})
	return candidates[0].item, candidates[0].path
}

func (r *routeFinder) Release() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ownedLookup != nil {
		r.ownedLookup.Release()
	}
	r.document = nil
	r.lookup = nil
	r.ownedLookup = nil
	r.regexCache = nil
	r.servers = nil
	r.paths = nil
	r.serverMatch = nil
}

func operationForMethod(item *v3.PathItem, method string) *v3.Operation {
	if item == nil {
		return nil
	}
	switch method {
	case http.MethodGet:
		return item.Get
	case http.MethodPut:
		return item.Put
	case http.MethodPost:
		return item.Post
	case http.MethodDelete:
		return item.Delete
	case http.MethodOptions:
		return item.Options
	case http.MethodHead:
		if item.Head != nil {
			return item.Head
		}
		return item.Get
	case http.MethodPatch:
		return item.Patch
	case http.MethodTrace:
		return item.Trace
	case "QUERY":
		return item.Query
	}
	if item.AdditionalOperations != nil {
		for pair := orderedmap.First(item.AdditionalOperations); pair != nil; pair = pair.Next() {
			if strings.EqualFold(pair.Key(), method) {
				return pair.Value()
			}
		}
	}
	return nil
}

func sameTrailingSlash(template, requestPath string) bool {
	return strings.HasSuffix(template, "/") == strings.HasSuffix(requestPath, "/")
}

func simpleRadixTemplate(path string) bool {
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		if !strings.ContainsAny(segment, "{}") {
			continue
		}
		if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' || strings.ContainsAny(segment[1:len(segment)-1], ":.*;") {
			return false
		}
	}
	return true
}

func normalizeFragment(path, requestPath string) string {
	if !strings.Contains(requestPath, "#") {
		if i := strings.IndexByte(path, '#'); i >= 0 {
			return path[:i]
		}
	}
	return path
}

func specificity(path string) int {
	score := 0
	for _, segment := range strings.Split(path, "/") {
		if segment == "" {
			continue
		}
		if strings.Contains(segment, "{") && strings.Contains(segment, "}") {
			score++
		} else {
			score += 1000
		}
	}
	return score
}

func compileDocumentPaths(document *v3.Document, cache RegexCache) map[string]*compiledPath {
	compiled := make(map[string]*compiledPath)
	if document == nil || document.Paths == nil || document.Paths.PathItems == nil {
		return compiled
	}
	for pair := orderedmap.First(document.Paths.PathItems); pair != nil; pair = pair.Next() {
		for _, path := range []string{pair.Key(), normalizeFragment(pair.Key(), "")} {
			if _, exists := compiled[path]; exists {
				continue
			}
			if cache != nil {
				if value, ok := cache.Load(path); ok {
					if expression, valid := value.(*regexp.Regexp); valid {
						if names, err := templateNames(path); err == nil {
							compiled[path] = &compiledPath{expression: expression, names: names}
							continue
						}
					}
				}
			}
			expression, names, err := compileTemplate(path, "[^/]*")
			if err == nil {
				compiled[path] = &compiledPath{expression: expression, names: names}
				if cache != nil {
					cache.Store(path, expression)
				}
			}
		}
	}
	return compiled
}

func templateNames(template string) ([]string, error) {
	indices, err := braceIndices(template)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(indices)/2)
	for i := 0; i < len(indices); i += 2 {
		parts := strings.SplitN(template[indices[i]+1:indices[i+1]-1], ":", 2)
		name := strings.TrimSuffix(strings.TrimLeft(parts[0], ".;"), "*")
		if name == "" {
			return nil, fmt.Errorf("invalid route parameter in %q", template)
		}
		names = append(names, name)
	}
	return names, nil
}

func (r *routeFinder) compiledPath(path, requestPath string) *compiledPath {
	if r == nil {
		return nil
	}
	normalized := normalizeFragment(path, requestPath)
	if compiled := r.paths[normalized]; compiled != nil {
		return compiled
	}
	expression, names, err := compileTemplate(normalized, "[^/]*")
	if err != nil {
		return nil
	}
	return &compiledPath{expression: expression, names: names}
}

func compileTemplate(template, defaultPattern string) (*regexp.Regexp, []string, error) {
	indices, err := braceIndices(template)
	if err != nil {
		return nil, nil, err
	}
	var pattern strings.Builder
	pattern.WriteByte('^')
	names := make([]string, 0, len(indices)/2)
	end := 0
	for i := 0; i < len(indices); i += 2 {
		pattern.WriteString(regexp.QuoteMeta(template[end:indices[i]]))
		end = indices[i+1]
		parts := strings.SplitN(template[indices[i]+1:end-1], ":", 2)
		name := strings.TrimSuffix(strings.TrimLeft(parts[0], ".;"), "*")
		match := defaultPattern
		if len(parts) == 2 {
			match = parts[1]
		}
		if name == "" || match == "" {
			return nil, nil, fmt.Errorf("invalid route parameter in %q", template)
		}
		names = append(names, name)
		pattern.WriteByte('(')
		pattern.WriteString(match)
		pattern.WriteByte(')')
	}
	pattern.WriteString(regexp.QuoteMeta(template[end:]))
	pattern.WriteByte('$')
	expression, err := regexp.Compile(pattern.String())
	if err != nil {
		return nil, nil, err
	}
	if expression.NumSubexp() != len(names) {
		return nil, nil, fmt.Errorf("route %s contains capturing groups", template)
	}
	return expression, names, nil
}

func braceIndices(value string) ([]int, error) {
	level, start := 0, 0
	var indices []int
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '{':
			level++
			if level == 1 {
				start = i
			}
		case '}':
			level--
			if level == 0 {
				indices = append(indices, start, i+1)
			} else if level < 0 {
				return nil, fmt.Errorf("unbalanced braces in %q", value)
			}
		}
	}
	if level != 0 {
		return nil, fmt.Errorf("unbalanced braces in %q", value)
	}
	return indices, nil
}

func (r *routeFinder) extractPathParams(template, requestPath string) (map[string]string, map[string]string) {
	compiled := r.compiledPath(template, requestPath)
	if compiled == nil || len(compiled.names) == 0 {
		return nil, nil
	}
	matches := compiled.expression.FindStringSubmatch(requestPath)
	if len(matches) != len(compiled.names)+1 {
		return nil, nil
	}
	raw := make(map[string]string, len(compiled.names))
	decoded := make(map[string]string, len(compiled.names))
	for i, name := range compiled.names {
		raw[name] = matches[i+1]
		decoded[name] = decodePathValue(matches[i+1])
	}
	return raw, decoded
}

func decodePathValue(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}
