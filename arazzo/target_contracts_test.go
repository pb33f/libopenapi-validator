// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"strings"
	"testing"

	upstream "github.com/pb33f/libopenapi/arazzo"
	"github.com/pb33f/libopenapi/arazzo/expression"
)

type targetContractOpaque string

func (a targetContractOpaque) SourceType() string { return string(a) }

type targetContractAsyncPresence struct{}

func (targetContractAsyncPresence) SourceType() string          { return "asyncapi" }
func (targetContractAsyncPresence) HasOperation(id string) bool { return id == "read" }
func (targetContractAsyncPresence) HasChannel(id string) bool   { return id == "events" }

// The harness enters the source boundary directly for malformed target strings
// that the document schema or expression parser rejects before source resolution.
func targetContractSession(t *testing.T, kind string, adapter upstream.SourceDocumentAdapter, data string) (*sourceSession, *linkedSource) {
	t.Helper()
	v := &validation{ctx: context.Background(), opts: options{limits: DefaultLimits()}, result: &Result{}, version: "1.1", doc: Document{URI: "https://example.test/main.yaml"}, root: map[string]any{}, budget: &workBudget{}}
	v.graph = newDependencyGraph()
	v.graph.run = v
	v.local = &semanticIndex{workflows: map[string]*semanticWorkflow{}, components: map[string]map[string]any{}, sources: map[string]string{"api": "api.yaml"}}
	s := &sourceSession{v: v, sourceState: &sourceState{byURI: map[string]*linkedSource{}, requests: map[string]*linkedSource{}, graphVisited: map[*linkedSource]bool{}, targets: map[dependencyID]*linkedTarget{}}, sourceScope: sourceScope{base: v.doc.URI, byName: map[string]map[string]any{"api": {"name": "api", "url": "api.yaml", "type": kind}}, descriptionPaths: map[string]string{"api": "/sourceDescriptions/0"}}}
	v.sources = s
	source := s.build(&upstream.ResolvedSource{Type: kind, RetrievalURI: "https://example.test/api.yaml", Adapter: adapter, SourceBytes: []byte(data)})
	if v.err != nil || source == nil || !s.register(source) {
		t.Fatalf("source setup: %v", v.err)
	}
	return s, source
}

func targetContractFinding(t *testing.T, r *Result, code Code, pointer, message string) {
	t.Helper()
	for _, d := range r.Diagnostics {
		if d.Code == code && d.Pointer == pointer && strings.Contains(d.Message, message) {
			return
		}
	}
	t.Fatalf("missing %s at %s (%s): %+v", code, pointer, message, r.Diagnostics)
}

func targetContractCheck(t *testing.T, r *Result, name, pointer string, status CheckStatus) {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name && c.Pointer == pointer && c.Status == status {
			return
		}
	}
	t.Fatalf("missing %s %s at %s: %+v", name, status, pointer, r.Checks)
}

