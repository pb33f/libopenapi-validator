// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi-validator/config"
	liberrors "github.com/pb33f/libopenapi-validator/errors"
	validatorhelpers "github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/schema_validation"
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

func TestBooleanExclusiveMin_ValidValue(t *testing.T) {
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

	assert.True(t, valid)
	assert.Empty(t, errors)
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
	hash := schema_validation.SchemaCacheKey(schema.GoLow().Hash(), openAPIVersion,
		schema_validation.SchemaValidationPurposeRequestBody)
	cached, ok := opts.SchemaCache.Load(hash)
	assert.True(t, ok, "Schema should be in cache")
	assert.NotNil(t, cached, "Cached entry should not be nil")
	assert.NotNil(t, cached.CompiledSchema, "Compiled schema should be cached")
	assert.NotNil(t, cached.RenderedInline, "Rendered schema should be cached")
	assert.NotNil(t, cached.RenderedJSON, "JSON schema should be cached")
}

func TestValidateRequestSchema_ReadOnlyRequiredIgnored(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object
required:
  - id
  - name
properties:
  id:
    type: string
    readOnly: true
  name:
    type: string`, 3.1)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"name":"John"}`),
		Schema:  schema,
		Version: 3.1,
	})

	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestValidateRequestSchema_WriteOnlyRequiredStillApplies(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object
required:
  - password
properties:
  password:
    type: string
    writeOnly: true`, 3.1)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{}`),
		Schema:  schema,
		Version: 3.1,
	})

	assert.False(t, valid)
	require.Len(t, errors, 1)
	require.Len(t, errors[0].SchemaValidationErrors, 1)
	assert.Equal(t, "missing property 'password'", errors[0].SchemaValidationErrors[0].Reason)
}

func TestValidateRequestSchema_NestedReadOnlyRequiredIgnored(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object
required:
  - profile
properties:
  profile:
    type: object
    required:
      - id
      - email
    properties:
      id:
        type: string
        readOnly: true
      email:
        type: string`, 3.1)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"profile":{"email":"john@example.com"}}`),
		Schema:  schema,
		Version: 3.1,
	})

	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestValidateRequestSchema_AllOfReadOnlyRequiredIgnored(t *testing.T) {
	schema := parseSchemaFromSpec(t, `allOf:
  - type: object
    required:
      - id
      - name
    properties:
      id:
        type: string
        readOnly: true
      name:
        type: string`, 3.1)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"name":"John"}`),
		Schema:  schema,
		Version: 3.1,
	})

	assert.True(t, valid)
	assert.Empty(t, errors)
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

func TestValidateRequestSchema_EmptyBodyOptional(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object
properties:
  name:
    type: string`, 3.1)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request:      emptyPostRequest(),
		Schema:       schema,
		Version:      3.1,
		BodyRequired: false,
	})

	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestValidateRequestSchema_EmptyBodyRequired(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object
properties:
  name:
    type: string`, 3.1)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request:      emptyPostRequest(),
		Schema:       schema,
		Version:      3.1,
		BodyRequired: true,
	})

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "request body is empty")
}

func TestValidateRequestSchema_CachedSchemaWithoutRenderedNodeFallsBackToRenderedBytes(t *testing.T) {
	schema := parseSchemaFromSpec(t, `anyOf:
  - type: string
  - type: integer`, 3.1)

	opts := config.NewValidationOptions()
	compiled, err := schema_validation.CompileSchemaForValidation(
		schema,
		schema_validation.SchemaValidationPurposeRequestBody,
		opts,
		3.1,
	)
	require.NoError(t, err)

	hash := schema_validation.SchemaCacheKey(
		schema.GoLow().Hash(),
		3.1,
		schema_validation.SchemaValidationPurposeRequestBody,
	)
	entry := compiled.ToCacheEntry(schema)
	entry.RenderedNode = nil
	opts.SchemaCache.Store(hash, entry)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`true`),
		Schema:  schema,
		Version: 3.1,
		Options: []config.Option{
			config.WithExistingOpts(opts),
		},
	})

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Len(t, errors[0].SchemaValidationErrors, 2)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "got boolean")
}

func TestValidateRequestSchema_IgnoresEmptyKeywordLocationErrors(t *testing.T) {
	schema := parseSchemaFromSpec(t, `type: object`, 3.1)
	opts := config.NewValidationOptions()
	compiledSchema, err := validatorhelpers.NewCompiledSchemaWithVersion(
		"schema",
		[]byte(`false`),
		opts,
		3.1,
	)
	require.NoError(t, err)

	hash := schema_validation.SchemaCacheKey(
		schema.GoLow().Hash(),
		3.1,
		schema_validation.SchemaValidationPurposeRequestBody,
	)
	opts.SchemaCache.Store(hash, &cache.SchemaCacheEntry{
		Schema:          schema,
		RenderedInline:  []byte("false"),
		ReferenceSchema: "false",
		RenderedJSON:    []byte("false"),
		CompiledSchema:  compiledSchema,
	})

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"name":"test"}`),
		Schema:  schema,
		Version: 3.1,
		Options: []config.Option{
			config.WithExistingOpts(opts),
		},
	})

	assert.False(t, valid)
	require.Len(t, errors, 1)
	assert.Empty(t, errors[0].SchemaValidationErrors)
}

