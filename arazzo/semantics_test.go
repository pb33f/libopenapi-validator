// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pb33f/go-yaml"
)

func semanticRun(t *testing.T, body string, version string) *validation {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		t.Fatal(err)
	}
	v := &validation{ctx: context.Background(), doc: Document{URI: "memory:local"}, opts: options{limits: DefaultLimits()}, root: root, nodes: map[string]nodeLocation{}, result: &Result{}, version: version}
	checkSemantics(v)
	checkExpressions(v)
	finalizeExpressionUses(v)
	checkDependencyGraph(v)
	if v.err != nil {
		t.Fatal(v.err)
	}
	return v
}

func TestSemanticPublicWorkflowInputParameters(t *testing.T) {
	base := foundationYAML("1.1.0") + "  - workflowId: target\n    inputs: {type: object, properties: {id: {type: string}}, additionalProperties: false}\n    steps: [{stepId: read, operationId: read}]\n"
	for _, action := range []bool{false, true} {
		for _, name := range []string{"id", "bogus"} {
			data := base
			path := "/workflows/0/steps/0/parameters/0/name"
			if action {
				data = strings.Replace(data, "operationId: read}", "operationId: read, onSuccess: [{name: call, type: goto, workflowId: target, parameters: [{name: "+name+", value: true}]}]}", 1)
				path = "/workflows/0/steps/0/onSuccess/0/parameters/0/name"
			} else {
				data = strings.Replace(data, "operationId: read}", "workflowId: target, parameters: [{name: "+name+", value: true}]}", 1)
			}
			r, err := ValidateBytes(context.Background(), []byte(data), "inputs.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if name == "id" {
				if !r.Valid() {
					t.Fatalf("declared input rejected: %+v", r.Diagnostics)
				}
			} else {
				foundationFinding(t, r, CodeParameter, path)
			}
		}
	}
}

func semanticHas(v *validation, code Code, path string) bool {
	for _, d := range v.result.Diagnostics {
		if d.Code == code && d.Pointer == path {
			return true
		}
	}
	return false
}

func TestSemanticInheritedParameterContext(t *testing.T) {
	v := semanticRun(t, `{"workflows":[{"workflowId":"run","parameters":[{"name":"X-ID","in":"header","value":false}],"steps":[{"stepId":"first","operationId":"fetch"}]}]}`, "1.0")
	if len(v.result.Diagnostics) != 0 {
		t.Fatalf("valid inherited header: %+v", v.result.Diagnostics)
	}
	v = semanticRun(t, `{"workflows":[{"workflowId":"run","parameters":[{"name":"id","value":0}],"steps":[{"stepId":"first","operationId":"fetch"}]}]}`, "1.0")
	if !semanticHas(v, CodeParameter, "/workflows/0/parameters/0") {
		t.Fatalf("missing effective location: %+v", v.result.Diagnostics)
	}
}

func TestSemanticReusableOverridesAndDuplicateIdentity(t *testing.T) {
	for _, raw := range []string{"false", "0", "null"} {
		t.Run(raw, func(t *testing.T) {
			v := semanticRun(t, `{"components":{"parameters":{"item":{"name":"id","in":"query","value":"original"}}},"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","parameters":[{"reference":"$components.parameters.item","value":`+raw+`}]}]}]}`, "1.0")
			items := v.resolveList(object(array(object(array(v.root["workflows"])[0])["steps"])[0])["parameters"], "/parameters", "parameters")
			var want any
			if err := json.Unmarshal([]byte(raw), &want); err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 || items[0].value["value"] != want {
				t.Fatalf("override lost: %+v", items)
			}
			if v.local.components["parameters"]["item"].(map[string]any)["value"] != "original" {
				t.Fatal("component mutated")
			}
		})
	}
	v := semanticRun(t, `{"components":{"parameters":{"a":{"name":"id","in":"query","value":1},"b":{"name":"id","in":"query","value":2}}},"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","parameters":[{"reference":"$components.parameters.a"},{"reference":"$components.parameters.b"}]}]}]}`, "1.0")
	if !semanticHas(v, CodeParameter, "/workflows/0/steps/0/parameters/1") {
		t.Fatalf("resolved identity duplicate missed: %+v", v.result.Diagnostics)
	}
}

