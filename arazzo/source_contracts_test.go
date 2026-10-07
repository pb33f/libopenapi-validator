// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi"
	upstream "github.com/pb33f/libopenapi/arazzo"
	highArazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

// These unit fixtures start at the source boundary, with no primary document
// resource charges. Each test sets only the budget that its behavior requires.
func sourceContractSession() *sourceSession {
	v := &validation{ctx: context.Background(), opts: options{limits: DefaultLimits()}, result: &Result{}, nodes: map[string]nodeLocation{}}
	return &sourceSession{v: v, base: "https://example.test/main.yaml", byURI: map[string]*linkedSource{}, byName: map[string]map[string]any{}, requests: map[string]*linkedSource{}, descriptionPaths: map[string]string{}}
}

func sourceContractError(t *testing.T, s *sourceSession, kind ErrorKind) {
	t.Helper()
	var e *Error
	if !errors.As(s.v.err, &e) || e.Kind != kind {
		t.Fatalf("wanted %s, got %v", kind, s.v.err)
	}
}

func sourceContractYAML(t *testing.T, data string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(data), &n); err != nil {
		t.Fatal(err)
	}
	return &n
}

func sourceContractLinked(s *sourceSession, kind string, root map[string]any) *linkedSource {
	return &linkedSource{owner: s, kind: kind, root: root, retrieval: "https://example.test/api.yaml", operations: map[string][]*linkedTarget{}, pointers: map[string]*linkedTarget{}, workflows: map[string]map[string]any{}}
}

func TestSourceContractBuildFailures(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		source                *upstream.ResolvedSource
		nodes, bytes, sources int
		kind                  ErrorKind
	}{
		{name: "nil resolver result"},
		{name: "malformed YAML", source: &upstream.ResolvedSource{SourceBytes: []byte("openapi: [")}, kind: ErrorInput},
		{name: "multiple documents", source: &upstream.ResolvedSource{SourceBytes: []byte("openapi: 3.1.0\n---\nopenapi: 3.1.0\n")}, kind: ErrorInput},
		{name: "non JSON mapping key", source: &upstream.ResolvedSource{SourceBytes: []byte("openapi: 3.1.0\n? [a, b]\n: value\n")}, kind: ErrorInput},
		{name: "raw byte budget", source: &upstream.ResolvedSource{SourceBytes: []byte("openapi: 3.1.0")}, bytes: 1, kind: ErrorLimit},
		{name: "source count", source: &upstream.ResolvedSource{}, sources: 1, kind: ErrorLimit},
		{name: "no remaining parsed bytes", source: &upstream.ResolvedSource{SourceBytes: []byte("{}")}, bytes: 2, kind: ErrorLimit},
		{name: "no remaining parsed nodes", source: &upstream.ResolvedSource{RootNode: sourceContractYAML(t, "{}")}, nodes: 1, kind: ErrorLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sourceContractSession()
			if tc.bytes > 0 {
				s.v.opts.limits.MaxBytes = tc.bytes
			}
			if tc.nodes > 0 {
				s.v.opts.limits.MaxNodes = tc.nodes
				s.nodes = tc.nodes
			}
			if tc.sources > 0 {
				s.v.opts.limits.MaxSources = tc.sources
				s.count = tc.sources
			}
			got := s.build(tc.source)
			if got != nil {
				t.Fatalf("failed load returned source: %+v", got)
			}
			if tc.kind != "" {
				sourceContractError(t, s, tc.kind)
			} else if s.v.err != nil {
				t.Fatal(s.v.err)
			}
		})
	}
	s := sourceContractSession()
	placeholder := s.build(&upstream.ResolvedSource{URL: "api.yaml"})
	if placeholder == nil || placeholder.identity != "https://example.test/api.yaml" || placeholder.kind != "" || len(s.v.result.Checks) != 1 || s.v.result.Checks[0].Name != "source-kind" || s.v.result.Checks[0].Status != CheckIncomplete {
		t.Fatalf("placeholder classification: %+v %+v", placeholder, s.v.result)
	}
	s = sourceContractSession()
	if s.build(&upstream.ResolvedSource{RetrievalURI: "https://example.test/api.yaml", Identity: "https://example.test/wrong.yaml", SourceBytes: []byte("arazzo: 1.1.0\n$self: canonical.yaml\n")}) != nil {
		t.Fatal("conflicting identity accepted")
	}
	sourceContractError(t, s, ErrorConfiguration)
	s = sourceContractSession()
	s.base = ""
	source := s.build(&upstream.ResolvedSource{Adapter: operationPresence{}})
	if source == nil || source.identity != "" || source.base != "" || !s.register(source) || len(s.byURI) != 0 {
		t.Fatalf("anonymous typed source: %+v %v", source, s.v.err)
	}
}

