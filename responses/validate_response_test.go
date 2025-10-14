package responses

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateResponseSchema(t *testing.T) {
	for name, tc := range map[string]struct {
		request                   *http.Request
		response                  *http.Response
		schemaYAML                string
		version                   float32
		assertValidResponseSchema assert.BoolAssertionFunc
		expectedErrorsCount       int
	}{
		"FailOnNumericExclusiveMinimum": {
			request:  postRequest(),
			response: responseWithBody(`{"exclusiveNumber": 10}`),
			schemaYAML: `type: object
properties:
  exclusiveNumber:
    type: number
    description: This number must be greater than 10
    exclusiveMinimum: 10`,
			version:                   3.1,
			assertValidResponseSchema: assert.False,
			expectedErrorsCount:       1,
		},
		"PassWithCorrectExclusiveMinimum": {
			request:  postRequest(),
			response: responseWithBody(`{"exclusiveNumber": 15}`),
			schemaYAML: `type: object
properties:
  exclusiveNumber:
    type: number
    description: This number is properly constrained by a numeric exclusive minimum.
    exclusiveMinimum: 12
    minimum: 12`,
			version:                   3.1,
			assertValidResponseSchema: assert.True,
			expectedErrorsCount:       0,
		},
		"PassWithValidStringType": {
			request:  postRequest(),
			response: responseWithBody(`{"greeting": "Hello, world!"}`),
			schemaYAML: `type: object
properties:
  greeting:
    type: string
    description: A simple greeting
    example: "Hello, world!"`,
			version:                   3.1,
			assertValidResponseSchema: assert.True,
			expectedErrorsCount:       0,
		},
		"PassWithNullablePropertyInOpenAPI30": {
			request:  postRequest(),
			response: responseWithBody(`{"name": "John", "middleName": null}`),
			schemaYAML: `type: object
properties:
  name:
    type: string
    description: User's first name
  middleName:
    type: string
    nullable: true
    description: User's middle name (optional)`,
			version:                   3.0,
			assertValidResponseSchema: assert.True,
			expectedErrorsCount:       0,
		},
		"PassWithNullablePropertyInOpenAPI31": {
			request:  postRequest(),
			response: responseWithBody(`{"name": "John", "middleName": null}`),
			schemaYAML: `type: object
properties:
  name:
    type: string
    description: User's first name
  middleName:
    type: string
    nullable: true
    description: User's middle name (optional)`,
			version:                   3.1,
			assertValidResponseSchema: assert.False,
			expectedErrorsCount:       1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			schema := parseSchemaFromSpec(t, tc.schemaYAML, tc.version)

			valid, errors := ValidateResponseSchema(&ValidateResponseSchemaInput{
				Request:  tc.request,
				Response: tc.response,
				Schema:   schema,
				Version:  tc.version,
			})

			tc.assertValidResponseSchema(t, valid)
			assert.Len(t, errors, tc.expectedErrorsCount)
		})
	}
}

func TestInvalidMin(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object
properties:
  exclusiveNumber:
    type: number
    minimum: 20`, 3.1)

	valid, errors := ValidateResponseSchema(&ValidateResponseSchemaInput{
		Request:  postRequest(),
		Response: responseWithBody(`{"exclusiveNumber": 13}`),
		Schema:   schema,
		Version:  3.1,
	})

	assert.False(t, valid)
	assert.Len(t, errors, 1)
}

func TestValidateResponseSchema_CachePopulation(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object
properties:
  name:
    type: string`, 3.1)

	// Create options with a cache
	opts := config.NewValidationOptions()
	require.NotNil(t, opts.SchemaCache)

	// First call should populate the cache
	valid, errors := ValidateResponseSchema(&ValidateResponseSchemaInput{
		Request:  postRequest(),
		Response: responseWithBody(`{"name": "test"}`),
		Schema:   schema,
		Version:  3.1,
		Options:  []config.Option{config.WithExistingOpts(opts)},
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

func postRequest() *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/test", io.NopCloser(strings.NewReader("")))
	return req
}

func responseWithBody(payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(payload))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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
