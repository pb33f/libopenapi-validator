// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/pb33f/go-yaml"
	"github.com/pb33f/jsonschema/v6"
)

func TestFoundationConfigurationAndNodeErrors(t *testing.T) {
	if (*Result)(nil).Valid() {
		t.Fatal("nil result is not valid")
	}
	for _, option := range []Option{WithBaseURI("http://[invalid"), WithResolver(nil)} {
		r, err := ValidateBytes(context.Background(), nil, "config.yaml", option)
		var tool *Error
		if r != nil || !errors.As(err, &tool) || tool.Kind != ErrorConfiguration {
			t.Fatalf("%+v %v", r, err)
		}
	}
	for _, doc := range []Document{{URI: "nil.yaml"}, {URI: "http://[invalid", Root: &yaml.Node{Kind: yaml.MappingNode}}} {
		r, err := Validate(context.Background(), doc)
		var tool *Error
		if !errors.As(err, &tool) || r.Complete || tool.Location.URI != doc.URI {
			t.Fatalf("%+v %v", r, err)
		}
	}
	l := DefaultLimits()
	l.MaxNodes = 2
	r, err := ValidateBytes(context.Background(), []byte("[a,b,c]"), "wide.yaml", WithLimits(l))
	var tool *Error
	if !errors.As(err, &tool) || tool.Kind != ErrorLimit || r.Complete {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestFoundationOverflowNumbersRemainNumeric(t *testing.T) {
	// The metaschema requires numeric bounds. Retagging a plain overflow number
	// as a string must fail this test, even though Parameter.value accepts Any.
	for _, number := range []string{strings.Repeat("9", 320), "1e309"} {
		for _, quoted := range []bool{false, true} {
			value := number
			if quoted {
				value = "'" + value + "'"
			}
			data := strings.Replace(foundationYAML("1.1.0"), "    steps:", "    inputs: {type: number, minimum: "+value+"}\n    steps:", 1)
			r, err := ValidateBytes(context.Background(), []byte(data), "numeric.yaml")
			if err != nil || r.Valid() == quoted {
				t.Fatalf("quoted=%v number=%s: %+v %v", quoted, number, r, err)
			}
			if quoted {
				foundationFinding(t, r, CodeStructure, "/workflows/0/inputs/minimum")
			}
		}
	}
}

func TestSchemaPatternAdapters(t *testing.T) {
	for _, tc := range []struct{ pattern, accepted, rejected string }{
		{`^(?=ab)ab$`, "ab", "ac"}, {`^(?!\$).+$`, "literal", "$expression"}, {`^a+$`, "aa", "b"},
	} {
		p, err := compileSchemaPattern(tc.pattern)
		if err != nil || p.String() != tc.pattern || !p.MatchString(tc.accepted) || p.MatchString(tc.rejected) {
			t.Fatalf("pattern %s: %v %v", tc.pattern, p, err)
		}
	}
	for _, pattern := range []string{"[", strings.Repeat("a", DefaultLimits().MaxPatternBytes+1)} {
		if _, err := compileSchemaPattern(pattern); err == nil {
			t.Fatal("invalid/unbounded pattern accepted")
		}
	}
	_, err := (denyLoader{}).Load("https://example.test/not-supplied")
	var unavailable *externalResourceError
	if !errors.As(err, &unavailable) || !strings.Contains(err.Error(), unavailable.URI) {
		t.Fatal(err)
	}
	regex, err := regexp2.Compile(`^(a+)+$`, regexp2.ECMAScript)
	if err != nil {
		t.Fatal(err)
	}
	regex.MatchTimeout = time.Nanosecond
	func() {
		defer func() {
			if _, ok := recover().(patternMatchError); !ok {
				t.Error("regexp timeout must cross the interface as an operational failure")
			}
		}()
		(ecmaPattern{regex}).MatchString(strings.Repeat("a", 1000) + "!")
	}()
}

func TestStructureCompilerFailureAndBudget(t *testing.T) {
	original := officialSchemas
	defer func() { officialSchemas = original }()
	compilerErr := errors.New("invalid embedded schema")
	officialSchemas = func() (map[string]*jsonschema.Schema, error) { return nil, compilerErr }
	r, err := ValidateBytes(context.Background(), []byte(foundationYAML("1.1.0")), "schema.yaml")
	var tool *Error
	if !errors.As(err, &tool) || tool.Kind != ErrorOperational || !errors.Is(err, compilerErr) || r.Complete {
		t.Fatalf("%+v %v", r, err)
	}
	officialSchemas = original
	data := foundationYAML("1.1.0") + "components:\n  parameters:\n    '': {name: q, in: query, value: a}\n"
	r, err = ValidateBytes(context.Background(), []byte(data), "key.yaml")
	if err != nil || r.Valid() {
		t.Fatalf("empty component key: %+v %v", r, err)
	}
	d := foundationFinding(t, r, CodeStructure, "/components/parameters/")
	if d.Line != 11 || d.Column != 5 || d.SchemaPointer == "" {
		t.Fatalf("property-name coordinates: %+v", d)
	}
	l := DefaultLimits()
	l.MaxDiagnostics = 1
	data = strings.Replace(foundationYAML("1.1.0"), "title: Example", "title: 123, one: true, two: false", 1)
	r, err = ValidateBytes(context.Background(), []byte(data), "invalid.yaml", WithLimits(l))
	if !errors.As(err, &tool) || tool.Kind != ErrorLimit || r.Complete || len(r.Diagnostics) != 1 {
		t.Fatalf("bounded structural errors: %+v %v", r, err)
	}
}

func TestValidationSharedBudgetAndOrdering(t *testing.T) {
	v := &validation{ctx: context.Background(), doc: Document{URI: "main.yaml"}, opts: options{limits: DefaultLimits()}, result: &Result{}, nodes: map[string]nodeLocation{}}
	if !v.workBytes(3, "") || v.budget.bytes != 3 {
		t.Fatal("byte work was not charged")
	}
	v.opts.limits.MaxBytes = 3
	if v.workBytes(1, "/payload") {
		t.Fatal("byte work exceeded cap")
	}
	var tool *Error
	if !errors.As(v.err, &tool) || tool.Kind != ErrorLimit || tool.Location.Pointer != "/payload" {
		t.Fatalf("%v", v.err)
	}
	if v.workBytes(1, "") {
		t.Fatal("failed budget resumed")
	}
	v.addKey(CodeStructure, "/key", "cannot emit after failure")
	if len(v.result.Diagnostics) != 0 {
		t.Fatal("failed operation emitted another error")
	}
	v.err, v.budget = nil, &workBudget{}
	v.result.Diagnostics = []Diagnostic{
		{Location: Location{URI: "z"}, Code: CodeStructure},
		{Location: Location{URI: "a", Pointer: "/b"}, Code: CodeStructure},
		{Location: Location{URI: "a", Pointer: "/a"}, Code: CodeReference},
		{Location: Location{URI: "a", Pointer: "/a"}, Code: CodeStructure, Message: "z"},
		{Location: Location{URI: "a", Pointer: "/a", Line: 2}, Code: CodeStructure, Message: "a"},
		{Location: Location{URI: "a", Pointer: "/a", Line: 1, Column: 2}, Code: CodeStructure, Message: "a"},
		{Location: Location{URI: "a", Pointer: "/a", Line: 1, Column: 1}, Code: CodeStructure, Message: "a"},
	}
	v.result.Checks = []Check{
		{Name: "c", URI: "z"},
		{Name: "c", URI: "a", Pointer: "/b"},
		{Name: "c", URI: "a", Pointer: "/a", Source: "z"},
		{Name: "c", URI: "a", Pointer: "/a", Source: "a", Reason: "z"},
		{Name: "c", URI: "a", Pointer: "/a", Source: "a", Reason: "a"},
	}
	v.finish()
	if len(v.result.Diagnostics) != 5 || v.result.Diagnostics[1].Column != 1 || v.result.Checks[0].Reason != "a" {
		t.Fatalf("ordering/dedup: %+v", v.result)
	}
	before := *v.result
	before.Diagnostics = append([]Diagnostic(nil), before.Diagnostics...)
	before.Checks = append([]Check(nil), before.Checks...)
	v.finish()
	if !reflect.DeepEqual(before, *v.result) {
		t.Fatal("finish is not idempotent")
	}
}

func TestDependencyGraphBudgetAndUnnamedCycle(t *testing.T) {
	for _, max := range []int{1, 2} {
		v := &validation{ctx: context.Background(), opts: options{limits: DefaultLimits()}, result: &Result{}}
		v.opts.limits.MaxNodes = max
		g := newDependencyGraph()
		g.run = v
		g.addEdge(dependencyID{step: "a"}, dependencyID{step: "b"}, "/dependsOn")
		var tool *Error
		if !errors.As(v.err, &tool) || tool.Kind != ErrorLimit || len(g.edges[dependencyID{step: "a"}]) != 0 {
			t.Fatalf("graph exceeded shared budget: %v %+v", v.err, g)
		}
	}
	g := newDependencyGraph()
	a, b := dependencyID{step: "a"}, dependencyID{step: "b"}
	g.addEdge(a, b, "")
	g.addEdge(b, a, "")
	if err := g.check(context.Background(), func(_, _ string) { t.Error("no authored edge exists for a diagnostic") }); err != nil {
		t.Fatal(err)
	}
}

func TestStructureDiagnosticAdapter(t *testing.T) {
	// Exercise compiler diagnostics independent of the current snapshot's shape.
	// These fixtures protect locations if a later official snapshot changes keywords.
	original := officialSchemas
	defer func() { officialSchemas = original }()
	for _, tc := range []struct {
		schema map[string]any
		value  map[string]any
		paths  []string
	}{
		{map[string]any{"type": "object", "additionalProperties": false}, map[string]any{"b": true, "a": true}, []string{"/a", "/b"}},
		{map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "number"}}}, map[string]any{}, []string{""}},
	} {
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource("https://example.test/schema", tc.schema); err != nil {
			t.Fatal(err)
		}
		schema, err := compiler.Compile("https://example.test/schema")
		if err != nil {
			t.Fatal(err)
		}
		officialSchemas = func() (map[string]*jsonschema.Schema, error) {
			return map[string]*jsonschema.Schema{"1.1": schema}, nil
		}
		v := &validation{ctx: context.Background(), opts: options{limits: DefaultLimits()}, root: tc.value, result: &Result{}, version: "1.1"}
		checkStructure(v)
		for _, path := range tc.paths {
			foundationFinding(t, v.result, CodeStructure, path)
		}
		if v.err != nil {
			t.Fatal(v.err)
		}
	}
}

