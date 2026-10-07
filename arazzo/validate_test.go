// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dlclark/regexp2"
	"github.com/pb33f/go-yaml"
)

func foundationYAML(version string) string {
	return "arazzo: " + version + "\ninfo: {title: Example, version: '1'}\nsourceDescriptions:\n  - {name: api, url: 'api.yaml', type: openapi}\nworkflows:\n  - workflowId: run\n    steps:\n      - {stepId: read, operationId: read}\n"
}

func foundationFinding(t *testing.T, r *Result, code Code, path string) Diagnostic {
	t.Helper()
	if r == nil {
		t.Fatal("missing result")
	}
	for _, d := range r.Diagnostics {
		if d.Code == code && d.Pointer == path {
			return d
		}
	}
	t.Fatalf("missing %s at %s: %+v", code, path, r.Diagnostics)
	return Diagnostic{}
}

func TestOfficialSchemaSnapshots(t *testing.T) {
	for name, want := range map[string]string{
		"schemas/arazzo-1.0-2025-10-15.json": "b8715bd824fffcb2accf5077977d37c9e7a15be60d785e7a3a51cf600fd46ad4",
		"schemas/arazzo-1.1-2026-04-15.json": "37be908409bdb2f7bffe61fa23685c7e84cbeebfafac475a1d01dbc50ff7ab9e",
	} {
		data, err := schemaFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Fatalf("schema snapshot changed: %s", name)
		}
	}
	if _, err := officialSchemas(); err != nil {
		t.Fatal(err)
	}
}

