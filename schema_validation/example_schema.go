// Copyright 2026 Dave Shanley / Quobix / Princess Beef Heavy Industries, LLC
// SPDX-License-Identifier: MIT AND Apache-2.0
// Schema preparation includes adaptations from @openlint/ruleset-openapi.
// Copyright 2018 Stoplight, Inc. and 2026 OpenLint. See OPENLINT-OPENAPI-LICENSE.

package schema_validation

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/jsonschema/v6"
)

// ValidateYAMLNode checks a parsed document or value against a compiled schema.
// Validation failures retain their *jsonschema.ValidationError type and instance location.
func ValidateYAMLNode(node *yaml.Node, compiled *jsonschema.Schema) error {
	if node == nil || compiled == nil {
		return fmt.Errorf("validation requires a YAML node and a compiled schema")
	}
	value, err := DecodeYAMLValue(node)
	if err != nil {
		return err
	}
	return compiled.Validate(value)
}

// DecodeYAMLValue returns the JSON data represented by a parsed YAML node.
// Scalar object keys and timestamps retain their authored spelling. Aliases and
// merge keys follow the YAML decoder's normal checks. The source is read-only.
func DecodeYAMLValue(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, errors.New("missing YAML node")
	}
	normalized, err := jsonInputNode(node, make(map[*yaml.Node]*yaml.Node))
	if err != nil {
		return nil, err
	}
	var value any
	err = normalized.Decode(&value)
	return value, err
}

// Copy only paths containing a non-string key or a timestamp. Cache alias
// targets during traversal so repeated references never expand the graph.
func jsonInputNode(n *yaml.Node, aliases map[*yaml.Node]*yaml.Node) (*yaml.Node, error) {
	if n.Kind == yaml.AliasNode {
		if n.Alias == nil {
			return nil, errors.New("missing YAML alias target")
		}
		target, seen := aliases[n.Alias]
		if !seen {
			aliases[n.Alias] = nil
			var err error
			target, err = jsonInputNode(n.Alias, aliases)
			if err != nil {
				return nil, err
			}
			aliases[n.Alias] = target
		}
		if target == nil {
			return nil, errors.New("cyclic YAML alias")
		}
		if target != n.Alias {
			copy := *n
			copy.Alias = target
			return &copy, nil
		}
		return n, nil
	}
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!timestamp" {
		copy := *n
		copy.Tag = "!!str"
		return &copy, nil
	}
	out := n
	for i, child := range n.Content {
		var next *yaml.Node
		if n.Kind == yaml.MappingNode && i%2 == 0 && child.ShortTag() != "!!merge" {
			key := child
			if key.Kind == yaml.AliasNode {
				key = key.Alias
			}
			if key == nil || key.Kind != yaml.ScalarNode {
				return nil, errors.New("JSON object keys must be scalars")
			}
			next = key
			if key.ShortTag() != "!!str" {
				copy := *key
				copy.Tag = "!!str"
				next = &copy
			}
		} else {
			var err error
			next, err = jsonInputNode(child, aliases)
			if err != nil {
				return nil, err
			}
		}
		if next != child {
			if out == n {
				copy := *n
				copy.Content = slices.Clone(n.Content)
				out = &copy
			}
			out.Content[i] = next
		}
	}
	return out, nil
}

type exampleSchemaKey struct {
	example, swagger bool
	purpose          SchemaValidationPurpose
	draft            *jsonschema.Draft
	digest           [sha256.Size]byte
}

type compiledExample func() (*jsonschema.Schema, error)

// ExampleSchemaCache shares compiled inline example schemas by dialect and prepared content.
// Its zero value is ready for concurrent use. External resources are never loaded.
type ExampleSchemaCache struct {
	mu      sync.Mutex
	schemas map[exampleSchemaKey]compiledExample
}