func TestTargetContractPointerBoundary(t *testing.T) {
	for _, test := range []struct{ value, message string }{
		{"{$sourceDescriptions.api.url}", "JSON Pointer fragment"},
		{"api.yaml#/paths/~1pets/get", "runtime expression"},
		{"{$sourceDescriptions.api.name}#/paths/~1pets/get", "description url"},
		{"{$sourceDescriptions.api.url}#%GG", "URI fragment encoding"},
		{"{$sourceDescriptions.api.url}#paths/~1pets/get", "JSON Pointer fragment"},
		{"{$sourceDescriptions.api.url}#/paths/~2pets/get", "JSON Pointer fragment"},
	} {
		t.Run(test.value, func(t *testing.T) {
			s, _ := targetContractSession(t, "openapi", operationPresence{}, "")
			if got := s.pointer(test.value, "/target", "operationPath", nil, "/step"); got != nil {
				t.Fatal("invalid pointer resolved")
			}
			targetContractFinding(t, s.v.result, CodeReference, "/target", test.message)
		})
	}
	for _, test := range []struct {
		kind, field, pointer string
		adapter              upstream.SourceDocumentAdapter
		check                string
		target               bool
	}{
		{"openapi", "operationPath", "/paths/~1pets/get", operationPresence{}, "target-metadata", true},
		{"asyncapi", "operationPath", "/operations/read", targetContractAsyncPresence{}, "target-metadata", true},
		{"asyncapi", "channelPath", "/channels/events", targetContractAsyncPresence{}, "target-metadata", true},
		{"asyncapi", "channelPath", "/channels/events", targetContractOpaque("asyncapi"), "target-presence", false},
		{"openapi", "operationPath", "/paths/~1pets/get", targetContractOpaque("openapi"), "target-presence", false},
	} {
		t.Run(test.kind+test.field+test.check, func(t *testing.T) {
			s, _ := targetContractSession(t, test.kind, test.adapter, "")
			got := s.pointer("{$sourceDescriptions.api.url}#"+test.pointer, "/target", test.field, nil, "/step")
			if (got != nil) != test.target || !s.v.result.Valid() {
				t.Fatalf("target=%+v diagnostics=%+v", got, s.v.result.Diagnostics)
			}
			targetContractCheck(t, s.v.result, test.check, "/target", CheckIncomplete)
		})
	}
	s, source := targetContractSession(t, "openapi", nil, sourceOpenAPI)
	source.unresolvedRefs = true
	if s.pointer("{$sourceDescriptions.api.url}#/paths/~1missing/get", "/target", "operationPath", nil, "/step") != nil {
		t.Fatal("missing pointer resolved")
	}
	targetContractCheck(t, s.v.result, "target-presence", "/target", CheckIncomplete)
	if s.pointer("{$sourceDescriptions.api.url}#/channels/events", "/channel", "channelPath", nil, "/step") != nil {
		t.Fatal("HTTP channel resolved")
	}
	targetContractFinding(t, s.v.result, CodeSourceType, "/channel", "wrong document type")
	if s.pointer("{$sourceDescriptions.missing.url}#/paths/~1pets/get", "/unknown", "operationPath", nil, "/step") != nil {
		t.Fatal("unknown source resolved")
	}
	targetContractFinding(t, s.v.result, CodeReference, "/unknown", "unknown source")
}

func TestTargetContractPointerShapes(t *testing.T) {
	for _, test := range []struct {
		pointer string
		valid   bool
	}{
		{"/paths/~1pets/get", true},
		{"/webhooks/event/post", true},
		{"/components/pathItems/pets/get", true},
		{"/components/callbacks/notify/expression/post", true},
		{"/paths/~1pets/additionalOperations/CUSTOM", true},
		{"/paths/~1pets/get/callbacks/notify/expression/post", true},
		{"", false},
		{"/paths/x", false},
		{"/components/pathItems/x", false},
		{"/components/schemas/x/get", false},
		{"/unknown/x/get", false},
		{"/paths/x/additionalOperations", false},
		{"/paths/x/GET", false},
		{"/paths/x/get/callbacks/x/y", false},
		{"/paths/x/get/responses/x/y/z", false},
		{"/components/callbacks/x/y", false},
	} {
		tokens, ok := linkedPointerTokens(test.pointer)
		if !ok {
			t.Fatalf("valid pointer syntax rejected: %s", test.pointer)
		}
		if got := linkedOperationPointerShape(tokens); got != test.valid {
			t.Errorf("%s: got %v want %v", test.pointer, got, test.valid)
		}
	}
	for _, pointer := range []string{"x", "/trailing~", "/bad~2"} {
		if _, ok := linkedPointerTokens(pointer); ok {
			t.Errorf("accepted invalid escape: %s", pointer)
		}
	}
}