func postRequestWithBody(payload string) *http.Request {
	return &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Path: "/test"},
		Body:   io.NopCloser(strings.NewReader(payload)),
	}
}

func emptyPostRequest() *http.Request {
	return &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Path: "/test"},
		Body:   http.NoBody,
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

func TestValidateRequestSchema_CircularReference(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Error:
      type: object
      properties:
        code:
          type: string
        details:
          type: array
          items:
            $ref: '#/components/schemas/Error'`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	// Verify circular reference was detected
	require.Len(t, model.Index.GetCircularReferences(), 1)

	schema := model.Model.Components.Schemas.GetOrZero("Error")
	require.NotNil(t, schema)

	valid, errors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"code": "abc", "details": [{"code": "def"}]}`),
		Schema:  schema.Schema(),
		Version: 3.1,
	})

	assert.True(t, valid)
	assert.Empty(t, errors)

	valid, errors = ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(`{"code": "abc", "details": [{"code": 42}]}`),
		Schema:  schema.Schema(),
		Version: 3.1,
	})

	assert.False(t, valid)
	require.Len(t, errors, 1)
	require.NotEmpty(t, errors[0].SchemaValidationErrors)
	assert.Contains(t, errors[0].SchemaValidationErrors[0].Reason, "got number")
}

func TestValidateRequestSchema_MultiFileComplexCircularReferences(t *testing.T) {
	tempDir := t.TempDir()

	files := map[string]string{
		"openapi.yaml": `openapi: 3.1.0
info:
  title: Multi-file circular refs
  version: 1.0.0
paths:
  /catalogs:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: './models.yaml#/components/schemas/Catalog'`,
		"models.yaml": `components:
  schemas:
    Catalog:
      type: object
      required: [products, featured]
      properties:
        products:
          type: array
          minItems: 1
          items:
            $ref: './product.yaml#/components/schemas/Product'
        featured:
          $ref: './product.yaml#/components/schemas/Product'`,
		"product.yaml": `components:
  schemas:
    Product:
      type: object
      required: [sku, name, children, variants]
      properties:
        sku:
          type: string
        name:
          type: string
        children:
          type: array
          items:
            $ref: '#/components/schemas/Product'
        variants:
          type: array
          items:
            $ref: './variant.yaml#/components/schemas/Variant'`,
		"variant.yaml": `components:
  schemas:
    Variant:
      type: object
      required: [code, parent]
      properties:
        code:
          type: string
        parent:
          $ref: './product.yaml#/components/schemas/Product'
        alternatives:
          type: array
          items:
            $ref: '#/components/schemas/Variant'`,
	}

	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0o600))
	}

	docConfig := datamodel.NewDocumentConfiguration()
	docConfig.AllowFileReferences = true
	docConfig.BasePath = tempDir
	docConfig.SpecFilePath = filepath.Join(tempDir, "openapi.yaml")
	docConfig.FileFilter = []string{"openapi.yaml", "models.yaml", "product.yaml", "variant.yaml"}
	docConfig.SkipCircularReferenceCheck = true

	rootSpec, err := os.ReadFile(filepath.Join(tempDir, "openapi.yaml"))
	require.NoError(t, err)
	doc, err := libopenapi.NewDocumentWithConfiguration(rootSpec, docConfig)
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	schema := model.Model.Paths.PathItems.GetOrZero("/catalogs").Post.RequestBody.Content.GetOrZero("application/json").Schema
	require.NotNil(t, schema)

	validPayload := `{
  "products": [
    {
      "sku": "root",
      "name": "Root",
      "children": [
        {
          "sku": "child",
          "name": "Child",
          "children": [],
          "variants": []
        }
      ],
      "variants": [
        {
          "code": "red",
          "parent": {
            "sku": "parent",
            "name": "Parent",
            "children": [],
            "variants": []
          },
          "alternatives": []
        }
      ]
    }
  ],
  "featured": {
    "sku": "featured",
    "name": "Featured",
    "children": [],
    "variants": []
  }
}`

	invalidPayload := strings.Replace(validPayload, `"code": "red"`, `"code": 42`, 1)
	valid, validationErrors := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(invalidPayload),
		Schema:  schema.Schema(),
		Version: 3.1,
		Options: []config.Option{
			config.WithSchemaCache(nil),
		},
	})

	assert.False(t, valid)
	require.Len(t, validationErrors, 1)
	coldFailure := requireSchemaFailureContaining(t, validationErrors[0].SchemaValidationErrors, "got number")
	assert.Equal(t, "code", coldFailure.FieldName)
	assert.Equal(t, 8, coldFailure.Line)
	assert.Greater(t, coldFailure.Column, 0)
	assert.Contains(t, coldFailure.KeywordLocation, "/type")

	valid, validationErrors = ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(validPayload),
		Schema:  schema.Schema(),
		Version: 3.1,
	})

	assert.True(t, valid)
	assert.Empty(t, validationErrors)

	valid, validationErrors = ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(invalidPayload),
		Schema:  schema.Schema(),
		Version: 3.1,
	})

	assert.False(t, valid)
	require.Len(t, validationErrors, 1)
	require.NotEmpty(t, validationErrors[0].SchemaValidationErrors)
	cachedFailure := requireSchemaFailureContaining(t, validationErrors[0].SchemaValidationErrors, "got number")
	assert.Equal(t, coldFailure.Line, cachedFailure.Line)
	assert.Equal(t, coldFailure.Column, cachedFailure.Column)
}

func requireSchemaFailureContaining(
	t *testing.T,
	failures []*liberrors.SchemaValidationFailure,
	expectedReason string,
) *liberrors.SchemaValidationFailure {
	t.Helper()
	for _, failure := range failures {
		if failure != nil && strings.Contains(failure.Reason, expectedReason) {
			return failure
		}
	}
	require.Failf(t, "schema failure not found", "expected reason containing %q", expectedReason)
	return nil
}

func TestValidateRequestSchema_NilParentProxy(t *testing.T) {
	// Schema with nil ParentProxy and empty body — should not panic (fix for wiretap #134)
	schema := &base.Schema{
		Type: []string{"object"},
	}

	valid, errs := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request:      postRequestWithBody(""),
		Schema:       schema,
		Version:      3.1,
		BodyRequired: true,
	})

	// Should return error about nil schema low-level info, not panic
	assert.False(t, valid)
	assert.NotEmpty(t, errs)
}
