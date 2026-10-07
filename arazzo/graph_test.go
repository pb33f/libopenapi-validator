// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"errors"
	"testing"
)

func TestDependencyGraphIdentityAndCancellation(t *testing.T) {
	g := newDependencyGraph()
	a := dependencyID{document: "a", workflow: "same", step: "same"}
	b := dependencyID{document: "b", workflow: "same", step: "same"}
	g.addEdge(a, b, "/dependsOn/0")
	if err := g.check(context.Background(), func(path, message string) { t.Fatal("different documents conflated") }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.check(ctx, func(path, message string) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestDependencyGraphLongChain(t *testing.T) {
	g := newDependencyGraph()
	for i := 0; i < 10000; i++ {
		g.addEdge(dependencyID{workflow: string(rune(i + 1))}, dependencyID{workflow: string(rune(i + 2))}, "/dependsOn/0")
	}
	if err := g.check(context.Background(), func(path, message string) { t.Fatal("linear chain reported as cycle") }); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyGraphCompletionBackEdge(t *testing.T) {
	g := newDependencyGraph()
	s := dependencyID{workflow: "run", step: "step"}
	w := dependencyID{workflow: "run"}
	g.addEdge(s, w, "/outputs/id")
	g.addEdge(w, s, "")
	found := false
	if err := g.check(context.Background(), func(path, message string) {
		found = true
		if path != "/outputs/id" {
			t.Fatalf("wrong cycle origin: %s", path)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("cycle closing through workflow completion was missed")
	}
}
