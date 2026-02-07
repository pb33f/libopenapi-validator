# Proposal: Clean Architecture for Path Resolution & Validation Context

**Status**: Draft  
**Author**: Zach Hamm  
**Date**: 2026-02-06  
**Related**: `perf/fix-parameter-schema-cache` branch

## Motivation

The current validation pipeline has grown organically. Each optimization (radix tree,
regex cache, schema cache, sync fast-path) was bolted on with conditionals:

```go
// This pattern is everywhere:
if options != nil && options.PathTree != nil {
    // try radix tree
}
// fall back to regex
if options != nil && options.RegexCache != nil {
    // check cache
}
if rgx == nil {
    // compile regex
    if options != nil && options.RegexCache != nil {
        // store in cache
    }
}
```

The result is:

1. **Duplicated work**: Path matching and parameter extraction are the same operation
   run twice with different code paths.
2. **Scattered caching**: `RegexCache`, `SchemaCache`, and `PathTree` are all separate
   fields on `ValidationOptions` with independent nil-check patterns.
3. **No information flow**: `FindPath` knows the parameter values but throws them away.
   Parameter validation re-derives them from scratch.
4. **Hard to extend**: Adding a new matching strategy means editing `FindPath` internals.
5. **Hard to optimize**: Per-request state (stripped path, split segments, extracted params)
   is recomputed by every validation phase independently.

This proposal replaces the ad-hoc approach with three clean abstractions:

- **`PathResolver`** — a chain-of-responsibility interface for path matching
- **`MatchResult`** — a struct that carries everything learned during matching
- **`RequestContext`** — a per-request scratchpad that validation phases share

## Architecture Overview

```
                        ┌──────────────────────┐
                        │   Validator (init)    │
                        │                       │
                        │  Builds PathResolver  │
                        │  chain + warm caches  │
                        └───────────┬──────────┘
                                    │
                                    ▼
┌───────────────────────────────────────────────────────────────────┐
│                     Per-Request Flow                              │
│                                                                   │
│  Request ──► PathResolver.Resolve(req) ──► MatchResult            │
│                  │                             │                   │
│                  │  Chain:                      │  Contains:        │
│                  │  1. RadixResolver            │  - PathItem       │
│                  │  2. RegexResolver            │  - PathTemplate   │
│                  │  3. (user-provided?)         │  - PathParams     │
│                  │                              │  - StrippedPath   │
│                  │  First match wins.           │  - Segments       │
│                  │                              │                   │
│                  ▼                              ▼                   │
│              RequestContext ◄──── populated from MatchResult        │
│                  │                                                  │
│                  │  Carries:                                        │
│                  │  - MatchResult                                   │
│                  │  - Decoded body (if needed, computed once)       │
│                  │  - Per-request scratch data                      │
│                  │                                                  │
│                  ├──► PathParamValidator.Validate(ctx)              │
│                  ├──► QueryParamValidator.Validate(ctx)             │
│                  ├──► HeaderParamValidator.Validate(ctx)            │
│                  ├──► CookieParamValidator.Validate(ctx)            │
│                  ├──► SecurityValidator.Validate(ctx)               │
│                  └──► BodyValidator.Validate(ctx)                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Types

### `PathResolver` Interface

```go
// PathResolver finds the matching path for an incoming request.
// Implementations are composed into a chain — the first resolver
// that returns a match wins.
type PathResolver interface {
    // Resolve attempts to match the request path against known API paths.
    // Returns nil if this resolver cannot handle the path.
    Resolve(path string, document *v3.Document) *MatchResult
}
```

Simple. One method. Each implementation focuses on one matching strategy.

### Built-in Resolvers

```go
// RadixResolver uses the pre-built radix tree for O(k) matching.
// Handles standard {param} paths. Extracts parameter values during traversal.
type RadixResolver struct {
    tree *radix.PathTree
}

func (r *RadixResolver) Resolve(path string, document *v3.Document) *MatchResult {
    pathItem, matchedPath, params, found := r.tree.LookupWithParams(path)
    if !found {
        return nil  // Let the next resolver in the chain try
    }
    return &MatchResult{
        PathItem:    pathItem,
        MatchedPath: matchedPath,
        PathParams:  params,
    }
}


