// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/pb33f/go-yaml"
	upstream "github.com/pb33f/libopenapi/arazzo"
	highArazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

type sourceResolverFunc func(context.Context, upstream.SourceRequest) (*upstream.ResolvedSource, error)

func (f sourceResolverFunc) Resolve(ctx context.Context, request upstream.SourceRequest) (*upstream.ResolvedSource, error) {
	return f(ctx, request)
}

type operationPresence struct{}

func (operationPresence) SourceType() string               { return "openapi" }
func (operationPresence) HasOperationID(value string) bool { return value == "read" }
func (operationPresence) HasOperationPath(string) bool     { return true }

type workflowPresence struct{}

func (workflowPresence) SourceType() string            { return "arazzo" }
func (workflowPresence) HasWorkflow(value string) bool { return value == "run" }

const sourceOpenAPI = `openapi: 3.1.0
info: {title: API, version: '1'}
paths:
  /pets/~demo:
    parameters:
      - {name: X-ID, in: header, schema: {type: string}}
    get:
      operationId: read
      responses: {'200': {description: OK}}
`

func sourceTestDocument(step string) string {
	return "arazzo: 1.1.0\ninfo: {title: Flow, version: '1'}\nsourceDescriptions:\n  - {name: api, url: 'api.yaml', type: openapi}\nworkflows:\n  - workflowId: run\n    steps:\n      - stepId: first\n" + step
}

func sourceCandidate(data string) upstream.CandidateDocument {
	return upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", SourceBytes: []byte(data)}
}

func sourceDiagnostic(t *testing.T, result *Result, code Code, pointer string) {
	t.Helper()
	for _, d := range result.Diagnostics {
		if d.Code == code && d.Pointer == pointer {
			if d.Line == 0 || d.Column == 0 {
				t.Fatalf("missing source coordinates: %+v", d)
			}
			return
		}
	}
	t.Fatalf("missing %s at %s: %+v", code, pointer, result.Diagnostics)
}

