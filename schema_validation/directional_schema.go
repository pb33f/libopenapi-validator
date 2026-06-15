// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"fmt"
	"math"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/utils"
	"go.yaml.in/yaml/v4"
)

// SchemaValidationPurpose identifies the context in which a schema is compiled.
// Request and response bodies need distinct cache entries because readOnly and
// writeOnly annotations change required-property semantics by direction.
type SchemaValidationPurpose uint64

const (
	SchemaValidationPurposeGeneric SchemaValidationPurpose = iota
	SchemaValidationPurposeRequestBody
	SchemaValidationPurposeResponseBody
)

const schemaCachePurposeSalt uint64 = 0x9e3779b97f4a7c15

// RenderedValidationSchema contains a rendered schema and its JSON equivalent.
type RenderedValidationSchema struct {
	RenderedInline  []byte
	ReferenceSchema string
	RenderedJSON    []byte
	RenderedNode    *yaml.Node
}

// SchemaCacheKey returns a cache key for a schema compiled in a validation context.
func SchemaCacheKey(schemaHash uint64, version float32, purpose SchemaValidationPurpose) uint64 {
	if purpose == SchemaValidationPurposeGeneric {
		return schemaHash
	}
	versionBits := uint64(math.Float32bits(version))
	return schemaHash ^ (versionBits << 32) ^ versionBits ^ (uint64(purpose) * schemaCachePurposeSalt)
}

// RenderSchemaForValidation renders schema for the supplied validation purpose.
// For request bodies it removes readOnly properties from required lists, and for
// response bodies it removes writeOnly properties from required lists.
func RenderSchemaForValidation(schema *base.Schema, purpose SchemaValidationPurpose) (*RenderedValidationSchema, error) {
	if schema == nil {
		return nil, nil
	}

	renderCtx := base.NewInlineRenderContextForValidation()
	renderedInline, err := schema.RenderInlineWithContext(renderCtx)
	if err != nil {
		return &RenderedValidationSchema{
			RenderedInline:  renderedInline,
			ReferenceSchema: string(renderedInline),
		}, err
	}

	return renderSchemaBytesForValidation(renderedInline, purpose)
}

func renderSchemaBytesForValidation(renderedInline []byte, purpose SchemaValidationPurpose) (*RenderedValidationSchema, error) {
	renderedNode := new(yaml.Node)
	if err := yaml.Unmarshal(renderedInline, renderedNode); err != nil {
		return nil, fmt.Errorf("schema render decode failed: %w", err)
	}

	if len(renderedNode.Content) > 0 {
		pruneDirectionalRequired(renderedNode.Content[0], purpose)
	}

	if purpose != SchemaValidationPurposeGeneric {
		renderedInline, _ = yaml.Marshal(renderedNode)
	}

	renderedJSON, _ := utils.ConvertYAMLtoJSON(renderedInline)

	return &RenderedValidationSchema{
		RenderedInline:  renderedInline,
		ReferenceSchema: string(renderedInline),
		RenderedJSON:    renderedJSON,
		RenderedNode:    renderedNode,
	}, nil
}

func pruneDirectionalRequired(schemaNode *yaml.Node, purpose SchemaValidationPurpose) {
	if schemaNode == nil || schemaNode.Kind != yaml.MappingNode {
		return
	}

	pruneRequiredAtSchema(schemaNode, purpose)

	for _, key := range []string{"properties", "patternProperties", "$defs", "definitions", "dependentSchemas"} {
		if childMap := mappingValue(schemaNode, key); childMap != nil && childMap.Kind == yaml.MappingNode {
			for i := 1; i < len(childMap.Content); i += 2 {
				pruneDirectionalRequired(childMap.Content[i], purpose)
			}
		}
	}

	for _, key := range []string{"items", "contains", "additionalProperties", "unevaluatedProperties", "propertyNames", "not", "if", "then", "else"} {
		pruneDirectionalRequired(mappingValue(schemaNode, key), purpose)
	}

	for _, key := range []string{"prefixItems", "allOf", "anyOf", "oneOf"} {
		if childSeq := mappingValue(schemaNode, key); childSeq != nil && childSeq.Kind == yaml.SequenceNode {
			for _, item := range childSeq.Content {
				pruneDirectionalRequired(item, purpose)
			}
		}
	}
}

func pruneRequiredAtSchema(schemaNode *yaml.Node, purpose SchemaValidationPurpose) {
	if purpose != SchemaValidationPurposeRequestBody && purpose != SchemaValidationPurposeResponseBody {
		return
	}

	requiredIndex, requiredNode := mappingPair(schemaNode, "required")
	if requiredNode == nil || requiredNode.Kind != yaml.SequenceNode {
		return
	}
	propertiesNode := mappingValue(schemaNode, "properties")
	if propertiesNode == nil || propertiesNode.Kind != yaml.MappingNode {
		return
	}

	prunedRequired := make([]*yaml.Node, 0, len(requiredNode.Content))
	for _, item := range requiredNode.Content {
		if item == nil || item.Kind != yaml.ScalarNode {
			prunedRequired = append(prunedRequired, item)
			continue
		}
		propertySchema := mappingValue(propertiesNode, item.Value)
		if propertySchemaHasDirectionalAnnotation(propertySchema, purpose) {
			continue
		}
		prunedRequired = append(prunedRequired, item)
	}

	if len(prunedRequired) == 0 {
		removeMappingPair(schemaNode, requiredIndex)
		return
	}
	requiredNode.Content = prunedRequired
}

func propertySchemaHasDirectionalAnnotation(schemaNode *yaml.Node, purpose SchemaValidationPurpose) bool {
	if schemaNode == nil || schemaNode.Kind != yaml.MappingNode {
		return false
	}

	switch purpose {
	case SchemaValidationPurposeRequestBody:
		if boolMappingValue(schemaNode, "readOnly") {
			return true
		}
	case SchemaValidationPurposeResponseBody:
		if boolMappingValue(schemaNode, "writeOnly") {
			return true
		}
	}

	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if childSeq := mappingValue(schemaNode, key); childSeq != nil && childSeq.Kind == yaml.SequenceNode {
			for _, item := range childSeq.Content {
				if propertySchemaHasDirectionalAnnotation(item, purpose) {
					return true
				}
			}
		}
	}

	return false
}

func mappingPair(node *yaml.Node, key string) (int, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return -1, nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode != nil && keyNode.Value == key {
			return i, node.Content[i+1]
		}
	}
	return -1, nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	_, value := mappingPair(node, key)
	return value
}

func boolMappingValue(node *yaml.Node, key string) bool {
	value := mappingValue(node, key)
	if value == nil || value.Kind != yaml.ScalarNode {
		return false
	}
	return value.Tag == "!!bool" && value.Value == "true"
}

func removeMappingPair(node *yaml.Node, keyIndex int) {
	if node == nil || keyIndex < 0 || keyIndex+1 >= len(node.Content) {
		return
	}
	node.Content = append(node.Content[:keyIndex], node.Content[keyIndex+2:]...)
}
