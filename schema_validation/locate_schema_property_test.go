// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"testing"

	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestLocateSchemaPropertyNodeByJSONPath_BadNode(t *testing.T) {
	assert.Nil(t, LocateSchemaPropertyNodeByJSONPath(nil, ""))
}

func TestLocateSchemaPropertyNode_EmptyPath(t *testing.T) {
	assert.Nil(t, locateSchemaPropertyNode(nil, ""))
}

func TestLocateSchemaPropertyNodeByJSONPathFallback_AbsoluteKeywordLocation(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer`), &root))

	located := LocateSchemaPropertyNodeByJSONPathFallback(
		root.Content[0],
		"/items/$ref/properties/id/type",
		"https://libopenapi-validator.local/schema/root.json#/components/schemas/Pet/properties/id/type",
	)

	require.NotNil(t, located)
	assert.Equal(t, "integer", located.Value)
}

func TestLocateSchemaPropertyNodeByJSONPath_PreservesHashInLocalPointerNames(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    Model#v1#beta:
      type: object
      properties:
        id#value:
          type: string`), &root))

	located := LocateSchemaPropertyNodeByJSONPath(
		root.Content[0],
		"/components/schemas/Model#v1#beta/properties/id#value/type",
	)

	require.NotNil(t, located)
	assert.Equal(t, "string", located.Value)
}

func TestLocateSchemaPropertyNodeByJSONPathWithResources_UsesExternalResourceNode(t *testing.T) {
	var entry yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    Entry:
      $ref: './models.yaml#/components/schemas/Model#v1#beta'`), &entry))

	var external yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    Model#v1#beta:
      type: object
      properties:
        id:
          type: integer`), &external))

	located := LocateSchemaPropertyNodeByJSONPathWithResources(
		entry.Content[0],
		map[string]*yaml.Node{"/tmp/models.yaml": &external},
		"/components/schemas/Entry/$ref/properties/id/type",
		"file:///tmp/models.yaml#/components/schemas/Model#v1#beta/properties/id/type",
	)

	require.NotNil(t, located)
	assert.Equal(t, "integer", located.Value)
}

func TestLocateSchemaPropertyNodeByJSONPathWithResources_UsesEntryResourceForLocalPointer(t *testing.T) {
	var entry yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    Entry:
      type: object
      properties:
        id:
          type: string`), &entry))

	var fallback yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    Entry:
      type: object
      properties:
        id:
          type: integer`), &fallback))

	located := LocateSchemaPropertyNodeByJSONPathWithResources(
		fallback.Content[0],
		map[string]*yaml.Node{"": entry.Content[0]},
		"/components/schemas/Missing/properties/id/type",
		"#/components/schemas/Entry/properties/id/type",
	)

	require.NotNil(t, located)
	assert.Equal(t, "string", located.Value)
}

func TestSplitKeywordLocation_DoesNotTreatLocalPointerHashAsResource(t *testing.T) {
	resourceName, pointer := splitKeywordLocation("/components/schemas/Model#v1#beta/properties/id/type")

	assert.Empty(t, resourceName)
	assert.Equal(t, "/components/schemas/Model#v1#beta/properties/id/type", pointer)

	resourceName, pointer = splitKeywordLocation("https://example.com/models.yaml#/components/schemas/Model#v1#beta")

	assert.Equal(t, "https://example.com/models.yaml", resourceName)
	assert.Equal(t, "#/components/schemas/Model#v1#beta", pointer)

	resourceName, pointer = splitKeywordLocation("schema")

	assert.Empty(t, resourceName)
	assert.Equal(t, "schema", pointer)
}

func TestLookupResourceNode_EdgeCases(t *testing.T) {
	assert.Nil(t, lookupResourceNode(nil, "https://example.com/models.yaml"))
	assert.Nil(t, lookupResourceNode(nil, "file:///tmp/%zz"))

	escapedPathNode := &yaml.Node{Kind: yaml.MappingNode}
	located := lookupResourceNode(
		map[string]*yaml.Node{"/tmp/models with space.yaml": escapedPathNode},
		"file:///tmp/models%20with%20space.yaml",
	)

	assert.Same(t, escapedPathNode, located)

	canonicalPathNode := &yaml.Node{Kind: yaml.MappingNode}
	located = lookupResourceNode(
		map[string]*yaml.Node{"file:///tmp/models.yaml": canonicalPathNode},
		"file://localhost/tmp/models.yaml",
	)

	assert.Same(t, canonicalPathNode, located)

	parsedStringNode := &yaml.Node{Kind: yaml.MappingNode}
	located = lookupResourceNode(
		map[string]*yaml.Node{"file:///tmp/models%20with%20space.yaml": parsedStringNode},
		"file:///tmp/models with space.yaml",
	)

	assert.Same(t, parsedStringNode, located)
}