func TestSourceContractParsedModels(t *testing.T) {
	t.Run("OpenAPI low root", func(t *testing.T) {
		doc, err := libopenapi.NewDocument([]byte(sourceOpenAPI))
		if err != nil {
			t.Fatal(err)
		}
		model, errs := doc.BuildV3Model()
		if errs != nil {
			t.Fatal(errs)
		}
		s := sourceContractSession()
		got := s.build(&upstream.ResolvedSource{URL: "api.yaml", OpenAPIDocument: &model.Model})
		if s.v.err != nil || got == nil || got.root == nil || len(got.operations["read"]) != 1 {
			t.Fatalf("parsed OpenAPI projection: %+v %v", got, s.v.err)
		}
	})
	t.Run("Arazzo low root", func(t *testing.T) {
		doc, err := libopenapi.NewArazzoDocument([]byte(foundationYAML("1.1.0")))
		if err != nil {
			t.Fatal(err)
		}
		s := sourceContractSession()
		got := s.build(&upstream.ResolvedSource{URL: "api.yaml", ArazzoDocument: doc})
		if s.v.err != nil || got == nil || got.root == nil || got.workflows["run"] == nil || got.partialExpressions {
			t.Fatalf("parsed Arazzo projection: %+v %v", got, s.v.err)
		}
	})
}

func TestSourceContractRootlessArazzoInputs(t *testing.T) {
	for _, tc := range []struct {
		name, data   string
		nodes, depth int
		kind         ErrorKind
	}{
		{name: "schema", data: "type: object\nproperties: {id: {type: string}}\n"},
		{name: "tagged schema", data: "!unsupported {type: object}", kind: ErrorInput},
		{name: "depth", data: "properties: {id: {type: string}}", depth: 1, kind: ErrorLimit},
		{name: "exact budget exhausted before inputs", data: "{}", nodes: 7, kind: ErrorLimit},
		{name: "workflow budget", data: "{}", nodes: 5, kind: ErrorLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sourceContractSession()
			if tc.nodes > 0 {
				s.v.opts.limits.MaxNodes = tc.nodes
			}
			if tc.depth > 0 {
				s.v.opts.limits.MaxDepth = tc.depth
			}
			inputs := sourceContractYAML(t, tc.data)
			before, _ := yaml.Marshal(inputs)
			model := &highArazzo.Arazzo{Arazzo: "1.1.0", Workflows: []*highArazzo.Workflow{nil, {WorkflowId: "remote", DependsOn: []string{"prior"}, Inputs: inputs, Steps: []*highArazzo.Step{nil, {StepId: "first", DependsOn: []string{"second"}}}}}}
			got := s.build(&upstream.ResolvedSource{URL: "api.yaml", ArazzoDocument: model})
			if tc.kind != "" {
				if got != nil {
					t.Fatal("invalid source accepted")
				}
				sourceContractError(t, s, tc.kind)
				return
			}
			if s.v.err != nil || got == nil || !got.partialExpressions {
				t.Fatalf("rootless model: %+v %v", got, s.v.err)
			}
			wf := got.workflows["remote"]
			if !reflect.DeepEqual(wf["dependsOn"], []any{"prior"}) || object(wf["inputs"])["type"] != "object" || !reflect.DeepEqual(got.steps["remote"]["first"]["dependsOn"], []any{"second"}) {
				t.Fatalf("lost input or dependency metadata: %+v", wf)
			}
			after, _ := yaml.Marshal(inputs)
			if string(before) != string(after) || len(model.Workflows) != 2 || model.Workflows[0] != nil {
				t.Fatal("caller model changed")
			}
		})
	}
	// Output projection failure must stop before the workflow enters the index.
	s := sourceContractSession()
	s.v.opts.limits.MaxNodes = 8
	outputs := orderedmap.New[string, *highArazzo.OutputValue]()
	outputs.Set("id", highArazzo.NewExpressionOutputValue("$response.body#/id"))
	if s.build(&upstream.ResolvedSource{ArazzoDocument: &highArazzo.Arazzo{Workflows: []*highArazzo.Workflow{{WorkflowId: "remote", Outputs: outputs}}}}) != nil {
		t.Fatal("partially projected workflow returned")
	}
	sourceContractError(t, s, ErrorLimit)
}

func TestSourceContractLookupFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		response  *upstream.ResolvedSource
		err       error
		kind      ErrorKind
		duplicate bool
	}{
		{name: "unavailable", err: upstream.ErrUnresolvedSourceDesc},
		{name: "nil response"},
		{name: "placeholder", response: &upstream.ResolvedSource{URL: "api.yaml"}},
		{name: "invalid source", response: &upstream.ResolvedSource{URL: "api.yaml", SourceBytes: []byte("[")}, kind: ErrorInput},
		{name: "conflicting registration", response: &upstream.ResolvedSource{URL: "api.yaml", Identity: "https://example.test/canonical.yaml", Adapter: operationPresence{}}, duplicate: true, kind: ErrorConfiguration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sourceContractSession()
			s.byName["api"] = map[string]any{"url": "api.yaml", "type": "openapi"}
			if tc.duplicate {
				s.byURI["https://example.test/canonical.yaml"] = &linkedSource{}
			}
			calls := 0
			s.v.opts.resolver = sourceResolverFunc(func(context.Context, upstream.SourceRequest) (*upstream.ResolvedSource, error) {
				calls++
				return tc.response, tc.err
			})
			if s.lookup("api", "/workflows/0/steps/0/operationId") != nil {
				t.Fatal("failed source returned a target")
			}
			if tc.kind != "" {
				sourceContractError(t, s, tc.kind)
			} else {
				if s.v.err != nil || len(s.v.result.Checks) == 0 {
					t.Fatalf("missing incomplete coverage: %+v %v", s.v.result, s.v.err)
				}
				// Unavailable and nil resolver results are cached to prevent repeat I/O.
				if tc.name != "placeholder" {
					s.lookup("api", "/second")
					if calls != 1 {
						t.Fatalf("resolver called %d times", calls)
					}
				}
			}
		})
	}
	s := sourceContractSession()
	if s.lookup("missing", "/target") != nil || len(s.v.result.Diagnostics) != 1 || s.v.result.Diagnostics[0].Code != CodeReference {
		t.Fatal("unknown description lacks finding")
	}
}

func TestSourceContractURIsAndNames(t *testing.T) {
	for _, tc := range []struct{ ref, base, want string }{
		{"https://EXAMPLE.test/API", "", "https://example.test/API"},
		{"%invalid", "https://example.test/", "%invalid"},
		{"child.yaml", "%invalid", "child.yaml"},
	} {
		if got := resolveLinkedURI(tc.ref, tc.base); got != tc.want {
			t.Fatalf("URI %q: %q", tc.ref, got)
		}
	}
	s := sourceContractSession()
	for _, tc := range []struct {
		value, name, target string
		ok                  bool
	}{
		{"read", "", "", false}, {"$sourceDescriptions.api", "", "", false}, {"$sourceDescriptions..read", "", "", false}, {"$sourceDescriptions.api.", "", "", false}, {"$sourceDescriptions.api.read", "api", "read", true},
	} {
		n, id, ok := s.qualified(tc.value)
		if n != tc.name || id != tc.target || ok != tc.ok {
			t.Fatalf("qualification %q: %q %q %v", tc.value, n, id, ok)
		}
	}
	if s.descriptionPath("missing") != "/sourceDescriptions" {
		t.Fatal("missing default description path")
	}
	s.scopePath = "https://example.test/other.yaml#/sourceDescriptions/0"
	if s.descriptionPath("missing") != s.scopePath {
		t.Fatal("missing scoped description path")
	}
	source := sourceContractLinked(s, "openapi", nil)
	source.retrieval = ""
	source.identity = "https://example.test/canonical.yaml"
	key := s.sourcePath(source, "unrooted")
	if s.v.foreignLocations[key].URI != source.identity || s.v.foreignLocations[key].Pointer != "unrooted" {
		t.Fatal("fallback source location lost")
	}
}

