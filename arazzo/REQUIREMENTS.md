# Static validation requirements

The normative sources are [Arazzo 1.0.1](https://spec.openapis.org/arazzo/v1.0.1.html)
and [Arazzo 1.1.0](https://spec.openapis.org/arazzo/v1.1.0.html). Section names below
refer to their named Object, Runtime Expressions and Specification Extensions
sections. Embedded schema provenance, dates, sizes and hashes are in
[`schemas/metadata.json`](schemas/metadata.json); `TestOfficialSchemaSnapshots`
checks the original bytes and offline compilation. No dependency change is needed.

This ledger describes static checks. `Complete` means all applicable checks in
this implementation ran; it does not mean that workflows were executed. A skipped
capability always produces an incomplete `Check`. Tests exercise the exported API
unless the test name explicitly describes an internal graph or normalization unit.

## Clause and regression ledger

| Specification section / requirement | Check and diagnostic | Positive and negative regression families |
| --- | --- | --- |
| Arazzo Object: version, Info Object, required fields, field types and extension shapes | Offline schema; `CodeStructure`; malformed/unsupported versions have typed errors | `TestFoundationVersionsAndJSONParity`, `TestFoundationStructuralLocations`, `TestFoundationInputAndLimits` |
| Arazzo Object 1.1: `$self` and relative base identity | Canonical identity and retrieval attribution remain distinct; duplicate identities fail | `TestSourcesSelfIdentityAndAllCandidates`, `TestSourcesDuplicateIdentityAndLimits`, `TestSourcesApplicationBaseAndRelativeSelf` |
| Source Description Object: unique names, URI references and versioned type enum | Schema formats, identity index, actual supplied kind; `CodeDuplicateID`, `CodeSourceType` | `TestSemanticPublicIdentityInheritanceAndActions`, `TestSourcesAbsenceTransportAndType`, `TestSourcesCachedAliasChecksDeclaredKind` |
| Workflow Object: scoped workflow IDs, nonempty steps and workflow dependencies | Schema and document-qualified identity/prerequisite graph; `CodeDuplicateID`, `CodeReference`, `CodeDependency` | `TestSemanticPublicIdentityInheritanceAndActions`, `TestSemanticDependencyFieldGrammarAndCycles`, `TestSourcesImportCycleAndPrerequisiteCycle` |
| Step Object: scoped step IDs and exactly one target | Versioned schema plus exact target count, full operation/channel pointers and qualified IDs | `TestFoundationStructuralLocations`, `TestSourcesOfflineTargets`, `TestSourcesAdapterCoverageAndExactPointer`, `TestSourcesDottedSourceNames` |
| Parameter Object: location, unique effective identity, inheritance and explicit overrides | Immutable reusable resolution and default merge; consuming context; `CodeParameter` | `TestSemanticInheritedParameterContext`, `TestSemanticReusableOverridesAndDuplicateIdentity`, `TestSemanticPublicWorkflowInputParameters`, `TestSourcesExternalStepMetadata` |
| Parameter Object 1.1: querystring and selector values | Query/querystring exclusion; allowed scalar/selector context | `TestSemanticPublicIdentityInheritanceAndActions`, `TestSelectorAndReplacementSyntax`, `TestFoundationNormativeCorrectionsAndAdvisories` |
| Success/Failure Action Objects: end/goto/retry unions, fields, criteria, targets and parameters | Schema checks every body; local contextual step targets checked at each use; `CodeAction`, `CodeReference`, `CodeParameter` | `TestSemanticActionsUseConsumingWorkflow`, `TestSemanticPublicIdentityInheritanceAndActions`, `TestSemanticPublicWorkflowInputParameters`, `TestCriteriaTraversalAndCapabilities` |
| Components and Reusable Object: correct collection, unused bodies and resolution cycles | Every defined body checked; exact collection/key resolution; `CodeReference` | `TestSemanticReusableCollectionAndCycles`, `TestSemanticReusableOverridesAndDuplicateIdentity`, `TestSemanticPublicImplicitCycleAndOverride` |
| Criterion and Expression Type Objects: language/version, required context, default dialect and syntax | Bounded simple grammar, regex, RFC 9535 JSONPath and RFC 6901 Pointer parsers; `CodeExpression` | `TestCriterionSyntax`, `TestCriteriaTraversalAndCapabilities` |
| Runtime Expressions: version and scoped inputs/outputs/source members | Upstream versioned grammar with 1.0 gates; static symbols checked without guessing runtime response properties; `CodeExpression`, `CodeReference` | `TestExpressionOutputSyntaxAndVersion`, `TestLinkedExpressionSymbols`, `TestLinkedExpressionKnownMissingOperation`, `TestSourcesRequestExpressionParameterDeclarations` |
| Selector Object 1.1: legal positions, context, dialect and extensions | Outputs and declared selector values checked; literal Any objects kept as data; `CodeSelector` | `TestSelectorAndReplacementSyntax`, `TestSelectorExtensionsAndLiteralSignature`, `TestExpressionsDoNotScanExtensionsOrBareDollar` |
| Request Body and Payload Replacement Objects | Expression-bearing payload/replacement values and target syntax; media type controls implicit replacement dialect | `TestSelectorAndReplacementSyntax`, `TestCriteriaTraversalAndCapabilities` |
| Input Schema Object: JSON Schema 2020-12 structure and local resource scope | Offline metaschema and compiler; local pointers, `$id`, anchors and dynamic references; `CodeInputSchema` for missing local refs | `TestInputSchema202012Structure`, `TestInputSchemaLocalRefsAndResourceScope`, `TestInputSchemaMissingLocalRefAndExternalCoverage`, `TestInputSchemaExternalSiblingDoesNotHideMissingLocalRef` |
| Input Schema Object: annotations, literals and conservative input membership | Schema-slot traversal excludes literal values; undeclared input is rejected only when a closed schema proves absence | `TestInputSchemaDataKeywordsDoNotDeclareReferences`, `TestInputUnknownAnnotationsRemainData`, `TestSemanticPublicWorkflowInputParameters`, `TestLinkedExpressionSymbols` |
| Step dependencies 1.1 and implicit output prerequisites | Field-specific grammar, scoped steps, linear shared workflow-start node and explicit/implicit cycle detection; `CodeDependency` | `TestSemanticDependencyFieldGrammarAndCycles`, `TestSemanticImplicitPrerequisitesAndDottedOutputs`, `TestSemanticPublicImplicitCycleAndOverride`, `TestSourcesExternalImplicitPrerequisiteCycle`, `TestDependencyGraphLongChain` |
| AsyncAPI Step Object 1.1: channel, action, correlation and timeout | Versioned schema and caller-supplied AsyncAPI 3 operation/channel metadata; no transport | `TestSourcesOfflineTargets`, `TestSourcesExternalStepMetadata`, `TestSourcesMixedOfflineComplete`, `ExampleValidate` |
| Specification Extensions and optional recommendations | Extension data is not scanned as expressions; advisory names/forward references use warnings | `TestExpressionsDoNotScanExtensionsOrBareDollar`, `TestFoundationNormativeCorrectionsAndAdvisories`, `TestSemanticImplicitPrerequisitesAndDottedOutputs` |

## Deliberate decisions

| Text/schema issue | Interpretation and scope | Evidence |
| --- | --- | --- |
| Source Description Object names: prose uses SHOULD; dated schemas enforce a pattern | Remove only that property pattern from the compiler view; naming warnings require `WithAdvisories`. Original snapshots are unchanged. | `TestFoundationNormativeCorrectionsAndAdvisories` |
| Qualified step prerequisites retain a strict source-name pattern after the advisory naming correction | The compiler view permits dotted source names here too. Semantic lookup selects an exact declared source, workflow and step. Runtime-expression ABNF has stricter identifier wording; this follows the package's documented advisory naming interpretation consistently. | `TestSourcesDottedStepDependencies` |
| Parameter Object 1.1 `value`: prose says Any; dated schema permits only Selector objects | Replace only that property's object restriction in the compiler view. Full selector signatures with runtime context are validated; ordinary literal objects retain their data. | `TestFoundationNormativeCorrectionsAndAdvisories`, `TestSelectorExtensionsAndLiteralSignature` |
| Action Object 1.1 `parameters`: schema leaves `items` unconstrained | Compiler view uses the existing Parameter/Reusable union for each item. Contextual input rules also apply. | `TestFoundationNormativeCorrectionsAndAdvisories`, `TestSemanticPublicWorkflowInputParameters` |
| Step Object operationPath/channelPath exclusivity has asymmetric prose | Require exactly one target in either feature set. A second authored target is never silently ignored. | `TestFoundationStructuralLocations`, `TestSourcesOfflineTargets` |
| Runtime Expressions 1.1 examples include structured body/payload property dereference that the upstream parser rejects | Accept the documented structured-source prefix followed by valid property dereference; retain all other upstream grammar checks. | `TestCriterionSyntax`, `TestSelectorAndReplacementSyntax` |
| Input Schema field schemas require objects, while JSON Schema also defines boolean schemas | Keep the authored Arazzo field object contract. Accept boolean schemas inside schema-valued keywords; do not coerce a top-level boolean input into an object. | `TestInputSchema202012Structure` and structural schema assertions |
| Input membership through composition, references or dynamic properties | Do not use absence from a direct properties map as proof. Only an unambiguous closed object excludes a name. | `TestSemanticPublicWorkflowInputParameters`, `TestLinkedExpressionSymbols` |
| AsyncAPI timeout schema has no minimum | Check integer type through the schema; do not add a nonnegative requirement. | `TestSourcesExternalStepMetadata` |
| Forward output ordering guidance is a recommendation; goto/retry and imports can cycle | Advisory forward references are warnings. Only prerequisite cycles are conformance errors. | `TestSemanticImplicitPrerequisitesAndDottedOutputs`, `TestSourcesImportCycleAndPrerequisiteCycle` |

## Explicit capability records

| Clause / unavailable evidence | Coverage record | Regression |
| --- | --- | --- |
| Source-linked target or dependency without a supplied document | Affected source reference and URI are incomplete; local checks still run | `TestSourcesAbsenceTransportAndType`, `TestSourcesUnusedResolverAndMissingMemberCapability` |
| Detailed parameters, inputs, steps or outputs with a presence-only adapter | The relevant metadata check is incomplete; a true existence result proves only presence | `TestSourcesAdapterCoverageAndExactPointer`, `TestLinkedExpressionPresenceAdapterCoverage` |
| Rootless Arazzo high models omit expression-bearing fields from their metadata projection | Consumed external prerequisite coverage is incomplete; full node/byte sources provide those checks | `TestSourcesRootlessArazzoPrerequisiteCoverage` |
| Source OpenAPI/AsyncAPI metadata behind unavailable external `$ref` | Detailed target/parameter check is incomplete; validation does not retrieve it | `TestSourcesUnavailableOpenAPIReferences` |
| Input Schema Object external resources | `input-schema-reference` records the resource URI and authored field. No schema downloader is installed. | `TestInputSchemaMissingLocalRefAndExternalCoverage`, `TestFoundationNoRetrieval` |
| XPath and legacy Goessner JSONPath syntax | `selector-syntax` records dialect and location | `TestCriteriaTraversalAndCapabilities`, `TestSelectorAndReplacementSyntax` |
| Criterion syntax after runtime substitution | Embedded expressions are checked; final `criterion-syntax` remains incomplete | `TestCriteriaTraversalAndCapabilities` |
| Replacement target whose dialect depends on unavailable operation content type | `replacement-target` is incomplete | `TestSelectorAndReplacementSyntax` |
| Receive-step criteria omission whose legality depends on message metadata | Source-dependent receive check remains incomplete when metadata cannot prove the condition | `TestSourcesExternalStepMetadata` |
| AsyncAPI parameter/message value compatibility | `asyncapi-parameters` records unavailable detailed compatibility; no runtime values are invented | `TestSourcesExternalStepMetadata` |

## Operational evidence

| Contract | Regression |
| --- | --- |
| No default file/network retrieval or operation execution | `TestFoundationNoRetrieval`; `ExampleValidate` uses application-owned file reads and supplied nodes. Production imports no executor or downloader. |
| Caller nodes, merges and aliases remain unchanged during concurrent validation | `TestFoundationMergeAliasImmutability`, `TestSourcesRootImmutable`, `TestSourcesConcurrentMergeRoots`, race run |
| High-model metadata fallback and retained result ownership | `TestSourcesRootlessOpenAPIModelTargets`, `TestResultDoesNotRetainSourceGraphs` (caller primary/source roots are collected while findings remain alive and serializable) |
| One document, precise numbers/null/false/zero, JSON-compatible YAML, duplicate keys and recursive aliases | `TestFoundationYAMLGraphsAndNumbers`, `TestFoundationInputAndLimits`, `TestFoundationVersionsAndJSONParity` |
| Stable detached locations, foreign URIs, escaped pointers and capped ordering | `TestFoundationPointerEscapesAndDeterminism`, `TestSemanticPublicDiagnosticLocations`, `TestSourcesExternalDiagnosticLocation`, `TestExpressionCappedDiagnosticsDeterministic` |
| Per-call source cache: diamond loaded once, legal import cycles, canonical identity and cancellation | `TestSourcesResolverMemoizesURI`, `TestSourcesDiamondCapsAndCancellation`, `TestSourcesImportCycleAndPrerequisiteCycle` |
| Aggregate input, node, depth, source, diagnostic and pattern limits | `TestFoundationCapsBeforeCompilation`, `TestSourcesDepthAndRootByteCaps`, `TestSourcesRootlessModelCollectionCaps`, `TestSourcesRootlessAdditionalOperationCaps`, `TestInputPatternLimitAndLiteralAnnotations` |
| Cross-document implicit prerequisites and RFC header tokens in criteria | `TestSourcesCrossDocumentImplicitCycle`, `TestExpressionHeaderPunctuationInCriteriaAndOutputs`, `TestCriterionHeaderOperatorBoundaries`, `TestSourcesCriterionHeaderTokenIdentity`; external step lookups share a bounded per-workflow index |
| Derived work and byte limits before inherited expansion and repeated parsing | `TestSemanticInheritedWorkLimit`, `TestExpressionInheritedParserByteBudget`; bounded shared graph and reference memoization |
| Cold schema compile and warm scaling for steps/sources/expressions, findings and shared reusable/dependency graphs | `BenchmarkCompileOfficialSchemas`, `BenchmarkValidate`, `BenchmarkValidateManyFindings`, `BenchmarkValidateSharedDependencies`, `BenchmarkValidateExternalSteps`; five samples per case |

The full module test and build run with `GOWORK=off` and released dependencies.
Vacuum detection, rule bridges, reports, commands and LSP support remain a separate
consumer change. The fixture bundle and `ExampleValidate` are its API handback.