type failingPattern struct{ failure any }

func (f failingPattern) String() string          { return "injected-regexp-failure" }
func (f failingPattern) MatchString(string) bool { panic(f.failure) }

func TestFoundationRegexpFailureBoundary(t *testing.T) {
	original := officialSchemas
	defer func() { officialSchemas = original }()
	cause := errors.New("pattern execution failure")
	for _, failure := range []any{patternMatchError{cause}, cause} {
		schema := &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"info": {Properties: map[string]*jsonschema.Schema{"title": {Pattern: failingPattern{failure}}}}}}
		officialSchemas = func() (map[string]*jsonschema.Schema, error) {
			return map[string]*jsonschema.Schema{"1.1": schema}, nil
		}
		func() {
			defer func() {
				got := recover()
				if failure == cause && got != cause {
					t.Errorf("unexpected failures must propagate: %v", got)
				}
				if failure != cause && got != nil {
					t.Errorf("pattern failure escaped: %v", got)
				}
			}()
			r, err := ValidateBytes(context.Background(), []byte(foundationYAML("1.1.0")), "regex.yaml")
			var tool *Error
			if !errors.As(err, &tool) || tool.Kind != ErrorOperational || !errors.Is(err, cause) || r.Complete {
				t.Fatalf("pattern error boundary: %+v %v", r, err)
			}
		}()
	}
}

func TestFoundationCancellationAtEveryPhase(t *testing.T) {
	// Cancel at each observed context check, including checks inside normalization,
	// diagnostic collection, expression parsing, source indexing and graph traversal.
	data := strings.Replace(foundationYAML("1.1.0"), "operationId: read", "operationId: read, parameters: [{name: id, in: query, value: $statusCode}]", 1)
	root := sourceContractYAML(t, data)
	probe := newSourceCancelContext(t, 100000)
	if _, err := Validate(probe, Document{Root: root, URI: "cancel.yaml"}); err != nil {
		t.Fatal(err)
	}
	for check := 1; check <= probe.checks; check++ {
		ctx := newSourceCancelContext(t, check)
		r, err := Validate(ctx, Document{Root: root, URI: "cancel.yaml"})
		if !errors.Is(err, context.Canceled) || r.Complete {
			t.Fatalf("cancellation check %d: %+v %v", check, r, err)
		}
	}
}
