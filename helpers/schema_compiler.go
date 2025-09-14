package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/openapi_vocabulary"
)

// ConfigureCompiler configures a JSON Schema compiler with the desired behavior.
func ConfigureCompiler(c *jsonschema.Compiler, o *config.ValidationOptions) {
	if o == nil {
		// Sanity
		return
	}

	// nil is the default so this is OK.
	c.UseRegexpEngine(o.RegexEngine)

	// Enable Format assertions if required.
	if o.FormatAssertions {
		c.AssertFormat()
	}

	// Content Assertions
	if o.ContentAssertions {
		c.AssertContent()
	}

	// Register custom formats
	for n, v := range o.Formats {
		c.RegisterFormat(&jsonschema.Format{
			Name:     n,
			Validate: v,
		})
	}
}

// NewCompilerWithOptions mints a new JSON schema compiler with custom configuration.
func NewCompilerWithOptions(o *config.ValidationOptions) *jsonschema.Compiler {
	// Build it
	c := jsonschema.NewCompiler()

	// Configure it
	ConfigureCompiler(c, o)

	// Return it
	return c
}

// NewCompiledSchema establishes a programmatic representation of a JSON Schema document that is used for validation.
// Defaults to OpenAPI 3.1+ behavior (strict JSON Schema compliance).
func NewCompiledSchema(name string, jsonSchema []byte, o *config.ValidationOptions) (*jsonschema.Schema, error) {
	return NewCompiledSchemaWithVersion(name, jsonSchema, o, 3.1)
}

// NewCompiledSchemaWithVersion establishes a programmatic representation of a JSON Schema document that is used for validation.
// The version parameter determines which OpenAPI keywords are allowed:
// - version 3.0: Allows OpenAPI 3.0 keywords like 'nullable'
// - version 3.1+: Rejects OpenAPI 3.0 keywords like 'nullable' (strict JSON Schema compliance)
func NewCompiledSchemaWithVersion(name string, jsonSchema []byte, o *config.ValidationOptions, version float32) (*jsonschema.Schema, error) {
	// Fake-Up a resource name for the schema
	resourceName := fmt.Sprintf("%s.json", name)

	// Establish a compiler with the desired configuration
	compiler := NewCompilerWithOptions(o)
	compiler.UseLoader(NewCompilerLoader())

	// For OpenAPI 3.1+, register vocabulary to validate and reject OpenAPI 3.0 keywords
	if o != nil && o.OpenAPIMode && version >= 3.05 { // Use 3.05 to avoid floating point precision issues
		vocab := openapi_vocabulary.NewOpenAPIVocabulary(openapi_vocabulary.Version31)
		compiler.RegisterVocabulary(vocab)
		compiler.AssertVocabs()
	}

	// For OpenAPI 3.0, transform schemas to JSON Schema compatible format
	if o != nil && o.OpenAPIMode && version < 3.05 { // Use 3.05 to avoid floating point precision issues
		// Register vocabulary for structure validation (without rejection)
		vocab := openapi_vocabulary.NewOpenAPIVocabulary(openapi_vocabulary.Version30)
		compiler.RegisterVocabulary(vocab)
		compiler.AssertVocabs()

		// Transform nullable keywords to proper JSON Schema format
		jsonSchema = transformOpenAPI30Schema(jsonSchema)
	}

	// Decode the JSON Schema into a JSON blob.
	decodedSchema, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonSchema))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON schema: %w", err)
	}

	// Give our schema to the compiler.
	if err = compiler.AddResource(resourceName, decodedSchema); err != nil {
		return nil, fmt.Errorf("failed to add resource to schema compiler: %w", err)
	}

	// Try to compile it.
	jsch, err := compiler.Compile(resourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JSON schema: %w", err)
	}

	// Done.
	return jsch, nil
}

// transformOpenAPI30Schema transforms OpenAPI 3.0 schemas to JSON Schema compatible format
// This specifically handles the nullable keyword by converting it to proper type arrays
func transformOpenAPI30Schema(jsonSchema []byte) []byte {
	var schema map[string]interface{}
	if err := json.Unmarshal(jsonSchema, &schema); err != nil {
		// If we can't parse it, return as-is
		return jsonSchema
	}

	transformed := transformNullableInSchema(schema)

	result, err := json.Marshal(transformed)
	if err != nil {
		// If we can't marshal the result, return original
		return jsonSchema
	}

	return result
}

// transformNullableInSchema recursively transforms nullable keywords in a schema object
func transformNullableInSchema(schema interface{}) interface{} {
	switch s := schema.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})

		// Copy all properties first
		for key, value := range s {
			result[key] = transformNullableInSchema(value)
		}

		// Check if this schema has nullable: true
		if nullable, ok := s["nullable"]; ok {
			if nullableBool, ok := nullable.(bool); ok && nullableBool {
				// Transform the schema to support null values
				return transformNullableSchema(result)
			}
		}

		return result

	case []interface{}:
		result := make([]interface{}, len(s))
		for i, item := range s {
			result[i] = transformNullableInSchema(item)
		}
		return result

	default:
		return schema
	}
}

// transformNullableSchema transforms a schema with nullable: true to JSON Schema compatible format
func transformNullableSchema(schema map[string]interface{}) map[string]interface{} {
	// Remove the nullable keyword
	delete(schema, "nullable")

	// Get the current type
	currentType, hasType := schema["type"]

	if hasType {
		// If there's already a type, convert it to include null
		switch t := currentType.(type) {
		case string:
			// Convert "string" to ["string", "null"]
			schema["type"] = []interface{}{t, "null"}
		case []interface{}:
			// If it's already an array, add null if not present
			found := false
			for _, item := range t {
				if str, ok := item.(string); ok && str == "null" {
					found = true
					break
				}
			}
			if !found {
				newTypes := make([]interface{}, len(t)+1)
				copy(newTypes, t)
				newTypes[len(t)] = "null"
				schema["type"] = newTypes
			}
		}
	} else {
		// If no type specified, this schema can be anything including null
		// We don't need to do anything special here
	}

	return schema
}

