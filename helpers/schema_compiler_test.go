package helpers

import (
	"testing"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/stretchr/testify/require"
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

	require.Nil(t, err, "Failed to compile Schema: %v", err)
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_SchemaWithDefaultOptions(t *testing.T) {
	valOptions := config.NewValidationOptions()
	jsch, err := NewCompiledSchema("test", []byte(stringSchema), valOptions)

	require.Nil(t, err, "Failed to compile Schema: %v", err)
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_SchemaWithOptions(t *testing.T) {
	valOptions := config.NewValidationOptions(config.WithFormatAssertions(), config.WithContentAssertions())

	jsch, err := NewCompiledSchema("test", []byte(stringSchema), valOptions)

	require.Nil(t, err, "Failed to compile Schema: %v", err)
	require.NotNil(t, jsch, "Did not return a compiled schema")
}

func Test_ObjectSchema(t *testing.T) {
	valOptions := config.NewValidationOptions()
	jsch, err := NewCompiledSchema("test", []byte(objectSchema), valOptions)

	require.Nil(t, err, "Failed to compile Schema: %v", err)
	require.NotNil(t, jsch, "Did not return a compiled schema")
}
