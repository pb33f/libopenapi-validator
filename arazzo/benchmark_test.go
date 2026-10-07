// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pb33f/go-yaml"
	upstream "github.com/pb33f/libopenapi/arazzo"
)

func benchmarkNodes(b *testing.B, data string) *yaml.Node {
	b.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(data), &root); err != nil {
		b.Fatal(err)
	}
	return &root
}

func BenchmarkValidateInputs(b *testing.B) {
	data := strings.Replace(foundationYAML("1.1.0"), "    steps:", "    inputs: {type: object, properties: {id: {type: string}}, additionalProperties: false}\n    steps:", 1)
	doc := Document{Root: benchmarkNodes(b, data), URI: "https://example.test/main.yaml"}
	opts := []Option{WithSources(upstream.CandidateDocument{RootNode: benchmarkNodes(b, sourceOpenAPI), RetrievalURI: "https://example.test/api.yaml"})}
	ctx := context.Background()
	if r, err := Validate(ctx, doc, opts...); err != nil || !r.Valid() || !r.Complete {
		b.Fatalf("%+v %v", r, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := Validate(ctx, doc, opts...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidate(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("steps-%d", count), func(b *testing.B) {
			var spec, api strings.Builder
			spec.WriteString("arazzo: 1.1.0\ninfo: {title: Bench, version: '1'}\nsourceDescriptions: [{name: api, url: 'api.yaml', type: openapi}]\nworkflows:\n- workflowId: run\n  steps:\n")
			api.WriteString("openapi: 3.1.0\ninfo: {title: Bench, version: '1'}\npaths:\n")
			for i := range count {
				fmt.Fprintf(&spec, "  - stepId: s%d\n    operationId: read%d\n    successCriteria: [{condition: '$statusCode == 200'}]\n    outputs: {id: '$response.body#/id'}\n", i, i)
				fmt.Fprintf(&api, "  /pets%d:\n    get:\n      operationId: read%d\n      responses: {'200': {description: OK}}\n", i, i)
			}
			root := benchmarkNodes(b, spec.String())
			source := benchmarkNodes(b, api.String())
			doc := Document{Root: root, URI: "https://example.test/main.yaml"}
			opts := []Option{WithSources(upstream.CandidateDocument{RootNode: source, RetrievalURI: "https://example.test/api.yaml"})}
			ctx := context.Background()
			if r, err := Validate(ctx, doc, opts...); err != nil || !r.Valid() || !r.Complete {
				b.Fatalf("%+v %v", r, err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := Validate(ctx, doc, opts...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidateManyFindings(b *testing.B) {
	for _, count := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("findings-%d", count), func(b *testing.B) {
			var spec strings.Builder
			spec.WriteString("arazzo: 1.1.0\ninfo: {title: Bench, version: '1'}\nsourceDescriptions: [{name: api, url: 'api.yaml'}]\nworkflows:\n- workflowId: run\n  steps:\n")
			for i := range count {
				fmt.Fprintf(&spec, "  - {stepId: s%d, operationId: read, outputs: {id: 'not an expression'}}\n", i)
			}
			doc := Document{Root: benchmarkNodes(b, spec.String())}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				r, err := Validate(context.Background(), doc)
				if err != nil || len(r.Diagnostics) != count {
					b.Fatalf("%+v %v", r, err)
				}
			}
		})
	}
}

// Compile both embedded schemas without the package cache to measure cold cost.
func BenchmarkCompileOfficialSchemas(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := compileOfficialSchemas(schemaFiles); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateSharedDependencies(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("workflows-%d", count), func(b *testing.B) {
			var spec strings.Builder
			spec.WriteString("arazzo: 1.1.0\ninfo: {title: Bench, version: '1'}\nsourceDescriptions: [{name: api, url: api.yaml}, {name: flow, url: flow.yaml, type: arazzo}]\ncomponents:\n  parameters:\n    shared: {name: trace, in: header, value: '$response.header.trace'}\nworkflows:\n")
			for i := range count {
				fmt.Fprintf(&spec, "- workflowId: w%d\n  dependsOn: ['$sourceDescriptions.flow.run']\n  parameters: [{reference: '$components.parameters.shared'}]\n  steps: [{stepId: read, operationId: read}]\n", i)
			}
			doc := Document{Root: benchmarkNodes(b, spec.String()), URI: "https://example.test/main.yaml"}
			sources := []upstream.CandidateDocument{
				{RootNode: benchmarkNodes(b, "openapi: 3.1.0\ninfo: {title: API, version: '1'}\npaths: {/items: {get: {operationId: read, parameters: [{name: trace, in: header, schema: {type: string}}], responses: {'200': {description: OK}}}}}\n"), RetrievalURI: "https://example.test/api.yaml"},
				{RootNode: benchmarkNodes(b, foundationYAML("1.1.0")), RetrievalURI: "https://example.test/flow.yaml"},
			}
			opts := []Option{WithSources(sources...)}
			ctx := context.Background()
			if r, err := Validate(ctx, doc, opts...); err != nil || !r.Valid() || !r.Complete {
				b.Fatalf("%+v %v", r, err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := Validate(ctx, doc, opts...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidateExternalSteps(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("dependencies-%d", count), func(b *testing.B) {
			var main, external strings.Builder
			main.WriteString("arazzo: 1.1.0\ninfo: {title: Main, version: '1'}\nsourceDescriptions: [{name: other, url: other.yaml, type: arazzo}]\nworkflows:\n- workflowId: noop\n  steps: [{stepId: noop, workflowId: noop}]\n- workflowId: run\n  steps:\n")
			external.WriteString("arazzo: 1.1.0\ninfo: {title: Other, version: '1'}\nsourceDescriptions: [{name: api, url: api.yaml, type: openapi}]\nworkflows:\n- workflowId: target\n  steps:\n")
			for i := range count {
				fmt.Fprintf(&main, "  - {stepId: s%d, workflowId: noop, dependsOn: ['$sourceDescriptions.other.target.steps.s%d']}\n", i, count-1)
				fmt.Fprintf(&external, "  - {stepId: s%d, operationId: read}\n", i)
			}
			doc := Document{Root: benchmarkNodes(b, main.String()), URI: "https://example.test/main.yaml"}
			options := []Option{WithSources(upstream.CandidateDocument{RootNode: benchmarkNodes(b, external.String()), RetrievalURI: "https://example.test/other.yaml"})}
			ctx := context.Background()
			if result, err := Validate(ctx, doc, options...); err != nil || !result.Valid() || !result.Complete {
				b.Fatalf("%+v %v", result, err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := Validate(ctx, doc, options...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