func TestSourceContractLocalReferences(t *testing.T) {
	target := map[string]any{"operationId": "read"}
	root := map[string]any{"entries": []any{target}, "scalar": "stop", "loop": map[string]any{"$ref": "#/loop"}, "chain": map[string]any{"$ref": "#/entries/0"}}
	for _, tc := range []struct {
		ref   string
		found bool
	}{
		{"#/entries/0", true}, {"#/chain", true}, {"#/entries/-1", false}, {"#/entries/1", false}, {"#/entries/bogus", false}, {"#/scalar/child", false}, {"#/bad~2token", false}, {"#/loop", false}, {"other.yaml#/paths", false},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			s := sourceContractSession()
			source := sourceContractLinked(s, "openapi", root)
			first := source.deref(map[string]any{"$ref": tc.ref})
			second := source.deref(map[string]any{"$ref": tc.ref})
			if (first != nil) != tc.found || !reflect.DeepEqual(first, second) || (tc.found && first["operationId"] != "read") {
				t.Fatalf("reference resolution: %+v %+v", first, second)
			}
			if strings.HasPrefix(tc.ref, "other") && !source.unresolvedRefs {
				t.Fatal("external reference was treated as complete")
			}
		})
	}
	s := sourceContractSession()
	s.v.opts.limits.MaxNodes = 1
	source := sourceContractLinked(s, "openapi", root)
	if source.deref(map[string]any{"$ref": "#/chain"}) != nil {
		t.Fatal("reference chain escaped work cap")
	}
	sourceContractError(t, s, ErrorLimit)
}

func TestSourceContractRawOperationIndex(t *testing.T) {
	s := sourceContractSession()
	callback := map[string]any{"x-ignore": map[string]any{"get": map[string]any{"operationId": "ignore"}}, "{$request.query.url}": map[string]any{"post": map[string]any{"operationId": "notify"}}}
	root := map[string]any{
		"components": map[string]any{"pathItems": map[string]any{"shared": map[string]any{"get": map[string]any{"operationId": "componentOnly"}}}, "callbacks": map[string]any{"notify": callback}},
		"paths":      map[string]any{"/pets": map[string]any{"additionalOperations": map[string]any{"CUSTOM/~": map[string]any{"operationId": "custom"}}, "get": map[string]any{"operationId": "read", "parameters": []any{map[string]any{"$ref": "other.yaml#/parameters/id"}}, "callbacks": map[string]any{"notify": map[string]any{"$ref": "#/components/callbacks/notify"}}}}},
	}
	source := sourceContractLinked(s, "openapi", root)
	source.indexRaw()
	if s.v.err != nil || len(source.operations["read"]) != 1 || len(source.operations["custom"]) != 1 || len(source.operations["notify"]) != 1 || len(source.operations["componentOnly"]) != 0 || len(source.operations["ignore"]) != 0 {
		t.Fatalf("operation identities: %+v %v", source.operations, s.v.err)
	}
	if source.pointers["/paths/~1pets/additionalOperations/CUSTOM~1~0"] == nil || source.pointers["/components/pathItems/shared/get"] == nil || source.operations["read"][0].parametersComplete || !source.unresolvedRefs {
		t.Fatalf("pointer/parameter coverage: %+v", source)
	}
	// A callback cycle ends without recursively expanding the same callback.
	recursive := map[string]any{"{$request.query.url}": map[string]any{"post": map[string]any{"operationId": "notify", "callbacks": map[string]any{"again": map[string]any{"$ref": "#/components/callbacks/notify"}}}}}
	root["components"].(map[string]any)["callbacks"].(map[string]any)["notify"] = recursive
	source = sourceContractLinked(sourceContractSession(), "openapi", root)
	source.indexRaw()
	if !source.unresolvedRefs || len(source.operations["notify"]) > 2 {
		t.Fatal("recursive callback did not stop")
	}
	// A pathItem reference recurring through a callback must also stop.
	recursive["{$request.query.url}"] = map[string]any{"$ref": "#/paths/~1pets"}
	source = sourceContractLinked(sourceContractSession(), "openapi", root)
	source.indexRaw()
	if !source.unresolvedRefs {
		t.Fatal("recursive path item did not stop")
	}
	// AsyncAPI 2 operations live on channel publish/subscribe objects.
	source = sourceContractLinked(sourceContractSession(), "asyncapi", map[string]any{"channels": map[string]any{"pets": map[string]any{"publish": map[string]any{"operationId": "publishPet"}, "subscribe": map[string]any{"operationId": "readPet"}}}})
	source.indexRaw()
	if len(source.operations["publishPet"]) != 1 || len(source.operations["readPet"]) != 1 {
		t.Fatal("AsyncAPI 2 channel operations omitted")
	}
}

