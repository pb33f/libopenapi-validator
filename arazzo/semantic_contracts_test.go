// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pb33f/jsonschema/v6"
)

func contractValidation(root map[string]any) *validation {
	return &validation{ctx: context.Background(), doc: Document{URI: "https://example.com/workflow.yaml"}, opts: options{limits: DefaultLimits()}, root: root, nodes: map[string]nodeLocation{}, result: &Result{}, version: "1.1"}
}

func TestContractInputSchemaDiagnostics(t *testing.T) {
	v := contractValidation(map[string]any{"workflows": []any{map[string]any{"inputs": map[string]any{"properties": map[string]any{"count": map[string]any{"type": "unknown"}}}}}})
	checkInputs(v)
	if v.err != nil || !semanticHas(v, CodeInputSchema, "/workflows/0/inputs/properties/count/type") {
		t.Fatalf("schema leaf location lost: %+v %v", v.result.Diagnostics, v.err)
	}
	c := jsonschema.NewCompiler()
	meta, err := c.Compile("https://json-schema.org/draft/2020-12/schema")
	if err != nil {
		t.Fatal(err)
	}
	err = meta.Validate(map[string]any{"type": "unknown"})
	if err == nil {
		t.Fatal("invalid schema accepted")
	}
	v = contractValidation(nil)
	reportInputCompile(v, &inputRegexpState{v: v}, &jsonschema.SchemaValidationError{Err: err}, "/inputs")
	if !semanticHas(v, CodeInputSchema, "/type") {
		t.Fatalf("nested compiler diagnostic lost: %+v", v.result.Diagnostics)
	}
	sentinel := errors.New("regex failure")
	v = contractValidation(nil)
	reportInputCompile(v, &inputRegexpState{v: v, err: sentinel}, errors.New("compile failure"), "/inputs")
	var tool *Error
	if !errors.As(v.err, &tool) || tool.Kind != ErrorOperational || !errors.Is(v.err, sentinel) || tool.Location.Pointer != "/inputs" {
		t.Fatalf("regex failure lost: %v", v.err)
	}
	v = contractValidation(nil)
	v.err = sentinel
	reportInputCompile(v, &inputRegexpState{v: v}, errors.New("later failure"), "/inputs")
	if !errors.Is(v.err, sentinel) || len(v.result.Diagnostics) != 0 {
		t.Fatalf("first failure replaced: %v %+v", v.err, v.result.Diagnostics)
	}
}

