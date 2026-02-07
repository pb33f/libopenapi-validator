# Architecture Review: libopenapi-validator

**Date**: 2026-02-06
**Purpose**: Document the current architecture, identify systemic inefficiencies, and provide
the foundation for a targeted redesign.

---

## 1. What the Library Does

libopenapi-validator validates HTTP requests and responses against an OpenAPI 3.x specification.
It handles:

- **Path matching**: Resolving a request URL to the correct `PathItem` in the spec
- **Parameter validation**: Path, query, header, cookie parameters + security requirements
- **Request body validation**: JSON request bodies against the spec's schema
- **Response body validation**: JSON response bodies against the spec's schema
- **Document validation**: The spec itself against the OpenAPI meta-schema

The library is designed to be instantiated once (during service startup) and reused across
all incoming requests.

---

## 2. Initialization Flow

```
NewValidator(document)
 ├── document.BuildV3Model()            → *v3.Document
 ├── Build radix tree                   → PathLookup (O(k) path matching)
 ├── warmSchemaCaches()                 → Pre-compile all request/response/param schemas
 ├── warmRegexCache()                   → Pre-compile all path segment regexes
 └── Create sub-validators (each gets a copy of the document + options):
      ├── parameters.NewParameterValidator(doc, opts)
      ├── requests.NewRequestBodyValidator(doc, opts)
      └── responses.NewResponseBodyValidator(doc, opts)
```

**Key detail**: Each sub-validator stores its own reference to the `*v3.Document` and
`*ValidationOptions`. The options contain shared caches (`SchemaCache`, `RegexCache`,
`PathTree`), so cache reads/writes are shared across sub-validators.

---

## 3. Per-Request Validation Flow

### 3.1 Entry Points

The top-level `Validator` interface offers three entry points:

| Method | What it validates |
|--------|-------------------|
| `ValidateHttpRequest(req)` | Path + params + request body |
| `ValidateHttpResponse(req, resp)` | Path + response body + response headers |
| `ValidateHttpRequestResponse(req, resp)` | All of the above combined |

### 3.2 Request Validation (POST with body)

```
ValidateHttpRequest(request)
 │
 ├── request has body? → YES → ValidateHttpRequestWithPathItem (async path)
 │                       NO  → ValidateHttpRequestSync (sync fast-path)
 │
 │   ┌─── Step 1: FindPath ─────────────────────────────────────────────┐
 │   │ StripRequestPath(request, document)  → stripped path             │
 │   │ PathTree.Lookup(stripped)             → PathItem + matchedPath   │
 │   │ (if miss: regex fallback over ALL paths)                         │
 │   │                                                                  │
 │   │ RETURNS: (PathItem, errors, matchedPath)                         │
 │   │ DISCARDS: stripped path, path segments, parameter values         │
 │   └──────────────────────────────────────────────────────────────────┘
 │
 │   ┌─── Step 2: Async Validation ─────────────────────────────────────┐
 │   │                                                                  │
 │   │  Creates 3 layers of channels:                                   │
 │   │    doneChan, errChan, controlChan    (top-level orchestration)   │
 │   │    paramErrs, paramControlChan       (param sub-orchestration)   │
 │   │    paramFunctionControlChan          (param completion signal)   │
 │   │                                                                  │
 │   │  Spawns goroutines:                                              │
 │   │    1× runValidation listener                                     │
 │   │    1× parameterValidationFunc wrapper                            │
 │   │      └── 1× paramListener goroutine                              │
 │   │      └── 5× validateParamFunction goroutines:                    │
 │   │           ├── ValidatePathParamsWithPathItem                     │
 │   │           ├── ValidateCookieParamsWithPathItem                   │
 │   │           ├── ValidateHeaderParamsWithPathItem                   │
 │   │           ├── ValidateQueryParamsWithPathItem                    │
 │   │           └── ValidateSecurityWithPathItem                       │
 │   │    1× requestBodyValidationFunc goroutine                        │
 │   │                                                                  │
 │   │  Total: 9 goroutines + 5 channels per POST request              │
 │   │                                                                  │
 │   │  After completion: sorts errors for deterministic ordering       │
 │   └──────────────────────────────────────────────────────────────────┘
 │
 │   ┌─── Step 3: What Each Parameter Validator Does ───────────────────┐
 │   │                                                                  │
 │   │  EVERY parameter validator independently:                        │
 │   │    1. Calls helpers.ExtractParamsForOperation(request, pathItem) │
 │   │       → switch on HTTP method, append path + operation params    │
 │   │    2. Filters to its own param type (in: path/query/header/etc)  │
 │   │    3. Performs type-specific extraction + schema validation       │
 │   │                                                                  │
 │   │  PATH PARAMS specifically:                                       │
 │   │    1. Re-strips the request path (StripRequestPath)              │
 │   │    2. Re-splits both paths into segments (strings.Split)         │
 │   │    3. For each path segment:                                     │
 │   │       - Loads/compiles regex (same regex used in FindPath)       │
 │   │       - Runs FindStringSubmatch (same match done in FindPath)    │
 │   │       - Calls BraceIndices (same parse done in regex compilation)│
 │   │       - Extracts parameter values from regex groups              │
 │   │       - Validates each value against its schema                  │
 │   │                                                                  │
 │   │  This means a single GET request with 2 path params does:       │
 │   │    - 1× StripRequestPath in FindPath                            │
 │   │    - 1× StripRequestPath in path param validator                │
 │   │    - 1× regex match per segment in FindPath                     │
 │   │    - 1× regex match per segment in path param validator         │
 │   │    - 5× ExtractParamsForOperation (once per validator type)      │
 │   └──────────────────────────────────────────────────────────────────┘
 │
 │   ┌─── Step 4: Body Validation ──────────────────────────────────────┐
 │   │  1. Extracts operation from PathItem + HTTP method               │
 │   │  2. Extracts content type from request header                    │
 │   │  3. Matches media type (supports wildcards)                      │
 │   │  4. Only validates JSON content types (checks for "json" substr) │
 │   │  5. Looks up compiled schema from cache (by GoLow().Hash())      │
 │   │  6. On cache miss: renders schema → YAML→JSON → compile → cache  │
 │   │  7. Reads body: io.ReadAll(request.Body)                        │
 │   │  8. Re-seats body: request.Body = io.NopCloser(bytes.NewBuffer) │
 │   │  9. Decodes JSON: json.Unmarshal into interface{}               │
 │   │ 10. Validates decoded object against compiled schema             │
 │   │ 11. Processes errors: locates violations in YAML node tree       │
 │   └──────────────────────────────────────────────────────────────────┘
```

