package requests

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
)

func TestValidateRequestSchema(t *testing.T) {
	for name, tc := range map[string]struct {
		request                  *http.Request
		schemaYAML               string
		version                  float32
		assertValidRequestSchema assert.BoolAssertionFunc
		expectedErrorsCount      int
	}{
		"FailOnBooleanExclusiveMinimum": {
			request: postRequestWithBody(`{"exclusiveNumber": 10}`),
			schemaYAML: `type: object
properties:
  exclusiveNumber:
    type: number
    description: This number starts its journey where most numbers are too scared to begin!
    exclusiveMinimum: true
    minimum: !!float 10`,
			version:                  3.0,
			assertValidRequestSchema: assert.False,
			expectedErrorsCount:      1,
		},
		"PassWithCorrectExclusiveMinimum": {
			request: postRequestWithBody(`{"exclusiveNumber": 15}`),
			schemaYAML: `type: object
properties:
  exclusiveNumber:
    type: number
    description: This number is properly constrained by a numeric exclusive minimum.
    exclusiveMinimum: 12
    minimum: 12`,
			version:                  3.1,
			assertValidRequestSchema: assert.True,
			expectedErrorsCount:      0,
		},
		"PassWithValidStringType": {
			request: postRequestWithBody(`{"greeting": "Hello, world!"}`),
			schemaYAML: `type: object
properties:
  greeting:
    type: string
    description: A simple greeting
    example: "Hello, world!"`,
			version:                  3.1,
			assertValidRequestSchema: assert.True,
			expectedErrorsCount:      0,
		},
		"PassWithNullablePropertyInOpenAPI30": {
			request: postRequestWithBody(`{"name": "John", "middleName": null}`),
			schemaYAML: `type: object
properties:
  name:
    type: string
    description: User's first name
  middleName:
    type: string
    nullable: true
    description: User's middle name (optional)`,
			version:                  3.0,
			assertValidRequestSchema: assert.True,
			expectedErrorsCount:      0,
		},
		"PassWithNullablePropertyInOpenAPI31": {
			request: postRequestWithBody(`{"name": "John", "middleName": null}`),
			schemaYAML: `type: object
properties:
  name:
    type: string
    description: User's first name
  middleName:
    type: string
    nullable: true
    description: User's middle name (optional)`,
			version:                  3.1,
			assertValidRequestSchema: assert.False,
			expectedErrorsCount:      1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			schema := parseSchemaFromSpec(t, tc.schemaYAML, tc.version)

			valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
				Request: tc.request,
				Schema:  schema,
				Version: tc.version,
			})

			tc.assertValidRequestSchema(t, valid)
			assert.Len(t, errors, tc.expectedErrorsCount)
		})
	}
}

func TestInvalidMin(t *testing.T) {
	openAPIVersion := float32(3.0)
	schema := parseSchemaFromSpec(t, `type: object
properties:
  exclusiveNumber:
    type: number
    description: This number starts its journey where most numbers are too scared to begin!
    exclusiveMinimum: true
    minimum: 10`, openAPIVersion)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"exclusiveNumber": 13}`),
		Schema:  schema,
		Version: openAPIVersion,
	})

	assert.False(t, valid)
	assert.Len(t, errors, 1)
}

func TestValidateRequestSchema_CachePopulation(t *testing.T) {
	openAPIVersion := float32(3.1)
	schema := parseSchemaFromSpec(t, `type: object
properties:
  name:
    type: string`, openAPIVersion)

	// Create options with a cache
	opts := config.NewValidationOptions()
	require.NotNil(t, opts.SchemaCache)

	// First call should populate the cache
	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"name": "test"}`),
		Schema:  schema,
		Version: openAPIVersion,
		Options: []config.Option{config.WithExistingOpts(opts)},
	})

	assert.True(t, valid)
	assert.Len(t, errors, 0)

	// Verify cache was populated
	hash := schema.GoLow().Hash()
	cached, ok := opts.SchemaCache.Load(hash)
	assert.True(t, ok, "Schema should be in cache")
	assert.NotNil(t, cached, "Cached entry should not be nil")
	assert.NotNil(t, cached.CompiledSchema, "Compiled schema should be cached")
	assert.NotNil(t, cached.RenderedInline, "Rendered schema should be cached")
	assert.NotNil(t, cached.RenderedJSON, "JSON schema should be cached")
}

func TestValidateRequestSchema_NilSchema(t *testing.T) {
	// Test when schema is nil
	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"name": "test"}`),
		Schema:  nil,
		Version: 3.1,
	})

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "schema is nil", errors[0].Message)
	assert.Equal(t, "The schema to validate against is nil", errors[0].Reason)
}

func TestValidateRequestSchema_NilSchemaGoLow(t *testing.T) {
	// Test when schema.GoLow() is nil by creating a schema without low-level data
	schema := &base.Schema{} // Empty schema without GoLow() data

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"name": "test"}`),
		Schema:  schema,
		Version: 3.1,
	})

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Equal(t, "schema cannot be rendered", errors[0].Message)
	assert.Contains(t, errors[0].Reason, "does not have low-level information")
}

func postRequestWithBody(payload string) *http.Request {
	return &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Path: "/test"},
		Body:   io.NopCloser(strings.NewReader(payload)),
	}
}

// parseSchemaFromSpec creates a base.Schema from an OpenAPI spec YAML string.
// This ensures that we're using the native libopenapi logic for generating the schema.
func parseSchemaFromSpec(t *testing.T, schemaYAML string, version float32) *base.Schema {
	// Convert version to API version string (3.0 -> "3.0.0", 3.1 -> "3.1.0")
	apiVersion := fmt.Sprintf("%.1f.0", version)

	spec := fmt.Sprintf(`openapi: %s
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    TestSchema:
%s`, apiVersion, indentLines(schemaYAML, "      "))

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)
	schema := model.Model.Components.Schemas.GetOrZero("TestSchema")
	require.NotNil(t, schema)
	return schema.Schema()
}

// indentLines adds the specified indentation to each line of the input string
func indentLines(s string, indent string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
