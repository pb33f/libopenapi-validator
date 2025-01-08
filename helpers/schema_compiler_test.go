package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
)

// A few simple JSON Schemas
const stringSchema = `{
  "type": "string",
  "format": "date",
  "minLength": 10
}`

const objectSchema = `{
  "type": "object",
  "title" : "Fish",
  "properties" : {
     "name" : {
	    "type": "string",
        "description": "The given name of the fish"
     },
     "species" : {
		"type" : "string",
		"enum" : [ "OTHER", "GUPPY", "PIKE", "BASS" ]
     }
  }
}`

func Test_SchemaWithNilOptions(t *testing.T) {
	jsch, err := NewCompiledSchema("test", []byte(stringSchema), nil)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_SchemaWithDefaultOptions(t *testing.T) {
	valOptions := config.NewValidationOptions()
	jsch, err := NewCompiledSchema("test", []byte(stringSchema), valOptions)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_SchemaWithOptions(t *testing.T) {
	valOptions := config.NewValidationOptions(config.WithFormatAssertions(), config.WithContentAssertions())

	jsch, err := NewCompiledSchema("test", []byte(stringSchema), valOptions)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_ObjectSchema(t *testing.T) {
	valOptions := config.NewValidationOptions()
	jsch, err := NewCompiledSchema("test", []byte(objectSchema), valOptions)

	require.NoError(t, err, "Failed to compile Schema")
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_ValidJSONSchemaWithInvalidContent(t *testing.T) {
	// An example of a dubious JSON Schema
	const badSchema = `{
  "type": "you-dont-know-me",
  "format": "date",
  "minLength": 10
}`

	jsch, err := NewCompiledSchema("test", []byte(badSchema), nil)

	assert.NotNil(t, err, "Expected an error to be thrown")
	assert.Nil(t, jsch, "invalid schema compiled!")
}

func Test_MalformedSONSchema(t *testing.T) {
	// An example of a JSON schema with malformed JSON
	const badSchema = `{
  "type": "you-dont-know-me",
  "format": "date"
  "minLength": 10
}`

	jsch, err := NewCompiledSchema("test", []byte(badSchema), nil)

	assert.NotNil(t, err, "Expected an error to be thrown")
	assert.Nil(t, jsch, "invalid schema compiled!")
}

func Test_ValidJSONSchemaWithIncompleteContent(t *testing.T) {
	// An example of a dJSON schema with references to non-existent stuff
	const incompleteSchema = `{
  "type": "object",
  "title" : "unresolvable",
  "properties" : {
     "name" : {
	    "type": "string",
     },
     "species" : {
      "$ref": "#/$defs/speciesEnum"
     }
  }
}`

	jsch, err := NewCompiledSchema("test", []byte(incompleteSchema), nil)

	assert.NotNil(t, err, "Expected an error to be thrown")
	assert.Nil(t, jsch, "invalid schema compiled!")
}