// Compile prepares a YAML schema for generic, request, or response examples.
// swagger enables the Swagger 2.0 x-nullable and parameter required conventions.
func (c *ExampleSchemaCache) Compile(node *yaml.Node, draft *jsonschema.Draft, purpose SchemaValidationPurpose, swagger bool) (*jsonschema.Schema, error) {
	if node == nil || draft == nil {
		return nil, fmt.Errorf("example validation requires a schema and a draft")
	}
	schema, err := DecodeYAMLValue(node)
	if err != nil {
		return nil, err
	}
	return c.compile(schema, exampleSchemaKey{draft: draft, example: true, purpose: purpose, swagger: swagger})
}

// CompileSchema compiles a JSON-compatible inline schema under the supplied
// default dialect, sharing the same offline cache as Compile.
func (c *ExampleSchemaCache) CompileSchema(schema any, draft *jsonschema.Draft) (*jsonschema.Schema, error) {
	return c.compile(schema, exampleSchemaKey{draft: draft})
}

func (c *ExampleSchemaCache) compile(schema any, key exampleSchemaKey) (*jsonschema.Schema, error) {
	if key.draft == nil {
		return nil, errors.New("missing default dialect")
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	key.digest = sha256.Sum256(data)
	c.mu.Lock()
	cached := c.schemas[key]
	if cached == nil {
		const maxSchemas = 1024
		if c.schemas == nil {
			c.schemas = make(map[exampleSchemaKey]compiledExample)
		} else if len(c.schemas) >= maxSchemas {
			clear(c.schemas)
		}
		cached = sync.OnceValues(func() (*jsonschema.Schema, error) {
			return key.compileResource(data)
		})
		c.schemas[key] = cached
	}
	c.mu.Unlock()
	return cached()
}

func (key exampleSchemaKey) compileResource(data []byte) (*jsonschema.Schema, error) {
	resource, err := key.decodeResource(data)
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	compiler.DefaultDraft(key.draft)
	const id = "urn:example:schema"
	var compiled *jsonschema.Schema
	err = compiler.AddResource(id, resource)
	if err == nil {
		compiled, err = compiler.Compile(id)
	}
	return compiled, err
}

func (key exampleSchemaKey) decodeResource(data []byte) (any, error) {
	if !key.example {
		// The compiler can retain enum and const values. Give it an owned copy.
		return jsonschema.UnmarshalJSON(bytes.NewReader(data))
	}
	// Work on a private node on cache misses. Merges were expanded before encoding.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	n := schemaRootNode(&doc)
	if key.swagger {
		if i, v := mappingPair(n, "required"); v != nil && v.ShortTag() == "!!bool" {
			removeMappingPair(n, i)
		}
	}
	prepareExampleSchema(n, key.purpose, key.swagger, false)
	return DecodeYAMLValue(n)
}

// Only schema-bearing keywords are traversed. A property named "not" or an
// example containing "required" must not be interpreted as a schema keyword.
func prepareExampleSchema(node *yaml.Node, purpose SchemaValidationPurpose, swagger, negated bool) {
	if node.Kind != yaml.MappingNode {
		return
	}
	if boolMappingValue(node, "nullable") || swagger && boolMappingValue(node, "x-nullable") {
		if t := mappingValue(node, "type"); t != nil && t.ShortTag() == "!!str" && t.Value != "null" {
			value := *t
			*t = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{&value, {Kind: yaml.ScalarNode, Tag: "!!str", Value: "null"}}}
		}
	}
	if !negated {
		pruneRequiredProperties(node, purpose, directDirectionalAnnotation)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i].Value, node.Content[i+1]
		switch k {
		case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas", "dependencies":
			if v.Kind == yaml.MappingNode {
				for j := 1; j < len(v.Content); j += 2 {
					prepareExampleSchema(v.Content[j], purpose, swagger, negated)
				}
			}
		case "not", "if", "then", "else", "additionalProperties", "additionalItems", "propertyNames", "contains", "items", "allOf", "anyOf", "oneOf", "prefixItems", "unevaluatedProperties", "unevaluatedItems":
			if v.Kind == yaml.SequenceNode {
				for _, child := range v.Content {
					prepareExampleSchema(child, purpose, swagger, negated)
				}
			} else {
				prepareExampleSchema(v, purpose, swagger, negated != (k == "not"))
			}
		}
	}
}