// RegexResolver handles complex paths (matrix, label, OData) via regex.
// Used as a fallback when the radix tree can't match.
type RegexResolver struct {
    regexCache config.RegexCache
}

func (r *RegexResolver) Resolve(path string, document *v3.Document) *MatchResult {
    // Current regex matching logic from paths.FindPath, but also
    // extracts and returns PathParams from the regex match groups.
    // ...
}
```

### Resolver Chain

```go
// ResolverChain tries each resolver in order. First match wins.
type ResolverChain struct {
    resolvers []PathResolver
}

func (c *ResolverChain) Resolve(path string, document *v3.Document) *MatchResult {
    for _, r := range c.resolvers {
        if result := r.Resolve(path, document); result != nil {
            return result
        }
    }
    return nil
}
```

This replaces the current hardcoded `if radixTree != nil { ... } // fall back to regex`
pattern in `FindPath`. Users can:

- Reorder the chain
- Insert their own resolver (e.g., a static map for known high-traffic paths)
- Remove the regex resolver entirely if they only use simple paths
- Add a resolver that does path rewriting before matching

### `MatchResult` Struct

```go
// MatchResult carries everything learned during path matching.
// This is the single source of truth for "what matched and what was extracted."
type MatchResult struct {
    // PathItem is the matched OpenAPI path item definition.
    PathItem *v3.PathItem

    // MatchedPath is the path template (e.g., "/users/{id}/posts").
    MatchedPath string

    // PathParams maps parameter names to their raw extracted values.
    // e.g., {"account_id": "abc123", "campaign_id": "xyz789"}
    // nil means the resolver didn't extract params (validator should fall back).
    PathParams map[string]string

    // StrippedPath is the request path with base path removed.
    // Computed once, reused by all validators.
    StrippedPath string

    // Segments is the pre-split path for validators that need it.
    // Avoids repeated strings.Split calls.
    Segments []string
}
```

### `RequestContext` — Per-Request Shared State

This is the key piece that eliminates redundant work across validation phases.

```go
// RequestContext carries per-request state that flows through the entire
// validation pipeline. It's created once per request and shared (read/write)
// by all validators.
//
// This replaces the pattern where each validator independently re-derives
// the same information (stripped path, split segments, decoded body, etc.).
type RequestContext struct {
    Request      *http.Request
    Match        *MatchResult
    Operation    *v3.Operation      // Resolved from PathItem + HTTP method
    Parameters   []*v3.Parameter    // Extracted from PathItem + Operation

    // Lazy-computed fields (populated on first access, reused after)
    decodedBody  any                // JSON-decoded request body
    bodyBytes    []byte             // Raw body bytes (read once)
    bodyRead     bool               // Whether body has been read
}

// Body returns the raw body bytes, reading from the request at most once.
func (ctx *RequestContext) Body() ([]byte, error) {
    if !ctx.bodyRead {
        // Read and store — subsequent calls return cached bytes
    }
    return ctx.bodyBytes, nil
}

// DecodedBody returns the JSON-decoded body, decoding at most once.
func (ctx *RequestContext) DecodedBody() (any, error) {
    if ctx.decodedBody == nil {
        // Decode and store
    }
    return ctx.decodedBody, nil
}
```

### Validator-Level Cache (Persistent)

The validator already has `SchemaCache` and `RegexCache` on `ValidationOptions`. This
proposal doesn't change those — they're validator-level caches that persist across
requests. But it clarifies the two-tier model:

```
┌──────────────────────────────────────────────────┐
│            Validator-Level (persistent)           │
│                                                    │
│  SchemaCache   — compiled JSON schemas             │
│  RegexCache    — compiled path regexes             │
│  PathTree      — radix tree (via RadixResolver)    │
│                                                    │
│  Created once during NewValidator.                 │
│  Shared across all requests. Thread-safe.          │
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│           Request-Level (per-request)             │
│                                                    │
│  RequestContext:                                    │
│    MatchResult  — which path matched, params       │
│    Operation    — resolved operation for method     │
│    Parameters   — params for this operation         │
│    Body bytes   — read once, shared across phases   │
│    Decoded body — decoded once, validated by many   │
│                                                    │
│  Created per request. Not shared across requests.  │
│  No synchronization needed.                        │
└──────────────────────────────────────────────────┘
```