func TestTargetContractOperationAndWorkflowCapabilities(t *testing.T) {
	for _, test := range []struct {
		kind, value, check string
		adapter            upstream.SourceDocumentAdapter
		code               Code
	}{
		{"arazzo", "$sourceDescriptions.api.run", "", workflowPresence{}, CodeSourceType},
		{"openapi", "read", "operation-presence", targetContractOpaque("openapi"), ""},
		{"asyncapi", "read", "operation-metadata", targetContractAsyncPresence{}, ""},
	} {
		s, _ := targetContractSession(t, test.kind, test.adapter, "")
		got := s.operation(test.value, "/op", nil, "/step")
		if test.code != "" {
			targetContractFinding(t, s.v.result, test.code, "/op", "requires")
			continue
		}
		if test.check == "operation-metadata" && got == nil {
			t.Fatal("known async operation did not resolve")
		}
		targetContractCheck(t, s.v.result, test.check, "/op", CheckIncomplete)
	}
	s, source := targetContractSession(t, "openapi", nil, sourceOpenAPI)
	source.operations["read"] = append(source.operations["read"], source.operations["read"][0])
	if s.operation("read", "/duplicate", nil, "/step") != nil {
		t.Fatal("duplicate operation resolved")
	}
	targetContractFinding(t, s.v.result, CodeReference, "/duplicate", "not unique")
	s.byName["other"] = map[string]any{"type": "asyncapi"}
	if s.operation("read", "/ambiguous", nil, "/step") != nil {
		t.Fatal("ambiguous operation resolved")
	}
	targetContractFinding(t, s.v.result, CodeReference, "/ambiguous", "multiple API sources")
	s.byName = map[string]map[string]any{"api": {"type": "arazzo"}}
	if s.operation("read", "/none", nil, "/step") != nil {
		t.Fatal("operation without API source resolved")
	}
	targetContractFinding(t, s.v.result, CodeReference, "/none", "requires an API source")
	s, _ = targetContractSession(t, "openapi", operationPresence{}, "")
	if s.workflow("$sourceDescriptions.api.run", "/wrong") != nil {
		t.Fatal("HTTP workflow resolved")
	}
	targetContractFinding(t, s.v.result, CodeSourceType, "/wrong", "requires an Arazzo")
	if s.workflow("run", "/bare") != nil {
		t.Fatal("bare external workflow resolved")
	}
	s, _ = targetContractSession(t, "arazzo", targetContractOpaque("arazzo"), "")
	if s.workflow("$sourceDescriptions.api.run", "/metadata") != nil {
		t.Fatal("opaque workflow resolved")
	}
	targetContractCheck(t, s.v.result, "workflow-target", "/metadata", CheckIncomplete)
}

func TestTargetContractAsyncMetadata(t *testing.T) {
	data := "asyncapi: 3.0.0\nchannels: {events: {messages: {one: {}, two: {}}}}\noperations: {read: {action: receive, channel: {$ref: '#/channels/events'}}}\n"
	for _, test := range []struct {
		action, correlation string
		criteria            bool
		code                Code
		pointer, check      string
	}{
		{"", "", false, CodeReference, "/step", ""},
		{"send", "id", true, CodeSourceType, "/step/correlationId", "asyncapi-correlation"},
		{"send", "", true, CodeReference, "/step/action", ""},
		{"receive", "id", true, "", "", "asyncapi-correlation"},
	} {
		s, source := targetContractSession(t, "asyncapi", nil, data)
		step := map[string]any{"action": test.action, "correlationId": test.correlation}
		if test.criteria {
			step["successCriteria"] = []any{map[string]any{"condition": "$message.payload#/id"}}
		}
		s.async(step, "/step", source.operations["read"][0])
		if test.code != "" {
			targetContractFinding(t, s.v.result, test.code, test.pointer, "")
		} else if !s.v.result.Valid() {
			t.Fatalf("receive rejected: %+v", s.v.result.Diagnostics)
		}
		if test.check != "" {
			targetContractCheck(t, s.v.result, test.check, "/step/correlationId", CheckIncomplete)
		}
	}
	s, source := targetContractSession(t, "asyncapi", nil, strings.Replace(data, "one: {}, two: {}", "one: {}", 1))
	s.async(map[string]any{"action": "receive"}, "/step", source.pointers["/channels/events"])
	targetContractCheck(t, s.v.result, "asyncapi-success", "/step", CheckIncomplete)
	s, source = targetContractSession(t, "openapi", nil, sourceOpenAPI)
	s.async(map[string]any{"action": "send", "correlationId": "id"}, "/step", source.operations["read"][0])
	targetContractFinding(t, s.v.result, CodeSourceType, "/step/action", "only to AsyncAPI")
	targetContractFinding(t, s.v.result, CodeSourceType, "/step/correlationId", "only to AsyncAPI")
}

