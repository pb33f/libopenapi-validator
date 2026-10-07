// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"strings"
	"testing"

	upstream "github.com/pb33f/libopenapi/arazzo"
)

func TestReviewAllFailuresReturnIncompleteResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, call := range []func() (*Result, error){
		func() (*Result, error) { return ValidateBytes(context.Background(), []byte("["), "input.yaml") },
		func() (*Result, error) {
			return ValidateBytes(context.Background(), []byte("---\n{}\n---\n{}"), "input.yaml")
		},
		func() (*Result, error) {
			return ValidateBytes(context.Background(), []byte(foundationYAML("2.0.0")), "input.yaml")
		},
		func() (*Result, error) { return ValidateBytes(ctx, []byte(foundationYAML("1.1.0")), "input.yaml") },
		func() (*Result, error) { return ValidateBytes(context.Background(), nil, "input.yaml", nil) },
		func() (*Result, error) { return Validate(context.Background(), Document{}) },
		func() (*Result, error) { return Validate(context.Background(), Document{}, nil) },
	} {
		r, err := call()
		var typed *Error
		if r == nil || r.Complete || !errors.As(err, &typed) {
			t.Fatalf("mixed failure contract: %+v %v", r, err)
		}
	}
}

func TestReviewTargetUnionReportsOnlyCause(t *testing.T) {
	for _, target := range []string{"operationId: read, workflowId: run", "operationId: read, channelPath: '{$sourceDescriptions.api.url}#/channels/events'", ""} {
		data := strings.Replace(foundationYAML("1.1.0"), "operationId: read", target, 1)
		r, err := ValidateBytes(context.Background(), []byte(data), "target.yaml")
		if err != nil || r.Valid() || len(r.Diagnostics) != 1 {
			t.Fatalf("target fallout: %+v %v", r, err)
		}
		foundationFinding(t, r, CodeStructure, "/workflows/0/steps/0")
	}
	for _, target := range []string{"operationId: read, workflowId: run", ""} {
		data := strings.Replace(foundationYAML("1.1.0"), "operationId: read", strings.TrimPrefix(target+", parameters: [{name: id, value: 1}]", ", "), 1)
		r, err := ValidateBytes(context.Background(), []byte(data), "parameters.yaml")
		if err != nil || len(r.Diagnostics) != 1 {
			t.Fatalf("target-dependent parameter fallout: %+v %v", r, err)
		}
		foundationFinding(t, r, CodeStructure, "/workflows/0/steps/0")
	}
	data := strings.Replace(foundationYAML("1.1.0"), "operationId: read", "operationId: read, workflowId: run", 1)
	data = strings.Replace(data, "title: Example", "title: 123", 1)
	r, err := ValidateBytes(context.Background(), []byte(data), "target.yaml")
	if err != nil || len(r.Diagnostics) != 2 {
		t.Fatalf("unrelated errors hidden: %+v %v", r, err)
	}
	foundationFinding(t, r, CodeStructure, "/info/title")
	for _, version := range []string{"1.0.0", "1.1.0"} {
		for _, defect := range []struct{ from, to, pointer string }{
			{"stepId: read", "stepId: 123", "/workflows/0/steps/0/stepId"},
			{"stepId: read, ", "", "/workflows/0/steps/0/stepId"},
			{"operationId: read", "operationId: read, parameters: 123", "/workflows/0/steps/0/parameters"},
			{"operationId: read", "operationId: read, surprise: 123", "/workflows/0/steps/0/surprise"},
		} {
			fixture := strings.Replace(foundationYAML(version), "operationId: read", "operationId: read, workflowId: run", 1)
			fixture = strings.Replace(fixture, defect.from, defect.to, 1)
			limits := DefaultLimits()
			limits.MaxDiagnostics = 2
			r, err := ValidateBytes(context.Background(), []byte(fixture), "target.yaml", WithLimits(limits))
			if err != nil || len(r.Diagnostics) != 2 {
				t.Fatalf("same-step independent errors lost: %+v %v", r, err)
			}
			foundationFinding(t, r, CodeStructure, defect.pointer)
		}
	}
}