## What This Simplifies

### Before: `ValidatePathParamsWithPathItem` (current code)

```go
func (v *paramValidator) ValidatePathParamsWithPathItem(
    request *http.Request, pathItem *v3.PathItem, pathValue string,
) (bool, []*errors.ValidationError) {

    // 1. Re-strip the request path (already done in FindPath)
    submittedSegments := strings.Split(paths.StripRequestPath(request, v.document), "/")
    pathSegments := strings.Split(pathValue, "/")

    // 2. Re-extract params for operation (could be shared)
    params := helpers.ExtractParamsForOperation(request, pathItem)

    for _, p := range params {
        if p.In == helpers.Path {
            // 3. For each param, iterate ALL segments
            for x := range pathSegments {
                // 4. Check regex cache (nil-check pattern)
                if v.options.RegexCache != nil {
                    if cached, found := v.options.RegexCache.Load(pathSegments[x]); found {
                        rgx = cached.(*regexp.Regexp)
                    }
                }
                // 5. Compile regex if not cached (even for literal segments!)
                if rgx == nil {
                    r, err := helpers.GetRegexForPath(pathSegments[x])
                    // ...
                }
                // 6. Regex match to extract value
                matches := rgx.FindStringSubmatch(submittedSegments[x])
                // 7. Parse brace indices
                idxs, _ := helpers.BraceIndices(pathSegments[x])
                // 8. Finally validate the value
                // ...
            }
        }
    }
}
```

Steps 1-7 are redundant — the path resolver already did this work (or could have).

### After: With `RequestContext`

```go
func (v *paramValidator) ValidatePathParams(ctx *RequestContext) []*errors.ValidationError {
    var errs []*errors.ValidationError

    for _, p := range ctx.Parameters {
        if p.In != helpers.Path {
            continue
        }

        // Fast path: parameter value already extracted during path matching
        if value, ok := ctx.Match.PathParams[p.Name]; ok {
            decodedValue, _ := url.PathUnescape(value)
            errs = append(errs, validatePathParamValue(p, decodedValue, v.options)...)
            continue
        }

        // Slow path: complex param style (matrix/label/etc.) needs regex extraction
        errs = append(errs, v.extractAndValidateComplexParam(p, ctx)...)
    }

    return errs
}
```

The common case (simple `{param}`) is a map lookup + value validation. No regex, no
segment iteration, no `BraceIndices`, no nil-check conditionals.

### Before: `ValidateHttpRequestSync` (current code)

```go
func (v *validator) ValidateHttpRequestSync(request *http.Request) (bool, []*errors.ValidationError) {
    // Step 1: Find the path (strips URL, splits segments, matches)
    pathItem, errs, foundPath := paths.FindPath(request, v.v3Model, v.options)
    if len(errs) > 0 {
        return false, errs
    }

    // Step 2: Each validator re-derives what it needs from scratch
    // (re-strips path, re-splits segments, re-extracts params, etc.)
    for _, validateFunc := range []validationFunction{
        paramValidator.ValidatePathParamsWithPathItem,    // re-strips path, re-splits, regex
        paramValidator.ValidateCookieParamsWithPathItem,  // re-extracts params
        paramValidator.ValidateHeaderParamsWithPathItem,  // re-extracts params
        paramValidator.ValidateQueryParamsWithPathItem,   // re-extracts params
        paramValidator.ValidateSecurityWithPathItem,      // re-extracts security
    } {
        valid, pErrs := validateFunc(request, pathItem, foundPath)
        // ...
    }

    // Step 3: Body validator reads and decodes body
    valid, pErrs := reqBodyValidator.ValidateRequestBodyWithPathItem(request, pathItem, foundPath)
    // ...
}
```