func TestSemanticActionsUseConsumingWorkflow(t *testing.T) {
	v := semanticRun(t, `{"components":{"successActions":{"move":{"name":"next","type":"goto","stepId":"last"}}},"workflows":[{"workflowId":"one","steps":[{"stepId":"last","operationId":"fetch","onSuccess":[{"reference":"$components.successActions.move"}]}]},{"workflowId":"two","steps":[{"stepId":"first","operationId":"fetch","onSuccess":[{"reference":"$components.successActions.move"}]}]}]}`, "1.0")
	if semanticHas(v, CodeReference, "/workflows/0/steps/0/onSuccess/0/stepId") {
		t.Fatal("valid component target rejected")
	}
	if !semanticHas(v, CodeReference, "/workflows/1/steps/0/onSuccess/0/stepId") {
		t.Fatalf("consuming workflow target missed: %+v", v.result.Diagnostics)
	}
	v = semanticRun(t, `{"components":{"successActions":{"a":{"name":"done","type":"end"},"b":{"name":"done","type":"end"}}},"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","onSuccess":[{"reference":"$components.successActions.a"},{"reference":"$components.successActions.b"}]}]}]}`, "1.0")
	if !semanticHas(v, CodeDuplicateID, "/workflows/0/steps/0/onSuccess/1") {
		t.Fatalf("effective action duplicate missed: %+v", v.result.Diagnostics)
	}
}

func TestSemanticDependencyFieldGrammarAndCycles(t *testing.T) {
	v := semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch"}]},{"workflowId":"other","steps":[{"stepId":"next","operationId":"fetch","dependsOn":["$workflows.run.steps.first"]}]}]}`, "1.1")
	if len(v.result.Diagnostics) != 0 {
		t.Fatalf("legal field-specific dependency rejected: %+v", v.result.Diagnostics)
	}
	v = semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","dependsOn":["second"]},{"stepId":"second","operationId":"fetch","dependsOn":["first"]}]}]}`, "1.1")
	if !semanticHas(v, CodeDependency, "/workflows/0/steps/1/dependsOn/0") {
		t.Fatalf("step cycle missed: %+v", v.result.Diagnostics)
	}
	v = semanticRun(t, `{"workflows":[{"workflowId":"run","dependsOn":["other"],"steps":[{"stepId":"first","operationId":"fetch"}]},{"workflowId":"other","dependsOn":["run"],"steps":[{"stepId":"first","operationId":"fetch"}]}]}`, "1.0")
	if !semanticHas(v, CodeDependency, "/workflows/1/dependsOn/0") {
		t.Fatalf("workflow cycle missed: %+v", v.result.Diagnostics)
	}
}

func TestSemanticImplicitPrerequisitesAndDottedOutputs(t *testing.T) {
	v := semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","parameters":[{"name":"id","in":"query","value":"$steps.second.outputs.id"}],"outputs":{"id":"$statusCode"}},{"stepId":"second","operationId":"fetch","parameters":[{"name":"id","in":"query","value":"$steps.first.outputs.id"}],"outputs":{"id":"$statusCode"}}]}]}`, "1.1")
	if !semanticHas(v, CodeDependency, "/workflows/0/steps/1/parameters/0/value") {
		t.Fatalf("implicit output cycle missed: %+v", v.result.Diagnostics)
	}
	v = semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","outputs":{"id.part":"$statusCode"}},{"stepId":"second","operationId":"fetch","parameters":[{"name":"id","in":"query","value":"$steps.first.outputs.id.part"}]}]}]}`, "1.1")
	if len(v.result.Diagnostics) != 0 {
		t.Fatalf("dotted output name rejected: %+v", v.result.Diagnostics)
	}
}

func TestSemanticCompletedStepOutputInAction(t *testing.T) {
	v := semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","outputs":{"id":"$statusCode"},"onSuccess":[{"name":"done","type":"end","criteria":[{"condition":"$steps.first.outputs.id == 200"}]}]}]}]}`, "1.1")
	if len(v.result.Diagnostics) != 0 {
		t.Fatalf("completed current output falsely became a prerequisite: %+v", v.result.Diagnostics)
	}
}

