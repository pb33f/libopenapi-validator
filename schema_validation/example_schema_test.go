// Copyright 2026 Dave Shanley / Quobix / Princess Beef Heavy Industries, LLC
// SPDX-License-Identifier: MIT

package schema_validation

import (
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
