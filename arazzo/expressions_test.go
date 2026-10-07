// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestExpressionInheritedParserByteBudget(t *testing.T) {
	for _, value := range []string{
		"'$response.body#/" + strings.Repeat("id", 200) + "'",
		"{context: $response.body, selector: '$." + strings.Repeat("id", 200) + "', type: jsonpath}",
	} {
		var fixture strings.Builder
		fixture.WriteString("arazzo: 1.1.0\ninfo: {title: Budget, version: '1'}\nsourceDescriptions: [{name: api, url: api.yaml}]\nworkflows:\n- workflowId: run\n  parameters:\n  - name: id\n    in: query\n    value: " + value + "\n  steps:\n")
		for i := range 40 {
			fmt.Fprintf(&fixture, "  - {stepId: s%d, operationId: read}\n", i)
		}
		data := []byte(fixture.String())
		baseline, err := ValidateBytes(context.Background(), data, "budget.yaml")
		if err != nil || !baseline.Valid() {
			t.Fatalf("valid inherited value rejected: %+v %v", baseline, err)
		}
		limits := DefaultLimits()
		limits.MaxBytes = len(data) * 2
		result, err := ValidateBytes(context.Background(), data, "budget.yaml", WithLimits(limits))
		var tool *Error
		if !errors.As(err, &tool) || tool.Kind != ErrorLimit || result.Complete || !strings.Contains(tool.Cause.Error(), "derived validation byte") {
			t.Fatalf("repeated parser input was not bounded: %+v %v", result, err)
		}
	}
}

const expressionFixture = `arazzo: 1.1.0
info:
  title: expressions
  version: '1'
sourceDescriptions:
  - name: api
    url: https://example.com/api.yaml
    type: openapi
workflows:
  - workflowId: run
    steps:
      - stepId: first
        operationId: getPet
`

func expressionValidate(t *testing.T, version, tail string) *Result {
	t.Helper()
	fixture := strings.Replace(expressionFixture, "1.1.0", version, 1) + tail
	result, err := ValidateBytes(context.Background(), []byte(fixture), "https://example.com/workflow.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func expressionFinding(t *testing.T, result *Result, code Code, path string, line int) {
	t.Helper()
	for _, d := range result.Diagnostics {
		if d.Code == code && d.Pointer == path {
			if d.Line != line || d.Column < 1 {
				t.Fatalf("location = %d:%d, want line %d", d.Line, d.Column, line)
			}
			return
		}
	}
	t.Fatalf("missing %s at %s: %+v", code, path, result.Diagnostics)
}

func TestExpressionOutputSyntaxAndVersion(t *testing.T) {
	for _, test := range []struct {
		name, version, value string
		valid                bool
	}{
		{"dynamic body", "1.0.99", "$response.body#/items/0", true},
		{"broad step reference", "1.0.1", "$steps.first", true},
		{"literal", "1.1.0", "definitely-not-an-expression", false},
		{"self in 1.0", "1.0.1", "$self", false},
		{"message in 1.0", "1.0.1", "$message.payload", false},
		{"request payload in 1.0", "1.0.1", "$request.payload", false},
		{"response payload in 1.0", "1.0.1", "$response.payload", false},
		{"body pointer escape", "1.1.0", "$response.body#/bad~2key", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := expressionValidate(t, test.version, "        outputs:\n          result: "+test.value+"\n")
			if test.valid {
				if !r.Valid() {
					t.Fatalf("valid output: %+v", r.Diagnostics)
				}
			} else {
				expressionFinding(t, r, CodeExpression, "/workflows/0/steps/0/outputs/result", 15)
			}
		})
	}
}

