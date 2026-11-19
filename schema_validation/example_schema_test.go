// Copyright 2026 Dave Shanley / Quobix / Princess Beef Heavy Industries, LLC
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/jsonschema/v6"
	"github.com/pb33f/testify/require"
)

func TestCompiledExampleValidation(t *testing.T) {
	parse := func(source string) *yaml.Node {
		t.Helper()
		var n yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(source), &n))
		return &n
	}
	schema := parse(`type: object
required: [id, secret]
properties:
  id: {type: integer, readOnly: true}
  secret: {type: string, writeOnly: true}`)
	var c ExampleSchemaCache
	request, err := c.Compile(schema, jsonschema.Draft4, SchemaValidationPurposeRequestBody, false)
	require.NoError(t, err)
	response, err := c.Compile(schema, jsonschema.Draft4, SchemaValidationPurposeResponseBody, false)
	require.NoError(t, err)
	value := parse("{secret: sample}")
	require.NoError(t, ValidateYAMLNode(value, request))
	var failure *jsonschema.ValidationError
	require.ErrorAs(t, ValidateYAMLNode(value, response), &failure)
	require.NoError(t, ValidateYAMLNode(parse("{id: 2}"), response))
	again, err := c.Compile(schema, jsonschema.Draft4, SchemaValidationPurposeRequestBody, false)
	require.NoError(t, err)
	require.Same(t, request, again)
	newer, err := c.Compile(schema, jsonschema.Draft2020, SchemaValidationPurposeRequestBody, false)
	require.NoError(t, err)
	require.NotSame(t, request, newer)
	// The same input can still be compiled for the opposite direction.
	require.Len(t, schema.Content[0].Content[3].Content, 2)
}

func TestExampleSchemaDialectAndResources(t *testing.T) {
	var c ExampleSchemaCache
	for _, tc := range []struct {
		schema  string
		draft   *jsonschema.Draft
		invalid bool
	}{
		{`{type: number, minimum: 3, exclusiveMinimum: true}`, jsonschema.Draft4, false},
		{`{type: number, minimum: 3, exclusiveMinimum: true}`, jsonschema.Draft2020, true},
		{`{$ref: 'file:///does-not-exist'}`, jsonschema.Draft7, true},
		{`{$ref: 'https://unavailable.invalid/schema'}`, jsonschema.Draft7, true},
	} {
		var n yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(tc.schema), &n))
		compiled, err := c.Compile(&n, tc.draft, SchemaValidationPurposeGeneric, false)
		if tc.invalid {
			require.Error(t, err)
			if tc.schema[1] == '$' {
				require.Contains(t, err.Error(), "no URLLoader")
			}
			continue
		}
		require.NoError(t, err)
		require.Error(t, compiled.Validate(3))
		require.NoError(t, compiled.Validate(4))
	}
}

func TestDecodeYAMLValuePreservesKeysAndDates(t *testing.T) {
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("01: &date 2026-11-15\n'1': *date\nnumber: &id 4\nmap: {? *id : text}\n"), &doc))
	got, err := DecodeYAMLValue(&doc)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"01": "2026-11-15", "1": "2026-11-15", "number": 4, "map": map[string]any{"4": "text"}}, got)
	require.Equal(t, "!!timestamp", doc.Content[0].Content[1].ShortTag())
}