func TestSourceContractRootlessOpenAPIMetadata(t *testing.T) {
	s := sourceContractSession()
	source := sourceContractLinked(s, "openapi", nil)
	source.indexOpenAPI(&v3.Document{}, s)
	paths := orderedmap.New[string, *v3.PathItem]()
	paths.Set("/nil", nil)
	additional := orderedmap.New[string, *v3.Operation]()
	additional.Set("CUSTOM/~", &v3.Operation{OperationId: "custom"})
	paths.Set("/pets", &v3.PathItem{AdditionalOperations: additional, Get: &v3.Operation{OperationId: "read", Parameters: []*v3.Parameter{nil, {Reference: "#/components/parameters/id"}}}})
	source.indexOpenAPI(&v3.Document{Paths: &v3.Paths{PathItems: paths}}, s)
	if s.v.err != nil || source.pointers["/paths/~1pets/additionalOperations/CUSTOM~1~0"] == nil || len(source.operations["custom"]) != 1 || source.operations["read"][0].parametersComplete {
		t.Fatalf("rootless metadata: %+v %v", source, s.v.err)
	}
	for _, tc := range []struct {
		name         string
		model        *v3.Document
		nodes, bytes int
	}{
		{name: "path", model: &v3.Document{Paths: &v3.Paths{PathItems: paths}}, nodes: 1},
		{name: "additional method bytes", model: &v3.Document{Paths: &v3.Paths{PathItems: paths}}, bytes: 12},
		{name: "operation", model: &v3.Document{Paths: &v3.Paths{PathItems: paths}}, nodes: 6},
		{name: "parameter", model: &v3.Document{Paths: &v3.Paths{PathItems: paths}}, nodes: 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sourceContractSession()
			if tc.nodes > 0 {
				s.v.opts.limits.MaxNodes = tc.nodes
			}
			if tc.bytes > 0 {
				s.v.opts.limits.MaxBytes = tc.bytes
			}
			source := sourceContractLinked(s, "openapi", nil)
			source.indexOpenAPI(tc.model, s)
			sourceContractError(t, s, ErrorLimit)
		})
	}
}

