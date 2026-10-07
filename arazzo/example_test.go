// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo_test

import (
	"context"
	"fmt"
	"os"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi-validator/arazzo"
	upstream "github.com/pb33f/libopenapi/arazzo"
)

func exampleNode(name string) *yaml.Node {
	data, err := os.ReadFile("testdata/mixed-source/" + name)
	if err != nil {
		panic(err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		panic(err)
	}
	return &root
}

func ExampleValidate() {
	// The application supplies parsed source documents. Validation needs no
	// OpenAPI Doctor model and performs no retrieval or workflow execution.
	document := arazzo.Document{Root: exampleNode("workflow.yaml"), URI: "https://retrieval.test/main.yaml"}
	var sources []upstream.CandidateDocument
	for _, name := range []string{"api.yaml", "events.yaml", "flows.yaml"} {
		sources = append(sources, upstream.CandidateDocument{
			RootNode: exampleNode(name), RetrievalURI: "https://example.test/" + name,
		})
	}
	result, err := arazzo.Validate(context.Background(), document, arazzo.WithSources(sources...))
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Version, result.Valid(), result.Complete)
	// Output: 1.1 true true
}