func TestTargetContractExternalPrerequisites(t *testing.T) {
	main := strings.Replace(sourceTestDocument("        workflowId: $sourceDescriptions.api.remote\n"), "type: openapi", "type: arazzo", 1)
	external := "arazzo: 1.1.0\ninfo: {title: Other, version: '1'}\nsourceDescriptions: [{name: known, url: main.yaml, type: arazzo}, {name: opaque, url: opaque.yaml, type: arazzo}]\nworkflows:\n- workflowId: remote\n  steps:\n  - stepId: first\n    operationId: read\n    dependsOn:\n    - DEPENDENCY\n- workflowId: other\n  steps: [{stepId: local, operationId: read}]\n"
	for _, test := range []struct{ dependency, message, check string }{
		{"missing", "external step prerequisite", ""},
		{"$workflows.other.steps.local", "", ""},
		{"$workflows.other.steps.missing", "cross-workflow step prerequisite", ""},
		{"$workflows.other", "invalid cross-workflow", ""},
		{"$sourceDescriptions.known.run.steps.first", "", ""},
		{"$sourceDescriptions.known.run.steps.missing", "external step prerequisite does not exist", ""},
		{"$sourceDescriptions.known.missing.steps.first", "does not exist in source", ""},
		{"$sourceDescriptions.known.run", "invalid external step dependency", ""},
		{"$sourceDescriptions.opaque.run.steps.first", "", "external-prerequisites"},
	} {
		t.Run(test.dependency, func(t *testing.T) {
			result, err := ValidateBytes(context.Background(), []byte(main), "https://example.test/main.yaml", WithSources(sourceCandidate(strings.Replace(external, "DEPENDENCY", test.dependency, 1)), upstream.CandidateDocument{RetrievalURI: "https://example.test/opaque.yaml", Adapter: workflowPresence{}}))
			if err != nil {
				t.Fatal(err)
			}
			pointer := "/workflows/0/steps/0/dependsOn/0"
			if test.message != "" {
				targetContractFinding(t, result, CodeReference, pointer, test.message)
				for _, d := range result.Diagnostics {
					if d.Code == CodeReference && d.Pointer == pointer && (d.URI != "https://example.test/api.yaml" || d.Line != 10 || d.Column != 7) {
						t.Fatalf("external location: %+v", d)
					}
				}
			} else if !result.Valid() {
				t.Fatalf("valid prerequisite rejected: %+v", result.Diagnostics)
			}
			if test.check != "" {
				targetContractCheck(t, result, test.check, pointer, CheckIncomplete)
			}
		})
	}
}

func TestTargetContractLinkedExpressionFallbacks(t *testing.T) {
	s, _ := targetContractSession(t, "arazzo", workflowPresence{}, "")
	s.v.expressionUses = []expressionUse{{Expression: expression.Expression{Type: expression.SourceDescriptions, Raw: "$sourceDescriptions.api.run"}, Path: "/presence"}}
	finalizeLinkedExpressionUses(s.v)
	targetContractCheck(t, s.v.result, "expression-symbol", "/presence", CheckIncomplete)
	for _, test := range []struct {
		raw, version, data string
		adapter            upstream.SourceDocumentAdapter
		message, check     string
	}{
		{"$sourceDescriptions.api.read", "1.1", sourceOpenAPI, nil, "", ""},
		{"$sourceDescriptions.api.url", "1.1", sourceOpenAPI, nil, "", ""},
		{"$sourceDescriptions.api.missing", "1.1", "", operationPresence{}, "", "expression-symbol"},
		{"$sourceDescriptions.api.read.property", "1.0", sourceOpenAPI, nil, "", "expression-symbol"},
		{"$sourceDescriptions.api.missing", "1.1", sourceOpenAPI, nil, "unknown source member", ""},
	} {
		s, _ := targetContractSession(t, "openapi", test.adapter, test.data)
		s.v.version = test.version
		s.v.expressionUses = []expressionUse{{Expression: expression.Expression{Type: expression.SourceDescriptions, Raw: test.raw}, Path: "/value"}}
		finalizeLinkedExpressionUses(s.v)
		if test.message != "" {
			targetContractFinding(t, s.v.result, CodeReference, "/value", test.message)
		} else if !s.v.result.Valid() {
			t.Fatalf("unavailable or known member rejected: %+v", s.v.result.Diagnostics)
		}
		if test.check != "" {
			targetContractCheck(t, s.v.result, test.check, "/value", CheckIncomplete)
		}
	}
	for _, raw := range []string{"$sourceDescriptions.api.remote", "$sourceDescriptions.api.remote.steps.produce.outputs.missing", "$sourceDescriptions.api.missing.outputs.id"} {
		s, _ := targetContractSession(t, "arazzo", nil, expressionLinkedSource)
		s.v.expressionUses = []expressionUse{{Expression: expression.Expression{Type: expression.SourceDescriptions, Raw: raw}, Path: "/value"}}
		finalizeLinkedExpressionUses(s.v)
		if raw == "$sourceDescriptions.api.remote" {
			if !s.v.result.Valid() {
				t.Fatal(s.v.result.Diagnostics)
			}
			continue
		}
		targetContractFinding(t, s.v.result, CodeReference, "/value", "")
	}
	// A missing supplied source cannot establish that an authored member is absent.
	s, _ = targetContractSession(t, "openapi", nil, sourceOpenAPI)
	s.byURI = map[string]*linkedSource{}
	s.v.expressionUses = []expressionUse{{Expression: expression.Expression{Type: expression.SourceDescriptions, Raw: "$sourceDescriptions.api.missing"}, Path: "/value"}}
	finalizeLinkedExpressionUses(s.v)
	targetContractCheck(t, s.v.result, "linked-target", "/value", CheckIncomplete)
}