### After: With `RequestContext`

```go
func (v *validator) ValidateHttpRequestSync(request *http.Request) (bool, []*errors.ValidationError) {
    // Step 1: Resolve path (one call, captures everything)
    ctx, errs := v.buildRequestContext(request)
    if errs != nil {
        return false, errs
    }

    // Step 2: Each validator reads from shared context — no redundant work
    validators := []func(*RequestContext) []*errors.ValidationError{
        v.paramValidator.ValidatePathParams,
        v.paramValidator.ValidateCookieParams,
        v.paramValidator.ValidateHeaderParams,
        v.paramValidator.ValidateQueryParams,
        v.paramValidator.ValidateSecurity,
        v.requestValidator.ValidateBody,
    }

    var validationErrors []*errors.ValidationError
    for _, validate := range validators {
        validationErrors = append(validationErrors, validate(ctx)...)
    }

    return len(validationErrors) == 0, validationErrors
}

func (v *validator) buildRequestContext(request *http.Request) (*RequestContext, []*errors.ValidationError) {
    result := v.resolver.Resolve(StripRequestPath(request, v.v3Model), v.v3Model)
    if result == nil {
        return nil, pathNotFoundErrors(request)
    }

    operation := getOperationForMethod(result.PathItem, request.Method)
    if operation == nil {
        return nil, methodNotAllowedErrors(request, result.MatchedPath)
    }

    return &RequestContext{
        Request:    request,
        Match:      result,
        Operation:  operation,
        Parameters: extractParams(result.PathItem, operation),
    }, nil
}
```

## User-Extensible Path Resolution

### Static Map Resolver (Example)

A user who knows their high-traffic paths could provide a zero-allocation resolver:

```go
type StaticResolver struct {
    paths map[string]*v3.PathItem
}

func (r *StaticResolver) Resolve(path string, _ *v3.Document) *MatchResult {
    if pathItem, ok := r.paths[path]; ok {
        return &MatchResult{PathItem: pathItem, MatchedPath: path}
    }
    return nil
}

// Usage:
validator, _ := validator.NewValidator(doc,
    config.WithResolvers(
        &StaticResolver{paths: myHotPaths},  // Check known paths first
        resolver.NewRadixResolver(doc),       // Then radix tree
        resolver.NewRegexResolver(),          // Then regex fallback
    ),
)
```

### Rewriting Resolver (Example)

A user who needs path normalization before matching:

```go
type RewriteResolver struct {
    rules map[string]string  // "/v2/campaigns" -> "/v3/campaigns"
    next  PathResolver
}

func (r *RewriteResolver) Resolve(path string, doc *v3.Document) *MatchResult {
    if rewritten, ok := r.rules[path]; ok {
        path = rewritten
    }
    return r.next.Resolve(path, doc)
}
```

## Configuration

### New Options

```go
// WithResolvers sets the path resolver chain.
// Default: [RadixResolver, RegexResolver]
func WithResolvers(resolvers ...PathResolver) Option {
    return func(o *ValidationOptions) {
        o.Resolvers = resolvers
    }
}

// WithoutRegexFallback removes the regex resolver from the default chain.
// Useful when all paths are simple {param} style and you want maximum performance.
func WithoutRegexFallback() Option {
    return func(o *ValidationOptions) {
        o.Resolvers = []PathResolver{NewRadixResolver(/* ... */)}
    }
}
```

### Default Behavior (Unchanged)

If no resolvers are configured, the default chain is `[RadixResolver, RegexResolver]`,
which matches today's behavior exactly. Existing code continues to work.

## Migration Path

This is a significant internal refactoring, but the public API can remain backward
compatible. Suggested approach:

### Phase 1: Internal Types (No Public API Change)

1. Define `MatchResult` and `RequestContext` as internal types
2. Add `LookupWithParams` to the radix tree
3. Have `FindPath` internally build a `MatchResult`
4. Thread `RequestContext` through the internal validation pipeline
5. Keep all existing public method signatures working via adapter code

