// Copyright 2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"testing"

	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
	"github.com/santhosh-tekuri/jsonschema/v6"
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

	failures := extractBasicErrors(
		flatErrors,
		renderedSchema,
		nil,
		nil,
		map[string]any{"item": map[string]any{}},
		payload,
		nil,
		nil,
	)
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

func TestExtractBasicErrors_UsesExternalResourceNodeForAbsoluteKeywordLocation(t *testing.T) {
	renderedSchema := []byte(`components:
  schemas:
    Entry:
      $ref: './models.yaml#/components/schemas/External'`)
	payload := []byte(`{"id":42}`)

	var entryRoot yaml.Node
	assert.NoError(t, yaml.Unmarshal(renderedSchema, &entryRoot))

	var externalRoot yaml.Node
	assert.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    External:
      type: object
      properties:
        id:
          type: string`), &externalRoot))

	flatErrors := []jsonschema.OutputUnit{
		{
			KeywordLocation:         "/components/schemas/Entry/$ref/properties/id/type",
			AbsoluteKeywordLocation: "file:///tmp/models.yaml#/components/schemas/External/properties/id/type",
			InstanceLocation:        "/id",
			Error: &jsonschema.OutputError{
				Kind: stubErrorKind{msg: "got number, want string"},
			},
		},
	}

	failures := extractBasicErrors(
		flatErrors,
		renderedSchema,
		&entryRoot,
		map[string]*yaml.Node{"file:///tmp/models.yaml": &externalRoot},
		map[string]any{"id": float64(42)},
		payload,
		nil,
		nil,
	)

	require.Len(t, failures, 1)
	located := LocateSchemaPropertyNodeByJSONPath(externalRoot.Content[0], "/components/schemas/External/properties/id/type")
	require.NotNil(t, located)
	assert.Equal(t, located.Line, failures[0].Line)
	assert.Equal(t, located.Column, failures[0].Column)
}

func TestExtractBasicErrors_IgnoresGenericSchemaNoise(t *testing.T) {
	failures := extractBasicErrors(
		[]jsonschema.OutputUnit{
			{
				KeywordLocation: "/anyOf",
				Error: &jsonschema.OutputError{
					Kind: stubErrorKind{msg: "anyOf failed, none matched"},
				},
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	assert.Empty(t, failures)
}