func TestTrustedPatternEquivalence(t *testing.T) {
	re, err := regexp2.Compile(`^(?!\$).+$`, regexp2.ECMAScript)
	if err != nil {
		t.Fatal(err)
	}
	alphabet := []string{"a", "$", "\n", "\r", "\u2028", "\u2029", "é", "😀"}
	var check func(string, int)
	check = func(s string, depth int) {
		want, err := re.MatchString(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := (nonDollarPattern{}).MatchString(s); got != want {
			t.Errorf("%q: got %v want %v", s, got, want)
		}
		if depth > 0 {
			for _, ch := range alphabet {
				check(s+ch, depth-1)
			}
		}
	}
	check("", 3)
}

func TestFoundationVersionsAndJSONParity(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.0.24", "1.1.0", "1.1.42", "1.1.0-preview"} {
		t.Run(version, func(t *testing.T) {
			data := []byte(foundationYAML(version))
			r, err := ValidateBytes(context.Background(), data, "https://example.test/main.yaml")
			if err != nil || !r.Valid() {
				t.Fatalf("%+v %v", r, err)
			}
			var root yaml.Node
			if err := yaml.Unmarshal(data, &root); err != nil {
				t.Fatal(err)
			}
			value, _, _, _, err := normalizeWithStats(context.Background(), Document{Root: &root}, DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			jsonData, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			jr, err := ValidateBytes(context.Background(), jsonData, "https://example.test/main.yaml")
			if err != nil || !jr.Valid() || jr.Version != r.Version {
				t.Fatalf("JSON parity: %+v %v", jr, err)
			}
		})
	}
	for version, kind := range map[string]ErrorKind{"1.2.0": ErrorUnsupported, "2.0.0": ErrorUnsupported, "1.0": ErrorInput, "1.0.x": ErrorInput, "01.0.0": ErrorInput} {
		r, err := ValidateBytes(context.Background(), []byte(foundationYAML("'"+version+"'")), "input.yaml")
		var tool *Error
		if !errors.As(err, &tool) || tool.Kind != kind || r.Complete {
			t.Fatalf("version %s: %+v %v", version, r, err)
		}
	}
	for _, value := range []string{"", "arazzo: false\n", "arazzo: null\n"} {
		fixture := strings.Replace(foundationYAML("1.1.0"), "arazzo: 1.1.0\n", value, 1)
		r, err := ValidateBytes(context.Background(), []byte(fixture), "version.yaml")
		if err != nil || r.Valid() || r.Complete || r.Version != "" {
			t.Fatalf("absent/nonstring version was reported as detected: %+v %v", r, err)
		}
		foundationFinding(t, r, CodeStructure, "/arazzo")
	}
}

func TestFoundationStructuralLocations(t *testing.T) {
	tests := []struct {
		name, from, to, path string
		key                  bool
	}{
		{"string type", "title: Example", "title: 13", "/info/title", false},
		{"null", "title: Example", "title: null", "/info/title", false},
		{"missing field", "title: Example, ", "", "/info/title", false},
		{"unknown", "title: Example", "title: Example, surprise: true", "/info/surprise", true},
		{"target union", "operationId: read", "operationId: read, workflowId: run", "/workflows/0/steps/0", false},
		{"source URI", "api.yaml", "http://[invalid", "/sourceDescriptions/0/url", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := strings.Replace(foundationYAML("1.1.0"), tt.from, tt.to, 1)
			r, err := ValidateBytes(context.Background(), []byte(data), "test.yaml")
			if err != nil {
				t.Fatal(err)
			}
			d := foundationFinding(t, r, CodeStructure, tt.path)
			if d.Line == 0 || d.Column == 0 || d.URI != "test.yaml" {
				t.Fatalf("bad location: %+v", d)
			}
			var root yaml.Node
			if err := yaml.Unmarshal([]byte(data), &root); err != nil {
				t.Fatal(err)
			}
			_, nodes, _, _, err := normalizeWithStats(context.Background(), Document{Root: &root}, DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			if tt.key && d.Column != nodes[tt.path].key.Column {
				t.Fatalf("expected key location: %+v", d)
			}
		})
	}
}

func TestFoundationInputAndLimits(t *testing.T) {
	for _, data := range []string{"", "[", foundationYAML("1.1.0") + "---\nnull\n"} {
		_, err := ValidateBytes(context.Background(), []byte(data), "input.yaml")
		var tool *Error
		if !errors.As(err, &tool) || tool.Kind != ErrorInput {
			t.Fatalf("expected input error: %v", err)
		}
	}
	for _, data := range []string{"null", "[]", "true"} {
		r, err := ValidateBytes(context.Background(), []byte(data), "")
		if err != nil || r.Valid() {
			t.Fatalf("%q: %+v %v", data, r, err)
		}
	}
	limits := DefaultLimits()
	limits.MaxBytes = 8
	r, err := ValidateBytes(context.Background(), []byte(foundationYAML("1.0.1")), "", WithLimits(limits))
	var tool *Error
	if !errors.As(err, &tool) || tool.Kind != ErrorLimit || r.Complete {
		t.Fatalf("%+v %v", r, err)
	}
	_, err = Validate(nil, Document{}) //nolint:staticcheck // Verify that the public API rejects a nil context.
	if !errors.As(err, &tool) || tool.Kind != ErrorConfiguration {
		t.Fatal(err)
	}
	_, err = Validate(context.Background(), Document{}, nil)
	if !errors.As(err, &tool) || tool.Kind != ErrorConfiguration {
		t.Fatal(err)
	}
	_, err = Validate(context.Background(), Document{}, WithLimits(Limits{}))
	if !errors.As(err, &tool) || tool.Kind != ErrorConfiguration {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err = ValidateBytes(ctx, []byte(foundationYAML("1.0.1")), "")
	if !errors.Is(err, context.Canceled) || !errors.As(err, &tool) || tool.Kind != ErrorOperational || r.Complete {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = Validate(ctx, Document{URI: "canceled.yaml"})
	if !errors.Is(err, context.Canceled) || !errors.As(err, &tool) || tool.Kind != ErrorOperational || r.Complete || len(r.Checks) != 1 {
		t.Fatalf("node API cancellation was not a typed incomplete operation: %+v %v", r, err)
	}
}

func TestFoundationNoRetrieval(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	data := strings.Replace(foundationYAML("1.1.0"), "api.yaml", server.URL+"/api.yaml", 1)
	data = strings.Replace(data, "    steps:", "    inputs: {$ref: '"+server.URL+"/schema.json'}\n    steps:", 1)
	r, err := ValidateBytes(context.Background(), []byte(data), "https://example.test/main.yaml")
	if err != nil || !r.Valid() || r.Complete || requests.Load() != 0 {
		t.Fatalf("%+v %v requests=%d", r, err, requests.Load())
	}
}

func TestFoundationMergeAliasImmutability(t *testing.T) {
	data := strings.Replace(foundationYAML("1.1.0"), "      - {stepId: read, operationId: read}", "      - &base {stepId: read, operationId: read}\n      - {<<: *base, stepId: second}", 1)
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(data), &root); err != nil {
		t.Fatal(err)
	}
	before := cloneNodeGraph(&root)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := Validate(context.Background(), Document{Root: &root, URI: "main.yaml"})
			if err != nil || !r.Valid() {
				t.Errorf("%+v %v", r, err)
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(before, &root) {
		t.Fatal("caller tree was modified")
	}
}

func TestFoundationYAMLGraphsAndNumbers(t *testing.T) {
	var root yaml.Node
	data := "huge: 9007199254740993123456789\nzero: 0\n'false': false\n'null': null\nempty: {}\nhex: 0xFF\n"
	if err := yaml.Unmarshal([]byte(data), &root); err != nil {
		t.Fatal(err)
	}
	value, _, diags, _, err := normalizeWithStats(context.Background(), Document{Root: &root}, DefaultLimits())
	if err != nil || len(diags) > 0 {
		t.Fatalf("%v %+v", err, diags)
	}
	m := object(value)
	if m["huge"] != json.Number("9007199254740993123456789") || m["hex"] != json.Number("255") || m["false"] != false || m["null"] != nil || len(object(m["empty"])) != 0 {
		t.Fatalf("lost values: %+v", m)
	}
	for _, data := range []string{"a: 1\na: 2\n", "true: false\n", "a: !!binary YQ==\n", "a: .inf\n", "a: &a {b: *a}\n"} {
		if err := yaml.Unmarshal([]byte(data), &root); err != nil {
			t.Fatal(err)
		}
		_, _, diags, _, err := normalizeWithStats(context.Background(), Document{Root: &root}, DefaultLimits())
		if err != nil || len(diags) == 0 {
			t.Fatalf("%q: %+v %v", data, diags, err)
		}
	}
	root = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	root.Content = []*yaml.Node{&root}
	_, _, diags, _, err = normalizeWithStats(context.Background(), Document{Root: &root}, DefaultLimits())
	if err != nil || len(diags) != 1 {
		t.Fatalf("recursive graph: %+v %v", diags, err)
	}
	limits := DefaultLimits()
	limits.MaxNodes = 1
	_, _, _, _, err = normalizeWithStats(context.Background(), Document{Root: &root}, limits)
	var tool *Error
	if !errors.As(err, &tool) || tool.Kind != ErrorLimit {
		t.Fatal(err)
	}
}

func TestFoundationPointerEscapesAndDeterminism(t *testing.T) {
	data := foundationYAML("1.1.0") + "components:\n  parameters:\n    'a/b~c': {name: q, in: query}\n    '123': {name: n, in: query}\n"
	r, err := ValidateBytes(context.Background(), []byte(data), "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	foundationFinding(t, r, CodeStructure, "/components/parameters/a~1b~0c/value")
	foundationFinding(t, r, CodeStructure, "/components/parameters/123/value")
	for range 8 {
		again, err := ValidateBytes(context.Background(), []byte(data), "test.yaml")
		if err != nil || !reflect.DeepEqual(r, again) {
			t.Fatalf("unstable diagnostics: %+v %v", again, err)
		}
	}
}

func TestFoundationNormativeCorrectionsAndAdvisories(t *testing.T) {
	data := strings.Replace(foundationYAML("1.1.0"), "name: api", "name: api.source", 1)
	data = strings.Replace(data, "operationId: read", "operationId: read, parameters: [{name: body, in: query, value: {context: literal, selector: '[', type: jsonpath}}]", 1)
	r, err := ValidateBytes(context.Background(), []byte(data), "https://example.test/main.yaml")
	if err != nil || !r.Valid() {
		t.Fatalf("valid normative Any/source name: %+v %v", r, err)
	}
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityWarning {
			t.Fatal("advice enabled without option")
		}
	}
	r, err = ValidateBytes(context.Background(), []byte(data), "main.yaml", WithAdvisories())
	if err != nil || !r.Valid() {
		t.Fatalf("%+v %v", r, err)
	}
	d := foundationFinding(t, r, CodeAdvisory, "/sourceDescriptions/0/name")
	if d.Severity != SeverityWarning {
		t.Fatal(d)
	}
	data = foundationYAML("1.1.0") + "components:\n  successActions:\n    go: {name: go, type: goto, workflowId: run, parameters: [123]}\n"
	r, err = ValidateBytes(context.Background(), []byte(data), "main.yaml")
	if err != nil || r.Valid() {
		t.Fatalf("invalid action parameter escaped shape checks: %+v %v", r, err)
	}
	foundationFinding(t, r, CodeStructure, "/components/successActions/go/parameters/0")
}

func TestFoundationCapsBeforeCompilation(t *testing.T) {
	tests := []struct {
		data   string
		limits Limits
	}{
		{strings.Replace(foundationYAML("1.1.0"), "    steps:", "    inputs: {type: string, pattern: 'a very long pattern'}\n    steps:", 1), func() Limits { l := DefaultLimits(); l.MaxPatternBytes = 4; return l }()},
		{strings.Replace(foundationYAML("1.1.0"), "operationId: read", "operationId: read, requestBody: {payload: 1e99999999999999}", 1), DefaultLimits()},
		{strings.Replace(foundationYAML("1.1.0"), "    steps:", "    x-a: {x-b: {x-c: {x-d: value}}}\n    steps:", 1), func() Limits { l := DefaultLimits(); l.MaxDepth = 4; return l }()},
	}
	for i, tt := range tests {
		r, err := ValidateBytes(context.Background(), []byte(tt.data), "input.yaml", WithLimits(tt.limits))
		var tool *Error
		if !errors.As(err, &tool) || tool.Kind != ErrorLimit || r.Complete {
			t.Fatalf("case %d: %+v %v", i, r, err)
		}
	}
}

// cloneNodeGraph snapshots caller-owned node graphs for immutability tests,
// preserving aliases and nil slices so comparisons detect actual changes.
func cloneNodeGraph(root *yaml.Node) *yaml.Node {
	clones := make(map[*yaml.Node]*yaml.Node)
	var clone func(*yaml.Node) *yaml.Node
	clone = func(node *yaml.Node) *yaml.Node {
		if node == nil {
			return nil
		}
		if existing := clones[node]; existing != nil {
			return existing
		}
		copyNode := *node
		clones[node] = &copyNode
		if node.Content != nil {
			copyNode.Content = make([]*yaml.Node, len(node.Content))
		}
		for i, child := range node.Content {
			copyNode.Content[i] = clone(child)
		}
		copyNode.Alias = clone(node.Alias)
		return &copyNode
	}
	return clone(root)
}
