// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package router

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func model(t testing.TB, specification string) *v3.Document {
	t.Helper()
	document, err := libopenapi.NewDocument([]byte(specification))
	require.NoError(t, err)
	built, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	return &built.Model
}

const routingSpec = `openapi: 3.1.0
info: {title: routing, version: 1.0.0}
servers:
  - url: https://{tenant}.example.com:{port}/doc/{version}
    variables:
      tenant: {default: acme}
      port: {default: "8443", enum: ["8443", "9443"]}
      version: {default: v1}
paths:
  /pets/mine:
    get: {responses: {"200": {description: ok}}}
  /pets/{id}:
    servers:
      - url: /path/{version}
        variables:
          version: {default: v2}
    get:
      servers:
        - url: /operation/{version}
          variables:
            version: {default: v3}
      responses: {"200": {description: ok}}
    head: {responses: {"200": {description: head}}}
  /fallback/{id}:
    get: {responses: {"200": {description: ok}}}
  /entities('{id}'):
    get: {responses: {"200": {description: ok}}}
  /orders/{id:[0-9]+}:
    get: {responses: {"200": {description: ok}}}
`

func TestRouterStrictMatchingAndContext(t *testing.T) {
	r := NewRouter(model(t, routingSpec))
	t.Cleanup(r.Release)

	req := httptestRequest(http.MethodGet, "http://ignored/operation/v3/pets/a%2Fb")
	route, err := r.FindRoute(req)
	require.NoError(t, err)
	require.NotNil(t, route)
	assert.Equal(t, "/pets/{id}", route.Path)
	assert.Equal(t, http.MethodGet, route.Method)
	assert.NotNil(t, route.Operation)
	assert.Equal(t, "a%2Fb", route.RawPathParams["id"])
	assert.Equal(t, "a/b", route.PathParams["id"])
	assert.Equal(t, "v3", route.ServerParams["version"])
	assert.Equal(t, "/operation/{version}", route.Server.URL)
	assert.Same(t, route.Document.Paths.PathItems.GetOrZero("/pets/{id}"), route.PathItem)
}

