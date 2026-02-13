// Copyright 2026 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/message"
)

type stubErrorKind struct {
	msg string
}

func (s stubErrorKind) KeywordPath() []string {
	return nil
}

func (s stubErrorKind) LocalizedString(_ *message.Printer) string {
	return s.msg
}

func adjustedLine(node *yaml.Node) int {
	line := node.Line
	if (node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode) && line > 0 {
		line--
	}
	return line
}

func TestExtractBasicErrors_FallbackRenderedSchema_AdjustsLines(t *testing.T) {
	renderedSchema := []byte(`type: object
required:
  - item
properties:
  item:
    type: object`)
	payload := []byte(`{"item":{}}`)

	flatErrors := []jsonschema.OutputUnit{
		{
			KeywordLocation:         "/properties/item",
			AbsoluteKeywordLocation: "#/properties/item",
			InstanceLocation:        "/item",
			Error: &jsonschema.OutputError{
				Kind: stubErrorKind{msg: "item is invalid"},
			},
		},
		{
			KeywordLocation:         "/required",
			AbsoluteKeywordLocation: "#/required",
			InstanceLocation:        "/item",
			Error: &jsonschema.OutputError{
				Kind: stubErrorKind{msg: "required is invalid"},
			},
		},
	}

	failures := extractBasicErrors(flatErrors, renderedSchema, nil, map[string]any{"item": map[string]any{}}, payload, nil, nil)
	assert.Len(t, failures, 2)

	var docNode yaml.Node
	err := yaml.Unmarshal(renderedSchema, &docNode)
	assert.NoError(t, err)
	assert.NotEmpty(t, docNode.Content)

	mappingNode := LocateSchemaPropertyNodeByJSONPath(docNode.Content[0], "/properties/item")
	sequenceNode := LocateSchemaPropertyNodeByJSONPath(docNode.Content[0], "/required")

	assert.NotNil(t, mappingNode)
	assert.NotNil(t, sequenceNode)
	assert.Equal(t, adjustedLine(mappingNode), failures[0].Line)
	assert.Equal(t, mappingNode.Column, failures[0].Column)
	assert.Equal(t, adjustedLine(sequenceNode), failures[1].Line)
	assert.Equal(t, sequenceNode.Column, failures[1].Column)
}