func TestContractInputResourceAndPatternContracts(t *testing.T) {
	unavailable := &inputResourceUnavailable{uri: "https://example.com/missing"}
	if got := unavailable.Error(); got != "input schema resource is not supplied: https://example.com/missing" {
		t.Fatal(got)
	}
	v := contractValidation(nil)
	state := &inputRegexpState{v: v}
	v.opts.limits.MaxPatternBytes = 3
	if _, err := state.compile("abcd"); err == nil {
		t.Fatal("oversize pattern compiled")
	}
	var tool *Error
	if !errors.As(v.err, &tool) || tool.Kind != ErrorLimit {
		t.Fatalf("limit classification: %v", v.err)
	}
	v = contractValidation(nil)
	state = &inputRegexpState{v: v}
	if _, err := state.compile("["); err == nil {
		t.Fatal("bad ECMAScript pattern compiled")
	}
	re, err := state.compile("^abc$")
	if err != nil {
		t.Fatal(err)
	}
	if re.String() != "^abc$" || !re.MatchString("abc") || re.MatchString("abcd") {
		t.Fatalf("pattern contract broken: %s", re.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.ctx = ctx
	cancel()
	if re.MatchString("abc") || !errors.Is(state.err, context.Canceled) {
		t.Fatalf("canceled regex returned match: %v", state.err)
	}
	limits := DefaultLimits()
	limits.MaxPatternBytes = 3
	doc := strings.Replace(expressionFixture, "    steps:\n", "    inputs:\n      patternProperties:\n        abcd: {type: string}\n    steps:\n", 1)
	result, err := ValidateBytes(context.Background(), []byte(doc), "schema.yaml", WithLimits(limits))
	if !errors.As(err, &tool) || tool.Kind != ErrorLimit || tool.Location.Pointer != "/workflows/0/inputs/patternProperties/abcd" || result.Complete {
		t.Fatalf("property-pattern limit lost: %+v %v", result, err)
	}
}

func TestContractInputSchemaResourceBases(t *testing.T) {
	for _, tc := range []struct{ uri, base, self, version, want string }{
		{"workflow.yaml", "https://example.com/root/", "", "1.0", "https://example.com/root/workflow.yaml"},
		{"https://example.com/workflow.yaml#old", "", "schemas/self.json", "1.1", "https://example.com/schemas/self.json"},
		{"local.yaml", "", "", "1.0", "https://arazzo.invalid/document"},
		{"%", "", "", "1.0", "https://arazzo.invalid/document"},
	} {
		v := contractValidation(map[string]any{"$self": tc.self})
		v.doc.URI = tc.uri
		v.opts.baseURI = tc.base
		v.version = tc.version
		if got := inputSchemaBase(v); got != tc.want {
			t.Fatalf("base=%q want %q", got, tc.want)
		}
	}
	if got := inputReferenceLocation("%", "https://example.com/base", inputSchemaIndex{}); got != "%" {
		t.Fatalf("malformed URI rewritten: %q", got)
	}
	for _, tc := range []struct {
		schema any
		want   string
	}{
		{map[string]any{"oneOf": []any{map[string]any{"type": "string"}, map[string]any{"$ref": "#absent"}}}, "/inputs/oneOf/1/$ref"},
		{map[string]any{"items": map[string]any{"not": map[string]any{"$ref": "#absent"}}}, "/inputs/items/not/$ref"},
		{map[string]any{"default": map[string]any{"$ref": "#data"}}, "/inputs"},
	} {
		if got := schemaReferencePath(tc.schema, "/inputs"); got != tc.want {
			t.Fatalf("schema reference path=%q want %q", got, tc.want)
		}
	}
}

func TestContractSemanticTargetsAndContext(t *testing.T) {
	v := semanticRun(t, `{"sourceDescriptions":[{"name":"api"},{"name":"api"}],"workflows":[{"workflowId":"run","steps":[{"stepId":"first","workflowId":"missing","parameters":[{"reference":"$components.parameters.absent"}]}]},{"workflowId":"run","steps":[]}]}`, "1.1")
	for _, tc := range []struct {
		code Code
		path string
	}{{CodeDuplicateID, "/sourceDescriptions/1/name"}, {CodeDuplicateID, "/workflows/1/workflowId"}, {CodeReference, "/workflows/0/steps/0/workflowId"}, {CodeReference, "/workflows/0/steps/0/parameters/0/reference"}} {
		if !semanticHas(v, tc.code, tc.path) {
			t.Fatalf("missing %s %s: %+v", tc.code, tc.path, v.result.Diagnostics)
		}
	}
	v = semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"read","parameters":[{"name":"raw","in":"querystring","value":42},{"name":"id","in":"query","value":1}],"onSuccess":[{"name":"bad","type":"goto","stepId":"first","workflowId":"run","parameters":[{"name":"id","in":"query","value":1}]},{"name":"no-target","type":"end","parameters":[]}]}]}]}`, "1.1")
	for _, tc := range []struct {
		code Code
		path string
	}{{CodeParameter, "/workflows/0/steps/0/parameters/0/value"}, {CodeParameter, "/workflows/0/steps/0/parameters/0/in"}, {CodeAction, "/workflows/0/steps/0/onSuccess/0"}, {CodeParameter, "/workflows/0/steps/0/onSuccess/0/parameters/0/in"}, {CodeAction, "/workflows/0/steps/0/onSuccess/1/parameters"}} {
		if !semanticHas(v, tc.code, tc.path) {
			t.Fatalf("missing %s %s: %+v", tc.code, tc.path, v.result.Diagnostics)
		}
	}
	for _, ref := range []string{"$workflows.absent.steps.first", "$steps.first", "missing"} {
		v = semanticRun(t, fmt.Sprintf(`{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"read","dependsOn":[%q]}]}]}`, ref), "1.1")
		if !semanticHas(v, CodeDependency, "/workflows/0/steps/0/dependsOn/0") {
			t.Fatalf("bad dependency accepted %q: %+v", ref, v.result.Diagnostics)
		}
	}
}

func TestContractRuntimeSymbolResolution(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"$steps.first.outputs.absent", true},
		{"$workflows.absent.outputs.id", true},
		{"$workflows.target.outputs.absent", true},
		{"$workflows.target.outputs.id", false},
		{"$workflows.target.inputs.absent", true},
		{"$outputs.absent", true},
		{"$inputs.absent", true},
		{"$components.parameters.absent", true},
		{"$components.successActions.absent", true},
		{"$components.failureActions.absent", true},
		{"$sourceDescriptions.absent.workflows.run", true},
	} {
		v := semanticRun(t, fmt.Sprintf(`{"sourceDescriptions":[{"name":"api"}],"workflows":[{"workflowId":"run","inputs":{"type":"object","additionalProperties":false},"steps":[{"stepId":"first","operationId":"read","outputs":{"id":"$statusCode"}},{"stepId":"second","operationId":"read","parameters":[{"name":"id","in":"query","value":%q}]}]},{"workflowId":"target","inputs":{"type":"object","additionalProperties":false},"outputs":{"id":"$steps.producer.outputs.id"},"steps":[{"stepId":"producer","operationId":"read","outputs":{"id":"$statusCode"}}]}]}`, tc.value), "1.1")
		path := "/workflows/0/steps/1/parameters/0/value"
		if got := semanticHas(v, CodeReference, path); got != tc.want {
			t.Fatalf("reference %q finding=%v want %v: %+v", tc.value, got, tc.want, v.result.Diagnostics)
		}
	}
	if !semanticOutputExists(map[string]any{"id": "$statusCode"}, "id.part", "1.0") || semanticOutputExists(map[string]any{"id": "$statusCode"}, "id.part", "1.1") {
		t.Fatal("versioned output contract lost")
	}
	for _, schema := range []map[string]any{{"allOf": []any{}, "additionalProperties": false}, {"additionalProperties": true}, {"type": "object"}} {
		if semanticInputAbsent(schema, "id") {
			t.Fatalf("open/composed schema proves absent input: %+v", schema)
		}
	}
}

func TestContractExpressionLiteralSelectorsAndContainers(t *testing.T) {
	for _, value := range []string{
		`{"context":"$response.body","type":"jsonpath"}`,
		`{"context":"$response.body","selector":"$.item","type":"unknown"}`,
		`{"context":"$response.body","selector":"$.item","type":"jsonpath","literal":true}`,
	} {
		v := semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"read","parameters":[{"name":"data","in":"query","value":`+value+`}]}]}]}`, "1.1")
		if len(v.result.Diagnostics) != 0 {
			t.Fatalf("literal selector rejected: %+v", v.result.Diagnostics)
		}
	}
	v := semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"read","correlationId":"$response.header.X-ID","requestBody":{"payload":["$steps.absent.outputs.id"]},"outputs":{"invalid":{"context":"$not-a-runtime-source","type":"jsonpointer","selector":"/id"}}}]}]}`, "1.1")
	if !semanticHas(v, CodeReference, "/workflows/0/steps/0/requestBody/payload/0") || !semanticHas(v, CodeExpression, "/workflows/0/steps/0/outputs/invalid/context") {
		t.Fatalf("container expressions unchecked: %+v", v.result.Diagnostics)
	}
}

func TestContractCriteriaEmbeddingAndReplacementCapabilities(t *testing.T) {
	for _, tc := range []struct {
		criterion   string
		code        Code
		path, check string
	}{
		{`{"condition":"prefix {$invalid-source}","type":"regex","context":"$response.body"}`, CodeExpression, "/criteria/0/condition", ""},
		{`{"condition":"prefix {$statusCode}","type":"regex","context":"$response.body"}`, "", "", "criterion-syntax"},
		{`{"condition":".*","type":"regex","context":"$invalid-source"}`, CodeExpression, "/criteria/0/context", ""},
		{`{"condition":".*","type":"regex"}`, CodeExpression, "/criteria/0", ""},
		{`{"condition":"$.item","type":{"type":"jsonpath","version":"unsupported"},"context":"$response.body"}`, "", "", "selector-syntax"},
	} {
		// semanticRun parses JSON and checks all applicable criterion scopes.
		v := semanticRun(t, `{"components":{"successActions":{"test":{"name":"test","type":"end","criteria":[`+tc.criterion+`]}}}}`, "1.1")
		path := strings.Replace(tc.path, "/criteria", "/components/successActions/test/criteria", 1)
		if tc.code != "" && !semanticHas(v, tc.code, path) {
			t.Fatalf("criterion finding lost: %+v", v.result.Diagnostics)
		}
		if tc.check != "" {
			found := false
			for _, check := range v.result.Checks {
				found = found || check.Name == tc.check && check.Status == CheckIncomplete
			}
			if !found {
				t.Fatalf("capability gap lost: %+v", v.result.Checks)
			}
		}
	}
	for _, tc := range []struct{ media, want string }{{"application/xml", "selector-syntax"}, {"application/octet-stream", "replacement-target"}} {
		v := semanticRun(t, fmt.Sprintf(`{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"read","requestBody":{"contentType":%q,"replacements":[{"target":"/root/id","value":"literal"}]}}]}]}`, tc.media), "1.1")
		found := false
		for _, check := range v.result.Checks {
			found = found || check.Name == tc.want && check.Status == CheckIncomplete
		}
		if !found {
			t.Fatalf("replacement capability gap lost: %+v", v.result.Checks)
		}
	}
}

func TestContractExpressionGrammarErrors(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"(true", "unclosed condition grouping"},
		{"$request.body..id == 1", "invalid property dereference"},
		{"true[ ]", "index must be a nonnegative integer"},
		{"true[1", "unclosed index"},
		{"(true).", "missing property name"},
		{"true != 'unterminated", "unclosed string literal"},
		{`true == "double"`, "condition strings must use single quotes"},
	} {
		v := contractValidation(nil)
		p := simpleParser{input: tc.input, v: v, path: "/condition"}
		err := p.parse()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("condition %q: %v want %q", tc.input, err, tc.want)
		}
	}
	for _, tc := range []struct {
		input string
		valid bool
	}{{"", true}, {"/id~0/~1", true}, {"id", false}, {string([]byte{0xff}), false}, {"/bad~", false}} {
		if got := validatePointer(tc.input) == nil; got != tc.valid {
			t.Fatalf("pointer %q valid=%v", tc.input, got)
		}
	}
	for _, text := range []string{"prefix {$statusCode", "prefix {$invalid-source}", "{$statusCode}"} {
		v := contractValidation(nil)
		v.checkExpressionString(text, "/value", exprScope{}, false)
		want := text != "{$statusCode}"
		if got := semanticHas(v, CodeExpression, "/value"); got != want {
			t.Fatalf("embedded %q finding=%v: %+v", text, got, v.result.Diagnostics)
		}
	}
}

func TestContractExpressionResourceLimits(t *testing.T) {
	for _, run := range []func(*validation){
		func(v *validation) { _ = v.recordExpression("$statusCode", "/expression", exprScope{}, false) },
		func(v *validation) { v.checkDialectSyntax("regex", "", "abcdef", "/pattern", CodeExpression) },
		func(v *validation) {
			p := simpleParser{input: "true == false", v: v, path: "/condition"}
			_ = p.parse()
		},
	} {
		v := contractValidation(nil)
		v.opts.limits.MaxPatternBytes = 4
		run(v)
		var tool *Error
		if !errors.As(v.err, &tool) || tool.Kind != ErrorLimit {
			t.Fatalf("pattern cap not applied: %v", v.err)
		}
	}
	v := contractValidation(nil)
	v.opts.limits.MaxDepth = 2
	p := simpleParser{input: "(((true)))", v: v, path: "/condition"}
	if err := p.parse(); err == nil || v.err == nil {
		t.Fatal("condition nesting unbounded")
	}
	v = contractValidation(nil)
	v.opts.limits.MaxBytes = 2
	v.checkValue("literal", "/value", exprScope{})
	var tool *Error
	if !errors.As(v.err, &tool) || tool.Kind != ErrorLimit || tool.Location.Pointer != "/value" {
		t.Fatalf("Any string budget lost: %v", v.err)
	}
	v = contractValidation(nil)
	v.opts.limits.MaxBytes = 2
	v.checkCriteria([]any{map[string]any{"condition": "true"}}, "/criteria", exprScope{})
	if !errors.As(v.err, &tool) || tool.Location.Pointer != "/criteria/0/condition" {
		t.Fatalf("criterion budget lost: %v", v.err)
	}
}

func TestContractSemanticForwardOutputAdvisory(t *testing.T) {
	data := strings.Replace(expressionFixture, "        operationId: getPet\n", `        operationId: getPet
        parameters:
          - {name: id, in: query, value: $steps.second.outputs.id}
      - stepId: second
        operationId: getPet
        outputs: {id: $statusCode}