func TestTargetContractSourceCancellationAndMetadata(t *testing.T) {
	s, source := targetContractSession(t, "arazzo", workflowPresence{}, "")
	if _, ok := s.externalStep("local", "/dep"); ok {
		t.Fatal("local dependency treated as external")
	}
	if _, ok := s.externalStep("$sourceDescriptions.api.run", "/bad"); ok {
		t.Fatal("missing step suffix resolved")
	}
	targetContractFinding(t, s.v.result, CodeReference, "/bad", "invalid external step")
	if _, ok := s.externalStep("$sourceDescriptions.api.run.steps.first", "/opaque"); ok {
		t.Fatal("presence-only step resolved")
	}
	targetContractCheck(t, s.v.result, "step-dependency", "/opaque", CheckIncomplete)
	s.graphSource(source, "/workflow", 1)
	targetContractCheck(t, s.v.result, "external-prerequisites", "/workflow", CheckIncomplete)
	for _, version := range []string{"1", "2.0.0"} {
		s, source = targetContractSession(t, "arazzo", nil, strings.Replace(expressionLinkedSource, "1.1.0", version, 1))
		s.graphExpressions(source)
		if version == "2.0.0" {
			targetContractCheck(t, s.v.result, "external-expression-grammar", "", CheckIncomplete)
		} else if len(s.v.result.Checks) != 0 {
			t.Fatalf("invalid version produced expression checks: %+v", s.v.result.Checks)
		}
	}
	s, source = targetContractSession(t, "arazzo", nil, expressionLinkedSource)
	source.retrieval = ""
	s.graphExpressions(source)
	if s.v.err != nil || !s.v.result.Valid() {
		t.Fatalf("canonical identity fallback failed: %+v %v", s.v.result, s.v.err)
	}
	s, source = targetContractSession(t, "arazzo", nil, expressionLinkedSource)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.v.ctx = ctx
	s.graphSource(source, "/workflow", 1)
	if !errors.Is(s.v.err, context.Canceled) {
		t.Fatalf("graph cancellation lost: %v", s.v.err)
	}
	s, source = targetContractSession(t, "arazzo", nil, expressionLinkedSource)
	s.v.opts.limits.MaxNodes = 1
	s.graphExpressions(source)
	var limit *Error
	if !errors.As(s.v.err, &limit) || limit.Kind != ErrorLimit {
		t.Fatalf("nested work limit lost: %v", s.v.err)
	}
}