func TestCriterionSyntax(t *testing.T) {
	for _, test := range []struct {
		name, criterion string
		valid           bool
		code            Code
		field           string
		line            int
	}{
		{"compound", "          - condition: \"($statusCode == 200 || $statusCode == 201) && !false\"\n", true, "", "", 0},
		{"pointer operator characters", "          - condition: '$response.body#/a==b[0] == 200'\n", true, "", "", 0},
		{"large numeric literal", "          - condition: '1e400 > 1e300'\n", true, "", "", 0},
		{"invalid numeric token", "          - condition: '1abc > 2'\n", false, CodeExpression, "condition", 15},
		{"strings", "          - condition: \"'isn''t' != 'yes'\"\n", true, "", "", 0},
		{"property indexing", "          - condition: '$response.body.items[0].id != null'\n", true, "", "", 0},
		{"missing operand", "          - condition: '$statusCode == '\n", false, CodeExpression, "condition", 15},
		{"unbalanced", "          - condition: '($statusCode == 200'\n", false, CodeExpression, "condition", 15},
		{"invalid regex", "          - context: $statusCode\n            type: regex\n            condition: '['\n", false, CodeExpression, "condition", 17},
		{"regex lookahead", "          - context: $statusCode\n            type: regex\n            condition: '^(?!400)\\d+$'\n", true, "", "", 0},
		{"JSONPath", "          - context: $response.body\n            type: jsonpath\n            condition: '$.items[*]'\n", true, "", "", 0},
		{"invalid JSONPath", "          - context: $response.body\n            type: jsonpath\n            condition: '$.items['\n", false, CodeExpression, "condition", 17},
		{"strict JSONPath", "          - context: $response.body\n            type: jsonpath\n            condition: '$.items[?(@.id === 1)]'\n", false, CodeExpression, "condition", 17},
		{"required context", "          - type: simple\n            condition: 'true'\n", false, CodeStructure, "", 15},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := expressionValidate(t, "1.1.0", "        successCriteria:\n"+test.criterion)
			if test.valid {
				if !r.Valid() {
					t.Fatalf("valid criterion: %+v", r.Diagnostics)
				}
			} else {
				path := "/workflows/0/steps/0/successCriteria/0"
				if test.field != "" {
					path += "/" + test.field
				}
				expressionFinding(t, r, test.code, path, test.line)
			}
		})
	}
}

func TestCriteriaTraversalAndCapabilities(t *testing.T) {
	r := expressionValidate(t, "1.1.0", `        onSuccess:
          - name: finish
            type: end
            criteria:
              - condition: '$statusCode == '
    failureActions:
      - name: stop
        type: end
        criteria:
          - condition: '$statusCode != '
components:
  successActions:
    unused:
      name: unused
      type: end
      criteria:
        - context: $response.body
          type: xpath
          condition: /root/item
  failureActions:
    invalid:
      name: invalid
      type: end
      criteria:
        - condition: '$statusCode > '
`)
	expressionFinding(t, r, CodeExpression, "/workflows/0/steps/0/onSuccess/0/criteria/0/condition", 18)
	expressionFinding(t, r, CodeExpression, "/workflows/0/failureActions/0/criteria/0/condition", 23)
	expressionFinding(t, r, CodeExpression, "/components/failureActions/invalid/criteria/0/condition", 38)
	found := false
	for _, check := range r.Checks {
		if check.Status == CheckIncomplete && check.Pointer == "/components/successActions/unused/criteria/0/condition" {
			found = true
		}
	}
	if !found {
		t.Fatal("XPath capability was not reported")
	}
}

func TestSelectorAndReplacementSyntax(t *testing.T) {
	r := expressionValidate(t, "1.1.0", `        outputs:
          selected:
            context: $response.body
            selector: /bad~2token
            type: jsonpointer
        parameters:
          - name: id
            in: query
            value:
              context: $inputs.body
              selector: '$.items['
              type: jsonpath
        requestBody:
          contentType: application/json
          payload:
            invoice:
              context: $inputs.invoice
              selector: /id
              type: jsonpointer
            literal:
              context: this is literal
              selector: '['
              type: invalid
              extra: true
            jsonTemplate: '{"id": "{$inputs.id}"}'
          replacements:
            - target: /bad~2token
              value: '$request.header.'
`)
	expressionFinding(t, r, CodeSelector, "/workflows/0/steps/0/outputs/selected/selector", 17)
	expressionFinding(t, r, CodeSelector, "/workflows/0/steps/0/parameters/0/value/selector", 24)
	expressionFinding(t, r, CodeSelector, "/workflows/0/steps/0/requestBody/replacements/0/target", 40)
	expressionFinding(t, r, CodeExpression, "/workflows/0/steps/0/requestBody/replacements/0/value", 41)
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Pointer, "/payload/literal") {
			t.Fatalf("literal map interpreted as selector: %+v", d)
		}
	}
}

func TestExpressionsDoNotScanExtensionsOrBareDollar(t *testing.T) {
	r := expressionValidate(t, "1.1.0", `        x-note: '$not-a-runtime-expression'
        requestBody:
          contentType: application/json
          payload:
            currency: '$'
            object: '{literal braces}'
            x-data: plain text
`)
	if !r.Valid() {
		t.Fatalf("literal values: %+v", r.Diagnostics)
	}
}

func TestExpressionCappedDiagnosticsDeterministic(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxDiagnostics = 1
	fixture := expressionFixture + `        outputs:
          gamma: invalid
          beta: invalid
          alpha: invalid
`
	for i := 0; i < 30; i++ {
		result, err := ValidateBytes(context.Background(), []byte(fixture), "https://example.com/workflow.yaml", WithLimits(limits))
		if err == nil {
			t.Fatal("expected diagnostic cap error")
		}
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Pointer != "/workflows/0/steps/0/outputs/alpha" {
			t.Fatalf("nondeterministic capped diagnostics: %+v", result.Diagnostics)
		}
	}
}
