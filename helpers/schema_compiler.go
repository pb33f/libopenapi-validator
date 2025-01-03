package helpers

import (
	"bytes"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/pb33f/libopenapi-validator/config"
)

// ConfigureCompiler configures a JSON Schema compiler with the desired behavior.
func ConfigureCompiler(c *jsonschema.Compiler, o *config.ValidationOptions) {
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
func NewCompiledSchema(name string, jsonSchema []byte, o *config.ValidationOptions) *jsonschema.Schema {
	// Establish a compiler with the desired configuration
	compiler := NewCompilerWithOptions(o)
	compiler.UseLoader(NewCompilerLoader())

	// Decode the JSON Schema into a JSON blob.
	decodedSchema, _ := jsonschema.UnmarshalJSON(bytes.NewReader(jsonSchema))
	_ = compiler.AddResource(fmt.Sprintf("%s.json", name), decodedSchema)

	// Try to compile it.
	jsch, _ := compiler.Compile(fmt.Sprintf("%s.json", name))

	// Done.
	return jsch
}