### 3.3 Request Validation (GET / no body)

```
ValidateHttpRequest(request)
 │
 ├── body == nil → ValidateHttpRequestSync
 │
 │   ┌─── FindPath (same as above) ────────────────────────────────────┐
 │   └──────────────────────────────────────────────────────────────────┘
 │
 │   ┌─── Sequential Parameter Validation ─────────────────────────────┐
 │   │  for each of [PathParams, CookieParams, HeaderParams,           │
 │   │               QueryParams, Security]:                            │
 │   │    valid, errs := validator(request, pathItem, pathValue)        │
 │   │                                                                  │
 │   │  (Same redundant work as async, just sequential)                │
 │   └──────────────────────────────────────────────────────────────────┘
 │
 │   ┌─── Body Validation (same as above, but body will be empty) ─────┐
 │   └──────────────────────────────────────────────────────────────────┘
```

**Note**: The sync path still calls the body validator even for GET requests.
The body validator just returns `true, nil` when `operation.RequestBody == nil`.

### 3.4 Response Validation

```
ValidateHttpResponse(request, response)
 │
 ├── FindPath(request, ...)    → PathItem, matchedPath
 │   (identical call to request validation — same work)
 │
 ├── ValidateResponseBodyWithPathItem(request, response, pathItem, matchedPath)
 │   │
 │   ├── ExtractOperation(request, pathItem)  → operation
 │   ├── Match status code to spec (exact → range → default)
 │   ├── Match content type to spec media types
 │   ├── Validate response body schema (same flow as request body)
 │   └── Validate response headers (if defined in spec)
```

### 3.5 Combined Request + Response Validation

```
ValidateHttpRequestResponse(request, response)
 │
 ├── FindPath(request, ...)    → PathItem, matchedPath
 │
 ├── ValidateHttpRequestWithPathItem(request, pathItem, matchedPath)
 │   └── (Full request validation: params + body, async or sync)
 │
 ├── ValidateResponseBodyWithPathItem(request, response, pathItem, matchedPath)
 │   └── (Full response validation: body + headers)
 │
 └── Combines errors from both
```

**Good**: `ValidateHttpRequestResponse` calls `FindPath` once and passes the
result to both validators. This is one of the few places where derived state
is reused.

---

## 4. The `validationFunction` Signature

This is the type signature that every parameter validator must conform to:

