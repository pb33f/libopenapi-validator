# Validation record

Checked on 7 October 2026 in isolated branch `codex/arazzo-validation`, based on
`b1808fec4e5a405c08332df7b64f747b4a2d2c5a`. This record covers local checks before
PR publication. The primary checkout remains unchanged.

All checks use Go 1.26.8, `GOWORK=off`, and the module's released dependencies.

| Check | Result |
| --- | --- |
| `go test ./... -count=1` | All module tests pass, including the external-package mixed-source example. |
| `go test -race -coverprofile=… ./arazzo -count=1` | Pass; 77.3% package statement coverage. |
| `go test -race ./arazzo -run TestResultDoesNotRetainSourceGraphs -count=20` | Pass; retained diagnostics do not retain primary/source root graphs. |
| `go build ./...` | Pass. |
| `go vet ./...` | Pass. |
| `go mod tidy -diff` | Pass, no module or checksum changes. |
| `golangci-lint run ./arazzo/...` | Pass, zero issues. |
| gofumpt, gci and `git diff --check` | Pass. |
| Schema snapshot checksums and all 69 ledger test references | Pass. |
| Independent correctness and resource reviews | All reported findings repaired and verified; no remaining P0–P2 findings in the reviewed scope. |

Statement coverage is measured coverage, not a claim of full branch coverage.
The [requirements ledger](REQUIREMENTS.md) records semantic fixtures and capability
limits. The [package guide](README.md) explains the result and coverage contract.

## Benchmarks

Apple M4 Max, darwin/arm64, default 16 logical processors. Values are medians of
five local samples. The warm benchmarks accept existing nodes; they include
per-call normalization, indexing, validation and detached result construction.
The cold benchmark compiles both embedded schemas without the global cache.

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Warm: 10 steps, operations and expressions | 254,632 | 342,606 | 4,058 |
| Warm: 100 steps, operations and expressions | 2,429,607 | 3,237,388 | 38,691 |
| Warm: 1,000 steps, operations and expressions | 21,610,055 | 33,074,060 | 387,775 |
| 10 findings | 163,165 | 211,506 | 2,851 |
| 100 findings | 1,585,993 | 1,988,292 | 27,090 |
| 500 findings | 7,223,785 | 9,953,842 | 136,037 |
| 10 workflows sharing a source prerequisite and reusable parameter | 396,294 | 340,435 | 3,967 |
| 100 workflows sharing a source prerequisite and reusable parameter | 3,042,558 | 2,964,794 | 35,805 |
| 1,000 workflows sharing a source prerequisite and reusable parameter | 21,864,433 | 29,375,616 | 356,961 |
| Cold: compile both official schemas | 4,679,181 | 5,043,806 | 66,463 |

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