func TestSemanticPublicDiagnosticLocations(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.1.0"} {
		doc := []byte("arazzo: " + version + "\ninfo: {title: Example, version: '1'}\nsourceDescriptions:\n  - {name: api, url: api.yaml, type: openapi}\nworkflows:\n  - workflowId: run\n    steps:\n      - stepId: read\n        operationId: read\n        outputs:\n          id: $steps.missing.outputs.id\n")
		r, err := ValidateBytes(context.Background(), doc, "memory:document")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range r.Diagnostics {
			if d.Code == CodeReference && d.Pointer == "/workflows/0/steps/0/outputs/id" {
				found = true
				if d.URI != "memory:document" || d.Line != 11 || d.Column != 15 {
					t.Fatalf("source location lost: %+v", d)
				}
			}
		}
		if !found {
			t.Fatalf("missing scoped expression diagnostic: %+v", r.Diagnostics)
		}
	}
}

func TestSemanticPublicIdentityInheritanceAndActions(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.1.0"} {
		base := "arazzo: " + version + "\ninfo: {title: Example, version: '1'}\nsourceDescriptions:\n  - {name: api, url: api.yaml, type: openapi}\n"
		t.Run(version, func(t *testing.T) {
			for _, value := range []string{"false", "0", "null"} {
				doc := base + "components:\n  parameters:\n    token: {name: X-ID, in: header, value: original}\n  successActions:\n    done: {name: done, type: end}\nworkflows:\n  - workflowId: run\n    parameters:\n      - reference: $components.parameters.token\n        value: " + value + "\n    successActions:\n      - reference: $components.successActions.done\n    steps:\n      - {stepId: first, operationId: read}\n"
				r, err := ValidateBytes(context.Background(), []byte(doc), "memory:document")
				if err != nil {
					t.Fatal(err)
				}
				if !r.Valid() {
					t.Fatalf("valid inheritance/value %s rejected: %+v", value, r.Diagnostics)
				}
			}
			doc := base + "workflows:\n  - workflowId: run\n    steps:\n      - {stepId: first, operationId: read}\n      - {stepId: first, operationId: other}\n"
			r, err := ValidateBytes(context.Background(), []byte(doc), "memory:document")
			if err != nil {
				t.Fatal(err)
			}
			d := foundationFinding(t, r, CodeDuplicateID, "/workflows/0/steps/1/stepId")
			if d.Line != 9 || d.Column != 18 {
				t.Fatalf("duplicate step location: %+v", d)
			}
			if len(d.Related) != 1 || d.Related[0].Pointer != "/workflows/0/steps/0/stepId" || d.Related[0].Line != 8 || d.Related[0].Column != 18 {
				t.Fatalf("first duplicate declaration location: %+v", d.Related)
			}
			doc = base + "components:\n  successActions:\n    jump: {name: jump, type: goto, stepId: absent}\nworkflows:\n  - workflowId: run\n    steps:\n      - stepId: first\n        operationId: read\n        onSuccess:\n          - reference: $components.successActions.jump\n"
			r, err = ValidateBytes(context.Background(), []byte(doc), "memory:document")
			if err != nil {
				t.Fatal(err)
			}
			d = foundationFinding(t, r, CodeReference, "/workflows/0/steps/0/onSuccess/0/stepId")
			if d.Line != 14 || d.Column != 13 {
				t.Fatalf("reusable target consuming location: %+v", d)
			}
		})
	}
}

