// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pb33f/go-yaml"
)

func TestNodeMalformedGraphs(t *testing.T) {
	str := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	merge := func() *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!merge", Value: "<<"} }
	for _, tc := range []struct {
		name    string
		root    *yaml.Node
		kind    ErrorKind
		message string
	}{
		{"nil", nil, ErrorInput, ""},
		{"empty document", &yaml.Node{Kind: yaml.DocumentNode}, ErrorInput, ""},
		{"unpaired key", &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{str("a")}}, ErrorInput, ""},
		{"nil key", &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{nil, str("a")}}, ErrorInput, ""},
		{"nil value", &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{str("a"), nil}}, ErrorInput, ""},
		{"unknown kind", &yaml.Node{Kind: 128}, "", "unsupported YAML node kind"},
		{"mapping tag", &yaml.Node{Kind: yaml.MappingNode, Tag: "!custom"}, "", "mapping tag"},
		{"sequence tag", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!custom"}, "", "sequence tag"},
		{"bad bool", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "yes"}, "", "scalar"},
		{"bad int", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0xinvalid"}, "", "scalar"},
		{"bad merge", &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{merge(), str("invalid")}}, "", "merge value"},
		{"duplicate merge", &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{merge(), {Kind: yaml.MappingNode}, merge(), {Kind: yaml.MappingNode}}}, "", "duplicate merge"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := cloneNodeGraph(tc.root)
			_, _, ds, err := normalize(context.Background(), Document{Root: tc.root, URI: "nodes.yaml"}, DefaultLimits())
			if tc.kind != "" {
				var tool *Error
				if !errors.As(err, &tool) || tool.Kind != tc.kind {
					t.Fatalf("want %s: %v", tc.kind, err)
				}
			} else if err != nil || len(ds) != 1 || !strings.Contains(ds[0].Message, tc.message) || ds[0].URI != "nodes.yaml" {
				t.Fatalf("diagnostics %+v, error %v", ds, err)
			}
			if !reflect.DeepEqual(before, tc.root) {
				t.Fatal("normalization modified caller nodes")
			}
		})
	}
}

func TestNodeNumericSpellings(t *testing.T) {
	for _, tc := range []struct{ tag, value, want string }{
		{"!!float", "+.5", "0.5"},
		{"!!float", "-.5", "-0.5"},
		{"!!float", "1.", "1.0"},
		{"!!float", "1_000.5e+2", "1000.5e+2"},
		{"!!int", "-0b101", "-5"},
		{"!!int", "0o17", "15"},
	} {
		root := &yaml.Node{Kind: yaml.ScalarNode, Tag: tc.tag, Value: tc.value}
		v, _, ds, err := normalize(context.Background(), Document{Root: root}, DefaultLimits())
		if err != nil || len(ds) != 0 || v != json.Number(tc.want) {
			t.Fatalf("%s: %v %+v %v", tc.value, v, ds, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err := normalize(ctx, Document{Root: &yaml.Node{}}, DefaultLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestNodeMergePrecedenceAndLocations(t *testing.T) {
	data := "first: &first {list: [{child: first}], explicit: inherited}\nsecond: &second {list: [second], other: second}\nmerged: {<<: [*first, *second], explicit: authored}\n"
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(data), &root); err != nil {
		t.Fatal(err)
	}
	v, locations, ds, err := normalize(context.Background(), Document{Root: &root}, DefaultLimits())
	if err != nil || len(ds) != 0 {
		t.Fatalf("%+v %v", ds, err)
	}
	m := object(object(v)["merged"])
	if m["explicit"] != "authored" || m["other"] != "second" || object(array(m["list"])[0])["child"] != "first" {
		t.Fatalf("merge precedence: %+v", m)
	}
	if locations["/merged/list/0/child"].value != locations["/merged/<</0/list/0/child"].value || locations["/merged/list/0/child"].value == nil {
		t.Fatal("merged child location lost")
	}
}

func TestNodeLimitsAtEachBoundary(t *testing.T) {
	str := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	for _, tc := range []struct {
		root                      *yaml.Node
		nodes, bytes, diagnostics int
	}{
		{&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{str("a"), str("b")}}, 1, 100, 100},
		{&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{str("longkey"), str("b")}}, 100, 2, 100},
		{&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!int", Value: "1"}, str("a")}}, 100, 100, 0},
		{&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{str("a"), str("b"), str("a"), str("c")}}, 100, 100, 0},
		{&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!merge"}, str("bad")}}, 100, 100, 0},
		{&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!merge"}, {Kind: yaml.MappingNode}, {Kind: yaml.ScalarNode, Tag: "!!merge"}, {Kind: yaml.MappingNode}}}, 100, 100, 0},
		{&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!merge"}, nil}}, 100, 100, 100},
	} {
		l := DefaultLimits()
		l.MaxNodes, l.MaxBytes, l.MaxDiagnostics = tc.nodes, tc.bytes, tc.diagnostics
		_, _, _, err := normalize(context.Background(), Document{Root: tc.root}, l)
		var tool *Error
		if !errors.As(err, &tool) || (tool.Kind != ErrorLimit && tool.Kind != ErrorInput) {
			t.Fatalf("expected bounded failure: %v", err)
		}
	}
}
