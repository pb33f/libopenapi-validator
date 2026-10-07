// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"weak"

	"github.com/pb33f/go-yaml"
	upstream "github.com/pb33f/libopenapi/arazzo"
)

func retainedResult(t *testing.T) (*Result, weak.Pointer[yaml.Node], weak.Pointer[yaml.Node]) {
	t.Helper()
	root, source := new(yaml.Node), new(yaml.Node)
	input := strings.Replace(foundationYAML("1.1.0"), "operationId: read}", "operationId: read, outputs: {id: 'invalid expression'}}", 1)
	if err := yaml.Unmarshal([]byte(input), root); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte("openapi: 3.1.0\ninfo: {title: API, version: '1'}\npaths: {/items: {get: {operationId: read, responses: {'200': {description: OK}}}}}\n"), source); err != nil {
		t.Fatal(err)
	}
	result, err := Validate(context.Background(), Document{Root: root, URI: "https://example.test/main.yaml"}, WithSources(upstream.CandidateDocument{RootNode: source, RetrievalURI: "https://example.test/api.yaml"}))
	if err != nil || !result.Complete || len(result.Diagnostics) == 0 {
		t.Fatalf("retention fixture did not produce completed findings: %+v %v", result, err)
	}
	return result, weak.Make(root), weak.Make(source)
}

func TestResultDoesNotRetainSourceGraphs(t *testing.T) {
	result, primary, source := retainedResult(t)
	for range 10 {
		runtime.GC()
		if primary.Value() == nil && source.Value() == nil {
			break
		}
		runtime.Gosched()
	}
	if primary.Value() != nil || source.Value() != nil {
		t.Fatal("retained diagnostics kept a caller source graph alive")
	}
	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("detached result is not serializable: %v", err)
	}
	runtime.KeepAlive(result)
}