func TestSourceContractOutputProjection(t *testing.T) {
	outputs := orderedmap.New[string, *highArazzo.OutputValue]()
	outputs.Set("scalar", highArazzo.NewExpressionOutputValue("$response.body#/id"))
	outputs.Set("selector", highArazzo.NewSelectorOutputValue(&highArazzo.Selector{Context: "$response.body", Selector: "$.id", Type: "jsonpath"}))
	outputs.Set("objectType", highArazzo.NewSelectorOutputValue(&highArazzo.Selector{Context: "$response.body", Selector: "$.id", ExpressionType: &highArazzo.ExpressionType{Type: "jsonpath", Version: "draft-goessner-dispatch-jsonpath-00"}}))
	outputs.Set("nil", nil)
	s := sourceContractSession()
	got := linkedOutputs(outputs, s)
	if s.v.err != nil || got["scalar"] != "$response.body#/id" || object(got["selector"])["type"] != "jsonpath" || object(object(got["objectType"])["type"])["version"] != "draft-goessner-dispatch-jsonpath-00" {
		t.Fatalf("output projection: %+v %v", got, s.v.err)
	}
	if value, exists := got["nil"]; !exists || value != nil {
		t.Fatal("nil output lost")
	}
	for _, tc := range []struct {
		name   string
		output *highArazzo.OutputValue
		nodes  int
	}{
		{"map entry", highArazzo.NewExpressionOutputValue("$response.body"), 1},
		{"scalar", highArazzo.NewExpressionOutputValue("$response.body"), 2},
		{"selector", highArazzo.NewSelectorOutputValue(&highArazzo.Selector{Context: "$response.body", Selector: "$.id", Type: "jsonpath"}), 8},
		{"object type", highArazzo.NewSelectorOutputValue(&highArazzo.Selector{Context: "$response.body", Selector: "$.id", ExpressionType: &highArazzo.ExpressionType{Type: "jsonpath", Version: "1"}}), 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sourceContractSession()
			s.v.opts.limits.MaxNodes = tc.nodes
			values := orderedmap.New[string, *highArazzo.OutputValue]()
			values.Set("id", tc.output)
			if linkedOutputs(values, s) != nil {
				t.Fatal("over-budget outputs returned")
			}
			sourceContractError(t, s, ErrorLimit)
		})
	}
	if outputs.Len() != 4 {
		t.Fatal("caller outputs changed")
	}
}

// A deterministic context models cancellation between source indexing phases.
// It avoids timing-sensitive goroutines while retaining normal ctx.Err behavior.
type sourceContractCancelContext struct {
	context.Context
	checks, after int
	cancel        context.CancelFunc
}

func newSourceCancelContext(t *testing.T, after int) *sourceContractCancelContext {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &sourceContractCancelContext{Context: ctx, after: after, cancel: cancel}
}

func (c *sourceContractCancelContext) Err() error {
	c.checks++
	if c.checks >= c.after {
		c.cancel()
	}
	return c.Context.Err()
}