func TestReviewInlineExpressionsRecordedOnce(t *testing.T) {
	for _, version := range []string{"1.0", "1.1"} {
		v := semanticUnit(t, `{"workflows":[{"workflowId":"run","inputs":{"type":"object"},"steps":[{"stepId":"read","operationId":"read","parameters":[{"name":"id","in":"query","value":"$inputs.foo"}],"onSuccess":[{"name":"done","type":"end","criteria":[{"condition":"$statusCode == 200"}]}]}]}]}`, version)
		for _, path := range []string{"/workflows/0/steps/0/parameters/0/value", "/workflows/0/steps/0/onSuccess/0/criteria/0/condition"} {
			count := 0
			for _, use := range v.expressionUses {
				if use.Path == path {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("%s recorded %d times in %s", path, count, version)
			}
		}
		l := DefaultLimits()
		l.MaxDiagnostics = 1
		data := strings.Replace(foundationYAML(version+".0"), "operationId: read", "operationId: read, parameters: [{name: id, in: query, value: '$inputs.foo'}]", 1)
		data = strings.Replace(data, "    steps:", "    inputs: {type: object, additionalProperties: false}\n    steps:", 1)
		r, err := ValidateBytes(context.Background(), []byte(data), "once.yaml", WithLimits(l))
		if err != nil || len(r.Diagnostics) != 1 {
			t.Fatalf("duplicate finding consumed diagnostics budget: %+v %v", r, err)
		}
		foundationFinding(t, r, CodeReference, "/workflows/0/steps/0/parameters/0/value")
	}
	for _, target := range []string{"parameter", "action parameter"} {
		data := strings.Replace(foundationYAML("1.1.0"), "    steps:", "    inputs: {type: object, properties: {id: {type: string}}, additionalProperties: false}\n    steps:", 1)
		value := "parameters: [{reference: '$components.parameters.id', value: '$inputs.foo'}]"
		if target == "action parameter" {
			value = "onSuccess: [{name: next, type: goto, workflowId: run, " + value + "}]"
		}
		data = strings.Replace(data, "operationId: read}", "operationId: read, "+value+"}", 1)
		data += "components:\n  parameters:\n    id: {name: id, in: query, value: literal}\n"
		if target == "action parameter" {
			data = strings.Replace(data, "name: id, in: query", "name: id", 1)
		}
		limits := DefaultLimits()
		limits.MaxDiagnostics = 1
		r, err := ValidateBytes(context.Background(), []byte(data), "override.yaml", WithLimits(limits))
		if err != nil || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != CodeReference {
			t.Fatalf("%s override double charged: %+v %v", target, r, err)
		}
	}
}

func TestReviewReferencedAnnotationMustBeValidSchema(t *testing.T) {
	r := inputValidate(t, "      $ref: '#/workflows/0/inputs/default'\n      default: {type: unknown}\n", "")
	if r.Valid() {
		t.Fatal("referenced malformed schema accepted")
	}
	foundationFinding(t, r, CodeInputSchema, "/workflows/0/inputs/$ref")
}

func TestReviewReusableActionParameterUsesConsumingInputs(t *testing.T) {
	for _, member := range []string{"id", "absent"} {
		data := strings.Replace(foundationYAML("1.1.0"), "    steps:", "    inputs: {type: object, properties: {id: {type: string}}, additionalProperties: false}\n    steps:", 1)
		data = strings.Replace(data, "operationId: read}", "operationId: read, onSuccess: [{name: call, type: goto, workflowId: target, parameters: [{reference: '$components.parameters.arg'}]}]}", 1)
		data += "  - workflowId: target\n    inputs: {type: object, properties: {id: {type: string}}, additionalProperties: false}\n    steps: [{stepId: read, operationId: read}]\ncomponents:\n  parameters:\n    arg: {name: id, value: '$inputs." + member + "'}\n"
		r, err := ValidateBytes(context.Background(), []byte(data), "actions.yaml")
		if err != nil || r.Valid() != (member == "id") {
			t.Fatalf("reusable action context: %+v %v", r, err)
		}
		if member == "absent" {
			foundationFinding(t, r, CodeReference, "/workflows/0/steps/0/onSuccess/0/parameters/0/value")
		}
	}
}

func TestReviewExternalInputSchemas(t *testing.T) {
	main := strings.Replace(sourceTestDocument("        workflowId: $sourceDescriptions.api.remote\n"), "type: openapi", "type: arazzo", 1)
	for _, kind := range []string{"string", "invalid"} {
		external := "arazzo: 1.1.0\ninfo: {title: Other, version: '1'}\nsourceDescriptions: []\nworkflows:\n- workflowId: remote\n  inputs: {type: " + kind + "}\n  steps: [{stepId: read, operationId: read}]\n"
		r, err := ValidateBytes(context.Background(), []byte(main), "https://example.test/main.yaml", WithSources(sourceCandidate(external)))
		if err != nil || r.Valid() != (kind == "string") {
			t.Fatalf("external inputs: %+v %v", r, err)
		}
		if kind == "invalid" {
			d := foundationFinding(t, r, CodeInputSchema, "/workflows/0/inputs")
			if d.URI != "https://example.test/api.yaml" || d.Line != 6 {
				t.Fatalf("foreign schema coordinates: %+v", d)
			}
		}
	}
}

func TestReviewSiblingSourceScopesShareLimits(t *testing.T) {
	main := "arazzo: 1.1.0\ninfo: {title: Main, version: '1'}\nsourceDescriptions:\n- {name: left, url: left/flows.yaml, type: arazzo}\n- {name: right, url: right/flows.yaml, type: arazzo}\nworkflows:\n- workflowId: main\n  dependsOn: ['$sourceDescriptions.left.run', '$sourceDescriptions.right.run']\n  steps: [{stepId: call, workflowId: '$sourceDescriptions.left.run'}]\n"
	parent := "arazzo: 1.1.0\ninfo: {title: Parent, version: '1'}\nsourceDescriptions: [{name: child, url: child.yaml, type: arazzo}]\nworkflows:\n- workflowId: run\n  dependsOn: ['$sourceDescriptions.child.leaf']\n  steps: [{stepId: read, operationId: read}]\n"
	leaf := "arazzo: 1.1.0\ninfo: {title: Leaf, version: '1'}\nsourceDescriptions: []\nworkflows: [{workflowId: leaf, steps: [{stepId: read, operationId: read}]}]\n"
	for _, maximum := range []int{4, 3} {
		calls := map[string]int{}
		resolver := sourceResolverFunc(func(_ context.Context, req upstream.SourceRequest) (*upstream.ResolvedSource, error) {
			calls[req.URL]++
			return &upstream.ResolvedSource{RetrievalURI: req.URL, SourceBytes: []byte(leaf)}, nil
		})
		limits := DefaultLimits()
		limits.MaxSources = maximum
		r, err := ValidateBytes(context.Background(), []byte(main), "https://example.test/main.yaml", WithSources(
			upstream.CandidateDocument{RetrievalURI: "https://example.test/left/flows.yaml", SourceBytes: []byte(parent)},
			upstream.CandidateDocument{RetrievalURI: "https://example.test/right/flows.yaml", SourceBytes: []byte(parent)}), WithResolver(resolver), WithLimits(limits))
		if maximum == 4 {
			if err != nil || !r.Valid() || !r.Complete || calls["https://example.test/left/child.yaml"] != 1 || calls["https://example.test/right/child.yaml"] != 1 {
				t.Fatalf("sibling scopes collided: %+v %v %+v", r, err, calls)
			}
		} else {
			var typed *Error
			if !errors.As(err, &typed) || typed.Kind != ErrorLimit || r.Complete {
				t.Fatalf("sibling limits not shared: %+v %v", r, err)
			}
		}
	}
}
