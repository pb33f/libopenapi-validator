// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestRenderSchemaForValidation_DirectionalRequiredProperties(t *testing.T) {
	schema := parseDirectionalTestSchema(t, `type: object
required:
  - id
  - name
  - password
properties:
  id:
    type: string
    readOnly: true
  name:
    type: string
  password:
    type: string
    writeOnly: true`)

	for _, tc := range []struct {
		name     string
		purpose  SchemaValidationPurpose
		expected []string
	}{
		{
			name:     "generic keeps all required properties",
			purpose:  SchemaValidationPurposeGeneric,
			expected: []string{"id", "name", "password"},
		},
		{
			name:     "request removes readOnly required properties",
			purpose:  SchemaValidationPurposeRequestBody,
			expected: []string{"name", "password"},
		},
		{
			name:     "response removes writeOnly required properties",
			purpose:  SchemaValidationPurposeResponseBody,
			expected: []string{"id", "name"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := RenderSchemaForValidation(schema, tc.purpose)
			require.NoError(t, err)
			require.NotNil(t, rendered)

			assert.Equal(t, tc.expected, renderedRequired(t, rendered.RenderedJSON))
		})
	}
}

func TestSchemaCacheKey_DirectionalKeysAreDistinct(t *testing.T) {
	const schemaHash = uint64(100)
	const version = float32(3.1)

	genericKey := SchemaCacheKey(schemaHash, version, SchemaValidationPurposeGeneric)
	requestKey := SchemaCacheKey(schemaHash, version, SchemaValidationPurposeRequestBody)
	responseKey := SchemaCacheKey(schemaHash, version, SchemaValidationPurposeResponseBody)
	request30Key := SchemaCacheKey(schemaHash, 3.0, SchemaValidationPurposeRequestBody)

	assert.Equal(t, schemaHash, genericKey)
	assert.NotEqual(t, genericKey, requestKey)
	assert.NotEqual(t, genericKey, responseKey)
	assert.NotEqual(t, requestKey, responseKey)
	assert.NotEqual(t, requestKey, request30Key)
}

func TestRenderSchemaForValidation_EdgeCases(t *testing.T) {
	rendered, err := RenderSchemaForValidation(nil, SchemaValidationPurposeRequestBody)
	require.NoError(t, err)
	assert.Nil(t, rendered)

	schema := parseDirectionalSpecSchema(t, `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Error:
      type: object
      properties:
        details:
          type: array
          items:
            $ref: '#/components/schemas/Error'`, "Error")

	rendered, err = RenderSchemaForValidation(schema, SchemaValidationPurposeRequestBody)
	require.Error(t, err)
	require.NotNil(t, rendered)
}

func TestRenderSchemaBytesForValidation_Errors(t *testing.T) {
	rendered, err := renderSchemaBytesForValidation([]byte(":\n"), SchemaValidationPurposeRequestBody)
	require.Error(t, err)
	assert.Nil(t, rendered)
	assert.Contains(t, err.Error(), "schema render decode failed")
}

func TestRenderSchemaBytesForValidation_RemovesEmptyRequired(t *testing.T) {
	rendered, err := renderSchemaBytesForValidation([]byte(`type: object
required:
  - id
properties:
  id:
    type: string
    readOnly: true
`), SchemaValidationPurposeRequestBody)
	require.NoError(t, err)
	require.NotNil(t, rendered)

	assert.Nil(t, renderedRequired(t, rendered.RenderedJSON))
}

func TestRenderSchemaBytesForValidation_PrunesPrefixItems(t *testing.T) {
	rendered, err := renderSchemaBytesForValidation([]byte(`type: array
prefixItems:
  - type: object
    required:
      - id
    properties:
      id:
        type: string
        readOnly: true
`), SchemaValidationPurposeRequestBody)
	require.NoError(t, err)
	require.NotNil(t, rendered)

	var renderedMap map[string]any
	require.NoError(t, json.Unmarshal(rendered.RenderedJSON, &renderedMap))
	prefixItems := renderedMap["prefixItems"].([]any)
	firstItem := prefixItems[0].(map[string]any)
	assert.NotContains(t, firstItem, "required")
}

func TestDirectionalSchemaHelpers_EdgeCases(t *testing.T) {
	pruneDirectionalRequired(nil, SchemaValidationPurposeRequestBody)
	pruneDirectionalRequired(&yaml.Node{Kind: yaml.ScalarNode, Value: "scalar"}, SchemaValidationPurposeRequestBody)

	pruneRequiredAtSchema(&yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "required"},
			{Kind: yaml.SequenceNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "id"}}},
		},
	}, SchemaValidationPurposeRequestBody)

	pruneRequiredAtSchema(&yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "required"},
			{Kind: yaml.SequenceNode, Content: []*yaml.Node{nil, {Kind: yaml.MappingNode}}},
			{Kind: yaml.ScalarNode, Value: "properties"},
			{Kind: yaml.MappingNode},
		},
	}, SchemaValidationPurposeRequestBody)

	assert.False(t, propertySchemaHasDirectionalAnnotation(nil, SchemaValidationPurposeRequestBody))
	assert.False(t, propertySchemaHasDirectionalAnnotation(&yaml.Node{Kind: yaml.ScalarNode}, SchemaValidationPurposeRequestBody))

	composed := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "oneOf"},
			{
				Kind: yaml.SequenceNode,
				Content: []*yaml.Node{{
					Kind: yaml.MappingNode,
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Value: "writeOnly"},
						{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
					},
				}},
			},
		},
	}
	assert.True(t, propertySchemaHasDirectionalAnnotation(composed, SchemaValidationPurposeResponseBody))

	index, value := mappingPair(nil, "missing")
	assert.Equal(t, -1, index)
	assert.Nil(t, value)

	removeMappingPair(nil, -1)
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "required"},
			{Kind: yaml.SequenceNode},
		},
	}
	removeMappingPair(node, 0)
	assert.Empty(t, node.Content)
}

func renderedRequired(t *testing.T, renderedJSON []byte) []string {
	t.Helper()

	var rendered map[string]any
	require.NoError(t, json.Unmarshal(renderedJSON, &rendered))

	required, ok := rendered["required"].([]any)
	if !ok {
		return nil
	}

	values := make([]string, 0, len(required))
	for _, item := range required {
		values = append(values, item.(string))
	}
	return values
}

func parseDirectionalTestSchema(t *testing.T, schemaYAML string) *base.Schema {
	t.Helper()

	return parseDirectionalSpecSchema(t, fmt.Sprintf(`openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    TestSchema:
%s`, indentDirectionalSchemaLines(schemaYAML, "      ")), "TestSchema")
}

func parseDirectionalSpecSchema(t *testing.T, spec string, schemaName string) *base.Schema {
	t.Helper()

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	schema := model.Model.Components.Schemas.GetOrZero(schemaName)
	require.NotNil(t, schema)
	return schema.Schema()
}

func indentDirectionalSchemaLines(s string, indent string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
