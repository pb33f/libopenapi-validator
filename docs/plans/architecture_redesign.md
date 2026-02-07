---
name: Validator Architecture Redesign
overview: Iterative internal refactoring of libopenapi-validator to eliminate redundant per-request work (~24% of allocations), simplify the async validation path, unify duplicated code, fix options/cache plumbing ergonomics, and add opt-in lazy error resolution — all while preserving the existing public API and using the existing 1,159 tests as a safety net.
todos:
  - id: phase-0
    content: "Phase 0: Create branch, Makefile, capture baseline benchmarks"
    status: pending
  - id: phase-1
    content: "Phase 1: pathMatcher interface + radix/regex matchers + matcherChain + operationForMethod consolidation"
    status: pending
  - id: phase-2
    content: "Phase 2: resolvedRoute + requestContext types + buildRequestContext + cached version float"
    status: pending
  - id: phase-3
    content: "Phase 3: Thread requestContext through sync validation path"
    status: pending
  - id: phase-4
    content: "Phase 4: Thread requestContext through async path + simplify channels"
    status: pending
  - id: phase-5
    content: "Phase 5: Regex matcher extracts path params into resolvedRoute"
    status: pending
  - id: phase-6
    content: "Phase 6: Lazy error schema resolution (WithLazyErrors option) + json.Marshal improvements"
    status: pending
  - id: phase-7
    content: "Phase 7: Unify request/response body validation + shared error loop + LocateSchema fix + IgnoreRegex to string checks"
    status: pending
  - id: phase-8
    content: "Phase 8: Options plumbing (ValidateSchemaInput type change, WithExistingOpts struct copy, NewValidationOptions lazy alloc, base paths, segments)"
    status: pending
  - id: phase-9
    content: "Phase 9: Final benchmarks, documentation, PR summary"
    status: pending
isProject: false
---

# Validator Architecture Redesign

## Guiding Principles

- **Backward compatible by default.** The `Validator`, `ParameterValidator`, `RequestBodyValidator`, and `ResponseBodyValidator` interfaces stay as-is. The `WithPathItem` method variants remain functional (as thin adapters over the new internals). New capabilities are opt-in via functional options.
- **Every phase is a commit.** Each commit must leave the tree green (tests pass, benchmarks no worse).
- **The unexported `validationFunction` type can change freely** — it is internal to `validator.go` and not part of the public API.
- **The unexported structs (`paramValidator`, `requestBodyValidator`, `responseBodyValidator`) can gain new internal methods freely** — only the interface methods are public.
- **The `PathLookup` interface is in PR and not released** — we can freely modify it.

## Things to Watch Out For

- **Race conditions**: When we change the async path, we must run tests with `-race`. The current channel-heavy approach is race-safe by design; a simpler approach must be equally safe.
- **Both matchers must produce `resolvedRoute`**: The `requestContext` must work for both radix-matched and regex-fallback-matched paths. Both matchers populate the same result type.
- **Response validation**: `ValidateHttpResponse` calls `FindPath` too. It should benefit from the same `resolvedRoute` infrastructure even though it doesn't do parameter validation.
- **`ValidateHttpRequestResponse`**: Already calls `FindPath` once and reuses the result. Our refactoring must not regress this.
- **golangci-lint**: The repo has a `.golangci.yml`. We should run it as part of our checks.

---

## Identified Issues

This section catalogs all issues found during the architecture review and final audit. Each issue is tagged with the phase that addresses it.

### Cache/Options Ergonomics

#### The allocate-then-discard problem *(Phase 8)*

Every call to `NewValidationOptions()` eagerly allocates two objects that get thrown away when `WithExistingOpts` overwrites them:

```go
// config.go:52-58 — allocates defaults that get immediately discarded
o := &ValidationOptions{
    SchemaCache: cache.NewDefaultCache(), // allocs: DefaultCache + sync.Map
    RegexCache:  &sync.Map{},             // alloc: sync.Map
}
```

