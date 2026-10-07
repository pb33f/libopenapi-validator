// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"strings"
	"testing"

	upstream "github.com/pb33f/libopenapi/arazzo"
)

const expressionLinkedFixture = `arazzo: 1.1.0
info: {title: main, version: '1'}
sourceDescriptions:
  - {name: flow, url: other.yaml, type: arazzo}
workflows:
  - workflowId: main
    steps:
      - stepId: first
        workflowId: $sourceDescriptions.flow.remote
        parameters:
          - name: id
            value: $sourceDescriptions.flow.remote.outputs.id
`

const expressionLinkedSource = `arazzo: 1.1.0
info: {title: other, version: '1'}
sourceDescriptions: [{name: api, url: api.yaml, type: openapi}]
workflows:
  - workflowId: remote
    inputs:
      type: object
      properties: {id: {type: string}}
      additionalProperties: false
    steps:
      - stepId: produce
        operationId: getId
        outputs: {id: $response.body#/id}
    outputs: {id: $steps.produce.outputs.id}
`

func TestLinkedExpressionSymbols(t *testing.T) {
	for _, test := range []struct {
		reference string
		valid     bool
	}{
		{"$sourceDescriptions.flow.remote.outputs.id", true},
		{"$sourceDescriptions.flow.remote.outputs.missing", false},
		{"$sourceDescriptions.flow.remote.inputs.missing", false},
		{"$sourceDescriptions.flow.remote.steps.produce.outputs.id", true},
		{"$sourceDescriptions.flow.remote.steps.missing.outputs.id", false},
	} {
		t.Run(test.reference, func(t *testing.T) {
			fixture := strings.Replace(expressionLinkedFixture, "$sourceDescriptions.flow.remote.outputs.id", test.reference, 1)
			r, err := ValidateBytes(context.Background(), []byte(fixture), "https://example.com/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.com/other.yaml", SourceBytes: []byte(expressionLinkedSource)}))
			if err != nil {
				t.Fatal(err)
			}
			if test.valid {
				if !r.Valid() {
					t.Fatalf("valid external symbol: %+v", r.Diagnostics)
				}
			} else {
				expressionFinding(t, r, CodeReference, "/workflows/0/steps/0/parameters/0/value", 12)
			}
		})
	}
}

func TestLinkedExpressionPresenceAdapterCoverage(t *testing.T) {
	fixture := strings.ReplaceAll(expressionLinkedFixture, ".remote", ".run")
	r, err := ValidateBytes(context.Background(), []byte(fixture), "https://example.com/main.yaml", WithSources(upstream.CandidateDocument{RetrievalURI: "https://example.com/other.yaml", Type: "arazzo", Adapter: workflowPresence{}}))
	if err != nil || !r.Valid() {
		t.Fatalf("presence adapter marked missing metadata invalid: %+v %v", r, err)
	}
	found := false
	for _, check := range r.Checks {
		if check.Name == "expression-symbol" && check.Status == CheckIncomplete && check.Pointer == "/workflows/0/steps/0/parameters/0/value" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing metadata coverage: %+v", r.Checks)
	}
}

func TestSelectorExtensionsAndLiteralSignature(t *testing.T) {
	r := expressionValidate(t, "1.1.0", `        parameters:
          - name: result
            in: query
            value:
              context: $response.body
              selector: $.id
              type: jsonpath
              x-note: '$bad-extension-expression'
          - name: literal
            in: query
            value:
              context: literal text
              selector: $.id
              type: jsonpath
              extra: true
`)
	if !r.Valid() {
		t.Fatalf("selector extension or literal object rejected: %+v", r.Diagnostics)
	}
}

func TestLinkedExpressionKnownMissingOperation(t *testing.T) {
	fixture := strings.Replace(foundationYAML("1.1.0"), "operationId: read}", "operationId: read, outputs: {id: '$sourceDescriptions.api.missing'}}", 1)
	result, err := ValidateBytes(context.Background(), []byte(fixture), "https://example.test/main.yaml", WithSources(upstream.CandidateDocument{
		RetrievalURI: "https://example.test/api.yaml",
		SourceBytes:  []byte("openapi: 3.1.0\ninfo: {title: API, version: '1'}\npaths: {/items: {get: {operationId: read, responses: {'200': {description: OK}}}}}\n"),
	}))
	if err != nil || !result.Complete {
		t.Fatalf("full source did not complete the absent-member check: %+v %v", result, err)
	}
	foundationFinding(t, result, CodeReference, "/workflows/0/steps/0/outputs/id")
}
