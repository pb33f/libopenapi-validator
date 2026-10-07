# Validate Arazzo workflows

The `arazzo` package validates Arazzo 1.0 and 1.1 documents. It checks document
structure, workflow and step names, parameters, actions, expressions, input
schemas and prerequisite cycles. Supply OpenAPI, AsyncAPI or Arazzo source
documents to check referenced operations, channels and workflows.

```go
result, err := arazzo.ValidateBytes(ctx, spec, "workflows.yaml",
    arazzo.WithSources(candidates...))
if err != nil {
    return err
}
if !result.Valid() {
    return fmt.Errorf("invalid Arazzo: %v", result.Diagnostics)
}
if !result.Complete {
    return fmt.Errorf("validation incomplete: %v", result.Checks)
}
```

`Validate` accepts existing nodes through `arazzo.Document`. Source candidates
use `libopenapi/arazzo.CandidateDocument`; they can contain nodes, bytes, models
or adapters. See the [complete example](example_test.go).

Always check the error, `Valid()` and `Complete`. `Valid()` means no conformance
errors were found. `Complete` means every applicable check ran. Missing sources,
partial adapter metadata and unsupported syntax can leave a valid document
incompletely checked. `Checks` gives the affected source, field and reason.
Errors return a non-nil incomplete result. Diagnostics include codes, source URIs,
JSON Pointers and line/column locations. Caller nodes and models stay unchanged.

The validator does not run workflows or fetch files or URLs by default.
`WithResolver` enables caller-controlled source retrieval. It does not retrieve
external input schemas. XPath, legacy JSONPath and conditions that need runtime
substitution report incomplete checks. Rootless Arazzo models also have partial
expression coverage; supply original nodes or bytes for full checks.

`WithLimits` bounds resource use; `WithAdvisories` enables optional authoring
warnings. Put whitespace around logical operators next to bare header
expressions: `&` and `|` are legal header-name characters.

The [requirements ledger](REQUIREMENTS.md) lists specification clauses, tests and
known schema differences.
