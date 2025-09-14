// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package openapi_vocabulary

import (
	"fmt"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
)

func TestCoercion_Vocabulary_CompilationSuccess(t *testing.T) {
	// Test that coercion vocabulary compiles successfully for all scalar types
	testCases := []string{
		`{"type": "boolean"}`,
		`{"type": "number"}`,
		`{"type": "integer"}`,
		`{"type": ["boolean", "null"]}`,
		`{"type": "string"}`, // Should not get coercion extension
	}

	for i, schemaJSON := range testCases {
		t.Run(fmt.Sprintf("Schema_%d", i), func(t *testing.T) {
			schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
			assert.NoError(t, err)

			compiler := jsonschema.NewCompiler()
			compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, true))
			compiler.AssertVocabs()

			err = compiler.AddResource("test.json", schema)
			assert.NoError(t, err)

			// Should compile successfully
			compiledSchema, err := compiler.Compile("test.json")
			assert.NoError(t, err)
			assert.NotNil(t, compiledSchema)
		})
	}
}

func TestCoercion_Vocabulary_DisabledCompilation(t *testing.T) {
	// Test that vocabulary compiles successfully when coercion is disabled
	schemaJSON := `{"type": "boolean"}`

	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	compiler.RegisterVocabulary(NewOpenAPIVocabularyWithCoercion(Version30, false)) // Disabled
	compiler.AssertVocabs()

	err = compiler.AddResource("test.json", schema)
	assert.NoError(t, err)

	// Should compile successfully even with coercion disabled
	compiledSchema, err := compiler.Compile("test.json")
	assert.NoError(t, err)
	assert.NotNil(t, compiledSchema)
}