func TestTargetContractLinkedEarlyReturns(t *testing.T) {
	v := &validation{}
	finalizeLinkedExpressionUses(v)
	for _, raw := range []string{"invalid", "$sourceDescriptions.unknown.member"} {
		s, _ := targetContractSession(t, "openapi", nil, sourceOpenAPI)
		s.v.expressionUses = []expressionUse{{Expression: expression.Expression{Type: expression.SourceDescriptions, Raw: raw}, Path: "/value"}}
		finalizeLinkedExpressionUses(s.v)
		if len(s.v.result.Checks)+len(s.v.result.Diagnostics) != 0 {
			t.Fatalf("local pass owns bad source symbols: %+v", s.v.result)
		}
	}
	s, _ := targetContractSession(t, "openapi", nil, sourceOpenAPI)
	s.v.expressionUses = []expressionUse{{Expression: expression.Expression{Type: expression.SourceDescriptions, Raw: "$sourceDescriptions.api.read"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.v.ctx = ctx
	finalizeLinkedExpressionUses(s.v)
	if !errors.Is(s.v.err, context.Canceled) {
		t.Fatalf("linked finalization ignored cancellation: %v", s.v.err)
	}
}

func TestTargetContractParameterMetadata(t *testing.T) {
	s, source := targetContractSession(t, "openapi", nil, sourceOpenAPI)
	s.v.root["components"] = map[string]any{"parameters": map[string]any{"header": map[string]any{"name": "X-ID", "in": "header"}}}
	defaults := []any{map[string]any{"name": "unused", "in": "header"}, map[string]any{"name": "X-ID", "in": "header"}}
	own := []any{map[string]any{"reference": "$components.parameters.header"}, map[string]any{"reference": "$other.unknown"}, map[string]any{}, map[string]any{"name": "a", "in": "query"}, map[string]any{"name": "b", "in": "query"}}
	s.parameters(defaults, "/defaults", own, "/own", source.operations["read"][0])
	targetContractCheck(t, s.v.result, "operation-parameters", "/own/0", CheckComplete)
	targetContractFinding(t, s.v.result, CodeParameter, "/defaults/0/name", "not declared")
	targetContractFinding(t, s.v.result, CodeParameter, "/own/3/name", "not declared")
	targetContractFinding(t, s.v.result, CodeParameter, "/own/4/name", "not declared")
	for _, d := range s.v.result.Diagnostics {
		if d.Pointer == "/defaults/1/name" || d.Pointer == "/own/0/name" {
			t.Fatalf("declared inherited header rejected: %+v", d)
		}
	}
	s, source = targetContractSession(t, "openapi", operationPresence{}, "")
	s.parameters(nil, "", []any{map[string]any{"name": "id", "in": "query"}}, "/own", &linkedTarget{source: source})
	targetContractCheck(t, s.v.result, "operation-parameters", "/own/0", CheckIncomplete)
	for _, kind := range []string{"asyncapi", "arazzo"} {
		s, source = targetContractSession(t, kind, targetContractOpaque(kind), "")
		target := &linkedTarget{source: source, kind: kind}
		check := "asyncapi-parameters"
		if kind == "arazzo" {
			target.kind = "workflow"
			check = "workflow-inputs"
		}
		s.parameters(nil, "", []any{map[string]any{"name": "id", "value": "literal"}}, "/own", target)
		targetContractCheck(t, s.v.result, check, "/own", CheckIncomplete)
	}
	for _, kind := range []string{"openapi", "arazzo"} {
		s, source = targetContractSession(t, kind, targetContractOpaque(kind), "")
		target := &linkedTarget{source: source, kind: kind, node: map[string]any{"inputs": map[string]any{"type": "object", "additionalProperties": false}}}
		if kind == "arazzo" {
			target.kind = "workflow"
		}
		s.v.budget.used = s.v.opts.limits.MaxNodes
		if kind == "arazzo" {
			s.v.budget.used -= 2
		} // Allow list resolution and merging; stop before checking the resulting parameter.
		s.parameters(nil, "", []any{map[string]any{"name": "id", "in": "query", "value": "literal"}}, "/own", target)
		var limit *Error
		if !errors.As(s.v.err, &limit) || limit.Kind != ErrorLimit {
			t.Fatalf("%s parameter work limit: %v", kind, s.v.err)
		}
	}
}

func TestTargetContractComponentActionTargets(t *testing.T) {
	for _, collection := range []string{"successActions", "failureActions"} {
		s, source := targetContractSession(t, "arazzo", nil, expressionLinkedSource)
		s.v.root["components"] = map[string]any{collection: map[string]any{"remote": map[string]any{"workflowId": "$sourceDescriptions.api.remote", "parameters": []any{map[string]any{"name": "missing", "value": "literal"}}}}}
		s.actions([]any{map[string]any{"reference": "$components." + collection + ".remote"}}, "/actions")
		targetContractFinding(t, s.v.result, CodeParameter, "/actions/0/parameters/0/name", "excluded")
		if len(source.workflows) != 1 {
			t.Fatal("action lookup changed caller metadata")
		}
	}
	s, _ := targetContractSession(t, "arazzo", workflowPresence{}, "")
	s.actions([]any{map[string]any{"workflowId": "$sourceDescriptions.api.absent"}}, "/actions")
	targetContractFinding(t, s.v.result, CodeReference, "/actions/0/workflowId", "does not exist")
}

func TestTargetContractRequestExpressionContexts(t *testing.T) {
	for _, test := range []struct {
		kind               expression.ExpressionType
		location, property string
	}{
		{expression.RequestHeader, "header", "x-id"},
		{expression.RequestQuery, "query", "q"},
		{expression.RequestPath, "path", "id"},
	} {
		s, source := targetContractSession(t, "openapi", nil, sourceOpenAPI)
		target := source.operations["read"][0]
		target.parameters[linkedParameterIdentity(test.property, test.location)] = true
		step := &semanticStep{id: "first", value: map[string]any{"operationId": "read"}, path: "/steps/0"}
		s.v.local.workflows["run"] = &semanticWorkflow{id: "run", steps: map[string]*semanticStep{"first": step}, order: []*semanticStep{step}}
		uses := []expressionUse{
			{Expression: expression.Expression{Type: test.kind, Property: test.property}, WorkflowID: "run", Path: "/parameters/0/value"},
			{Expression: expression.Expression{Type: test.kind, Property: "missing"}, WorkflowID: "run", StepID: "first", Path: "/steps/0/parameters/0/value"},
		}
		s.requestExpressions(uses)
		targetContractCheck(t, s.v.result, "request-parameter-context", "/parameters/0/value", CheckComplete)
		targetContractFinding(t, s.v.result, CodeReference, "/steps/0/parameters/0/value", "not declared")
		if s.targets[dependencyID{document: s.v.documentID(), workflow: "run", step: "first"}] != target {
			t.Fatal("operation metadata not cached")
		}
	}
	s, source := targetContractSession(t, "openapi", operationPresence{}, "")
	step := &semanticStep{id: "first", value: map[string]any{"operationId": "read"}, path: "/steps/0"}
	s.v.local.workflows["run"] = &semanticWorkflow{id: "run", steps: map[string]*semanticStep{"first": step}, order: []*semanticStep{step}}
	s.requestExpressions([]expressionUse{
		{Expression: expression.Expression{Type: expression.RequestQuery}, WorkflowID: "missing", Path: "/missing"},
		{Expression: expression.Expression{Type: expression.RequestQuery}, WorkflowID: "run", StepID: "missing", Path: "/missing-step"},
		{Expression: expression.Expression{Type: expression.RequestQuery}, WorkflowID: "run", Path: "/outputs/id"},
		{Expression: expression.Expression{Type: expression.RequestQuery}, WorkflowID: "run", StepID: "first", Path: "/value"},
	})
	targetContractCheck(t, s.v.result, "request-parameter-context", "/outputs/id", CheckIncomplete)
	targetContractCheck(t, s.v.result, "request-parameter-context", "/value", CheckIncomplete)
	if !s.v.result.Valid() || source.root != nil {
		t.Fatal("incomplete request context became invalid")
	}
	s.targets[dependencyID{document: s.v.documentID(), workflow: "run", step: "first"}] = nil
	before := len(s.v.result.Checks)
	s.requestExpressions([]expressionUse{{Expression: expression.Expression{Type: expression.RequestQuery}, WorkflowID: "run", StepID: "first", Path: "/unresolved"}})
	if len(s.v.result.Checks) != before {
		t.Fatal("failed target treated as declared operation")
	}
}