`, 1)
	r, err := ValidateBytes(context.Background(), []byte(data), "forward.yaml", WithAdvisories())
	if err != nil || !r.Valid() {
		t.Fatalf("valid forward reference rejected: %+v %v", r, err)
	}
	found := false
	for _, d := range r.Diagnostics {
		if d.Code == CodeForwardReference {
			found = true
			if d.Severity != SeverityWarning || d.Pointer != "/workflows/0/steps/0/parameters/0/value" {
				t.Fatalf("bad advisory: %+v", d)
			}
		}
	}
	if !found {
		t.Fatalf("sequential ordering advice missing: %+v", r.Diagnostics)
	}
}

func TestContractSemanticResolutionBudgets(t *testing.T) {
	for _, tc := range []struct {
		name  string
		nodes int
		run   func(*validation)
		path  string
	}{
		{"reference", 1, func(v *validation) {
			_, _ = v.resolveReusable(map[string]any{"reference": "$components.parameters.id"}, "/use", "parameters")
		}, "/use/reference"},
		{"clone", 4, func(v *validation) {
			_, _ = v.resolveReusable(map[string]any{"reference": "$components.parameters.id"}, "/use", "parameters")
		}, "/use"},
		{"action", 0, func(v *validation) {
			v.checkAction(effectiveObject{value: map[string]any{"type": "end"}, path: "/action"}, nil)
		}, "/action"},
		{"workflow input", 0, func(v *validation) {
			v.checkWorkflowInputParameters([]effectiveObject{{value: map[string]any{"name": "id"}, path: "/parameter"}}, "run")
		}, "/parameter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := contractValidation(nil)
			v.local = &semanticIndex{components: map[string]map[string]any{"parameters": {"id": map[string]any{"name": "id", "in": "query", "value": true}}}, workflows: map[string]*semanticWorkflow{"run": {value: map[string]any{"inputs": map[string]any{"additionalProperties": false}}}}}
			v.opts.limits.MaxNodes = tc.nodes
			tc.run(v)
			var tool *Error
			if !errors.As(v.err, &tool) || tool.Kind != ErrorLimit || tool.Location.Pointer != tc.path {
				t.Fatalf("budget %s lost: %v", tc.name, v.err)
			}
		})
	}
}

func TestContractPassesStopOnCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*validation)
	}{
		{"semantics", checkSemantics},
		{"expressions", checkExpressions},
		{"input schemas", checkInputs},
		{"input walk", func(v *validation) {
			walkInputSchemas(v, map[string]any{"type": "string"}, "/inputs", func(map[string]any, string) { t.Fatal("canceled schema visited") })
		}},
		{"criteria", func(v *validation) {
			v.checkCriteria([]any{map[string]any{"condition": "true"}}, "/criteria", exprScope{})
		}},
		{"reference resolution", func(v *validation) {
			_, _ = v.resolveReusable(map[string]any{"reference": "$components.parameters.id"}, "/reference", "parameters")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := contractValidation(map[string]any{"workflows": []any{map[string]any{"workflowId": "run", "inputs": map[string]any{"type": "string"}}}})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			v.ctx = ctx
			tc.run(v)
			if !errors.Is(v.err, context.Canceled) || len(v.result.Diagnostics) != 0 {
				t.Fatalf("cancellation changed into findings: %v %+v", v.err, v.result.Diagnostics)
			}
		})
	}
}

func TestContractInputResourceCollision(t *testing.T) {
	r, err := ValidateBytes(context.Background(), []byte(strings.Replace(expressionFixture, "    steps:\n", "    inputs: {type: string}\n    steps:\n", 1)), "https://json-schema.org/draft/2020-12/schema")
	var tool *Error
	if !errors.As(err, &tool) || tool.Kind != ErrorOperational || r.Complete || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("resource collision hidden: %+v %v", r, err)
	}
}

func TestContractInputRegexTimeout(t *testing.T) {
	v := contractValidation(nil)
	state := &inputRegexpState{v: v}
	compiled, err := state.compile("^(a+)+$")
	if err != nil {
		t.Fatal(err)
	}
	re := compiled.(*inputRegexp)
	// Negative MatchTimeout is already expired. This exercises propagation
	// without depending on scheduler speed or waiting for catastrophic backtracking.
	re.re.MatchTimeout = -1
	if re.MatchString(strings.Repeat("a", 64)+"!") || state.err == nil || !strings.Contains(state.err.Error(), "match timeout") {
		t.Fatalf("timeout became mismatch without cause: %v", state.err)
	}
}

func TestContractPassStructuralOwnership(t *testing.T) {
	v := contractValidation(nil)
	v.checkCriteria([]any{map[string]any{"condition": 42}}, "/criteria", exprScope{})
	v.checkDialectSyntax("unknown", "", "literal", "/selector", CodeSelector)
	v.checkRequestBody(map[string]any{"replacements": []any{map[string]any{"target": 42, "value": "literal"}}}, "/requestBody", exprScope{})
	if v.err != nil || len(v.result.Diagnostics) != 0 || len(v.result.Checks) != 0 {
		t.Fatalf("semantic pass duplicated structural failures: %+v %+v %v", v.result.Diagnostics, v.result.Checks, v.err)
	}
	v.checkExpressionString("$response.body.id[0]", "/value", exprScope{}, true)
	if !semanticHas(v, CodeExpression, "/value") {
		t.Fatalf("invalid runtime dereference accepted: %+v", v.result.Diagnostics)
	}
}

func TestContractSemanticComponentScopeAndGraphCancellation(t *testing.T) {
	v := semanticRun(t, `{"components":{"parameters":{"id":{"name":"id","in":"query","value":"$steps.first.outputs.id"}}}}`, "1.1")
	if len(v.result.Diagnostics) != 0 {
		t.Fatalf("unconsumed component acquired workflow scope: %+v", v.result.Diagnostics)
	}
	finalizeExpressionUses(contractValidation(nil)) // A missing semantic index has no symbols to resolve.
	v = contractValidation(nil)
	checkDependencyGraph(v)
	if v.err != nil {
		t.Fatalf("absent dependency graph failed: %v", v.err)
	}
	v.graph = newDependencyGraph()
	v.graph.addNode(dependencyID{document: v.doc.URI, workflow: "run"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v.ctx = ctx
	checkDependencyGraph(v)
	if !errors.Is(v.err, context.Canceled) {
		t.Fatalf("graph cancellation lost: %v", v.err)
	}
	v = contractValidation(map[string]any{"components": map[string]any{"successActions": map[string]any{"done": map[string]any{"name": "done", "type": "end"}}}, "workflows": []any{map[string]any{"workflowId": "run"}}})
	v.opts.limits.MaxNodes = 0
	checkSemantics(v)
	var tool *Error
	if !errors.As(v.err, &tool) || tool.Kind != ErrorLimit || len(v.graph.order) != 0 {
		t.Fatalf("semantics continued after component work cap: %v %+v", v.err, v.graph.order)
	}
}

func TestContractFinalizationCancellation(t *testing.T) {
	v := semanticRun(t, `{"workflows":[{"workflowId":"run","steps":[{"stepId":"first","operationId":"read","outputs":{"id":"$statusCode"}}]}]}`, "1.1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v.ctx = ctx
	finalizeExpressionUses(v)
	if !errors.Is(v.err, context.Canceled) {
		t.Fatalf("canceled symbols finalized: %v", v.err)
	}
	v = contractValidation(nil)
	v.ctx = ctx
	inputSchemaFindings(v, &jsonschema.ValidationError{}, "/inputs")
	if !errors.Is(v.err, context.Canceled) || len(v.result.Diagnostics) != 0 {
		t.Fatalf("canceled schema emitted findings: %v %+v", v.err, v.result.Diagnostics)
	}
	w := &semanticWorkflow{steps: map[string]*semanticStep{}}
	if semanticPreExecution(w, expressionUse{StepID: "absent", Path: "/workflows/0/steps/0/parameters/0/value"}) {
		t.Fatal("unknown step classified as pre-execution")
	}
}

// contractScheduledCancel lets the caller cancel at each observed validation
// checkpoint. It uses the standard cancellation context to retain Err semantics.
type contractScheduledCancel struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *contractScheduledCancel) Err() error {
	if c.remaining > 0 {
		c.remaining--
		if c.remaining == 0 {
			c.cancel()
		}
	}
	return c.Context.Err()
}

func TestContractInputCancellationCheckpoints(t *testing.T) {
	reachedEnd := false
	for checkpoint := 1; checkpoint <= 200; checkpoint++ {
		ctx, cancel := context.WithCancel(context.Background())
		scheduled := &contractScheduledCancel{Context: ctx, cancel: cancel, remaining: checkpoint}
		v := contractValidation(map[string]any{"workflows": []any{
			map[string]any{"inputs": map[string]any{"$anchor": "first", "$ref": "https://example.com/external.json", "type": "object", "properties": map[string]any{"id": map[string]any{"$ref": "#absent"}}}},
			map[string]any{"inputs": map[string]any{"$anchor": "second", "type": "string"}},
		}})
		v.ctx = scheduled
		checkInputs(v)
		wasCanceled := ctx.Err() != nil
		cancel()
		if !wasCanceled {
			reachedEnd = true
			break
		}
		if !errors.Is(v.err, context.Canceled) {
			t.Fatalf("checkpoint %d lost cancellation: %v", checkpoint, v.err)
		}
	}
	if !reachedEnd {
		t.Fatal("input cancellation sweep did not reach normal completion")
	}
}

func TestContractReusableCancellationCheckpoints(t *testing.T) {
	reachedEnd := false
	for checkpoint := 1; checkpoint <= 40; checkpoint++ {
		ctx, cancel := context.WithCancel(context.Background())
		scheduled := &contractScheduledCancel{Context: ctx, cancel: cancel, remaining: checkpoint}
		v := contractValidation(nil)
		v.ctx = scheduled
		v.local = &semanticIndex{components: map[string]map[string]any{"parameters": {
			"outer": map[string]any{"reference": "$components.parameters.inner", "value": false},
			"inner": map[string]any{"name": "id", "in": "query", "value": true},
		}}}
		items := v.resolveList([]any{map[string]any{"reference": "$components.parameters.outer"}}, "/parameters", "parameters")
		wasCanceled := ctx.Err() != nil
		cancel()
		if !wasCanceled {
			reachedEnd = true
			if len(items) != 1 || items[0].value["value"] != false || items[0].definition != "/components/parameters/inner" {
				t.Fatalf("completed reusable resolution lost override: %+v", items)
			}
			break
		}
		if !errors.Is(v.err, context.Canceled) || len(items) != 0 {
			t.Fatalf("checkpoint %d lost cancellation/returned partial item: %v %+v", checkpoint, v.err, items)
		}
	}
	if !reachedEnd {
		t.Fatal("reusable cancellation sweep did not reach normal completion")
	}
}
