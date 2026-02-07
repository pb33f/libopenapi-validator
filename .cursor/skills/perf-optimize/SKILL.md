---
name: perf-optimize
description: |
  Performance optimization workflow for libopenapi-validator. Use when the user wants to find
  and fix performance bottlenecks, run benchmarks, analyze memory/CPU usage, or optimize
  validation throughput. This skill orchestrates a multi-step workflow using specialized
  subagents for benchmarking, analysis, investigation, and implementation.
---

# Performance Optimization Workflow

This skill orchestrates a systematic approach to finding and fixing performance bottlenecks
in the libopenapi-validator library. It uses five specialized subagents with verification
loops to ensure findings are evidence-based before any code changes are made.

## Architecture

```
benchmarker → analyzer → investigator ⇄ verifier → fixer → benchmarker (re-verify) → summarizer
                              ↑              |
                              └──── FAIL ────┘  (loop until PASS)
```

The key principle is that NO claim about a bottleneck proceeds to implementation without
being independently verified. The `perf-verifier` subagent acts as a skeptical reviewer
that challenges conclusions against actual benchmark data and source code.

## Context

The libopenapi-validator library has known performance issues in production:
- **Memory**: ~1MB/s per endpoint for validation (19 endpoints = 15-23MB/s)
- **Path matching**: The regex fallback iterates ALL paths with regex for every request
- **Schema validation**: Schema compilation and rendering may be duplicated per-request
- **Goroutine overhead**: Async validation spawns goroutines even for simple validations

## Benchmark Suites

There are two benchmark suites. **Use the fast suite for all iterative work.**

### Fast suite (use this during optimization)
- **File**: `benchmarks/validator_bench_test.go`
- **Spec**: `test_specs/ads_api_bulk_actions.yaml` (~25 endpoints)
- **Run time**: ~5 minutes for full suite with `-count=5`
- **Run with**: `go test -bench='Benchmark(PathMatch|RequestValidation|ResponseValidation|RequestResponseValidation|ConcurrentValidation|Memory)' -benchmem -count=5 -timeout=10m ./benchmarks/`

Covers: path matching (radix vs regex), request body (small/medium/large), sync vs
async, concurrent, memory, schema cache impact, endpoint count scaling.

**IMPORTANT**: The regex deliberately EXCLUDES `BenchmarkValidatorInit_*` and
`BenchmarkProd_Init*`. Initialization only happens once at service startup and is NOT
representative of request-time performance. Agents should focus exclusively on per-request
CPU and memory — the benchmarks that simulate actual production traffic.

### Production suite (final snapshot only)
- **File**: `benchmarks/production_bench_test.go`
- **Spec**: `~/src/ads-api/open_api_spec/v3/complete.yaml` (69K lines, real prod spec)
- **Run time**: 10-30 minutes
- **Run with**: `go test -bench=BenchmarkProd -benchmem -count=3 -timeout=30m ./benchmarks/`

The fast benchmarks produce numbers in the same range as production for per-request validation.
The production suite has higher init cost (2.7s) but init is a one-time startup cost — we
don't care about it.

**DO NOT run production benchmarks in the optimization loop.** They take too long. Only run
them once at the end for a final before/after comparison if the user asks.

## Workflow Steps

Execute these steps IN ORDER, using the corresponding subagent for each.

### Step 1: Run Benchmarks (`/perf-benchmarker`)

Use the `perf-benchmarker` subagent to:
- Run the full benchmark suite with `-benchmem -count=5`
- Save baseline results to `benchmarks/results/baseline.txt`
- Generate CPU and memory profiles
- Report raw numbers

### Step 2: Analyze Results (`/perf-analyzer`)

Use the `perf-analyzer` subagent to:
- Parse benchmark results and identify the worst performers
- Compare radix tree vs regex fallback performance
- Identify memory allocation hotspots (high B/op and allocs/op)
- Determine which validation phase is the bottleneck
- Produce a prioritized list of issues to focus on

### Step 3: Investigate Bottleneck (`/perf-investigator`)

Use the `perf-investigator` subagent to:
- Read the source code of the identified bottleneck area
- Use CPU/memory profiles to find hot functions AND drill into line-level detail
- Trace the allocation path from the benchmark to the source
- Identify the root cause (unnecessary allocations, missing caches, etc.)
- Document exactly where in the code the problem originates
- Propose a specific fix with expected improvement

### Step 4: Verify the Findings (`/perf-verifier`) ← CRITICAL STEP

**DO NOT SKIP THIS STEP.** Use the `perf-verifier` subagent to independently fact-check
the investigator's findings. Pass it the full output from Step 3.