At this point, the internal flow is clean but the public API is unchanged.

### Phase 2: PathResolver Interface (Public, Additive)

1. Export `PathResolver` interface and `MatchResult`
2. Add `WithResolvers(...)` option
3. Export `RadixResolver` and `RegexResolver` as usable implementations
4. Default behavior unchanged — opt-in for users who want custom resolvers

### Phase 3: Simplified Validator Methods (Breaking, Major Version)

1. Simplify `ParameterValidator` interface to accept `RequestContext`
2. Remove the duplicated `WithPathItem` variants
3. Deprecate direct `FindPath` usage in favor of resolver chain

This phase is a breaking change and should be a major version bump.

### Phase 4: Remove Legacy Code

1. Remove old code paths once the new architecture is stable
2. Remove nil-check conditionals throughout the codebase

## Expected Impact

### Performance (Per-Request, Simple GET with 2 Path Params)

| Metric | Current (no cache) | Current (with cache) | With RequestContext |
|--------|--------------------|---------------------|---------------------|
| Time | 21,200 ns | 4,500 ns | ~2,000 ns (est.) |
| Memory | 23,833 B | 4,687 B | ~1,500 B (est.) |
| Allocs | 400 | 115 | ~30 (est.) |

The remaining ~30 allocs would be:
- Radix tree lookup: ~4 allocs (path splitting)
- PathParams map: ~1 alloc
- RequestContext: ~1 alloc
- Parameters slice: ~1 alloc
- Validation error slice: ~1 alloc
- Value validation (schema checks): ~20 allocs

### Code Complexity

- Removes ~15 nil-check conditional blocks across the codebase
- Removes duplicated path stripping / segment splitting in every validator
- Removes the `WithPathItem` method variants (or makes them thin adapters)
- Each validator becomes a simple function of `RequestContext` → `[]ValidationError`

## Open Questions

1. **Should `PathResolver.Resolve` accept `*http.Request` instead of just the path
   string?** Some resolvers might want access to headers or method. But keeping it
   to just the path string keeps resolvers simpler and more testable. Method matching
   is currently done separately after path matching, which seems right.

2. **Should `RequestContext` use `context.Context` or be a standalone struct?**
   Using `context.Context` would let us attach it to the request, but adds
   `context.Value` type assertions. A standalone struct is simpler and type-safe.
   Recommendation: standalone struct, passed explicitly.

3. **How to handle the `ValidateHttpRequestWithPathItem` methods?** These accept
   an explicit PathItem + pathValue (bypassing FindPath). Options:
   - Keep them as thin adapters that build a partial `RequestContext`
   - Deprecate them in favor of a single entry point
   - Keep them but have them create a `MatchResult` from the provided PathItem

4. **Should `RegexResolver` populate `PathParams` too?** Yes — `comparePaths` already
   does the regex work. Extracting submatch groups at that point means parameter
   validation can use the fast path even for regex-matched paths.

5. **Thread safety of `RequestContext`**: Since it's per-request and not shared across
   goroutines (even in the async validation path, each goroutine reads from it but
   doesn't write), it should be safe without synchronization. But the async path
   needs careful review — if multiple goroutines call `ctx.Body()` simultaneously,
   the lazy-init needs a `sync.Once` or similar. Worth thinking through the async
   validation story separately.

6. **Naming**: `PathResolver` vs `PathMatcher` vs `RouteMatcher`? `RequestContext`
   vs `ValidationContext` vs `RequestState`? Open to better names.

## Summary

The current architecture bolts features on with conditionals. This proposal replaces
that with three clean abstractions:

| Abstraction | Replaces | Benefit |
|------------|----------|---------|
| **PathResolver chain** | Hardcoded if/else in FindPath | Extensible, testable, composable |
| **MatchResult** | Thrown-away radix tree knowledge | Zero redundant work in param validation |
| **RequestContext** | Repeated derivation in every validator | Compute once, share everywhere |

The default behavior stays the same. The internal code gets simpler. Users get extension
points they don't have today. And per-request allocations drop from ~115 to ~30 for
the common case.