This happens on **every request body validation** (not just init), because `ValidateRequestSchema` (line 45) and `ValidateResponseSchema` (line 49) both call `config.NewValidationOptions(input.Options...)`, where `input.Options` is `[]config.Option{config.WithExistingOpts(v.options)}`.

**Per-request waste**: 2 heap allocations (DefaultCache + sync.Map for SchemaCache, 1 sync.Map for RegexCache) created then immediately garbage-collected. For a POST request that validates both request and response body, that is 4 wasted allocations per request.

**Fix (two-part)**:

1. Change `ValidateRequestSchemaInput.Options` and `ValidateResponseSchemaInput.Options` from `[]config.Option` to `*config.ValidationOptions` so callers pass the options struct directly instead of wrapping in a closure. This eliminates the reconstruction entirely.
2. As a safety net, make `NewValidationOptions` not allocate caches when it detects options will be applied (or defer allocation with a lazy init pattern).

#### `WithExistingOpts` is a fragile field-by-field copy *(Phase 8)*

`config.go:70-91` — every new field added to `ValidationOptions` must be manually added to `WithExistingOpts` or it silently gets lost. This is a maintenance trap.

**Fix**: Replace field-by-field copy with struct dereference: `*o = *options`, then re-apply any overrides from additional opts.

### Per-Request Redundancy

#### Path matching runs twice *(Phase 1, 2, 3)*

The validator first matches the request path to a PathItem (via regex or radix tree), then later re-runs similar regex matching to extract path parameter values. The `resolvedRoute` + `requestContext` approach extracts path params during the initial match.

#### `ExtractParamsForOperation` called 5 times per request *(Phase 2, 3)*

Each parameter validator independently calls `ExtractParamsForOperation`, which switches on HTTP method to find the operation, then iterates its parameters. The `requestContext` extracts parameters once and shares them.

#### `ExtractOperation` called multiple times *(Phase 1, 2)*

Multiple code paths independently resolve the operation from the PathItem + HTTP method. The `resolvedRoute` can carry the resolved operation.

#### `StripRequestPath` called twice *(Phase 2)*

The request path is stripped of its base path in multiple places. The `requestContext` strips once and caches the result.

### Code Duplication

#### Four identical HTTP method switch statements *(Phase 1)*

- `helpers/operation_utilities.go:16-34` — `ExtractOperation`
- `helpers/parameter_utilities.go:31-64` — `ExtractParamsForOperation`
- `helpers/parameter_utilities.go:71-104` — `ExtractSecurityForOperation`
- `paths/specificity.go:56-74` — `pathHasMethod`

All switch on HTTP method with the same 8 cases (GET, POST, PUT, DELETE, OPTIONS, HEAD, PATCH, TRACE).

**Fix**: Create a single `operationForMethod(method string, pathItem *v3.PathItem) *v3.Operation` that returns the operation pointer. All four functions delegate to it.

#### Duplicated error processing loop *(Phase 7)*

`validate_request.go:240-321` and `validate_response.go:258-335` — These ~80-line blocks are ~95% identical. They iterate schema errors, call `LocateSchemaPropertyNodeByJSONPath`, check `IgnoreRegex`, extract `instanceLocation`, build `SchemaValidationFailure`, call `json.MarshalIndent`, etc. Also, `var instanceLocationRegex = regexp.MustCompile(...)` is defined identically in both files.

#### ~95% identical request/response body validation *(Phase 7)*

`requests/validate_request.go` (362 lines) and `responses/validate_response.go` (376 lines) share nearly all logic: cache lookup, cache-miss compilation, JSON decoding, schema validation, error processing, strict mode.

### Per-Error Overhead

#### `LocateSchemaPropertyNodeByJSONPath` spawns a goroutine per error *(Phase 7)*

