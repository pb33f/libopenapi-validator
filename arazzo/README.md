# Static Arazzo validation

`arazzo.Validate` accepts an existing `yaml.Node` and its retrieval URI.
`arazzo.ValidateBytes` accepts exactly one JSON or YAML document. Both use the
same validation path. Arazzo 1.0 and 1.1 feature sets accept compatible patch
versions and nonempty prerelease suffixes, as permitted by the official schemas.
The existing OpenAPI and HTTP validator APIs are unchanged.

```go
result, err := arazzo.Validate(ctx, arazzo.Document{
    Root: root,
    URI: "https://example.test/workflows.yaml",
}, arazzo.WithSources(candidates...))
```

`candidates` uses the existing `libopenapi/arazzo.CandidateDocument` type. Sources
can contain original nodes, bytes, high models, or typed presence adapters.
`WithResolver` accepts the existing `SourceDocumentResolver` interface. A resolver
is explicit caller-owned retrieval authority; it must enforce its own access
policy and honor context cancellation. There is no implicit file or network I/O.
The executable [example](example_test.go) supplies OpenAPI, AsyncAPI and Arazzo
source nodes. Its fixture bundle is available to downstream consumers.

## Outcomes

| Outcome | Result |
| --- | --- |
| Conformance error | Diagnostic, normally no tool error; `Valid()` is false. |
| All applicable checks ran successfully | `Valid()` and `Complete` are true. |
| Source, detailed adapter metadata, or syntax capability unavailable | Local findings remain; `Checks` contains the affected URI, pointer, source and reason; `Complete` is false. |
| Known source proves missing target or wrong kind | Reference or source-type diagnostic at the authored field. |
| Unparseable bytes, unsupported feature version, invalid API configuration | Typed `*Error` with input, unsupported-version or configuration kind. |
| Cancellation, resource limit, resolver failure, compiler failure | Operational error with an incomplete partial result. Cancellation remains accessible through `errors.Is`. |

Always check the returned error and `Complete`; `Valid()` alone means only that
no conformance errors were found. Structural failures stop semantic passes and
record that prerequisite failure in coverage. Missing or non-string versions are
structural findings; malformed version strings are typed input errors.

Diagnostics have stable typed codes, severity, source URI, RFC 6901 pointer, and
one-based line/column where supplied. Missing fields use their nearest existing
parent. Invalid map names use key locations. Duplicate IDs include the first
declaration as a related location. Results contain only detached values and do
not retain nodes, models, indexes or compiler errors.

## Boundaries

Validation checks static structure, identities, references, expressions,
inheritance and prerequisites. It does not execute operations, evaluate criteria
against invented runtime data, fetch input schema references, authenticate,
schedule retries, or transport messages. Dynamic input schemas are checked
conservatively: absence from a direct properties map is not proof of absence
when references, composition or dynamic properties can supply that input.

XPath and the legacy Goessner JSONPath dialect currently record unavailable
syntax coverage. RFC 9535 JSONPath, JSON Pointer, regex and simple conditions
have static parsers. Runtime-substituted condition syntax records incomplete
coverage after checking the embedded expressions. Presence-only source adapters
record the detailed metadata checks they cannot prove. Receive-step success can
also require message semantics that are unavailable statically.
Rootless Arazzo high models expose only a partial expression projection; their
prerequisite coverage remains incomplete. Supply original nodes or bytes to
check all expression-bearing fields.
Separate logical operators from bare header expressions with whitespace:
`&` and `|` are legal characters inside header names.

`WithLimits` replaces positive per-call limits. Defaults are 16 MiB of input and
expanded bytes, 200,000 nodes/derived work units, depth 128, 50 source documents,
1,000 diagnostics and 64 KiB per expression/pattern. Node/byte limits account for
alias expansion and cumulative supplied source data. Separate shared work budgets
bound derived graph edges, inherited values, reference hops, expression uses,
coverage records and repeated literal/condition bytes. Numeric exponents charge
their arbitrary-precision expansion before schema arithmetic. Source caches and
reference memoization last for one call. Only immutable official schemas are
cached globally. No goroutine is started per field or diagnostic.

`WithAdvisories` enables naming and sequential forward-reference warnings.
Recommendations remain separate from required conformance errors. Vacuum can
map library codes and coverage to its own rules and severity policy.

The [requirements ledger](REQUIREMENTS.md) records clauses, fixtures and schema
discrepancies. The [validation record](VALIDATION.md) contains checks, coverage and
benchmark results. Vacuum integration remains a separate consumer task.