func TestSemanticPublicImplicitCycleAndOverride(t *testing.T) {
	base := "arazzo: 1.1.0\ninfo: {title: Example, version: '1'}\nsourceDescriptions:\n  - {name: api, url: api.yaml, type: openapi}\n"
	doc := base + "workflows:\n  - workflowId: run\n    steps:\n      - stepId: first\n        operationId: read\n        outputs: {id: '$statusCode'}\n        parameters:\n          - {name: id, in: query, value: '$steps.second.outputs.id'}\n      - stepId: second\n        operationId: read\n        outputs: {id: '$statusCode'}\n        parameters:\n          - {name: id, in: query, value: '$steps.first.outputs.id'}\n"
	r, err := ValidateBytes(context.Background(), []byte(doc), "memory:document")
	if err != nil {
		t.Fatal(err)
	}
	d := foundationFinding(t, r, CodeDependency, "/workflows/0/steps/1/parameters/0/value")
	if d.Line != 17 || d.Column != 42 {
		t.Fatalf("implicit cycle location: %+v", d)
	}
	// The step-level value replaces the inherited prerequisite completely.
	doc = base + "workflows:\n  - workflowId: run\n    parameters:\n      - {name: id, in: query, value: '$steps.second.outputs.id'}\n    steps:\n      - stepId: first\n        operationId: read\n        outputs: {id: '$statusCode'}\n        parameters:\n          - {name: id, in: query, value: false}\n      - stepId: second\n        operationId: read\n        outputs: {id: '$statusCode'}\n        parameters:\n          - {name: id, in: query, value: false}\n"
	r, err = ValidateBytes(context.Background(), []byte(doc), "memory:document")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Valid() {
		t.Fatalf("overridden prerequisite remained effective: %+v", r.Diagnostics)
	}
	// Existing control-flow loops remain valid: they are not prerequisites.
	doc = base + "workflows:\n  - workflowId: run\n    steps:\n      - stepId: first\n        operationId: read\n        onSuccess:\n          - {name: again, type: goto, stepId: first}\n"
	r, err = ValidateBytes(context.Background(), []byte(doc), "memory:document")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Valid() {
		t.Fatalf("goto loop rejected: %+v", r.Diagnostics)
	}
}

func TestSemanticReusableCollectionAndCycles(t *testing.T) {
	v := semanticRun(t, `{"components":{"parameters":{"a":{"reference":"$components.parameters.b"},"b":{"reference":"$components.parameters.a"}}},"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","parameters":[{"reference":"$components.parameters.a"}]}]}]}`, "1.0")
	if !semanticHas(v, CodeReference, "/workflows/0/steps/0/parameters/0/reference") {
		t.Fatalf("reusable cycle missed: %+v", v.result.Diagnostics)
	}
	v = semanticRun(t, `{"components":{"parameters":{"a":{"name":"id","value":0}}},"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"fetch","onSuccess":[{"reference":"$components.parameters.a"}]}]}]}`, "1.0")
	if !semanticHas(v, CodeReference, "/workflows/0/steps/0/onSuccess/0/reference") {
		t.Fatalf("wrong collection missed: %+v", v.result.Diagnostics)
	}
}

func TestSemanticInheritedWorkLimit(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.1.0"} {
		for _, field := range []string{"successActions", "parameters"} {
			t.Run(version+"/"+field, func(t *testing.T) {
				defaults := make([]any, 12)
				for i := range defaults {
					if field == "successActions" {
						defaults[i] = map[string]any{"name": fmt.Sprintf("end%d", i), "type": "end"}
					} else {
						defaults[i] = map[string]any{"name": fmt.Sprintf("item%d", i), "in": "query", "value": "fixed"}
					}
				}
				steps := make([]any, 24)
				for i := range steps {
					steps[i] = map[string]any{"stepId": fmt.Sprintf("step%d", i), "operationId": "read"}
				}
				doc := map[string]any{"arazzo": version, "info": map[string]any{"title": "Bounded inheritance", "version": "1"}, "sourceDescriptions": []any{map[string]any{"name": "api", "url": "api.yaml", "type": "openapi"}}, "workflows": []any{map[string]any{"workflowId": "run", field: defaults, "steps": steps}}}
				data, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				baseline, err := ValidateBytes(context.Background(), data, "memory:inherited")
				if err != nil || !baseline.Valid() {
					t.Fatalf("fixture must pass with normal limits: result=%+v err=%v", baseline, err)
				}
				var node yaml.Node
				if err := yaml.Unmarshal(data, &node); err != nil {
					t.Fatal(err)
				}
				_, _, _, stats, err := normalizeWithStats(context.Background(), Document{Root: &node, URI: "memory:inherited"}, DefaultLimits())
				if err != nil {
					t.Fatal(err)
				}
				limits := DefaultLimits()
				limits.MaxNodes = stats.nodes + 64
				result, err := ValidateBytes(context.Background(), data, "memory:inherited", WithLimits(limits))
				var limit *Error
				if !errors.As(err, &limit) || limit.Kind != ErrorLimit || !strings.Contains(limit.Error(), "derived validation work") {
					t.Fatalf("expected derived expansion limit above %d authored nodes: result=%+v err=%v", stats.nodes, result, err)
				}
				if result == nil || result.Complete {
					t.Fatalf("bounded run must return incomplete coverage: %+v", result)
				}
			})
		}
	}
}