func TestSourcesOfflineTargets(t *testing.T) {
	cases := []struct {
		name, step string
		code       Code
		pointer    string
	}{
		{"qualified operation", "        operationId: $sourceDescriptions.api.read\n", "", ""},
		{"bare operation", "        operationId: read\n", "", ""},
		{"escaped pointer", "        operationPath: '{$sourceDescriptions.api.url}#/paths/~1pets~1~0demo/get'\n", "", ""},
		{"percent encoded fragment", "        operationPath: '{$sourceDescriptions.api.url}#%2Fpaths%2F~1pets~1~0demo%2Fget'\n", "", ""},
		{"absent operation", "        operationId: $sourceDescriptions.api.missing\n", CodeReference, "/workflows/0/steps/0/operationId"},
		{"trailing pointer member", "        operationPath: '{$sourceDescriptions.api.url}#/paths/~1pets~1~0demo/get/responses'\n", CodeReference, "/workflows/0/steps/0/operationPath"},
		{"uppercase method", "        operationPath: '{$sourceDescriptions.api.url}#/paths/~1pets~1~0demo/GET'\n", CodeReference, "/workflows/0/steps/0/operationPath"},
		{"header case", "        operationId: read\n        parameters: [{name: x-id, in: header, value: test}]\n", "", ""},
		{"undeclared parameter", "        operationId: read\n        parameters: [{name: other, in: query, value: test}]\n", CodeParameter, "/workflows/0/steps/0/parameters/0/name"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateBytes(context.Background(), []byte(sourceTestDocument(tt.step)), "https://example.test/main.yaml", WithSources(sourceCandidate(sourceOpenAPI)))
			if err != nil {
				t.Fatal(err)
			}
			if tt.code != "" {
				sourceDiagnostic(t, result, tt.code, tt.pointer)
			} else if !result.Valid() || !result.Complete {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestSourcesAbsenceTransportAndType(t *testing.T) {
	input := []byte(sourceTestDocument("        operationId: read\n"))
	result, err := ValidateBytes(context.Background(), input, "https://example.test/main.yaml")
	if err != nil || !result.Valid() || result.Complete {
		t.Fatalf("missing source must be incomplete: %+v %v", result, err)
	}
	failure := errors.New("transport failed")
	result, err = ValidateBytes(context.Background(), input, "https://example.test/main.yaml", WithResolver(sourceResolverFunc(func(context.Context, upstream.SourceRequest) (*upstream.ResolvedSource, error) { return nil, failure })))
	if !errors.Is(err, failure) || result.Complete {
		t.Fatalf("transport failure lost: %+v %v", result, err)
	}
	result, err = ValidateBytes(context.Background(), input, "https://example.test/main.yaml", WithSources(sourceCandidate("asyncapi: 3.0.0\nchannels: {}\noperations: {}\n")))
	if err != nil {
		t.Fatal(err)
	}
	sourceDiagnostic(t, result, CodeSourceType, "/sourceDescriptions/0/type")
}

func TestSourcesAdapterCoverageAndExactPointer(t *testing.T) {
	candidate := upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", Adapter: operationPresence{}}
	result, err := ValidateBytes(context.Background(), []byte(sourceTestDocument("        operationId: read\n")), "https://example.test/main.yaml", WithSources(candidate))
	if err != nil || !result.Valid() || result.Complete {
		t.Fatalf("adapter must expose coverage limit: %+v %v", result, err)
	}
	result, err = ValidateBytes(context.Background(), []byte(sourceTestDocument("        operationPath: '{$sourceDescriptions.api.url}#/paths/~1pets/get/responses'\n")), "https://example.test/main.yaml", WithSources(candidate))
	if err != nil {
		t.Fatal(err)
	}
	sourceDiagnostic(t, result, CodeReference, "/workflows/0/steps/0/operationPath")
}

func TestSourcesResolverMemoizesURI(t *testing.T) {
	input := strings.Replace(sourceTestDocument("        operationId: $sourceDescriptions.api.read\n      - stepId: second\n        operationId: $sourceDescriptions.alias.read\n"), "workflows:", "  - {name: alias, url: './api.yaml', type: openapi}\nworkflows:", 1)
	calls := 0
	resolver := sourceResolverFunc(func(ctx context.Context, request upstream.SourceRequest) (*upstream.ResolvedSource, error) {
		calls++
		if request.URL != "https://example.test/api.yaml" || request.BaseURI != "" {
			t.Fatalf("unexpected request: %+v", request)
		}
		return &upstream.ResolvedSource{RetrievalURI: request.URL, SourceBytes: []byte(sourceOpenAPI)}, nil
	})
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithResolver(resolver))
	if err != nil || !result.Valid() || !result.Complete || calls != 1 {
		t.Fatalf("memoization failed: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestSourcesSelfIdentityAndAllCandidates(t *testing.T) {
	external := "arazzo: 1.1.0\n$self: canonical.yaml\ninfo: {title: External, version: '1'}\nsourceDescriptions: [{name: api, url: api.yaml, type: openapi}]\nworkflows: [{workflowId: remote, steps: [{stepId: first, operationId: read}]}]\n"
	candidate := upstream.CandidateDocument{RetrievalURI: "https://example.test/retrieved.yaml", SourceBytes: []byte(external)}
	input := strings.Replace(sourceTestDocument("        workflowId: $sourceDescriptions.api.remote\n"), "url: 'api.yaml', type: openapi", "url: 'canonical.yaml', type: arazzo", 1)
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(sourceCandidate(sourceOpenAPI), candidate))
	if err != nil || !result.Valid() {
		t.Fatalf("canonical candidate should resolve: %+v %v", result, err)
	}
	input = strings.Replace(input, "canonical.yaml", "retrieved.yaml", 1)
	result, err = ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(candidate))
	if err != nil {
		t.Fatal(err)
	}
	sourceDiagnostic(t, result, CodeReference, "/sourceDescriptions/0/url")
}

func TestSourcesDuplicateIdentityAndLimits(t *testing.T) {
	input := []byte(sourceTestDocument("        operationId: read\n"))
	_, err := ValidateBytes(context.Background(), input, "https://example.test/main.yaml", WithSources(sourceCandidate(sourceOpenAPI), sourceCandidate(sourceOpenAPI)))
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorConfiguration {
		t.Fatalf("duplicate identity must fail configuration: %v", err)
	}
	limits := DefaultLimits()
	limits.MaxSources = 1
	_, err = ValidateBytes(context.Background(), input, "https://example.test/main.yaml", WithSources(sourceCandidate(sourceOpenAPI), upstream.CandidateDocument{RetrievalURI: "https://example.test/other.yaml", SourceBytes: []byte(sourceOpenAPI)}), WithLimits(limits))
	if !errors.As(err, &typed) || typed.Kind != ErrorLimit {
		t.Fatalf("total candidate limit not enforced: %v", err)
	}
}

func TestSourcesRootImmutable(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(sourceOpenAPI), &root); err != nil {
		t.Fatal(err)
	}
	before, _ := yaml.Marshal(&root)
	result, err := ValidateBytes(context.Background(), []byte(sourceTestDocument("        operationId: read\n")), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", RootNode: &root}))
	if err != nil || !result.Valid() {
		t.Fatalf("root candidate failed: %+v %v", result, err)
	}
	after, _ := yaml.Marshal(&root)
	if string(before) != string(after) {
		t.Fatal("caller source root was modified")
	}
}

func TestSourcesImportCycleAndPrerequisiteCycle(t *testing.T) {
	main := `arazzo: 1.1.0
info: {title: Main, version: '1'}
sourceDescriptions: [{name: other, url: other.yaml, type: arazzo}]
workflows:
  - workflowId: run
    dependsOn: [$sourceDescriptions.other.remote]
    steps: [{stepId: first, workflowId: $sourceDescriptions.other.remote}]
`
	other := `arazzo: 1.1.0
info: {title: Other, version: '1'}
sourceDescriptions: [{name: main, url: main.yaml, type: arazzo}]
workflows:
  - workflowId: remote
    steps: [{stepId: first, workflowId: $sourceDescriptions.main.run}]
`
	candidate := upstream.CandidateDocument{RetrievalURI: "https://example.test/other.yaml", SourceBytes: []byte(other)}
	result, err := ValidateBytes(context.Background(), []byte(main), "https://example.test/main.yaml", WithSources(candidate))
	if err != nil || !result.Valid() || !result.Complete {
		t.Fatalf("imports are not prerequisite cycles: %+v %v", result, err)
	}
	candidate.SourceBytes = []byte(strings.Replace(other, "    steps:", "    dependsOn: [$sourceDescriptions.main.run]\n    steps:", 1))
	result, err = ValidateBytes(context.Background(), []byte(main), "https://example.test/main.yaml", WithSources(candidate))
	if err != nil {
		t.Fatal(err)
	}
	sourceDiagnostic(t, result, CodeDependency, "/workflows/0/dependsOn/0")
}

func TestSourcesExternalStepMetadata(t *testing.T) {
	input := strings.Replace(sourceTestDocument("        workflowId: $sourceDescriptions.api.remote\n        dependsOn: [$sourceDescriptions.api.remote.steps.first]\n"), "type: openapi", "type: arazzo", 1)
	external := `arazzo: 1.1.0
info: {title: Other, version: '1'}
sourceDescriptions: [{name: main, url: main.yaml, type: arazzo}]
workflows: [{workflowId: remote, steps: [{stepId: first, workflowId: $sourceDescriptions.main.run}]}]
`
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(sourceCandidate(external)))
	if err != nil || !result.Valid() {
		t.Fatalf("valid external step failed: %+v %v", result, err)
	}
	input = strings.Replace(input, ".steps.first", ".steps.missing", 1)
	result, err = ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(sourceCandidate(external)))
	if err != nil {
		t.Fatal(err)
	}
	sourceDiagnostic(t, result, CodeReference, "/workflows/0/steps/0/dependsOn/0")
	input = strings.Replace(input, ".steps.missing", ".steps.first", 1)
	result, err = ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", Adapter: workflowPresence{}}))
	if err != nil {
		t.Fatal(err)
	}
	// The adapter proves remote absent rather than providing step metadata.
	sourceDiagnostic(t, result, CodeReference, "/workflows/0/steps/0/workflowId")
}

func TestSourcesUnusedResolverAndMissingMemberCapability(t *testing.T) {
	input := `arazzo: 1.1.0
info: {title: Main, version: '1'}
sourceDescriptions: [{name: api, url: api.yaml, type: openapi}]
workflows: [{workflowId: first, steps: [{stepId: local, workflowId: second}]}, {workflowId: second, steps: [{stepId: local, workflowId: first}]}]
`
	calls := 0
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithResolver(sourceResolverFunc(func(context.Context, upstream.SourceRequest) (*upstream.ResolvedSource, error) {
		calls++
		return nil, errors.New("must not retrieve")
	})))
	if err != nil || !result.Valid() || !result.Complete || calls != 0 {
		t.Fatalf("unused source was loaded: %+v %v calls=%d", result, err, calls)
	}
}