func TestSourceContractIndexStops(t *testing.T) {
	t.Run("workflow work cap", func(t *testing.T) {
		s := sourceContractSession()
		s.v.opts.limits.MaxNodes = 1
		source := sourceContractLinked(s, "arazzo", map[string]any{"workflows": []any{map[string]any{"workflowId": "run", "steps": []any{map[string]any{"stepId": "read"}}}}})
		source.indexSteps()
		sourceContractError(t, s, ErrorLimit)
		if len(source.steps) != 0 {
			t.Fatal("over-budget workflow indexed")
		}
	})
	t.Run("step cancellation", func(t *testing.T) {
		s := sourceContractSession()
		s.v.ctx = newSourceCancelContext(t, 2)
		source := sourceContractLinked(s, "arazzo", map[string]any{"workflows": []any{map[string]any{"workflowId": "run", "steps": []any{map[string]any{"stepId": "read"}}}}})
		source.indexSteps()
		if !errors.Is(s.v.err, context.Canceled) || len(source.steps) != 0 {
			t.Fatal("canceled steps indexed")
		}
	})
	t.Run("operation absent", func(t *testing.T) {
		source := sourceContractLinked(sourceContractSession(), "openapi", nil)
		if source.addOperation(nil, "/get", nil) != nil {
			t.Fatal("nil operation indexed")
		}
	})
	t.Run("operation node budget", func(t *testing.T) {
		s := sourceContractSession()
		s.v.opts.limits.MaxNodes = 1
		s.nodes = 1
		source := sourceContractLinked(s, "openapi", nil)
		if source.addOperation(map[string]any{"operationId": "read"}, "/get", nil) != nil {
			t.Fatal("over-budget operation indexed")
		}
		sourceContractError(t, s, ErrorLimit)
	})
	t.Run("parameter work budget", func(t *testing.T) {
		s := sourceContractSession()
		s.v.opts.limits.MaxNodes = 1
		source := sourceContractLinked(s, "openapi", nil)
		operation := map[string]any{"operationId": "read", "parameters": []any{map[string]any{"name": "id", "in": "query"}, map[string]any{"name": "other", "in": "query"}}}
		if source.addOperation(operation, "/get", nil) != nil {
			t.Fatal("over-budget parameters indexed")
		}
		sourceContractError(t, s, ErrorLimit)
		if len(source.pointers) != 0 {
			t.Fatal("partially checked target registered")
		}
	})
	t.Run("path budget", func(t *testing.T) {
		s := sourceContractSession()
		s.v.opts.limits.MaxNodes = 1
		s.nodes = 1
		source := sourceContractLinked(s, "openapi", nil)
		source.indexPathItem(map[string]any{"get": map[string]any{"operationId": "read"}}, "/paths/~1pets", true, map[string]bool{})
		sourceContractError(t, s, ErrorLimit)
		if len(source.pointers) != 0 {
			t.Fatal("over-budget path registered")
		}
	})
	t.Run("rootless input cancellation", func(t *testing.T) {
		s := sourceContractSession()
		s.v.ctx = newSourceCancelContext(t, 4)
		model := &highArazzo.Arazzo{Workflows: []*highArazzo.Workflow{{WorkflowId: "remote", Inputs: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "schema"}}}}
		if s.build(&upstream.ResolvedSource{ArazzoDocument: model}) != nil || !errors.Is(s.v.err, context.Canceled) {
			t.Fatalf("canceled input accepted: %v", s.v.err)
		}
	})
}

func TestSourceContractStageStopsAndComponentActions(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		t.Run(map[bool]string{false: "workflow", true: "candidate"}[candidate], func(t *testing.T) {
			s := sourceContractSession()
			s.v.opts.limits.MaxNodes = 1
			s.v.root = map[string]any{"arazzo": "1.1.0", "workflows": []any{map[string]any{"workflowId": "run", "steps": []any{map[string]any{"stepId": "read"}}}}}
			if candidate {
				s.v.opts.sources = []upstream.CandidateDocument{{Adapter: operationPresence{}}}
			}
			checkSources(s.v)
			sourceContractError(t, s, ErrorLimit)
			if s.v.sources.count != 0 {
				t.Fatal("candidate loaded after primary index failed")
			}
		})
	}
	s := sourceContractSession()
	s.v.doc.URI = "https://example.test/main.yaml"
	s.v.root = map[string]any{"arazzo": "1.1.0", "sourceDescriptions": []any{map[string]any{"name": "flows", "url": "flows.yaml", "type": "arazzo"}}, "components": map[string]any{"successActions": map[string]any{"next": map[string]any{"workflowId": "$sourceDescriptions.flows.run"}}, "failureActions": map[string]any{"recover": map[string]any{"workflowId": "$sourceDescriptions.flows.run"}}}}
	s.v.opts.sources = []upstream.CandidateDocument{{RetrievalURI: "https://example.test/flows.yaml", Adapter: workflowPresence{}}}
	checkSources(s.v)
	if s.v.err != nil {
		t.Fatal(s.v.err)
	}
	pointers := map[string]bool{}
	for _, check := range s.v.result.Checks {
		if check.Name == "workflow-target" {
			pointers[check.Pointer] = true
		}
	}
	if !pointers["/components/successActions/next/workflowId"] || !pointers["/components/failureActions/recover/workflowId"] {
		t.Fatalf("component workflow targets omitted: %+v", s.v.result.Checks)
	}
	if name, target, ok := sourceQualified("read"); name != "" || target != "" || ok {
		t.Fatal("unqualified name accepted")
	}
}