func TestCompileExamplePreservesNegatedRequiredAndSwaggerNullable(t *testing.T) {
	var cache ExampleSchemaCache
	var n yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object
not:
  required: [id]
  properties:
    id: {type: string, readOnly: true}
properties:
  value: {type: integer, x-nullable: true}`), &n))
	sch, err := cache.Compile(&n, jsonschema.Draft4, SchemaValidationPurposeRequestBody, true)
	require.NoError(t, err)
	require.NoError(t, sch.Validate(map[string]any{"value": nil}))
	require.Error(t, sch.Validate(map[string]any{"id": "present"}))
}

func TestCompileExamplePreservesConditionalRequired(t *testing.T) {
	for _, direction := range []struct {
		name, annotation string
		purpose          SchemaValidationPurpose
	}{
		{"request", "readOnly", SchemaValidationPurposeRequestBody},
		{"response", "writeOnly", SchemaValidationPurposeResponseBody},
	} {
		predicate := fmt.Sprintf(`{required: [id], properties: {id: {type: string, %s: true}}}`, direction.annotation)
		for _, condition := range []struct{ name, schema string }{
			{"direct", predicate},
			{"allOf", "{allOf: [" + predicate + "]}"},
			{"doubleNot", "{not: {not: " + predicate + "}}"},
		} {
			t.Run(direction.name+"/"+condition.name, func(t *testing.T) {
				var node yaml.Node
				require.NoError(t, yaml.Unmarshal([]byte("type: object\nif: "+condition.schema+"\nthen: {required: [kind]}\nelse: {required: [fallback]}"), &node))
				original := cloneYAMLNode(&node)
				var cache ExampleSchemaCache
				compiled, err := cache.Compile(&node, jsonschema.Draft7, direction.purpose, false)
				require.NoError(t, err)
				require.NoError(t, compiled.Validate(map[string]any{"fallback": "ok"}))
				require.NoError(t, compiled.Validate(map[string]any{"id": "present", "kind": "ok"}))
				require.Error(t, compiled.Validate(map[string]any{}))
				require.Error(t, compiled.Validate(map[string]any{"id": "present", "fallback": "ok"}))
				require.Equal(t, original, &node)
			})
		}
	}
}

func TestCompileExampleNullableDialect(t *testing.T) {
	for _, tc := range []struct {
		name, schema string
		draft        *jsonschema.Draft
		swagger      bool
		acceptNull   bool
	}{
		{"openapi30", "{type: string, nullable: true}", jsonschema.Draft4, false, true},
		{"draft6", "{type: string, nullable: true}", jsonschema.Draft6, false, false},
		{"draft7", "{type: string, nullable: true}", jsonschema.Draft7, false, false},
		{"draft2019", "{type: string, nullable: true}", jsonschema.Draft2019, false, false},
		{"openapi31", "{type: string, nullable: true}", jsonschema.Draft2020, false, false},
		{"openapi31NullType", "{type: [string, 'null']}", jsonschema.Draft2020, false, true},
		{"swagger", "{type: string, x-nullable: true}", jsonschema.Draft4, true, true},
		{"extensionWithoutSwagger", "{type: string, x-nullable: true}", jsonschema.Draft4, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var node yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte("type: object\nproperties:\n  value: "+tc.schema), &node))
			var cache ExampleSchemaCache
			compiled, err := cache.Compile(&node, tc.draft, SchemaValidationPurposeGeneric, tc.swagger)
			require.NoError(t, err)
			err = compiled.Validate(map[string]any{"value": nil})
			if tc.acceptNull {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.NoError(t, compiled.Validate(map[string]any{"value": "ok"}))
			require.Error(t, compiled.Validate(map[string]any{"value": 1}))
		})
	}
}

func TestCompileExampleComposedDirectionalAnnotations(t *testing.T) {
	for _, composition := range []string{"allOf", "anyOf", "oneOf"} {
		t.Run(composition, func(t *testing.T) {
			var node yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(fmt.Sprintf(`type: object
required: [id, secret, name]
properties:
  id: {%s: [{allOf: [{type: string, readOnly: true}]}]}
  secret: {%s: [{allOf: [{type: string, writeOnly: true}]}]}
  name: {type: string}`, composition, composition)), &node))
			original := cloneYAMLNode(&node)
			var cache ExampleSchemaCache
			for _, purpose := range []SchemaValidationPurpose{SchemaValidationPurposeRequestBody, SchemaValidationPurposeResponseBody, SchemaValidationPurposeGeneric} {
				compiled, err := cache.Compile(&node, jsonschema.Draft2020, purpose, false)
				require.NoError(t, err)
				value := map[string]any{"id": "public", "secret": "private", "name": "example"}
				require.NoError(t, compiled.Validate(value))
				switch purpose {
				case SchemaValidationPurposeRequestBody:
					delete(value, "id")
				case SchemaValidationPurposeResponseBody:
					delete(value, "secret")
				}
				require.NoError(t, compiled.Validate(value))
				delete(value, "name")
				require.Error(t, compiled.Validate(value))
				require.Error(t, compiled.Validate(map[string]any{"name": "example"}))
				require.Error(t, compiled.Validate(map[string]any{"id": 1, "secret": 2, "name": "example"}))
			}
			require.Equal(t, original, &node)
		})
	}
}

func TestExampleSchemaNonFiniteNumbers(t *testing.T) {
	for _, value := range []string{".inf", "-.inf", ".nan"} {
		t.Run(value, func(t *testing.T) {
			var node yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte("type: object\nproperties:\n  value: {type: number, maximum: "+value+"}"), &node))
			var cache ExampleSchemaCache
			_, err := cache.Compile(&node, jsonschema.Draft2020, SchemaValidationPurposeGeneric, false)
			require.ErrorContains(t, err, "non-finite")
			require.ErrorContains(t, err, "JSON Schema")
			var unsupported *json.UnsupportedValueError
			require.ErrorAs(t, err, &unsupported)
		})
	}
}

func TestExampleSchemaCacheCustomLimit(t *testing.T) {
	for _, limit := range []int{2, 1025} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			cache := ExampleSchemaCache{MaxSchemas: limit}
			compiled := make([]*jsonschema.Schema, limit)
			for i := range limit {
				var err error
				compiled[i], err = cache.CompileSchema(map[string]any{"const": i}, jsonschema.Draft7)
				require.NoError(t, err)
			}
			// A second pass retains every schema, even above the default limit.
			for i := range limit {
				again, err := cache.CompileSchema(map[string]any{"const": i}, jsonschema.Draft7)
				require.NoError(t, err)
				require.Same(t, compiled[i], again)
			}
			// A new entry clears the full cache; existing pointers stay valid.
			_, err := cache.CompileSchema(true, jsonschema.Draft7)
			require.NoError(t, err)
			again, err := cache.CompileSchema(map[string]any{"const": 0}, jsonschema.Draft7)
			require.NoError(t, err)
			require.NotSame(t, compiled[0], again)
			require.NoError(t, compiled[0].Validate(0))
			require.Error(t, compiled[0].Validate(1))
		})
	}
}

func TestExampleSchemaLocalReferences(t *testing.T) {
	for _, example := range []bool{false, true} {
		t.Run(fmt.Sprintf("example=%t", example), func(t *testing.T) {
			var cache ExampleSchemaCache
			compile := func(source string) (*jsonschema.Schema, error) {
				t.Helper()
				var node yaml.Node
				require.NoError(t, yaml.Unmarshal([]byte(source), &node))
				if example {
					return cache.Compile(&node, jsonschema.Draft2020, SchemaValidationPurposeGeneric, false)
				}
				schema, err := DecodeYAMLValue(&node)
				require.NoError(t, err)
				return cache.CompileSchema(schema, jsonschema.Draft2020)
			}
			_, err := compile(`{$ref: '#/components/schemas/Pet'}`)
			require.ErrorContains(t, err, "json-pointer")
			compiled, err := compile(`$ref: '#/$defs/item'
$defs:
  item:
    type: object
    required: [name]
    properties:
      name: {type: string}
      next: {$ref: '#/$defs/item'}`)
			require.NoError(t, err)
			require.NoError(t, compiled.Validate(map[string]any{"name": "first", "next": map[string]any{"name": "second"}}))
			require.Error(t, compiled.Validate(map[string]any{"name": "first", "next": map[string]any{}}))
		})
	}
}

func TestCompileSchemaReusesFailures(t *testing.T) {
	var c ExampleSchemaCache
	invalid := map[string]any{"type": "not-a-type"}
	_, a := c.CompileSchema(invalid, jsonschema.Draft7)
	_, b := c.CompileSchema(invalid, jsonschema.Draft7)
	require.Error(t, a)
	require.Same(t, a, b)
}

func TestCachedInlineSchemaOwnsEnumValues(t *testing.T) {
	var c ExampleSchemaCache
	v := map[string]any{"v": "original"}
	schema := map[string]any{"enum": []any{v}}
	compiled, err := c.CompileSchema(schema, jsonschema.Draft7)
	require.NoError(t, err)
	v["v"] = "changed"
	require.NoError(t, compiled.Validate(map[string]any{"v": "original"}))
	require.Error(t, compiled.Validate(v))
}

func TestYAMLValidationInvalidInputs(t *testing.T) {
	var c ExampleSchemaCache
	compiled, err := c.CompileSchema(true, jsonschema.Draft7)
	require.NoError(t, err)
	require.Error(t, ValidateYAMLNode(nil, compiled))
	_, err = DecodeYAMLValue(nil)
	require.Error(t, err)
	_, err = c.Compile(nil, jsonschema.Draft7, SchemaValidationPurposeGeneric, false)
	require.Error(t, err)
	_, err = c.CompileSchema(true, nil)
	require.Error(t, err)
	_, err = c.CompileSchema(make(chan int), jsonschema.Draft7)
	require.Error(t, err)
	cyclicValue := map[string]any{}
	cyclicValue["cycle"] = cyclicValue
	_, err = c.CompileSchema(cyclicValue, jsonschema.Draft7)
	require.ErrorContains(t, err, "cycle")

	var cyclic, complexKey yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("cycle: &cycle [*cycle]"), &cyclic))
	require.NoError(t, yaml.Unmarshal([]byte("? [a,b]\n: value"), &complexKey))
	for _, n := range []*yaml.Node{&cyclic, &complexKey, {Kind: yaml.AliasNode}} {
		require.Error(t, ValidateYAMLNode(n, compiled))
		_, err := c.Compile(n, jsonschema.Draft7, SchemaValidationPurposeGeneric, false)
		require.Error(t, err)
	}
	for _, example := range []bool{false, true} {
		key := exampleSchemaKey{draft: jsonschema.Draft7, example: example}
		_, err := key.compileResource([]byte("{"), invalidExampleSchema{})
		require.Error(t, err)
	}
}

func TestMergedExampleAndSwaggerParameter(t *testing.T) {
	var node yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("source: &defaults {type: string}\nvalue: {<<: *defaults, required: true, allOf: [{minLength: 2}]}"), &node))
	var c ExampleSchemaCache
	schema, err := c.Compile(mappingValue(node.Content[0], "value"), jsonschema.Draft4, SchemaValidationPurposeGeneric, true)
	require.NoError(t, err)
	require.NoError(t, schema.Validate("ok"))
	require.Error(t, schema.Validate("x"))
	require.NoError(t, yaml.Unmarshal([]byte("true"), &node))
	boolean, err := c.Compile(&node, jsonschema.Draft7, SchemaValidationPurposeGeneric, false)
	require.NoError(t, err)
	require.NoError(t, boolean.Validate(nil))
}

func TestExampleSchemaCacheConcurrentReuseAndEviction(t *testing.T) {
	var c ExampleSchemaCache
	const workers = 8
	var wg sync.WaitGroup
	var schemas [workers]*jsonschema.Schema
	var errs [workers]error
	for i := range workers {
		wg.Go(func() {
			schemas[i], errs[i] = c.CompileSchema(true, jsonschema.Draft7)
		})
	}
	wg.Wait()
	for i := range workers {
		require.NoError(t, errs[i])
		require.Same(t, schemas[0], schemas[i])
	}

	for i := range 1024 {
		_, err := c.CompileSchema(map[string]any{"title": fmt.Sprint(i)}, jsonschema.Draft7)
		require.NoError(t, err)
	}
	again, err := c.CompileSchema(true, jsonschema.Draft7)
	require.NoError(t, err)
	require.NotSame(t, schemas[0], again)
	require.NoError(t, schemas[0].Validate("still usable"))
}

// The example compiler must propagate an error from the YAML encoder.
type invalidExampleSchema struct{}

func (invalidExampleSchema) MarshalYAML() (any, error) {
	return nil, fmt.Errorf("cannot encode test schema")
}

func TestExampleSchemaPreservesUnicodeAndNumbers(t *testing.T) {
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`{const: ["\u0085", 9007199254740993]}`), &doc))
	var cache ExampleSchemaCache
	compiled, err := cache.Compile(&doc, jsonschema.Draft7, SchemaValidationPurposeGeneric, false)
	require.NoError(t, err)
	require.NoError(t, compiled.Validate([]any{"\u0085", int64(9007199254740993)}))
	require.Error(t, compiled.Validate([]any{" ", int64(9007199254740993)}))
	require.Error(t, compiled.Validate([]any{"\u0085", int64(9007199254740992)}))
}