func TestSourcesUnavailableOpenAPIReferences(t *testing.T) {
	external := `openapi: 3.1.0
info: {title: API, version: '1'}
paths: {/pets: {$ref: other.yaml#/paths/~1pets}}
`
	result, err := ValidateBytes(context.Background(), []byte(sourceTestDocument("        operationId: read\n")), "https://example.test/main.yaml", WithSources(sourceCandidate(external)))
	if err != nil || !result.Valid() || result.Complete {
		t.Fatalf("unavailable nested reference is coverage, not absence: %+v %v", result, err)
	}
}

func TestSourcesMixedOfflineComplete(t *testing.T) {
	input := `arazzo: 1.1.0
$self: https://example.test/main.yaml
info: {title: Mixed sources, version: '1'}
sourceDescriptions:
  - {name: api, url: api.yaml, type: openapi}
  - {name: events, url: events.yaml, type: asyncapi}
  - {name: flows, url: flows.yaml, type: arazzo}
workflows:
  - workflowId: run
    steps:
      - {stepId: read, operationId: $sourceDescriptions.api.read}
      - {stepId: publish, operationId: $sourceDescriptions.events.publish, action: send}
      - {stepId: remote, workflowId: $sourceDescriptions.flows.remote}
`
	events := `asyncapi: 3.0.0
info: {title: Events, version: '1'}
channels:
  pets: {address: pets, messages: {created: {$ref: '#/components/messages/created'}}}
operations:
  publish: {action: send, channel: {$ref: '#/channels/pets'}}
components:
  messages: {created: {payload: {type: object, properties: {id: {type: string}}}}}
`
	flows := `arazzo: 1.1.0
info: {title: Flows, version: '1'}
sourceDescriptions: [{name: api, url: api.yaml, type: openapi}]
workflows: [{workflowId: remote, steps: [{stepId: first, operationId: read}]}]
`
	result, err := ValidateBytes(context.Background(), []byte(input), "https://retrieval.test/flow.yaml", WithSources(sourceCandidate(sourceOpenAPI), upstream.CandidateDocument{RetrievalURI: "https://example.test/events.yaml", SourceBytes: []byte(events)}, upstream.CandidateDocument{RetrievalURI: "https://example.test/flows.yaml", SourceBytes: []byte(flows)}))
	if err != nil || !result.Valid() || !result.Complete {
		t.Fatalf("complete offline mixed-source validation failed: %+v %v", result, err)
	}
}