`schema_validation/locate_schema_property.go:14-42` — Creates 2 channels and spawns a goroutine just for panic recovery on every schema validation error. This adds goroutine scheduling overhead and heap escapes for the channel allocations.

**Fix**: Replace with a plain function that uses `defer/recover` without goroutines:

```go
func LocateSchemaPropertyNodeByJSONPath(doc *yaml.Node, JSONPath string) (result *yaml.Node) {
    defer func() { recover() }()
    _, path := utils.ConvertComponentIdIntoFriendlyPathSearch(JSONPath)
    if path == "" { return nil }
    jp, _ := jsonpath.NewPath(path)
    nodes := jp.Query(doc)
    if len(nodes) > 0 { return nodes[0] }
    return nil
}
```

#### `IgnoreRegex` runs a regex match on every error message *(Phase 7)*

`helpers/ignore_regex.go` — Pattern: `^'?(anyOf|allOf|oneOf|validation)'? failed(, none matched)?$`. This runs for every schema error across 4 call sites.

**Fix**: Replace with string suffix checks. The errors are short and well-known — a simple `strings.HasSuffix` or small set of exact-match comparisons is sufficient.

#### `json.MarshalIndent` called per error for `ReferenceObject` *(Phase 6)*

`validate_request.go:261` and `validate_response.go:277` — Pretty-prints a potentially large JSON object per error for the `ReferenceObject` field.

**Fix**: In lazy mode (Phase 6), defer entirely. In eager mode, use `json.Marshal` (no indent) to reduce formatting overhead.

#### `GoLow().Hash()` called multiple times for same schema *(Phase 7)*

In `validate_parameter.go`, `Hash()` is called at line 43 (cache check) and again at line 66 (cache store on miss). Same pattern in `validate_request.go` and `validate_response.go`. `GoLow().Hash()` traverses the low-level schema AST each time.

**Fix**: Store hash in a local variable and reuse it across the check/store boundary.

### Minor Inefficiencies

#### `VersionToFloat` called per body validation *(Phase 2)*

`requests/validate_body.go:86` and `responses/validate_body.go:147` — Calls `helpers.VersionToFloat(v.document.Version)` on every validation. The version is constant per document.

**Fix**: Cache the computed `float32` version on the sub-validator struct at construction time.

#### Async validation channel overhead *(Phase 4)*

The current async path for POST/PUT/PATCH uses 9 goroutines and 5 channels to orchestrate 6 validation functions. This is significant overhead for what is essentially "run N things concurrently, collect errors."

#### Strict mode re-walks the schema tree *(Future)*

`strict/schema_walker.go` recursively traverses the entire schema after base JSON Schema validation already traversed it. Out of scope for the current redesign but worth noting as a future optimization target.

---

## Phase 0: Baseline and Safety Net

**Goal**: Establish a reproducible test + benchmark baseline so we can verify every subsequent phase.

**Work**:

1. Create a new branch `refactor/request-context` from the current HEAD.
2. Add a `Makefile` with targets:
  - `make test` — runs `go test ./... -count=1 -race` (all 1,159 tests with race detector)
  - `make test-short` — runs `go test ./... -count=1 -short` (faster iteration)
  - `make lint` — runs `golangci-lint run`
  - `make bench-fast` — runs the fast benchmark suite (excludes init + prod benchmarks)
  - `make bench-baseline` — runs fast suite with `-count=5` and saves to `benchmarks/results/baseline.txt`
  - `make bench-compare` — runs fast suite with `-count=5`, saves to `benchmarks/results/current.txt`, runs `benchstat baseline.txt current.txt`
3. Run `make test` and `make bench-baseline` to capture the starting point.
4. Commit: *"Add Makefile with test, lint, and benchmark targets; save baseline"*

---

## Phase 1: `pathMatcher` Interface + Radix/Regex Matchers + Matcher Chain

**Goal**: Replace the hardcoded `if radixTree != nil { ... } // fallback to regex` pattern in `FindPath` with a composable chain of matchers that share a common interface. Also consolidate the 4 duplicated HTTP method switch statements into a single helper.