```go
type validationFunction func(
    request *http.Request,
    pathItem *v3.PathItem,
    pathValue string,
) (bool, []*errors.ValidationError)
```

**This signature is the root cause of most redundant work.** Each validator receives
the raw `request`, the matched `PathItem`, and the matched path string — but no
pre-computed derived state. Every validator must independently:

1. Extract the operation from the PathItem for the request's HTTP method
2. Extract the parameter list from the operation + path-level params
3. Filter to the relevant parameter type
4. Re-derive any path/URL information it needs

There is no shared context object. There is no way to pass pre-computed results
between validators.

---

## 5. Identified Inefficiencies

### 5.1 Path Matching Runs Twice for Path Parameters

| Step | Where | What |
|------|-------|------|
| 1 | `FindPath` → `comparePaths` | Regex-matches each path segment to find the matching PathItem |
| 2 | `ValidatePathParamsWithPathItem` | Re-regex-matches the same segments to extract parameter values |

Both steps compile/load the same regex, match the same segments, and parse the
same brace indices. The only difference: Step 1 calls `MatchString` (boolean),
Step 2 calls `FindStringSubmatch` (captures groups).

**Impact**: ~21 allocs/op wasted for a path with 2 parameters.

### 5.2 `StripRequestPath` Called Twice

| Where | Result |
|-------|--------|
| `FindPath` (line 36) | Strips base path, returns stripped path |
| `ValidatePathParamsWithPathItem` (line 46) | Strips base path again from scratch |

Both calls parse server URLs, extract base paths, and strip them from the request
URL. The result from `FindPath` is not passed forward.

**Impact**: ~5 allocs/op wasted (URL parsing, string operations).

### 5.3 `ExtractParamsForOperation` Called 5 Times

Every parameter validator calls `helpers.ExtractParamsForOperation(request, pathItem)`,
which does a method switch and appends path-level + operation-level parameters.
The result is identical each time.

| Caller | Line |
|--------|------|
| `ValidatePathParamsWithPathItem` | path_parameters.go:50 |
| `ValidateQueryParamsWithPathItem` | query_parameters.go:52 |
| `ValidateHeaderParamsWithPathItem` | header_parameters.go:46 |
| `ValidateCookieParamsWithPathItem` | cookie_parameters.go:45 |
| `ValidateSecurityWithPathItem` | validate_security.go:46 (indirect) |

**Impact**: ~5 × 2 allocs/op (slice creation + append) = ~10 allocs/op wasted.

### 5.4 `ExtractOperation` Called Multiple Times

| Caller | Purpose |
|--------|---------|
| `ValidateRequestBodyWithPathItem` | To find the request body schema |
| `ValidateResponseBodyWithPathItem` | To find the response schema |
| Each `WithPathItem` param validator (indirectly via `ExtractParamsForOperation`) | To find params |

Same method switch, same result, called independently.

### 5.5 Async Validation Channel Overhead

For every POST/PUT/PATCH request, the async path creates:

| Resource | Count | Purpose |
|----------|-------|---------|
| Goroutines | 9 | 1 orchestrator + 1 param wrapper + 1 param listener + 5 param validators + 1 body validator |
| Channels | 5 | `doneChan`, `errChan`, `controlChan`, `paramErrs`, `paramControlChan` + `paramFunctionControlChan` |
| Mutex | 1 | Inside `runValidation` for error collection |

The 5 parameter validators (path, query, header, cookie, security) each run in
their own goroutine with their own channel signaling. For a typical request where
parameter validation takes microseconds, the goroutine scheduling and channel
send/receive overhead likely exceeds the actual validation cost.

**Comparison**: The sync path (used for GET/no-body) runs the same 5 validators
sequentially and avoids all this overhead. The fast-path optimization (line 207)
already recognizes that sync is better for simple requests.

### 5.6 Request and Response Body Validation Are ~95% Identical Code

Compare `requests/validate_request.go` (362 lines) with `responses/validate_response.go`
(376 lines). The only differences:

| Aspect | Request | Response |
|--------|---------|----------|
| Body source | `request.Body` | `response.Body` |
| Empty body | Error (if schema defined) | Success (no body is fine) |
| Error messages | "request body" | "response body" |
| Strict direction | `DirectionRequest` | `DirectionResponse` |

The schema cache lookup, compilation, JSON decoding, schema validation, error
processing, and strict mode checking are copy-pasted between the two files.
Both even define their own `var instanceLocationRegex = regexp.MustCompile(...)`.

