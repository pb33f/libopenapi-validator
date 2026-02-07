# Performance Investigation: Per-Request Allocations

**Date:** 2026-02-06
**Branch:** `default-regex-cache`
**Benchmark spec:** `test_specs/ads_api_bulk_actions.yaml` (~25 endpoints)

## Baseline

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| GET (no body) | 4,580 | 4,680 | 115 |
| GET (query params) | 5,638 | 6,074 | 149 |
| POST small | 17,020 | 12,058 | 250 |
| POST medium | 35,940 | 33,642 | 644 |
| POST large | 138,900 | 188,610 | 3,169 |
| Radix path match | 167–245 | 192–256 | 4 |
| Regex fallback | 2,446–6,335 | 3,272–5,736 | 64–140 |

## Profile: GET Request Allocation Breakdown

Clean profile (`-run='^$'`, excludes init and tests). Top flat allocators per operation:

| Function | allocs/op | % | Location |
|---|---|---|---|
| `ValidatePathParamsWithPathItem` | ~12 | 10% | `parameters/path_parameters.go` |
| `jsonschema.(*Schema).validate` | ~10 | 9% | external: santhosh-tekuri/jsonschema |
| `formatJsonSchemaValidationError` | ~10 | 9% | `parameters/validate_parameter.go` |
| `regexp.FindStringSubmatch` | ~9 | 8% | stdlib (path param matching) |
| `Schema.hash()` | ~7 | 6% | external: pb33f/libopenapi |
| `fmt.Sprintf` | ~5 | 4% | stdlib |
| `kind.(*Pattern).LocalizedString` | ~4 | 3% | external: santhosh-tekuri/jsonschema |
| `ExtractParamsForOperation` | ~4 | 3% | `helpers/parameter_utilities.go` |
| ~50 other functions | ~54 | 47% | spread across validation pipeline |

## Key Findings

### 1. YAML rendering is init-only, not per-request

The `yaml.(*Emitter).Emit` at 22.71% in the full profile is from `warmMediaTypeSchema` (init). Confirmed via `-peek`:

```
16076.96MB 99.69% | warmMediaTypeSchema
   50.13MB  0.31% | warmParameterSchema
```

The schema cache works correctly. No per-request YAML rendering occurs.

### 2. `Schema.GoLow().Hash()` is expensive but uncacheable

Hash computation allocates ~7 objects per call (ordered map creation for sorting). Attempted to cache hash values indexed by schema pointer, but **libopenapi creates different `*low.Schema` instances** between cache warming and per-request validation. Verified with debug logging: STORE and LOAD use different pointers for the same logical schema.

### 3. Benchmark GET request produces validation errors

Path parameters `acc_12345` / `camp_67890` fail pattern validation, causing `formatJsonSchemaValidationError` to run (~10 allocs/op). Production requests with valid params would skip this cost.

### 4. No single bottleneck dominates

Allocations are distributed across ~50 functions. The largest single contributor (`ValidatePathParamsWithPathItem` at 12 flat allocs) is mostly dispatch overhead. Most allocations come from external libraries (jsonschema, libopenapi) that we don't control.

## Attempted Fixes

| Approach | Result | Reason |
|---|---|---|
| Pre-allocate slices, early returns, `strings.Builder` | +2 allocs (regression) | Pre-allocation overhead > savings when no errors |
| Cache hash by `*base.Schema` pointer | 0 change | High-level schema pointers differ between warming and validation |
| Cache hash by `*low.Schema` pointer | 0 change | Low-level schema pointers also differ (confirmed via debug) |

## Recommended Next Steps

1. **Upstream libopenapi**: File issue/PR to either cache `Hash()` results internally or guarantee `GoLow()` pointer stability. Would save ~7 allocs/op (6%).

2. **Structural refactoring** (medium effort): Extract parameters once per request and pass pre-resolved cache entries keyed by `*v3.Parameter` pointer (stable between warming and validation). Requires changing internal function signatures across ~6 files. Expected savings: ~11 allocs/op (10%).

3. **Regex fallback optimization** (separate PR): Already proven at 14–26x improvement when radix tree misses. Only affects fallback path, but eliminates 60–136 unnecessary allocs when triggered.

4. **Benchmark with valid params**: Current GET benchmark inflates allocs by ~10/op due to pattern validation failures. Using valid param values would give a more representative production baseline.