**Files changed**: New file `path_matcher.go` (root package, unexported), `helpers/operation_utilities.go`, `radix/tree.go`, `radix/path_tree.go`, `paths/paths.go`

**Work**:

1. **Consolidate HTTP method switches.** Create a single `operationForMethod(method string, pathItem *v3.PathItem) *v3.Operation` helper that returns the operation pointer for a given HTTP method. Refactor `ExtractOperation`, `ExtractParamsForOperation`, `ExtractSecurityForOperation`, and `pathHasMethod` to delegate to it.
2. Define the internal `pathMatcher` interface and `resolvedRoute` type:
  ```go
   // pathMatcher finds the matching path for an incoming request path.
   // Implementations are composed into a chain — first match wins.
   type pathMatcher interface {
       Match(path string, doc *v3.Document) *resolvedRoute
   }

   // resolvedRoute carries everything learned during path matching.
   // This is the single source of truth for "what matched and what was extracted."
   type resolvedRoute struct {
       pathItem    *v3.PathItem
       matchedPath string            // path template, e.g. "/users/{id}"
       pathParams  map[string]string  // extracted param values, nil if not extracted
       operation   *v3.Operation      // resolved from pathItem + HTTP method (nil until resolved)
   }

   // matcherChain tries each matcher in order. First match wins.
   type matcherChain []pathMatcher

   func (c matcherChain) Match(path string, doc *v3.Document) *resolvedRoute {
       for _, m := range c {
           if result := m.Match(path, doc); result != nil {
               return result
           }
       }
       return nil
   }
  ```
3. Add `LookupWithParams` to `Tree[T]` in `radix/tree.go`:
  - During `lookupRecursive`, when a `paramChild` matches, record `paramName -> segment` in a map.
  - Only allocate the map when the first param segment is encountered.
  - Add to `PathTree` and the `PathLookup` interface (interface is unreleased, safe to change).
4. Implement `radixMatcher` — wraps `PathLookup.LookupWithParams`, returns `resolvedRoute` with `pathParams` populated.
5. Implement `regexMatcher` — wraps the current `comparePaths` + `selectMatches` logic from `FindPath`. For now, does NOT extract path params (that comes in Phase 5). Returns `resolvedRoute` with `pathParams = nil`.
6. During `NewValidatorFromV3Model`, build the default chain `[radixMatcher, regexMatcher]` and store it on the `validator` struct.
7. Wire the chain into `FindPath` (or a new internal `resolvePath` method) so it replaces the current if/else logic.
8. Add tests for the chain, individual matchers, `LookupWithParams`, and `operationForMethod`.
9. Run `make test && make bench-compare` — expect no regression (same logic, cleaner structure).
10. Commit: *"validator: introduce pathMatcher chain with radix and regex matchers; consolidate operationForMethod"*

**Future**: Export `pathMatcher` as `PathMatcher`, add `WithPathMatchers(...)` option so users can inject static route matchers, rewriters, etc. Deferred until internals are proven.

---

## Phase 2: Define `requestContext` + `buildRequestContext`

**Goal**: Define the internal per-request context type and the function that builds it. Also cache the document version float at init time. No behavioral changes yet — just the plumbing.

**Files changed**: New file `request_context.go` (root package, unexported types), `requests/request_body.go`, `responses/response_body.go`

**Work**:

1. **Cache `VersionToFloat` at init time.** Add a `version float32` field to the sub-validator structs (`requestBodyValidator`, `responseBodyValidator`). Compute `helpers.VersionToFloat(document.Version)` once during construction and store it. Replace the per-request `helpers.VersionToFloat(v.document.Version)` calls in `validate_body.go` with `v.version`.
2. Define `requestContext`:
  ```go
   // requestContext is per-request shared state that flows through the entire
   // validation pipeline. Created once per request, shared by all validators.
   type requestContext struct {
       request    *http.Request
       route      *resolvedRoute
       operation  *v3.Operation
       parameters []*v3.Parameter           // path + operation params, extracted once
       security   []*base.SecurityRequirement
       stripped   string                     // request path with base path removed
       segments   []string                   // pre-split path segments
       version    float32                    // cached OAS version (3.0 or 3.1)
   }
  ```