**Impact**: Not a runtime cost, but a maintenance burden. A bug fix in one must
be duplicated in the other.

### 5.7 `config.NewValidationOptions` Called Per Schema Validation

Both `ValidateRequestSchema` and `ValidateResponseSchema` call
`config.NewValidationOptions(input.Options...)` on every invocation. This
creates a new `ValidationOptions`, a new `sync.Map` for `RegexCache`, and a new
`DefaultCache` for `SchemaCache` — then immediately overwrites them from the
`WithExistingOpts` option.

```go
// validate_request.go:44
validationOptions := config.NewValidationOptions(input.Options...)
```

The caller passes `config.WithExistingOpts(v.options)`, which copies the parent's
caches. But the default caches are still allocated then thrown away.

### 5.8 Body Read + Re-seat Pattern

```go
requestBody, _ = io.ReadAll(request.Body)
request.Body.Close()
request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
```

This pattern appears in both request and response body validators. It reads the
entire body into memory, closes the original reader, then wraps the bytes in a
new `NopCloser(NewBuffer(...))` so downstream code can re-read it.

If `ValidateHttpRequestResponse` validates both request and response, the request
body is read once (by request validation), re-seated, then the bytes sit in memory
unused by response validation (which reads `response.Body` instead). This is fine,
but if multiple validators needed the same body, each would call `io.ReadAll` on
the re-seated reader — reading the same bytes repeatedly.

### 5.9 `GoLow().Hash()` Called Multiple Times Per Schema

The schema cache uses `schema.GoLow().Hash()` as its key. This hash:
- Traverses the low-level schema structure
- Creates ordered maps for deterministic key ordering
- Allocates ~7 objects per call

It's called once for cache lookup, and if there's a cache miss, again for cache
storage. The perf investigation confirmed that caching the hash externally doesn't
work because libopenapi creates different pointer instances for the same logical
schema between warming and per-request validation.

### 5.10 Standalone Sub-Validator Public APIs Call FindPath Independently

Each sub-validator (`ParameterValidator`, `RequestBodyValidator`,
`ResponseBodyValidator`) has a "standalone" method that can be called directly:

```go
// Called by users who want to validate just params:
paramValidator.ValidatePathParams(request)           // calls FindPath internally
paramValidator.ValidateQueryParams(request)          // calls FindPath again
requestBodyValidator.ValidateRequestBody(request)    // calls FindPath again
responseBodyValidator.ValidateResponseBody(req, resp)// calls FindPath again
```

If a user calls these standalone methods individually, `FindPath` runs once per
call. The `WithPathItem` variants exist to avoid this, but they require the caller
to have already called `FindPath` themselves.

---

## 6. Caching Architecture

### 6.1 Two-Tier Model

| Tier | Scope | Contents | Thread-safe |
|------|-------|----------|-------------|
| Validator-level | Persistent across requests | `SchemaCache`, `RegexCache`, `PathTree` | Yes (`sync.Map`) |
| Per-request | None exists | Nothing | N/A |

There is no per-request cache or context object. Each validation phase starts from
scratch with only the raw `(request, pathItem, pathValue)` tuple.

### 6.2 SchemaCache

- **Key**: `uint64` from `schema.GoLow().Hash()`
- **Value**: `SchemaCacheEntry` containing rendered YAML, JSON, compiled schema, parsed YAML node
- **Populated**: During `warmSchemaCaches()` at init, or lazily on first access
- **Hit rate**: ~100% after warming (all schemas pre-compiled)
- **Issue**: Hash computation itself is expensive (~7 allocs) and uncacheable due to pointer instability

### 6.3 RegexCache

- **Key**: Path segment string (e.g., `"{ad_account_id}"`)
- **Value**: `*regexp.Regexp`
- **Populated**: During `warmRegexCache()` at init, or lazily in `comparePaths()`
- **Hit rate**: ~100% after warming
- **Pattern**: Every caller does the same nil-check-load-miss-compile-store dance

### 6.4 PathTree (Radix)

- **Lookup**: O(k) where k = path depth (typically 3-5 segments)
- **Handles**: Standard `{param}` paths
- **Doesn't handle**: Matrix (`{;param}`), label (`{.param}`), OData-style paths
- **Fallback**: Regex matching over all paths when radix misses

---

## 7. Code Structure Issues

### 7.1 Scattered Nil-Check Conditionals

The "is this optimization available?" pattern appears everywhere:

```go
// This pattern appears 10+ times across the codebase:
if options != nil && options.RegexCache != nil {
    if cached, found := options.RegexCache.Load(key); found {
        rgx = cached.(*regexp.Regexp)
    }
}
if rgx == nil {
    rgx, _ = helpers.GetRegexForPath(seg)
    if options != nil && options.RegexCache != nil {
        options.RegexCache.Store(seg, rgx)
    }
}
```

### 7.2 Information Loss at Function Boundaries

`FindPath` returns `(*v3.PathItem, []*errors.ValidationError, string)`. The string
is the matched path template. Everything else computed during matching — stripped
path, split segments, matched parameter values, specificity score, base paths —
is discarded.

`ValidatePathParamsWithPathItem` receives `(request, pathItem, pathValue)` and must
re-derive the stripped path, split segments, and parameter values that `FindPath`
already computed.

### 7.3 The `WithPathItem` Variant Explosion

To support both "standalone" and "pre-resolved" calling patterns, nearly every
validator method exists in two forms:

| Standalone (calls FindPath) | Pre-resolved (skips FindPath) |
|-----------------------------|-------------------------------|
| `ValidatePathParams` | `ValidatePathParamsWithPathItem` |
| `ValidateQueryParams` | `ValidateQueryParamsWithPathItem` |
| `ValidateHeaderParams` | `ValidateHeaderParamsWithPathItem` |
| `ValidateCookieParams` | `ValidateCookieParamsWithPathItem` |
| `ValidateSecurity` | `ValidateSecurityWithPathItem` |
| `ValidateRequestBody` | `ValidateRequestBodyWithPathItem` |
| `ValidateResponseBody` | `ValidateResponseBodyWithPathItem` |

That's 14 methods where 7 would suffice if there were a shared context.

---

## 8. Summary of Waste Per Request

### GET request with 2 path params, 1 query param (sync path)

| Wasted operation | Times | Est. allocs |
|-----------------|-------|-------------|
| `StripRequestPath` (redundant call) | 1 | ~5 |
| `ExtractParamsForOperation` (4 redundant calls) | 4 | ~8 |
| Path segment regex match (already done in FindPath) | 2 segments | ~9 |
| `BraceIndices` (already done in regex compilation) | 2 segments | ~4 |
| Path splitting (already done in FindPath) | 1 | ~2 |
| **Total estimated waste** | | **~28 allocs** |
| **Current measured total** | | **~115 allocs** |
| **Waste as % of total** | | **~24%** |

### POST request with body (async path)

All of the above waste, plus:

| Overhead | Est. allocs |
|----------|-------------|
| 9 goroutines created + scheduled | ~18 |
| 5 channels created | ~5 |
| Channel send/receive operations | ~15 |
| Error sorting after completion | ~2 |
| **Additional async overhead** | **~40 allocs** |

---

## 9. What a Redesign Should Address

Based on this review, the key problems are:

1. **No per-request shared state**: Each validator independently re-derives the same
   information. A `RequestContext` carrying the match result, extracted parameters,
   operation reference, and lazily-decoded body would eliminate ~24% of per-request
   allocations.

2. **Path matching discards parameter values**: The radix tree and regex matching
   already visit every path segment. Extracting parameter values during this traversal
   (instead of after) would eliminate the entire second regex pass in path parameter
   validation.

3. **Excessive goroutine/channel overhead for async validation**: 9 goroutines and 5
   channels for 6 validators is disproportionate. A `sync.WaitGroup` with a shared
   error slice (or `errgroup`) would achieve the same concurrency with far less
   overhead. Better yet, benchmarks should determine if async is even faster than
   sync for typical payloads — the scheduling overhead may exceed the parallelism
   benefit.

4. **Copy-pasted request/response body validation**: Extracting the shared validation
   logic into a single `validateBodySchema(body []byte, schema, version, direction)`
   function would halve the maintenance surface and ensure bug fixes apply to both
   paths.

5. **The `validationFunction` signature forces redundant work**: Changing it from
   `func(request, pathItem, pathValue)` to `func(*RequestContext)` would let
   validators access pre-computed state instead of re-deriving it.

6. **`NewValidationOptions` allocates throwaway caches**: The body validators create
   default caches that are immediately overwritten by `WithExistingOpts`. Either
   skip defaults when `WithExistingOpts` is provided, or pass the options directly
   instead of re-creating them.

7. **Dual API surface (`Standalone` + `WithPathItem`)**: With a `RequestContext`,
   both collapse into a single method. The standalone variant just builds the context
   first.