func TestSourcesDiamondCapsAndCancellation(t *testing.T) {
	document := func(id string, deps []string) string {
		descriptions := []string{}
		refs := []string{}
		for _, dep := range deps {
			descriptions = append(descriptions, "{name: "+dep+", url: "+dep+".yaml, type: arazzo}")
			refs = append(refs, "$sourceDescriptions."+dep+"."+dep)
		}
		if len(descriptions) == 0 {
			descriptions = append(descriptions, "{name: api, url: api.yaml, type: openapi}")
		}
		dependency := ""
		if len(refs) > 0 {
			dependency = "    dependsOn: [" + strings.Join(refs, ", ") + "]\n"
		}
		return "arazzo: 1.1.0\ninfo: {title: " + id + ", version: '1'}\nsourceDescriptions: [" + strings.Join(descriptions, ", ") + "]\nworkflows:\n  - workflowId: " + id + "\n" + dependency + "    steps: [{stepId: first, operationId: placeholder}]\n"
	}
	input := `arazzo: 1.1.0
info: {title: Main, version: '1'}
sourceDescriptions: [{name: a, url: a.yaml, type: arazzo}]
workflows:
  - workflowId: run
    dependsOn: [$sourceDescriptions.a.a]
    steps: [{stepId: first, workflowId: $sourceDescriptions.a.a}]
`
	docs := map[string]string{"a.yaml": document("a", []string{"b", "c"}), "b.yaml": document("b", []string{"d"}), "c.yaml": document("c", []string{"d"}), "d.yaml": document("d", nil)}
	loads := map[string]int{}
	resolver := sourceResolverFunc(func(ctx context.Context, request upstream.SourceRequest) (*upstream.ResolvedSource, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := strings.TrimPrefix(request.URL, "https://example.test/")
		loads[key]++
		return &upstream.ResolvedSource{RetrievalURI: request.URL, SourceBytes: []byte(docs[key])}, nil
	})
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithResolver(resolver))
	if err != nil || !result.Valid() || !result.Complete {
		t.Fatalf("diamond failed: %+v %v", result, err)
	}
	for _, key := range []string{"a.yaml", "b.yaml", "c.yaml", "d.yaml"} {
		if loads[key] != 1 {
			t.Fatalf("%s loaded %d times", key, loads[key])
		}
	}
	limits := DefaultLimits()
	limits.MaxSources = 3
	result, err = ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithResolver(resolver), WithLimits(limits))
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorLimit || result.Complete {
		t.Fatalf("fallback total source cap failed: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, err = ValidateBytes(ctx, []byte(input), "https://example.test/main.yaml", WithResolver(sourceResolverFunc(func(ctx context.Context, request upstream.SourceRequest) (*upstream.ResolvedSource, error) {
		cancel()
		return nil, ctx.Err()
	})))
	if !errors.Is(err, context.Canceled) || result.Complete {
		t.Fatalf("resolver cancellation lost: %+v %v", result, err)
	}
}