3. Add a `buildRequestContext` method on `validator`:
  - Strips the request path (once).
  - Calls `matcherChain.Match(stripped, doc)` to get a `resolvedRoute`.
  - Resolves the operation from `pathItem + HTTP method` using `operationForMethod`.
  - Extracts parameters from `pathItem + operation` (once).
  - Extracts security requirements (once).
  - Returns `(*requestContext, []*errors.ValidationError)` — errors for path-not-found or method-not-found.
4. Tests: Unit tests for `buildRequestContext` with various request types.
5. Run `make test` — nothing calls `buildRequestContext` yet, tree stays green.
6. Commit: *"validator: define requestContext and buildRequestContext; cache version float"*

---

## Phase 3: Thread `requestContext` Through the Sync Path

**Goal**: Eliminate redundant work in the sync validation path (GET/HEAD/OPTIONS/DELETE). First measurable improvement.

**Files changed**: `validator.go`, `parameters/path_parameters.go`, `parameters/query_parameters.go`, `parameters/header_parameters.go`, `parameters/cookie_parameters.go`, `parameters/validate_security.go`

**Work**:

1. Change the unexported `validationFunction` type to:
  ```go
   type validationFunction func(ctx *requestContext) (bool, []*errors.ValidationError)
  ```
2. Add internal methods on `paramValidator` that accept `*requestContext`:
  - `validatePathParamsCtx(ctx)` — uses `ctx.route.pathParams` for the fast path (map lookup), falls back to regex for complex params (matrix/label).
  - `validateQueryParamsCtx(ctx)` — uses `ctx.parameters` instead of calling `ExtractParamsForOperation`.
  - `validateHeaderParamsCtx(ctx)` — same.
  - `validateCookieParamsCtx(ctx)` — same.
  - `validateSecurityCtx(ctx)` — uses `ctx.security`.
3. Keep the public `WithPathItem` methods as adapters: they construct a partial `requestContext` from their arguments and delegate to the `Ctx` methods.
4. Modify `ValidateHttpRequestSync` to call `buildRequestContext` once, then pass the context to the new internal methods sequentially.
5. Run `make test && make bench-compare`. Expected: GET benchmarks improve (fewer allocs), POST unchanged (still async).
6. Commit: *"validator: thread requestContext through sync validation path"*

---

## Phase 4: Thread `requestContext` Through the Async Path + Simplify Channels

**Goal**: Apply context-sharing to POST/PUT/PATCH requests AND replace the 9-goroutine/5-channel choreography with `sync.WaitGroup`.

**Files changed**: `validator.go`

**Work**:

1. Replace the current async implementation with:
  ```go
   func (v *validator) validateWithContext(ctx *requestContext) (bool, []*errors.ValidationError) {
       var mu sync.Mutex
       var wg sync.WaitGroup
       var allErrors []*errors.ValidationError

       validators := []validationFunction{
           v.paramValidator.validatePathParamsCtx,
           v.paramValidator.validateQueryParamsCtx,
           v.paramValidator.validateHeaderParamsCtx,
           v.paramValidator.validateCookieParamsCtx,
           v.paramValidator.validateSecurityCtx,
           v.requestValidator.validateBodyCtx,
       }

       wg.Add(len(validators))
       for _, fn := range validators {
           go func(validate validationFunction) {
               defer wg.Done()
               if valid, errs := validate(ctx); !valid {
                   mu.Lock()
                   allErrors = append(allErrors, errs...)
                   mu.Unlock()
               }
           }(fn)
       }
       wg.Wait()
       sortValidationErrors(allErrors)
       return len(allErrors) == 0, allErrors
   }
  ```