func TestRouterLiteralPrecedenceAndHeadBehavior(t *testing.T) {
	r := NewRouter(model(t, routingSpec))
	t.Cleanup(r.Release)

	literal, err := r.FindRoute(httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/pets/mine"))
	require.NoError(t, err)
	assert.Equal(t, "/pets/mine", literal.Path)
	assert.Empty(t, literal.PathParams)

	explicit, err := r.FindRoute(httptestRequest(http.MethodHead, "http://ignored/path/v2/pets/7"))
	require.NoError(t, err)
	assert.Same(t, explicit.PathItem.Head, explicit.Operation)

	fallback, err := r.FindRoute(httptestRequest(http.MethodHead, "https://acme.example.com:8443/doc/v1/fallback/7"))
	require.NoError(t, err)
	assert.Same(t, fallback.PathItem.Get, fallback.Operation)
}

func TestRouterMethodAndPathErrors(t *testing.T) {
	r := NewRouter(model(t, routingSpec))
	t.Cleanup(r.Release)

	route, err := r.FindRoute(httptestRequest(http.MethodPost, "http://ignored/path/v2/pets/7"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMethodNotAllowed)
	require.NotNil(t, route)
	assert.Nil(t, route.Operation)
	assert.Equal(t, "/pets/{id}", route.Path)
	var routeErr *RouteError
	require.True(t, errors.As(err, &routeErr))
	assert.Same(t, route, routeErr.Route)

	route, err = r.FindRoute(httptestRequest(http.MethodGet, "http://ignored/doc/v1/missing"))
	assert.Nil(t, route)
	assert.ErrorIs(t, err, ErrPathNotFound)
}

func TestRouterServerPrecedenceAndValidation(t *testing.T) {
	r := NewRouter(model(t, routingSpec))
	t.Cleanup(r.Release)

	for _, target := range []string{
		"http://ignored/doc/v1/pets/7",
		"http://ignored/path/v2/pets/7",
		"https://acme.example.com:7443/doc/v1/pets/mine",
	} {
		route, err := r.FindRoute(httptestRequest(http.MethodGet, target))
		assert.Nil(t, route, target)
		assert.ErrorIs(t, err, ErrPathNotFound, target)
	}

	route, err := r.FindRoute(httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/pets/mine"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"tenant": "acme", "port": "8443", "version": "v1"}, route.ServerParams)
}

func TestRouterRegexFallbackAndTrailingSlash(t *testing.T) {
	r := NewRouter(model(t, routingSpec))
	t.Cleanup(r.Release)

	odata, err := r.FindRoute(httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/entities('42')"))
	require.NoError(t, err)
	assert.Equal(t, "42", odata.PathParams["id"])

	custom, err := r.FindRoute(httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/orders/123"))
	require.NoError(t, err)
	assert.Equal(t, "123", custom.PathParams["id"])

	missing, err := r.FindRoute(httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/orders/nope"))
	assert.Nil(t, missing)
	assert.ErrorIs(t, err, ErrPathNotFound)

	missing, err = r.FindRoute(httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/pets/mine/"))
	assert.Nil(t, missing)
	assert.ErrorIs(t, err, ErrPathNotFound)
}

func TestRouterMatrixLabelExplodedAndMultipleSegmentParameters(t *testing.T) {
	doc := model(t, `openapi: 3.1.0
info: {title: templates, version: 1.0.0}
paths:
  /matrix/{;id}:
    get: {responses: {"200": {description: ok}}}
  /label/{.id}:
    get: {responses: {"200": {description: ok}}}
  /exploded/{id*}:
    get: {responses: {"200": {description: ok}}}
  /pair/{x}-{y}:
    get: {responses: {"200": {description: ok}}}`)
	r := NewRouter(doc)
	t.Cleanup(r.Release)
	tests := []struct {
		target string
		path   string
		raw    map[string]string
	}{
		{"http://example.com/matrix/;id=5", "/matrix/{;id}", map[string]string{"id": ";id=5"}},
		{"http://example.com/label/.a%2Fb", "/label/{.id}", map[string]string{"id": ".a%2Fb"}},
		{"http://example.com/exploded/a,b", "/exploded/{id*}", map[string]string{"id": "a,b"}},
		{"http://example.com/pair/left-right", "/pair/{x}-{y}", map[string]string{"x": "left", "y": "right"}},
	}
	for _, test := range tests {
		route, err := r.FindRoute(httptestRequest(http.MethodGet, test.target))
		require.NoError(t, err)
		assert.Equal(t, test.path, route.Path)
		assert.Equal(t, test.raw, route.RawPathParams)
	}
}

func TestRouterImplicitServerAndPathOnlyCompatibility(t *testing.T) {
	doc := model(t, `openapi: 3.1.0
info: {title: test, version: 1.0.0}
paths:
  /things/{id}:
    get: {responses: {"200": {description: ok}}}`)
	strict := NewRouter(doc)
	t.Cleanup(strict.Release)
	route, err := strict.FindRoute(httptestRequest(http.MethodGet, "http://example.com/things/1"))
	require.NoError(t, err)
	assert.Nil(t, route.Server)
	assert.Nil(t, route.ServerParams)

	compatible := NewRouter(model(t, `openapi: 3.1.0
info: {title: test, version: 1.0.0}
servers: [{url: /doc/v1}]
paths:
  /pets/mine:
    get: {responses: {"200": {description: ok}}}`), WithPathOnlyMatching())
	t.Cleanup(compatible.Release)
	route, err = compatible.FindRoute(httptestRequest(http.MethodGet, "https://wrong.example/doc/v1/pets/mine"))
	require.NoError(t, err)
	assert.Equal(t, "/pets/mine", route.Path)
}

func TestRouterImplicitServerSurvivesUnrelatedOperationOverride(t *testing.T) {
	doc := model(t, `openapi: 3.1.0
info: {title: mixed servers, version: 1.0.0}
paths:
  /implicit:
    get: {responses: {"200": {description: ok}}}
  /explicit:
    get:
      servers: [{url: /api}]
      responses: {"200": {description: ok}}`)
	r := NewRouter(doc)
	t.Cleanup(r.Release)

	implicit, err := r.FindRoute(httptestRequest(http.MethodGet, "http://example.com/implicit"))
	require.NoError(t, err)
	assert.Equal(t, "/implicit", implicit.Path)
	assert.Nil(t, implicit.Server)

	explicit, err := r.FindRoute(httptestRequest(http.MethodGet, "http://example.com/api/explicit"))
	require.NoError(t, err)
	assert.Equal(t, "/explicit", explicit.Path)
	require.NotNil(t, explicit.Server)
	assert.Equal(t, "/api", explicit.Server.URL)

	partial, err := r.FindRoute(httptestRequest(http.MethodPost, "http://example.com/implicit"))
	assert.ErrorIs(t, err, ErrMethodNotAllowed)
	require.NotNil(t, partial)
	assert.Equal(t, "/implicit", partial.Path)
}

type borrowedLookup struct {
	item     *v3.PathItem
	released bool
}

func (b *borrowedLookup) Lookup(string) (*v3.PathItem, string, bool) {
	return b.item, "/borrowed/{id}", b.item != nil
}

func (b *borrowedLookup) Release() { b.released = true }

func TestRouterBorrowedLookupAndRelease(t *testing.T) {
	doc := model(t, `openapi: 3.1.0
info: {title: test, version: 1.0.0}
paths:
  /borrowed/{id}:
    get: {responses: {"200": {description: ok}}}`)
	lookup := &borrowedLookup{item: doc.Paths.PathItems.GetOrZero("/borrowed/{id}")}
	r := NewRouter(doc, WithPathLookup(lookup), WithPathOnlyMatching())
	route, err := r.FindRoute(httptestRequest(http.MethodGet, "http://example.com/anything"))
	require.NoError(t, err)
	assert.Equal(t, "/borrowed/{id}", route.Path)
	r.Release()
	assert.False(t, lookup.released)
	route, err = r.FindRoute(httptestRequest(http.MethodGet, "http://example.com/anything"))
	assert.Nil(t, route)
	assert.ErrorIs(t, err, ErrPathNotFound)
	r.Release()
}

type regexCache struct{ sync.Map }

func TestRouterRegexCacheAndConcurrentLookup(t *testing.T) {
	cache := &regexCache{}
	r := NewRouter(model(t, routingSpec), WithRegexCache(cache))
	t.Cleanup(r.Release)

	const workers = 24
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			route, err := r.FindRoute(httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/entities('42')"))
			assert.NoError(t, err)
			assert.NotNil(t, route)
		}()
	}
	wg.Wait()
	_, cached := cache.Load("/entities('{id}')")
	assert.True(t, cached)
}

func TestRouterUsesPrecompiledRegexCache(t *testing.T) {
	doc := model(t, `openapi: 3.1.0
info: {title: cache, version: 1.0.0}
paths:
  /cached/{id}:
    get: {responses: {"200": {description: ok}}}`)
	cache := &regexCache{}
	cache.Store("/cached/{id}", regexp.MustCompile(`^/cached/([^/]*)$`))
	r := NewRouter(doc, WithRegexCache(cache))
	t.Cleanup(r.Release)
	route, err := r.FindRoute(httptestRequest(http.MethodGet, "http://example.com/cached/value"))
	require.NoError(t, err)
	assert.Equal(t, "value", route.PathParams["id"])
	assert.Len(t, compileDocumentServers([]*v3.Server{{URL: "/{"}}), 0)
	_, err = templateNames("/{")
	assert.Error(t, err)
	_, err = templateNames("/{}")
	assert.Error(t, err)
}

func TestRouterNilAndErrorContracts(t *testing.T) {
	var nilRouter *routeFinder
	route, err := nilRouter.FindRoute(nil)
	assert.Nil(t, route)
	assert.ErrorIs(t, err, ErrPathNotFound)
	nilRouter.Release()

	r := NewRouter(nil, nil)
	route, err = r.FindRoute(&http.Request{})
	assert.Nil(t, route)
	assert.ErrorIs(t, err, ErrPathNotFound)
	r.Release()

	assert.Equal(t, "openapi route error", (*RouteError)(nil).Error())
	assert.Nil(t, (*RouteError)(nil).Unwrap())
	assert.Equal(t, "openapi route error", (&RouteError{}).Error())
	assert.Equal(t, ErrPathNotFound.Error(), (&RouteError{Kind: ErrPathNotFound}).Error())
}

func TestRouterAdditionalOperationAndDeterministicAmbiguity(t *testing.T) {
	doc := model(t, `openapi: 3.2.0
info: {title: test, version: 1.0.0}
paths:
  /{first}/fixed:
    query: {responses: {"200": {description: ok}}}
  /fixed/{second}:
    query: {responses: {"200": {description: ok}}}`)
	r := NewRouter(doc)
	t.Cleanup(r.Release)
	route, err := r.FindRoute(httptestRequest("QUERY", "http://example.com/fixed/fixed"))
	require.NoError(t, err)
	assert.Equal(t, "/fixed/{second}", route.Path)
}

func TestTemplateHelpersRejectInvalidPatterns(t *testing.T) {
	_, _, err := compileTemplate("/{", "[^/]*")
	assert.Error(t, err)
	_, _, err = compileTemplate("/{name:}", "[^/]*")
	assert.Error(t, err)
	_, _, err = compileTemplate("/{name:(x)}", "[^/]*")
	assert.Error(t, err)
	_, _, err = compileTemplate("/{name:[}", "[^/]*")
	assert.Error(t, err)
	_, err = braceIndices("/bad}")
	assert.Error(t, err)
	_, _, err = compileServerTemplate("/{")
	assert.Error(t, err)
	_, _, err = compileServerTemplate("/{}")
	assert.Error(t, err)

	raw, decoded := (&routeFinder{}).extractPathParams("/{", "/x")
	assert.Nil(t, raw)
	assert.Nil(t, decoded)
	var nilFinder *routeFinder
	assert.Nil(t, nilFinder.compiledPath("/x", "/x"))
	assert.Equal(t, "%zz", decodePathValue("%zz"))
	assert.Equal(t, "a/b", decodePathValue("a%2Fb"))
}

func TestOperationExtractionAllMethods(t *testing.T) {
	operations := []*v3.Operation{{}, {}, {}, {}, {}, {}, {}, {}, {}}
	item := &v3.PathItem{
		Get: operations[0], Put: operations[1], Post: operations[2], Delete: operations[3],
		Options: operations[4], Head: operations[5], Patch: operations[6], Trace: operations[7], Query: operations[8],
	}
	methods := []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete, http.MethodOptions, http.MethodHead, http.MethodPatch, http.MethodTrace, "QUERY"}
	for i, method := range methods {
		assert.Same(t, operations[i], operationForMethod(item, method))
	}
	item.Head = nil
	assert.Same(t, item.Get, operationForMethod(item, http.MethodHead))
	assert.Nil(t, operationForMethod(nil, http.MethodGet))
	assert.Nil(t, operationForMethod(item, "UNKNOWN"))
	item.AdditionalOperations = orderedmap.New[string, *v3.Operation]()
	additional := &v3.Operation{}
	item.AdditionalOperations.Set("CUSTOM", additional)
	assert.Same(t, additional, operationForMethod(item, "custom"))
}

func TestRegexSelectionBranchesAndFragments(t *testing.T) {
	doc := model(t, `openapi: 3.1.0
info: {title: test, version: 1.0.0}
paths:
  /{left}/fixed:
    post: {responses: {"200": {description: ok}}}
  /fixed/{right}:
    get: {responses: {"200": {description: ok}}}
  /{one}/{two}:
    get: {responses: {"200": {description: ok}}}
  /fixed/{value}#fragment:
    get: {responses: {"200": {description: ok}}}`)
	r := &routeFinder{document: doc}
	item, path := r.regexLookup("/fixed/fixed", http.MethodGet)
	assert.NotNil(t, item)
	assert.Equal(t, "/fixed/{right}", path)
	item, path = r.regexLookup("/fixed/fixed", http.MethodPost)
	assert.NotNil(t, item)
	assert.Equal(t, "/{left}/fixed", path)
	item, path = r.regexLookup("/none", http.MethodGet)
	assert.Nil(t, item)
	assert.Empty(t, path)
	assert.Equal(t, "/fixed/{value}", normalizeFragment("/fixed/{value}#fragment", "/fixed/x"))
	assert.Equal(t, "/fixed/{value}#fragment", normalizeFragment("/fixed/{value}#fragment", "/fixed/x#fragment"))
	assert.Equal(t, 2000, specificity("//literal/fixed"))
}

func TestFindEmptyPathAndPathParameterMismatch(t *testing.T) {
	doc := model(t, `openapi: 3.1.0
info: {title: test, version: 1.0.0}
paths:
  /:
    get: {responses: {"200": {description: ok}}}`)
	r := NewRouter(doc).(*routeFinder)
	t.Cleanup(r.Release)
	route, err := r.find(httptestRequest(http.MethodGet, "http://example.com/"), "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "/", route.Path)
	raw, decoded := r.extractPathParams("/{id}", "/no/match")
	assert.Nil(t, raw)
	assert.Nil(t, decoded)
}

func TestServerHelperEdges(t *testing.T) {
	assert.Nil(t, collectServers(nil))
	doc := &v3.Document{Servers: []*v3.Server{nil, {URL: "/one"}}}
	servers := collectServers(doc)
	require.Len(t, servers, 1)

	_, _, ok := matchServer(nil, &v3.Server{URL: "/"})
	assert.False(t, ok)
	_, _, ok = matchCompiledServer(httptestRequest(http.MethodGet, "http://example.com/x"), &v3.Server{URL: "/"}, nil)
	assert.False(t, ok)
	_, _, ok = matchServer(&http.Request{}, &v3.Server{URL: "/"})
	assert.False(t, ok)
	_, _, ok = matchServer(httptestRequest(http.MethodGet, "http://example.com/x"), nil)
	assert.False(t, ok)

	request := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/api/pets"}, Host: "secure.example.com", TLS: &tls.ConnectionState{}}
	path, _, ok := matchServer(request, &v3.Server{URL: "https://secure.example.com/api"})
	assert.True(t, ok)
	assert.Equal(t, "/pets", path)
	request.TLS = nil
	path, _, ok = matchServer(request, &v3.Server{URL: "http://secure.example.com/api"})
	assert.True(t, ok)
	assert.Equal(t, "/pets", path)

	path, _, ok = matchServer(httptestRequest(http.MethodGet, "http://example.com/api"), &v3.Server{URL: "api/"})
	assert.True(t, ok)
	assert.Equal(t, "/", path)
	path, _, ok = matchServer(httptestRequest(http.MethodGet, "http://example.com/anything"), &v3.Server{})
	assert.True(t, ok)
	assert.Equal(t, "/anything", path)
	_, _, ok = matchServer(httptestRequest(http.MethodGet, "http://example.com/x"), &v3.Server{URL: "/{"})
	assert.False(t, ok)
	assert.Nil(t, serverVariable(nil, "x"))
	assert.Nil(t, serverVariable(&v3.Server{}, "x"))
}

func TestCompatibilityPathEdges(t *testing.T) {
	request := &http.Request{URL: &url.URL{Path: "plain", Fragment: "frag"}}
	assert.Equal(t, "/plain#frag", compatibilityPath(request, nil))
	doc := &v3.Document{Servers: []*v3.Server{nil, {URL: "http://[::1"}, {URL: "/api"}}}
	request.URL = &url.URL{Path: "/api/items"}
	assert.Equal(t, "/items", compatibilityPath(request, doc))
	assert.Nil(t, compatibilityServer(nil, doc))
	assert.Nil(t, compatibilityServer(&http.Request{}, doc))
	assert.Nil(t, compatibilityServer(request, nil))
	assert.Same(t, doc.Servers[2], compatibilityServer(request, doc))
	assert.Nil(t, compatibilityServer(httptestRequest(http.MethodGet, "http://example.com/other"), doc))
}

func TestConcurrentLookupAndRelease(t *testing.T) {
	r := NewRouter(model(t, `openapi: 3.1.0
info: {title: test, version: 1.0.0}
paths:
  /items/{id}:
    get: {responses: {"200": {description: ok}}}`))
	request := httptestRequest(http.MethodGet, "http://example.com/items/1")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_, _ = r.FindRoute(request)
			}
		}()
	}
	r.Release()
	wg.Wait()
}

func httptestRequest(method, target string) *http.Request {
	parsed, err := url.Parse(target)
	if err != nil {
		panic(err)
	}
	return &http.Request{Method: method, URL: parsed, Host: parsed.Host, Header: make(http.Header)}
}

func BenchmarkRouterStatic(b *testing.B) {
	r := NewRouter(model(b, routingSpec))
	b.Cleanup(r.Release)
	request := httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/pets/mine")
	b.ReportAllocs()
	for b.Loop() {
		_, _ = r.FindRoute(request)
	}
}

func BenchmarkRouterTemplated(b *testing.B) {
	r := NewRouter(model(b, routingSpec))
	b.Cleanup(r.Release)
	request := httptestRequest(http.MethodGet, "http://ignored/operation/v3/pets/123")
	b.ReportAllocs()
	for b.Loop() {
		_, _ = r.FindRoute(request)
	}
}

func BenchmarkRouterRegexFallback(b *testing.B) {
	r := NewRouter(model(b, routingSpec))
	b.Cleanup(r.Release)
	request := httptestRequest(http.MethodGet, "https://acme.example.com:8443/doc/v1/entities('42')")
	b.ReportAllocs()
	for b.Loop() {
		_, _ = r.FindRoute(request)
	}
}

func BenchmarkRouterCompatibilityStatic(b *testing.B) {
	r := NewRouter(model(b, `openapi: 3.1.0
info: {title: benchmark, version: 1.0.0}
paths:
  /health:
    get: {responses: {"204": {description: ok}}}`), WithPathOnlyMatching())
	b.Cleanup(r.Release)
	request := httptestRequest(http.MethodGet, "http://example.com/health")
	b.ReportAllocs()
	for b.Loop() {
		_, _ = r.FindRoute(request)
	}
}
