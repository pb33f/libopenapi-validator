# Validation record

Checked on 7 October 2026 in isolated branch `codex/arazzo-validation`, based on
`b1808fec4e5a405c08332df7b64f747b4a2d2c5a`. This record covers local validation of the package. The primary checkout remains unchanged.

All checks use Go 1.26.8, `GOWORK=off`, and the module's released dependencies.

| Check | Result |
| --- | --- |
| `go test ./... -count=1` | All module tests pass, including the external-package mixed-source example. |
| `go test -coverprofile=… ./... -count=1` | Pass; 98.8% repository statement coverage, including the CLI. Existing packages measure 98.662%; Arazzo measures 99.130%. |
| `go test -race -coverprofile=… ./arazzo -count=1` | Pass; 99.1% package statement coverage. |
| `go test -race ./arazzo -run TestResultDoesNotRetainSourceGraphs -count=20` | Pass; retained diagnostics do not retain primary/source root graphs. |
| `go build ./...` | Pass. |
| `go vet ./...` | Pass. |
| `go mod tidy -diff` | Pass, no module or checksum changes. |
| `golangci-lint run ./...` | Pass, zero issues. |
| gofumpt, gci and `git diff --check` | Pass. |
| Schema snapshot checksums and all 77 ledger test references | Pass. |
| Independent correctness and resource reviews | All reported findings repaired and verified; no remaining P0–P2 findings in the reviewed scope. |

Statement coverage is measured coverage, not a claim of full branch coverage.
The five boundary test files cover source/model projection, references, target
metadata, dependency graphs, expressions, schema diagnostics, cancellation and
resource limits. Tests assert findings and capabilities, with external source
coordinates and caller immutability where applicable. Disabling integer or float
retagging makes the public numeric-bound regression fail. Independent correctness
and de-slop reviews found no remaining material issues in these tests.

The initial `f8f46d2` report measured 68.44% Codecov patch coverage and 91.38%
project coverage against the 98.27% base. The updated tests and six review repairs
have passed local validation. Codecov line coverage on the final published head
must meet or exceed the base before the PR is ready. Thresholds, exclusions and
the CI coverage command are unchanged.

The [requirements ledger](REQUIREMENTS.md) records semantic fixtures and capability
limits. The [package guide](README.md) explains the result and coverage contract.

## Benchmarks

Apple M4 Max, darwin/arm64, default 16 logical processors. Values are medians of
five local samples. The warm benchmarks accept existing nodes; they include
per-call normalization, indexing, validation and detached result construction.
The cold benchmark compiles both embedded schemas without the global cache.

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Warm: 10 steps, operations and expressions | 304,601 | 343,487 | 4,065 |
| Warm: 100 steps, operations and expressions | 2,375,296 | 3,241,226 | 38,697 |
| Warm: 1,000 steps, operations and expressions | 23,764,141 | 33,128,780 | 387,783 |
| 10 findings | 182,586 | 212,284 | 2,858 |
| 100 findings | 1,709,425 | 1,991,975 | 27,096 |
| 500 findings | 7,729,827 | 9,981,505 | 136,043 |
| 10 workflows sharing a source prerequisite and reusable parameter | 280,759 | 344,519 | 4,007 |
| 100 workflows sharing a source prerequisite and reusable parameter | 2,459,634 | 3,000,619 | 36,122 |
| 1,000 workflows sharing a source prerequisite and reusable parameter | 22,486,128 | 29,770,868 | 360,728 |
| 10 prerequisites sharing the final external step | 301,489 | 360,126 | 4,432 |
| 100 prerequisites sharing the final external step | 2,991,594 | 3,126,669 | 38,691 |
| 1,000 prerequisites sharing the final external step | 23,188,979 | 30,686,837 | 386,776 |
| Cold: compile both official schemas | 5,419,673 | 5,042,258 | 66,395 |

Time and allocation growth are approximately linear across these workloads.
These are local measurements, not service latency promises. Resolver transport
time is excluded: benchmark sources are supplied in memory.

The resource review also probed workflow prerequisite fan-out and shared local
OpenAPI reference chains. Workflow prerequisites use a shared start node rather
than a steps × dependencies edge expansion. Reference chains use a per-call memo.
Inherited action/list expansion and repeated parser input have explicit budgets
before allocation or parsing, with public limit regressions.

## Consumer handback

Use `Validate` for parsed nodes and `ValidateBytes` for one JSON/YAML document.
Use upstream `CandidateDocument` and `SourceDocumentResolver` through options.
Always inspect the returned error, `Result.Valid()` and `Result.Complete`.

The library performs no default retrieval or workflow execution. XPath, legacy
JSONPath, runtime-substituted conditions and unavailable detailed source metadata
produce explicit incomplete coverage. No upstream exports or dependency upgrades
are required. Vacuum integration is the next separate consumer change.
