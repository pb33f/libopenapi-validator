// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pb33f/go-yaml"
	upstream "github.com/pb33f/libopenapi/arazzo"
	highArazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

func TestSourcesRootlessArazzoPrerequisiteCoverage(t *testing.T) {
	output := func() *orderedmap.Map[string, *highArazzo.OutputValue] {
		result := orderedmap.New[string, *highArazzo.OutputValue]()
		result.Set("id", highArazzo.NewExpressionOutputValue("$response.body#/id"))
		return result
	}
	value := func(expression string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: expression}
	}
	model := &highArazzo.Arazzo{Arazzo: "1.1.0", Workflows: []*highArazzo.Workflow{{WorkflowId: "remote", Steps: []*highArazzo.Step{
		{StepId: "first", OperationId: "read", Outputs: output(), Parameters: []*highArazzo.Parameter{{Name: "id", In: "query", Value: value("$steps.second.outputs.id")}}},
		{StepId: "second", OperationId: "read", Outputs: output(), Parameters: []*highArazzo.Parameter{{Name: "id", In: "query", Value: value("$steps.first.outputs.id")}}},
	}}}}
	input := strings.Replace(sourceTestDocument("        workflowId: $sourceDescriptions.api.remote\n"), "type: openapi", "type: arazzo", 1)
	result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", ArazzoDocument: model}))
	if err != nil || !result.Valid() || result.Complete {
		t.Fatalf("partial model falsely completed prerequisites: %+v %v", result, err)
	}
	for _, check := range result.Checks {
		if check.Name == "external-prerequisites" && check.Status == CheckIncomplete && check.URI == "https://example.test/main.yaml" && check.Pointer == "/workflows/0/steps/0/workflowId" && check.Source == "https://example.test/api.yaml" {
			return
		}
	}
	t.Fatalf("partial source coverage lacked authored coordinates: %+v", result.Checks)
}

func TestSourcesCrossDocumentImplicitCycle(t *testing.T) {
	main := `arazzo: 1.1.0
info: {title: Main, version: '1'}
sourceDescriptions: [{name: flows, url: flows.yaml, type: arazzo}]
workflows:
- workflowId: main
  steps:
  - stepId: first
    workflowId: $sourceDescriptions.flows.remote
    parameters: [{name: id, value: $sourceDescriptions.flows.remote.outputs.id}]
    outputs: {id: $response.body#/id}
  outputs: {id: $steps.first.outputs.id}
`
	external := `arazzo: 1.1.0
info: {title: Other, version: '1'}
sourceDescriptions: [{name: main, url: main.yaml, type: arazzo}]
workflows:
- workflowId: remote
  steps:
  - stepId: first
    workflowId: $sourceDescriptions.main.main
    parameters: [{name: id, value: $sourceDescriptions.main.main.outputs.id}]
    outputs: {id: $response.body#/id}
  outputs: {id: $steps.first.outputs.id}
`
	for _, cycle := range []bool{false, true} {
		source := external
		if !cycle {
			source = strings.Replace(source, "$sourceDescriptions.main.main.outputs.id", "literal", 1)
		}
		result, err := ValidateBytes(context.Background(), []byte(main), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/flows.yaml", SourceBytes: []byte(source)}))
		if err != nil || !result.Complete {
			t.Fatalf("cross-document check did not finish: %+v %v", result, err)
		}
		if !cycle {
			if !result.Valid() {
				t.Fatalf("import cycle became a prerequisite cycle: %+v", result.Diagnostics)
			}
			continue
		}
		diagnostic := foundationFinding(t, result, CodeDependency, "/workflows/0/steps/0/parameters/0/value")
		if diagnostic.URI != "https://example.test/flows.yaml" || diagnostic.Line != 9 || diagnostic.Column != 36 {
			t.Fatalf("external cycle attribution lost: %+v", diagnostic)
		}
	}
}

func TestSourcesDottedStepDependencies(t *testing.T) {
	input := `arazzo: 1.1.0
info: {title: Main, version: '1'}
sourceDescriptions: [{name: other.name, url: other.yaml, type: arazzo}]
workflows:
- workflowId: main
  steps:
  - {stepId: first, workflowId: main, dependsOn: ['$sourceDescriptions.other.name.target.steps.first']}
`
	external := strings.Replace(foundationYAML("1.1.0"), "workflowId: run", "workflowId: target", 1)
	external = strings.Replace(external, "stepId: read", "stepId: first", 1)
	for _, step := range []string{"first", "missing"} {
		data := strings.Replace(input, "target.steps.first", "target.steps."+step, 1)
		result, err := ValidateBytes(context.Background(), []byte(data), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/other.yaml", SourceBytes: []byte(external)}))
		if err != nil || !result.Complete {
			t.Fatalf("dotted source dependency did not finish: %+v %v", result, err)
		}
		if step == "first" {
			if !result.Valid() {
				t.Fatalf("known dotted source rejected: %+v", result.Diagnostics)
			}
		} else {
			foundationFinding(t, result, CodeReference, "/workflows/0/steps/0/dependsOn/0")
		}
	}
}

func TestSourcesRootlessAdditionalOperationCaps(t *testing.T) {
	for _, populated := range []bool{false, true} {
		additional := orderedmap.New[string, *v3.Operation]()
		for i := range 1000 {
			var operation *v3.Operation
			if populated {
				operation = &v3.Operation{OperationId: fmt.Sprintf("read%d", i)}
			}
			additional.Set(fmt.Sprintf("METHOD/%d", i), operation)
		}
		paths := orderedmap.New[string, *v3.PathItem]()
		paths.Set("/items", &v3.PathItem{Get: &v3.Operation{OperationId: "read"}, AdditionalOperations: additional})
		model := &v3.Document{Version: "3.2.0", Paths: &v3.Paths{PathItems: paths}}
		limits := DefaultLimits()
		limits.MaxNodes, limits.MaxBytes = 100, 1000
		result, err := ValidateBytes(context.Background(), []byte(foundationYAML("1.1.0")), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.test/api.yaml", OpenAPIDocument: model}), WithLimits(limits))
		var tool *Error
		if !errors.As(err, &tool) || tool.Kind != ErrorLimit || result.Complete {
			t.Fatalf("rootless additional operations escaped caps: %+v %v", result, err)
		}
		if additional.Len() != 1000 {
			t.Fatal("caller operation map changed")
		}
	}
}

func TestSourcesCriterionHeaderTokenIdentity(t *testing.T) {
	for _, version := range []string{"1.0.1", "1.1.0"} {
		for _, name := range []string{"X&&Id", "X||true", "X!"} {
			for _, declared := range []bool{false, true} {
				api := strings.Replace(sourceOpenAPI, "X-ID", name, 1)
				used := name
				if !declared {
					used += "missing"
				}
				input := sourceTestDocument("        operationId: read\n        successCriteria:\n          - condition: \"$request.header." + used + "=='x'\"\n")
				input = strings.Replace(input, "1.1.0", version, 1)
				result, err := ValidateBytes(context.Background(), []byte(input), "https://example.test/main.yaml", WithSources(sourceCandidate(api)))
				if err != nil || !result.Complete {
					t.Fatalf("header token check did not finish: %+v %v", result, err)
				}
				if declared {
					if !result.Valid() {
						t.Fatalf("declared full RFC token was split: %+v", result.Diagnostics)
					}
				} else {
					foundationFinding(t, result, CodeReference, "/workflows/0/steps/0/successCriteria/0/condition")
				}
			}
		}
	}
}