2. Body validation also needs a `validateBodyCtx` internal method.
3. Benchmark sync vs async for typical POST payloads. If sync is within 5% of async, consider using sync for all requests (massive simplification). Data-driven decision.
4. Run `make test -race && make bench-compare`. Expected: POST benchmarks improve.
5. Commit: *"validator: simplify async validation with WaitGroup, thread requestContext"*

---

## Phase 5: Regex Matcher Extracts Path Params

**Goal**: The radix matcher already populates `resolvedRoute.pathParams` (Phase 1). Make the regex matcher also extract params, so path parameter validation uses the fast path regardless of which matcher hit.

**Files changed**: `path_matcher.go` (the `regexMatcher` implementation)

**Work**:

1. In the regex matcher, after `comparePaths` finds a match, use `FindStringSubmatch` (instead of just `MatchString`) and `BraceIndices` to extract param name-value pairs from the regex groups.
2. Populate `resolvedRoute.pathParams` in the regex matcher's `Match` method.
3. Run `make test && make bench-compare`. Expected: regex-fallback path benchmarks improve.
4. Commit: *"validator: regex matcher extracts path params into resolvedRoute"*

---

## Phase 6: Lazy Error Schema Resolution (`WithLazyErrors`)

**Goal**: Reduce allocation overhead when creating validation errors by deferring the expensive `ReferenceSchema` and `ReferenceObject` population until the consumer actually needs them.

**Files changed**: `errors/validation_error.go`, `requests/validate_request.go`, `responses/validate_response.go`, `config/config.go`

**Approach**: Opt-in via `WithLazyErrors()`. Default behavior is unchanged (eager population, full backward compatibility). When enabled, `ReferenceSchema` and `ReferenceObject` fields are left empty, and the consumer calls getter methods to resolve them on demand.

**Work**:

1. Add `WithLazyErrors()` option to `config.go`:
  ```go
   LazyErrors bool // When true, defer expensive error field population
  ```
2. Add a private `schemaRef` field to `SchemaValidationFailure` that holds a pointer to the `SchemaCacheEntry` (for schema) and the decoded object + instance location (for reference object).
3. Add `GetReferenceSchema() string` and `GetReferenceObject() string` methods on `SchemaValidationFailure`:
  - If `ReferenceSchema` is already populated (eager mode), return it directly.
  - If lazy mode, resolve from `schemaRef` on first call, cache the result in the field.
4. Mark `ReferenceSchema` and `ReferenceObject` fields with `// Deprecated: Use GetReferenceSchema() / GetReferenceObject() instead.`
5. **Even in eager (non-lazy) mode**, use `json.Marshal` instead of `json.MarshalIndent` for `ReferenceObject` to reduce formatting overhead per error.
6. When `LazyErrors` is enabled, skip the `json.Marshal` for reference objects and the schema string copy during error construction entirely.
7. Add tests verifying both modes produce identical results.
8. Run `make test && make bench-compare` (default mode: slight improvement from json.Marshal; benchmark with lazy mode: fewer allocs on error paths).
9. Commit: *"errors: add WithLazyErrors option for deferred schema resolution"*

---

## Phase 7: Unify Request/Response Body Validation

**Goal**: Eliminate the ~95% code duplication between `requests/validate_request.go` (362 lines) and `responses/validate_response.go` (376 lines). Also fix several per-error inefficiencies that live in the duplicated code.

**Files changed**: New shared file, `requests/validate_request.go`, `responses/validate_response.go`, `schema_validation/locate_schema_property.go`, `helpers/ignore_regex.go`

**Work**:

1. **Fix `LocateSchemaPropertyNodeByJSONPath`** — Remove the goroutine/channel pattern and replace with a plain `defer/recover`:
  ```go
   func LocateSchemaPropertyNodeByJSONPath(doc *yaml.Node, JSONPath string) (result *yaml.Node) {
       defer func() { recover() }()
       _, path := utils.ConvertComponentIdIntoFriendlyPathSearch(JSONPath)
       if path == "" { return nil }
       jp, _ := jsonpath.NewPath(path)
       nodes := jp.Query(doc)
       if len(nodes) > 0 { return nodes[0] }
       return nil
   }
  ```
2. **Replace `IgnoreRegex` with string checks** — The pattern `^'?(anyOf|allOf|oneOf|validation)'? failed(, none matched)?$` can be replaced with a simple function using `strings.HasSuffix` and/or exact-match comparisons. Update all 4 call sites.
3. **Extract the shared error loop** into a common internal function (e.g., `buildSchemaValidationErrors(...)`) that handles: iterate flat errors, skip ignored messages, locate schema property nodes, extract instance locations, build `SchemaValidationFailure` structs. This eliminates the duplicate `instanceLocationRegex` declaration.
4. **Reuse `GoLow().Hash()` local variable** — In the shared body validation function, compute the hash once and reuse it for both the cache-check and cache-store paths (fixing the double-compute in `validate_parameter.go`, `validate_request.go`, and `validate_response.go`).
5. Extract the shared body validation logic into a common internal function:
  ```go
   func validateBodySchema(body []byte, schema *base.Schema, version float32,
       options *config.ValidationOptions, direction string,
       requestInfo bodyRequestInfo) (bool, []*errors.ValidationError)
  ```
   This handles: cache lookup, cache-miss compilation, JSON decoding, schema validation, error processing (using the new shared error loop), strict mode. The `direction` parameter controls error message wording.
6. Request and response validators become thin wrappers: read body from their respective sources, handle empty-body semantics, then delegate to the shared function.
7. Run `make test && make bench-compare`. Expected: Slight improvement from LocateSchema and IgnoreRegex fixes; structural dedup is allocation-neutral.
8. Commit: *"validation: unify request/response body schema validation; fix per-error overhead"*

---

## Phase 8: Options Plumbing + Minor Optimizations

**Goal**: Fix the cache/options ergonomics issues and remaining small inefficiencies.

**Files changed**: `config/config.go`, `requests/validate_request.go`, `requests/request_body.go`, `responses/validate_response.go`, `responses/response_body.go`, `validator.go`

**Work**:

1. **Change `ValidateRequestSchemaInput.Options` and `ValidateResponseSchemaInput.Options`** from `[]config.Option` to `*config.ValidationOptions`. Callers pass the options struct directly instead of wrapping in a `WithExistingOpts` closure. This eliminates the allocate-then-discard overhead (2-4 wasted allocs per request).
2. **Fix `WithExistingOpts`** to use struct dereference (`*o = *options`) instead of field-by-field copy. This prevents silent field-loss bugs when new fields are added to `ValidationOptions`. Keep the function for external consumers.
3. **Make `NewValidationOptions` allocation-aware** — As a safety net for any remaining callers, defer the `DefaultCache` and `sync.Map` allocations until after options are applied (only allocate if no `WithExistingOpts` override is provided).
4. **Cache `basePaths` at init**: `getBasePaths(document)` parses server URLs every time `StripRequestPath` is called. Compute once at init, store on the validator or options.
5. **Pre-split path segments during warming**: If we know the matched path template, pre-split its segments once rather than per-request.
6. Run `make test && make bench-compare`.
7. Commit: *"validator: fix options plumbing ergonomics; minor allocation optimizations"*

---

## Phase 9: Final Benchmarks + Documentation

**Goal**: Capture the final state and document the changes for the PR.

**Work**:

1. Run final fast-suite benchmarks and save results.
2. Run production benchmarks (`BenchmarkProd_*`) once for a before/after snapshot.
3. Run `benchstat` comparing Phase 0 baseline to final results.
4. Update `docs/architecture-review.md` with an "After" section showing measured improvements.
5. Write PR summary.
6. Commit: *"docs: update architecture review with measured results"*

---

## Verification Protocol (Every Phase)

After each phase, before committing:

```bash
make test          # All 1,159 tests pass (with -race)
make lint          # No new lint warnings
make bench-compare # No regressions (or improvements documented)
```

If a phase introduces a regression in benchmarks, investigate before committing. Phases 2 and 7 are structural and expected to be allocation-neutral. Phases 3, 4, 5, 6, and 8 should show measurable improvement.

---

## Design Decisions

1. **Matcher chain is internal for now.** The `pathMatcher` interface, `radixMatcher`, `regexMatcher`, and `matcherChain` are unexported. Once proven, a future PR can export them as `PathMatcher` with a `WithPathMatchers(...)` option, letting users inject static-route matchers, path rewriters, etc.
2. **`resolvedRoute` naming.** Communicates "we resolved which API route this request maps to, and here is everything we know about it." It carries more than just the PathItem (params, matched template, operation), so a route-level name fits better than a PathItem-level name.
3. **Lazy errors are opt-in.** `WithLazyErrors()` enables deferred schema resolution. Default behavior is unchanged. `ReferenceSchema`/`ReferenceObject` fields are deprecated in favor of `GetReferenceSchema()`/`GetReferenceObject()` methods that work in both modes. This gives a clean migration path without breaking existing consumers.
4. **Race detector is mandatory.** Every `make test` run includes `-race`. Non-negotiable when changing concurrency patterns.
5. **Async vs sync is data-driven.** Phase 4 benchmarks whether the async path is actually faster than sync after optimization. If not, we simplify to sync-only.
6. **`operationForMethod` consolidation.** A single helper replaces 4 identical switch statements. All existing functions delegate to it, keeping backward compatibility while eliminating maintenance burden.
7. **Options plumbing fix is late (Phase 8).** We defer the `ValidateSchemaInput` type change until after the body validation unification (Phase 7) to avoid churning the same files twice.

---

## Summary of All Fixes by Phase

| Phase | Fix | Impact |
|-------|-----|--------|
| 1 | `pathMatcher` chain replaces if/else | Extensible matching, cleaner code |
| 1 | `operationForMethod` consolidates 4 switches | Code dedup, single maintenance point |
| 1 | `resolvedRoute` carries operation | Eliminates downstream `ExtractOperation` |
| 2 | `requestContext` shared state | Foundation for per-request dedup |
| 2 | `VersionToFloat` cached at init | Eliminates per-request string comparison |
| 3 | Sync path uses context | ~24% fewer allocs for GET requests |
| 4 | WaitGroup replaces 9-goroutine/5-channel | Reduces async overhead for POST requests |
| 5 | Regex matcher extracts params | Fast path for all matchers |
| 6 | `WithLazyErrors` defers schema resolution | Major alloc reduction on error paths |
| 6 | `json.Marshal` replaces `json.MarshalIndent` | Reduces formatting overhead per error |
| 7 | Unified body validation function | Eliminates ~350 lines of duplication |
| 7 | `LocateSchemaPropertyNodeByJSONPath` fix | Eliminates goroutine + 2 channels per error |
| 7 | `IgnoreRegex` to string checks | Eliminates regex engine per error |
| 7 | `GoLow().Hash()` local reuse | Avoids redundant AST traversal |
| 7 | `instanceLocationRegex` dedup | Minor code cleanup |
| 8 | `ValidateSchemaInput.Options` type change | Eliminates 2-4 wasted allocs per request |
| 8 | `WithExistingOpts` struct copy | Prevents silent field-loss bugs |
| 8 | `NewValidationOptions` lazy alloc | Safety net for remaining callers |
| 8 | Cache `basePaths` at init | Eliminates per-request URL parsing |