func TestSourcesConcurrentMergeRoots(t *testing.T) {
	data := `openapi: 3.1.0
info: {title: API, version: '1'}
operation: &operation
  operationId: read
  responses: {'200': {description: OK}}
paths:
  /pets:
    get:
      <<: *operation
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(data), &root); err != nil {
		t.Fatal(err)
	}
	before, _ := yaml.Marshal(&root)
	candidate := upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", RootNode: &root}
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			result, err := ValidateBytes(context.Background(), []byte(sourceTestDocument("        operationId: read\n")), "https://example.test/main.yaml", WithSources(candidate))
			if err == nil && (!result.Valid() || !result.Complete) {
				err = errors.New("concurrent source validation failed")
			}
			done <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	after, _ := yaml.Marshal(&root)
	if string(before) != string(after) {
		t.Fatal("caller root changed during concurrent validation")
	}
}

func TestSourcesExternalDiagnosticLocation(t *testing.T) {
	input := `arazzo: 1.1.0
info: {title: Main, version: '1'}
sourceDescriptions: [{name: other, url: other.yaml, type: arazzo}]
workflows:
  - workflowId: run
    dependsOn: [$sourceDescriptions.other.remote]
    steps: [{stepId: first, workflowId: $sourceDescriptions.other.remote}]
`
	external := `arazzo: 1.1.0
info: {title: Other, version: '1'}
sourceDescriptions: [{name: api, url: api.yaml, type: openapi}]
workflows:
  - workflowId: remote
    dependsOn: [missing]
    steps: [{stepId: first, operationId: placeholder}]
`
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/other.yaml", SourceBytes: []byte(external)}))
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == CodeReference && diagnostic.URI == "https://example.test/other.yaml" && diagnostic.Pointer == "/workflows/0/dependsOn/0" && diagnostic.Line == 6 && diagnostic.Column == 17 {
			return
		}
	}
	t.Fatalf("external source location lost: %+v", result.Diagnostics)
}

func TestSourcesDottedSourceNames(t *testing.T) {
	input := strings.Replace(sourceTestDocument("        operationId: $sourceDescriptions.customer.api.read\n"), "name: api", "name: customer.api", 1)
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(sourceCandidate(sourceOpenAPI)))
	if err != nil || !result.Valid() || !result.Complete {
		t.Fatalf("dotted source name was split as an internal separator: %+v %v", result, err)
	}
}

func TestSourcesDepthAndRootByteCaps(t *testing.T) {
	input := `arazzo: 1.1.0
info: {title: Main, version: '1'}
sourceDescriptions: [{name: chain, url: '1.yaml', type: arazzo}]
workflows:
  - workflowId: run
    dependsOn: [$sourceDescriptions.chain.chain]
    steps: [{stepId: first, workflowId: $sourceDescriptions.chain.chain}]
`
	resolver := sourceResolverFunc(func(ctx context.Context, request upstream.SourceRequest) (*upstream.ResolvedSource, error) {
		number := strings.TrimSuffix(strings.TrimPrefix(request.URL, "https://example.test/"), ".yaml")
		index, err := strconv.Atoi(number)
		if err != nil {
			return nil, err
		}
		dependency := ""
		if index < 12 {
			dependency = "    dependsOn: [$sourceDescriptions.chain.chain]\n"
		}
		doc := "arazzo: 1.1.0\ninfo: {title: Chain, version: '1'}\nsourceDescriptions: [{name: chain, url: '" + strconv.Itoa(index+1) + ".yaml', type: arazzo}]\nworkflows:\n  - workflowId: chain\n" + dependency + "    steps: [{stepId: first, operationId: placeholder}]\n"
		return &upstream.ResolvedSource{RetrievalURI: request.URL, SourceBytes: []byte(doc)}, nil
	})
	limits := DefaultLimits()
	limits.MaxDepth = 8
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithResolver(resolver), WithLimits(limits))
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorLimit || result.Complete {
		t.Fatalf("source dependency depth cap failed: %+v %v", result, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(sourceOpenAPI+"x-data: "+strings.Repeat("a", 3000)+"\n"), &root); err != nil {
		t.Fatal(err)
	}
	limits = DefaultLimits()
	limits.MaxBytes = 2048
	result, err = ValidateBytes(context.Background(), []byte(sourceTestDocument("        operationId: read\n")), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", RootNode: &root}), WithLimits(limits))
	if !errors.As(err, &typed) || typed.Kind != ErrorLimit || result.Complete {
		t.Fatalf("root-only source byte cap failed: %+v %v", result, err)
	}
}

func TestSourcesExternalImplicitPrerequisiteCycle(t *testing.T) {
	input := strings.Replace(sourceTestDocument("        workflowId: $sourceDescriptions.api.remote\n"), "type: openapi", "type: arazzo", 1)
	external := `arazzo: 1.1.0
info: {title: Other, version: '1'}
sourceDescriptions: [{name: api, url: api.yaml, type: openapi}]
workflows:
  - workflowId: remote
    steps:
      - stepId: first
        operationId: read
        parameters: [{name: id, in: query, value: $steps.second.outputs.id}]
        outputs: {id: $response.body#/id}
      - stepId: second
        operationId: read
        parameters: [{name: id, in: query, value: $steps.first.outputs.id}]
        outputs: {id: $response.body#/id}
`
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(sourceCandidate(external)))
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == CodeDependency && diagnostic.URI == "https://example.test/api.yaml" && strings.HasSuffix(diagnostic.Pointer, "/parameters/0/value") {
			return
		}
	}
	t.Fatalf("external implicit prerequisite cycle missed: %+v", result)
}

func TestSourcesRequestExpressionParameterDeclarations(t *testing.T) {
	for _, version := range []string{"1.0.1", "1.1.0"} {
		for _, tt := range []struct {
			expression string
			missing    bool
		}{{"$request.header.x-id", false}, {"$request.header.bogus", true}, {"$request.query.bogus", true}, {"$request.path.bogus", true}} {
			t.Run(version+tt.expression, func(t *testing.T) {
				input := strings.Replace(sourceTestDocument("        operationId: read\n        outputs: {value: "+tt.expression+"}\n"), "1.1.0", version, 1)
				result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(sourceCandidate(sourceOpenAPI)))
				if err != nil {
					t.Fatal(err)
				}
				if tt.missing {
					sourceDiagnostic(t, result, CodeReference, "/workflows/0/steps/0/outputs/value")
				} else if !result.Valid() || !result.Complete {
					t.Fatalf("declared inherited header expression failed: %+v", result)
				}
			})
		}
	}
	input := sourceTestDocument("        operationId: read\n        outputs: {value: $request.header.bogus}\n")
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", Adapter: operationPresence{}}))
	if err != nil || !result.Valid() || result.Complete {
		t.Fatalf("adapter cannot prove request parameter absence: %+v %v", result, err)
	}
}

func TestSourcesCachedAliasChecksDeclaredKind(t *testing.T) {
	input := strings.Replace(sourceTestDocument("        operationId: $sourceDescriptions.api.read\n      - stepId: second\n        workflowId: $sourceDescriptions.alias.remote\n"), "workflows:", "  - {name: alias, url: './api.yaml', type: arazzo}\nworkflows:", 1)
	calls := 0
	resolver := sourceResolverFunc(func(ctx context.Context, request upstream.SourceRequest) (*upstream.ResolvedSource, error) {
		calls++
		return &upstream.ResolvedSource{RetrievalURI: request.URL, SourceBytes: []byte(sourceOpenAPI)}, nil
	})
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("cached source alias loaded %d times", calls)
	}
	sourceDiagnostic(t, result, CodeSourceType, "/sourceDescriptions/1/type")
}

func TestSourcesRootlessModelCollectionCaps(t *testing.T) {
	for _, collection := range []string{"sourceDescriptions", "dependsOn", "outputs"} {
		t.Run(collection, func(t *testing.T) {
			model := &highArazzo.Arazzo{Arazzo: "1.1.0", Workflows: []*highArazzo.Workflow{{WorkflowId: "remote", Steps: []*highArazzo.Step{{StepId: "first", OperationId: "read"}}}}}
			for i := 0; i < 2000; i++ {
				switch collection {
				case "sourceDescriptions":
					model.SourceDescriptions = append(model.SourceDescriptions, &highArazzo.SourceDescription{Name: strconv.Itoa(i), URL: "api.yaml", Type: "openapi"})
				case "dependsOn":
					model.Workflows[0].DependsOn = append(model.Workflows[0].DependsOn, "unused")
				case "outputs":
					if model.Workflows[0].Outputs == nil {
						model.Workflows[0].Outputs = orderedmap.New[string, *highArazzo.OutputValue]()
					}
					model.Workflows[0].Outputs.Set(strconv.Itoa(i), highArazzo.NewExpressionOutputValue("$response.body"))
				}
			}
			input := strings.Replace(sourceTestDocument("        workflowId: $sourceDescriptions.api.remote\n"), "type: openapi", "type: arazzo", 1)
			limits := DefaultLimits()
			limits.MaxNodes = 1000
			limits.MaxBytes = 8000
			result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", ArazzoDocument: model}), WithLimits(limits))
			var typed *Error
			if !errors.As(err, &typed) || typed.Kind != ErrorLimit || result.Complete {
				t.Fatalf("rootless %s collection escaped caps: %+v %v", collection, result, err)
			}
		})
	}
}

func TestSourcesRootlessOpenAPIModelTargets(t *testing.T) {
	operation := &v3.Operation{OperationId: "read"}
	header := &v3.Parameter{Name: "X-ID", In: "header"}
	pathItem := &v3.PathItem{Get: operation, Parameters: []*v3.Parameter{header}}
	paths := orderedmap.New[string, *v3.PathItem]()
	paths.Set("/pets/~demo", pathItem)
	model := &v3.Document{Version: "3.1.0", Paths: &v3.Paths{PathItems: paths}}
	if model.GoLow() != nil {
		t.Fatal("fixture must exercise the rootless high-model fallback")
	}
	candidate := upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", OpenAPIDocument: model}
	cases := []struct {
		name, step string
		code       Code
		pointer    string
	}{
		{"operation ID", "        operationId: $sourceDescriptions.api.read\n", "", ""},
		{"escaped operation pointer", "        operationPath: '{$sourceDescriptions.api.url}#/paths/~1pets~1~0demo/get'\n", "", ""},
		{"inherited header parameter", "        operationId: read\n        parameters: [{name: x-id, in: header, value: test}]\n", "", ""},
		{"inherited header expression", "        operationId: read\n        outputs: {header: $request.header.x-id}\n", "", ""},
		{"absent operation ID", "        operationId: absent\n", CodeReference, "/workflows/0/steps/0/operationId"},
		{"pointer targets a response member", "        operationPath: '{$sourceDescriptions.api.url}#/paths/~1pets~1~0demo/get/responses'\n", CodeReference, "/workflows/0/steps/0/operationPath"},
		{"absent header parameter", "        operationId: read\n        parameters: [{name: absent, in: header, value: test}]\n", CodeParameter, "/workflows/0/steps/0/parameters/0/name"},
		{"absent request header expression", "        operationId: read\n        outputs: {header: $request.header.absent}\n", CodeReference, "/workflows/0/steps/0/outputs/header"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateBytes(context.Background(), []byte(sourceTestDocument(tt.step)), "https://example.test/main.yaml", WithSources(candidate))
			if err != nil {
				t.Fatal(err)
			}
			if tt.code != "" {
				sourceDiagnostic(t, result, tt.code, tt.pointer)
			} else if !result.Valid() || !result.Complete {
				t.Fatalf("rootless high model failed: %+v", result)
			}
			if !result.Complete {
				t.Fatalf("rootless high model unexpectedly lost coverage: %+v", result.Checks)
			}
			item, ok := model.Paths.PathItems.Get("/pets/~demo")
			if !ok || item != pathItem || item.Get != operation || operation.OperationId != "read" || len(item.Parameters) != 1 || item.Parameters[0] != header || header.Name != "X-ID" || header.In != "header" || model.GoLow() != nil {
				t.Fatal("validation changed the caller's high model")
			}
		})
	}
}

func TestSourcesApplicationBaseAndRelativeSelf(t *testing.T) {
	t.Run("relative retrieval and source locations", func(t *testing.T) {
		input := strings.Replace(sourceTestDocument("        operationId: read\n"), "url: 'api.yaml'", "url: '../api.yaml'", 1)
		candidate := upstream.CandidateDocument{RetrievalURI: "../api.yaml", SourceBytes: []byte(sourceOpenAPI)}
		result, err := ValidateBytes(context.Background(), []byte(input), "flows/main.yaml", WithBaseURI("https://example.test/bundle/"), WithSources(candidate))
		if err != nil || !result.Valid() || !result.Complete {
			t.Fatalf("application base did not resolve relative locations: %+v %v", result, err)
		}
		found := false
		for _, check := range result.Checks {
			if check.Name == "operation-target" && check.Source == "https://example.test/bundle/api.yaml" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("resolved source identity missing from coverage: %+v", result.Checks)
		}
	})
	t.Run("relative self resolves against retrieval plus application base", func(t *testing.T) {
		input := `arazzo: 1.1.0
$self: ../identity/main.yaml
info: {title: Main, version: '1'}
sourceDescriptions: [{name: other, url: remote.yaml, type: arazzo}]
workflows: [{workflowId: run, steps: [{stepId: first, workflowId: $sourceDescriptions.other.remote}]}]
`
		external := `arazzo: 1.1.0
$self: ../identity/remote.yaml
info: {title: Remote, version: '1'}
sourceDescriptions: [{name: api, url: api.yaml, type: openapi}]
workflows: [{workflowId: remote, steps: [{stepId: first, operationId: read}]}]
`
		candidate := upstream.CandidateDocument{RetrievalURI: "https://example.test/bundle/retrieved/other.yaml", SourceBytes: []byte(external)}
		result, err := ValidateBytes(context.Background(), []byte(input), "retrieved/main.yaml", WithBaseURI("https://example.test/bundle/"), WithSources(candidate))
		if err != nil || !result.Valid() || !result.Complete {
			t.Fatalf("relative $self identity did not resolve: %+v %v", result, err)
		}
		found := false
		for _, check := range result.Checks {
			if check.Name == "workflow-target" && check.Source == "https://example.test/bundle/identity/remote.yaml" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("canonical $self identity missing from coverage: %+v", result.Checks)
		}
		input = strings.Replace(input, "url: remote.yaml", "url: ../retrieved/other.yaml", 1)
		result, err = ValidateBytes(context.Background(), []byte(input), "retrieved/main.yaml", WithBaseURI("https://example.test/bundle/"), WithSources(candidate))
		if err != nil {
			t.Fatal(err)
		}
		sourceDiagnostic(t, result, CodeReference, "/sourceDescriptions/0/url")
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code == CodeReference && diagnostic.Pointer == "/sourceDescriptions/0/url" && diagnostic.URI != "retrieved/main.yaml" {
				t.Fatalf("retrieval URI for authored diagnostic was replaced: %+v", diagnostic)
			}
		}
	})
}