The verifier will:
- Check that cited benchmark numbers match the actual results file
- Verify that profile data supports the claimed bottleneck
- Cross-reference the proposed root cause against source code
- Check for logical errors (correlation ≠ causation, misleading aggregates, etc.)
- Look for bigger bottlenecks that may have been overlooked
- Return PASS or FAIL with specific objections

**If FAIL**: Resume the `perf-investigator` (using the `resume` parameter) with the
verifier's objections. The investigator should address each objection and produce an
updated finding. Then run the verifier again. **Repeat until PASS.**

Example orchestration:
```
1. investigator produces findings → save agent ID
2. verifier reviews findings → FAIL with objections
3. resume investigator (agent ID) with: "The verifier found these issues: [objections]. 
   Please address each one and update your findings."
4. investigator produces updated findings
5. verifier reviews again → PASS
6. proceed to Step 5
```

**Maximum 3 loops.** If verification doesn't pass after 3 rounds, present the disagreement
to the user for a human judgment call.

### Step 5: Fix and Verify (`/perf-fixer`)

**IMPORTANT**: The fixer MUST create a new git branch before making any code changes.
If it fails to do so, stop and re-invoke it with an explicit reminder to branch first.

Use the `perf-fixer` subagent to:
- **Create a new branch FIRST**: `perf/fix-<bottleneck-description>` (before any edits!)
- Implement the optimization
- Run benchmarks again and save to `benchmarks/results/optimized.txt`
- Run `benchstat baseline.txt optimized.txt` to compare
- Run `go test ./...` to ensure no regressions
- Report the improvement metrics

After the fixer completes, verify the branch was created:
```bash
git branch --show-current  # Should NOT be the original branch
```
If it's still on the original branch, the fixer did not follow instructions. Revert the changes
and re-invoke with: "You MUST create branch perf/fix-<name> FIRST before any code changes."

### Step 6: Verify the Fix (`/perf-verifier` again)

Run the verifier one more time on the fixer's results to confirm:
- Benchmark numbers actually improved (not just noise)
- The improvement is in the RIGHT benchmark (the one identified in Step 3)
- All tests pass
- No unexpected regressions in other benchmarks

If the fix doesn't show improvement, the verifier should flag this and you should
either iterate on the fix or go back to Step 3 to re-investigate.

### Step 7: Generate PR Summary (`/perf-summarizer`)

Use the `perf-summarizer` subagent to produce a concise, paste-ready PR description.

Pass it:
- The investigator's findings (the bottleneck that was identified)
- The fixer's report (what code changed, benchstat output)
- The verifier's final assessment

The summarizer will return markdown that can be directly pasted into a GitHub PR body.
It follows strict rules to keep things short: no filler, tables for numbers, bullets for changes.

Present the summary to the user. They can paste it directly or ask for edits.

## Commands Reference

```bash
# Run fast benchmarks (save baseline) — use this during optimization
cd /Users/zach.hamm/src/libopenapi-validator
go test -bench='Benchmark(PathMatch|RequestValidation|ResponseValidation|RequestResponseValidation|ConcurrentValidation|Memory)' -benchmem -count=5 -timeout=10m ./benchmarks/ | tee benchmarks/results/baseline.txt

# Run with CPU profiling
go test -bench=BenchmarkRequestValidation_BulkActions_Medium -cpuprofile=benchmarks/results/cpu.prof -benchmem -count=1 ./benchmarks/

# Run with memory profiling
go test -bench=BenchmarkRequestValidation_BulkActions_Medium -memprofile=benchmarks/results/mem.prof -benchmem -count=1 ./benchmarks/

# Analyze profiles
go tool pprof -top benchmarks/results/cpu.prof
go tool pprof -top benchmarks/results/mem.prof

# Compare results
benchstat benchmarks/results/baseline.txt benchmarks/results/optimized.txt

# Run unit tests
go test ./...
```

## Key Areas to Investigate

Based on production observations, these are the most likely bottleneck areas:

1. **`paths/paths.go` - FindPath() regex fallback**: Iterates ALL paths with regex compilation
   for every request that doesn't hit the radix tree. Should sort by specificity and exit early.

2. **`validator.go` - ValidateHttpRequest()**: Spawns goroutines + channels for every request.
   For simple validations, this overhead may exceed the validation cost itself.

3. **`requests/validate_body.go`**: Schema rendering and compilation may happen per-request
   even with caching if cache keys don't match.

4. **`helpers/schema_compiler.go`**: JSON schema compilation is expensive. Check if schemas
   are being recompiled unnecessarily.

5. **`schema_validation/`**: The core JSON schema validation may have allocation-heavy paths.

## Success Criteria

- Per-request memory allocation reduced by 50%+ for GET requests
- Per-request memory allocation reduced by 30%+ for POST requests with body
- Path matching with radix tree: 0 allocations
- Regex fallback: sorted by specificity with early exit (not scanning all paths)
- No test regressions (`go test ./...` passes)
