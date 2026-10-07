// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"strings"
	"testing"
)

func inputValidate(t *testing.T, schema, components string) *Result {
	t.Helper()
	fixture := strings.Replace(expressionFixture, "    steps:\n", "    inputs:\n"+schema+"    steps:\n", 1) + components
	result, err := ValidateBytes(context.Background(), []byte(fixture), "https://example.com/workflow.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestInputSchema202012Structure(t *testing.T) {
	r := inputValidate(t, "      type: object\n      properties:\n        count:\n          type: not-a-json-schema-type\n", "")
	expressionFinding(t, r, CodeStructure, "/workflows/0/inputs/properties/count/type", 15)
	valid := inputValidate(t, `      type: object
      properties:
        nullable:
          type: [string, 'null']
        choice:
          oneOf:
            - type: string
            - type: integer
        forbidden: false
      additionalProperties: true
`, "")
	if !valid.Valid() {
		t.Fatalf("JSON Schema 2020-12 rejected: %+v", valid.Diagnostics)
	}
}

func TestInputSchemaLocalRefsAndResourceScope(t *testing.T) {
	for _, test := range []struct{ name, schema, components string }{
		{"component pointer", "      $ref: '#/components/inputs/shared'\n", `components:
  inputs:
    shared:
      type: object
      properties:
        id: {type: integer}
`},
		{"component anchor", "      $ref: '#shared'\n", `components:
  inputs:
    shared:
      $anchor: shared
      type: object
`},
		{"nested resource", "      $ref: 'schemas/shared.json#value'\n", `components:
  inputs:
    shared:
      $id: schemas/shared.json
      $defs:
        value:
          $anchor: value
          type: string
`},
		{"recursive dynamic", "      $dynamicAnchor: node\n      type: object\n      properties:\n        child:\n          $dynamicRef: '#node'\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := inputValidate(t, test.schema, test.components)
			if !r.Valid() {
				t.Fatalf("valid input schema: %+v", r.Diagnostics)
			}
			for _, check := range r.Checks {
				if check.Name == "input-schema-reference" && check.Status == CheckIncomplete {
					t.Fatalf("local reference skipped: %+v", check)
				}
			}
		})
	}
}

func TestInputSchemaMissingLocalRefAndExternalCoverage(t *testing.T) {
	missing := inputValidate(t, "      $ref: '#/components/inputs/missing'\n", "")
	expressionFinding(t, missing, CodeInputSchema, "/workflows/0/inputs/$ref", 12)
	external := inputValidate(t, "      $ref: 'https://example.com/external-schema.json'\n", "")
	if !external.Valid() {
		t.Fatalf("unavailable resource is not malformed: %+v", external.Diagnostics)
	}
	found := false
	for _, check := range external.Checks {
		if check.Name == "input-schema-reference" && check.Status == CheckIncomplete && check.Pointer == "/workflows/0/inputs/$ref" && check.Source == "https://example.com/external-schema.json" {
			found = true
		}
	}
	if !found {
		t.Fatalf("external resource coverage missing: %+v", external.Checks)
	}
}

func TestInputSchemaDataKeywordsDoNotDeclareReferences(t *testing.T) {
	r := inputValidate(t, `      type: object
      const:
        $ref: https://example.com/not-a-schema.json
        properties:
          id: {type: not-a-schema}
`, "")
	if !r.Valid() {
		t.Fatalf("schema literal data scanned as schema: %+v", r.Diagnostics)
	}
	for _, check := range r.Checks {
		if check.Name == "input-schema-reference" {
			t.Fatalf("data caused resource load: %+v", check)
		}
	}
}

func TestInputSchemaExternalSiblingDoesNotHideMissingLocalRef(t *testing.T) {
	r := inputValidate(t, `      type: object
      properties:
        remote:
          $ref: https://example.com/missing-schema.json
        local:
          $ref: '#/$defs/missing'
`, "")
	expressionFinding(t, r, CodeInputSchema, "/workflows/0/inputs/properties/local/$ref", 17)
}

func TestInputPatternLimitAndLiteralAnnotations(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxPatternBytes = 4
	fixture := strings.Replace(expressionFixture, "    steps:\n", "    inputs:\n      type: string\n      pattern: '12345'\n    steps:\n", 1)
	r, err := ValidateBytes(context.Background(), []byte(fixture), "https://example.com/workflow.yaml", WithLimits(limits))
	if err == nil || r.Complete {
		t.Fatalf("pattern limit was not operational: result=%+v err=%v", r, err)
	}
	fixture = strings.Replace(expressionFixture, "    steps:\n", "    inputs:\n      type: string\n      default: {pattern: '12345'}\n    steps:\n", 1)
	r, err = ValidateBytes(context.Background(), []byte(fixture), "https://example.com/workflow.yaml", WithLimits(limits))
	if err != nil || !r.Valid() {
		t.Fatalf("literal schema default capped as pattern: result=%+v err=%v", r, err)
	}
}

func TestInputUnknownAnnotationsRemainData(t *testing.T) {
	r := inputValidate(t, `      type: object
      workflows:
        - inputs:
            $id: ':invalid URI'
      components:
        inputs:
          literal: {$anchor: 'invalid anchor'}
`, "")
	if !r.Valid() {
		t.Fatalf("unknown JSON Schema annotation treated as schema: %+v", r.Diagnostics)
	}
}
