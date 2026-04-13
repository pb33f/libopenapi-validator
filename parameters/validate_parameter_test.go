package parameters

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
)

func Test_ForceCompilerError(t *testing.T) {
	// Try to force a panic
	result := ValidateSingleParameterSchema(nil, nil, "", "", "", "", "", nil, "", "")

	// Ideally this would result in an error response, current behavior swallows the error
	require.Empty(t, result)
}

func TestHeaderSchemaNoType(t *testing.T) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "API Spec With Mandatory Header",
    "version": "1.0.0"
  },
  "paths": {
    "/api-endpoint": {
      "get": {
        "summary": "Restricted API Endpoint",
        "parameters": [
          {
            "name": "apiKey",
            "in": "header",
            "required": true,
            "schema": {
              "oneOf": [
                {
                  "type": "boolean"
                },
                {
                  "type": "integer"
                }
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response"
          }
        }
      }
    }
  },
  "components": {
    "securitySchemes": {
      "ApiKeyHeader": {
        "type": "apiKey",
        "name": "apiKey",
        "in": "header"
      }
    }
  },
  "security": [
    {
      "ApiKeyHeader": []
    }
  ]
}`)

	doc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		t.Fatalf("error while creating open api spec document: %v", err)
	}

	req, err := http.NewRequest("GET", "/api-endpoint", nil)
	if err != nil {
		t.Fatalf("error while creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apiKey", "headerValue")

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		t.Fatalf("error while building v3 model: %v", errs)
	}

	v3Model.Model.Servers = nil
	// render the document back to bytes and reload the model.
	_, _, v3Model, _ = doc.RenderAndReload()

	validator := NewParameterValidator(&v3Model.Model)

	isSuccess, valErrs := validator.ValidateHeaderParams(req)

	assert.False(t, isSuccess)
	assert.Len(t, valErrs, 1)
	assert.Equal(t, "schema 'apiKey' is defined as an boolean or integer, however it failed to pass a schema validation", valErrs[0].Reason)
	assert.Len(t, valErrs[0].SchemaValidationErrors, 2)
	assert.Equal(t, "got string, want boolean", valErrs[0].SchemaValidationErrors[0].Reason)
	assert.Equal(t, "got string, want integer", valErrs[0].SchemaValidationErrors[1].Reason)
}

func TestHeaderSchemaNoType_AllPoly(t *testing.T) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "API Spec With Mandatory Header",
    "version": "1.0.0"
  },
  "paths": {
    "/api-endpoint": {
      "get": {
        "summary": "Restricted API Endpoint",
        "parameters": [
          {
            "name": "apiKey",
            "in": "header",
            "required": true,
            "schema": {
              "oneOf": [
                {
                  "type": "boolean"
                },
                {
                  "type": "integer"
                }
              ],
			  "allOf": [
                {
                  "type": "boolean"
                }
              ]
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response"
          }
        }
      }
    }
  },
  "components": {
    "securitySchemes": {
      "ApiKeyHeader": {
        "type": "apiKey",
        "name": "apiKey",
        "in": "header"
      }
    }
  },
  "security": [
    {
      "ApiKeyHeader": []
    }
  ]
}`)

	doc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		t.Fatalf("error while creating open api spec document: %v", err)
	}

	req, err := http.NewRequest("GET", "/api-endpoint", nil)
	if err != nil {
		t.Fatalf("error while creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apiKey", "headerValue")

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		t.Fatalf("error while building v3 model: %v", errs)
	}

	v3Model.Model.Servers = nil
	// render the document back to bytes and reload the model.
	_, _, v3Model, _ = doc.RenderAndReload()

	validator := NewParameterValidator(&v3Model.Model)

	isSuccess, valErrs := validator.ValidateHeaderParams(req)

	assert.False(t, isSuccess)
	assert.Len(t, valErrs, 1)
	assert.Equal(t, "schema 'apiKey' is defined as an boolean and a integer, however it failed to pass a schema validation", valErrs[0].Reason)
	assert.Len(t, valErrs[0].SchemaValidationErrors, 3)
}

// TestUnifiedErrorFormatWithFormatValidation tests that format validation errors
// use the unified SchemaValidationFailure format consistently
// https://github.com/pb33f/libopenapi-validator/issues/168
func TestUnifiedErrorFormatWithFormatValidation(t *testing.T) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "API Spec With Format Validation",
    "version": "1.0.0"
  },
  "paths": {
    "/test": {
      "get": {
        "parameters": [
          {
            "name": "email_param",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string",
              "format": "email"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}`)

	doc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		t.Fatalf("error while creating open api spec document: %v", err)
	}

	req, err := http.NewRequest("GET", "/test?email_param=invalid-email-format", nil)
	if err != nil {
		t.Fatalf("error while creating request: %v", err)
	}

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		t.Fatalf("error while building v3 model: %v", errs)
	}

	v3Model.Model.Servers = nil
	_, _, v3Model, _ = doc.RenderAndReload()

	validator := NewParameterValidator(&v3Model.Model, config.WithFormatAssertions())

	isSuccess, valErrs := validator.ValidateQueryParams(req)

	assert.False(t, isSuccess)
	assert.Len(t, valErrs, 1)
	assert.Equal(t, "Query parameter 'email_param' failed to validate", valErrs[0].Message)

	// verify ParameterName is populated for easy programmatic access
	assert.Equal(t, "email_param", valErrs[0].ParameterName)

	// verify unified error format - SchemaValidationErrors should be populated
	assert.Len(t, valErrs[0].SchemaValidationErrors, 1)
	assert.Contains(t, valErrs[0].SchemaValidationErrors[0].Reason, "is not valid email")
	assert.Equal(t, "/paths/test/get/parameters/email_param/schema/format", valErrs[0].SchemaValidationErrors[0].KeywordLocation)
	assert.NotEmpty(t, valErrs[0].SchemaValidationErrors[0].ReferenceSchema)
}

// TestParameterNameFieldPopulation tests that ParameterName field is consistently populated
// for both basic validation errors and JSONSchema validation errors
func TestParameterNameFieldPopulation(t *testing.T) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "Parameter Name Test",
    "version": "1.0.0"
  },
  "paths": {
    "/test": {
      "get": {
        "parameters": [
          {
            "name": "integer_param",
            "in": "query",
            "required": true,
            "schema": {
              "type": "integer"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}`)

	doc, err := libopenapi.NewDocument(bytes)
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/test?integer_param=not_a_number", nil)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	validator := NewParameterValidator(&v3Model.Model)
	isSuccess, valErrs := validator.ValidateQueryParams(req)

	assert.False(t, isSuccess)
	assert.Len(t, valErrs, 1)

	// verify ParameterName is populated for basic type validation errors
	assert.Equal(t, "integer_param", valErrs[0].ParameterName)
	assert.Equal(t, "Query parameter 'integer_param' is not a valid integer", valErrs[0].Message)

	// basic type errors SHOULD have SchemaValidationErrors because we know the parameter schema
	assert.Len(t, valErrs[0].SchemaValidationErrors, 1)
	assert.Equal(t, "integer_param", valErrs[0].SchemaValidationErrors[0].FieldName)
}

func TestHeaderSchemaStringNoJSON(t *testing.T) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "API Spec With Mandatory Header",
    "version": "1.0.0"
  },
  "paths": {
    "/api-endpoint": {
      "get": {
        "summary": "Restricted API Endpoint",

        "responses": {
          "200": {
            "description": "Successful response",
             "headers": {
               "chicken-nuggets": {
				"required": true,
				"schema": {
				  "oneOf": [
					{
					  "type": "boolean"
					},
					{
					  "type": "integer"
					}
				  ]
				}
			  }
			}
          }
        }
      }
    }
  },
  "components": {
    "securitySchemes": {
      "ApiKeyHeader": {
        "type": "apiKey",
        "name": "apiKey",
        "in": "header"
      }
    }
  },
  "security": [
    {
      "ApiKeyHeader": []
    }
  ]
}`)

	doc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		t.Fatalf("error while creating open api spec document: %v", err)
	}

	req, err := http.NewRequest("GET", "/api-endpoint", nil)
	if err != nil {
		t.Fatalf("error while creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apiKey", "headerValue")

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		t.Fatalf("error while building v3 model: %v", errs)
	}

	v3Model.Model.Servers = nil
	// render the document back to bytes and reload the model.
	_, _, v3Model, _ = doc.RenderAndReload()

	headers := v3Model.Model.Paths.PathItems.GetOrZero("/api-endpoint").Get.Responses.Codes.GetOrZero("200").Headers
	headerSchema := headers.GetOrZero("chicken-nuggets").Schema.Schema()

	headerErrors := ValidateParameterSchema(headerSchema, nil, "bubbles", "header",
		"response header", "chicken-nuggets", helpers.ResponseBodyValidation, lowv3.HeadersLabel, nil)

	assert.Len(t, headerErrors, 1)
	assert.Equal(t, "response header 'chicken-nuggets' is defined as an boolean or integer, however it failed to pass a schema validation", headerErrors[0].Reason)
}

// TestComplexRegexSchemaCompilationError tests that complex regex patterns in parameter schemas
// that cause schema compilation to fail are handled gracefully instead of causing panics
func TestComplexRegexSchemaCompilationError(t *testing.T) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "API Spec With Complex Regex Pattern",
    "version": "1.0.0"
  },
  "paths": {
    "/api-endpoint": {
      "get": {
        "summary": "API Endpoint with complex regex",
        "parameters": [
          {
            "name": "complexParam",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string",
              "pattern": "[\\w\\W]{1,1024}$"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response"
          }
        }
      }
    }
  }
}`)

	doc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		t.Fatalf("error while creating open api spec document: %v", err)
	}

	req, err := http.NewRequest("GET", "/api-endpoint?complexParam=testvalue", nil)
	if err != nil {
		t.Fatalf("error while creating request: %v", err)
	}

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		t.Fatalf("error while building v3 model: %v", errs)
	}

	validator := NewParameterValidator(&v3Model.Model)

	// validate - this should not panic even if schema compilation fails due to complex regex
	isSuccess, valErrs := validator.ValidateQueryParams(req)

	// if schema compilation failed, we should get validation errors instead of a panic
	if !isSuccess {
		// verify we got schema compilation errors instead of a panic
		assert.NotEmpty(t, valErrs)
		found := false
		for _, err := range valErrs {
			if err.ParameterName == "complexParam" &&
				len(err.SchemaValidationErrors) == 0 {
				// Schema compilation errors don't have SchemaValidationFailure objects
				if strings.Contains(err.Reason, "failed to compile JSON schema") {
					found = true
					assert.Contains(t, err.Reason, "failed to compile JSON schema")
					assert.Contains(t, err.HowToFix, "complex regex patterns")
					break
				}
			}
		}
		if !found {
			// if it didn't fail compilation, it should have succeeded or failed with a different error
			t.Logf("Schema compilation succeeded or failed with different error, validation result: %v, errors: %v", isSuccess, valErrs)
		}
	} else {
		// schema compiled and validated successfully
		assert.True(t, isSuccess)
		assert.Empty(t, valErrs)
	}
}

// TestValidateParameterSchema_SchemaCompilationFailure tests that ValidateParameterSchema
// handles schema compilation failures gracefully instead of causing panics
func TestValidateParameterSchema_SchemaCompilationFailure(t *testing.T) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "API Spec With Complex Regex Pattern",
    "version": "1.0.0"
  },
  "paths": {
    "/api-endpoint": {
      "get": {
        "summary": "API Endpoint with complex regex that causes compilation failure",
        "parameters": [
          {
            "name": "failParam",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string",
              "pattern": "[\\w\\W]{1,2048}$"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response"
          }
        }
      }
    }
  }
}`)

	doc, err := libopenapi.NewDocument(bytes)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	// get the parameter schema that should cause compilation failure
	pathItem := v3Model.Model.Paths.PathItems.GetOrZero("/api-endpoint")
	param := pathItem.Get.Parameters[0]
	schema := param.Schema.Schema()

	// call ValidateParameterSchema directly with the problematic schema
	validationErrors := ValidateParameterSchema(
		schema,
		"test-value",
		"",
		"Query parameter",
		"query parameter",
		"failParam",
		helpers.ParameterValidation,
		helpers.ParameterValidationQuery,
		nil,
	)

	// should get schema compilation error instead of panic
	if len(validationErrors) > 0 {
		found := false
		for _, validationError := range validationErrors {
			if validationError.ParameterName == "failParam" &&
				validationError.ValidationSubType == helpers.ParameterValidationQuery &&
				len(validationError.SchemaValidationErrors) == 0 {
				// Schema compilation errors don't have SchemaValidationFailure objects
				if strings.Contains(validationError.Reason, "failed to compile JSON schema") {
					assert.Contains(t, validationError.Reason, "failed to compile JSON schema")
					assert.Contains(t, validationError.HowToFix, "complex regex patterns")
					assert.Equal(t, "Query parameter 'failParam' failed schema compilation", validationError.Message)
					found = true
					break
				}
			}
		}
		if !found {
			// schema compilation succeeded, might have failed for other reasons or succeeded
			t.Logf("Schema compilation succeeded or failed for different reasons: %v", validationErrors)
		}
	} else {
		// no validation errors - schema compiled and validated successfully
		t.Logf("Schema compiled and validated successfully")
	}
}

func preparePathsBenchmark(b *testing.B, cache config.RegexCache) (ParameterValidator, *http.Request) {
	bytes := []byte(`{
  "openapi": "3.0.0",
  "info": {
    "title": "API Spec With Complex Regex Pattern",
    "version": "1.0.0"
  },
  "paths": {
  "/test/other/path": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
     "/static/test/{imageName}": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
     "/request/to/my/image.png": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
     "/api/v2/{url}/{other}/{oncemore}/{url}": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
     "/api/v1/{path}": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
     "/each/url/{is}/{a_new_regex}": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
     "/my-test/with-so-many/urls": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
    "/test/other/path": {
      "get": {"responses": {"200": {"description": "test"}}}
    },
    "/api/endpoint/{address}/{domain}": {
      "get": {
        "summary": "API Endpoint with complex regex",
        "parameters": [
          {
            "name": "complexParam",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string",
              "pattern": "[\\w\\W]{1,1024}$"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response"
          }
        }
      }
    }
  }
}`)

	doc, err := libopenapi.NewDocument(bytes)
	if err != nil {
		b.Fatalf("error while creating open api spec document: %v", err)
	}

	req, err := http.NewRequest("GET", "/api/endpoint/127.0.0.1/domain.com?complexParam=testvalue", nil)
	if err != nil {
		b.Fatalf("error while creating request: %v", err)
	}

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		b.Fatalf("error while building v3 model: %v", errs)
	}

	validator := NewParameterValidator(&v3Model.Model, config.WithRegexCache(cache))

	return validator, req
}

func BenchmarkValidationWithoutCache(b *testing.B) {
	validator, req := preparePathsBenchmark(b, nil)

	b.ResetTimer()

	for b.Loop() {
		validator.ValidateHeaderParams(req)
		validator.ValidateCookieParams(req)
		validator.ValidateQueryParams(req)
		validator.ValidateSecurity(req)
		validator.ValidatePathParams(req)
	}
}

func BenchmarkValidationWithRegexCache(b *testing.B) {
	validator, req := preparePathsBenchmark(b, &sync.Map{})

	b.ResetTimer()

	for b.Loop() {
		validator.ValidateHeaderParams(req)
		validator.ValidateCookieParams(req)
		validator.ValidateQueryParams(req)
		validator.ValidateSecurity(req)
		validator.ValidatePathParams(req)
	}
}

// cacheTestSpec is an OpenAPI spec for testing cache behavior
var cacheTestSpec = []byte(`{
  "openapi": "3.1.0",
  "info": {
    "title": "Cache Test API",
    "version": "1.0.0"
  },
  "paths": {
    "/items/{id}": {
      "get": {
        "operationId": "getItem",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string",
              "minLength": 1,
              "maxLength": 64
            }
          },
          {
            "name": "limit",
            "in": "query",
            "schema": {
              "type": "integer",
              "minimum": 1,
              "maximum": 100
            }
          }
        ],
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    }
  }
}`)

// Test_ParameterValidation_CacheUsage verifies that parameter validation uses the schema cache.
// This test validates that:
// 1. Cache is populated after the first validation
// 2. Subsequent validations reuse the cached compiled schemas
// 3. Validation still produces correct results when using cached schemas
func Test_ParameterValidation_CacheUsage(t *testing.T) {
	doc, err := libopenapi.NewDocument(cacheTestSpec)
	require.NoError(t, err, "Failed to create document")

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs, "Failed to build v3 model")

	// Create options with cache (default behavior)
	opts := config.NewValidationOptions()
	require.NotNil(t, opts.SchemaCache, "Schema cache should be initialized by default")

	validator := NewParameterValidator(&v3Model.Model, config.WithExistingOpts(opts))

	// First request - should populate cache
	req1, _ := http.NewRequest("GET", "/items/abc123?limit=50", nil)
	isSuccess1, errors1 := validator.ValidateQueryParams(req1)
	assert.True(t, isSuccess1, "First validation should succeed")
	assert.Empty(t, errors1, "First validation should have no errors")

	// Count cached entries (should have at least the limit parameter schema)
	cacheCount := 0
	opts.SchemaCache.Range(func(key uint64, value *cache.SchemaCacheEntry) bool {
		cacheCount++
		return true
	})
	assert.Greater(t, cacheCount, 0, "Cache should have entries after first validation")

	// Second request with different valid value - should use cached schema
	req2, _ := http.NewRequest("GET", "/items/xyz789?limit=75", nil)
	isSuccess2, errors2 := validator.ValidateQueryParams(req2)
	assert.True(t, isSuccess2, "Second validation should succeed")
	assert.Empty(t, errors2, "Second validation should have no errors")

	// Third request with invalid value - should still use cached schema but fail validation
	req3, _ := http.NewRequest("GET", "/items/test?limit=999", nil)
	isSuccess3, errors3 := validator.ValidateQueryParams(req3)
	assert.False(t, isSuccess3, "Third validation should fail (limit > maximum)")
	assert.NotEmpty(t, errors3, "Third validation should have errors")
}

// Test_ParameterValidation_WithoutCache verifies that validation works when cache is disabled.
func Test_ParameterValidation_WithoutCache(t *testing.T) {
	doc, err := libopenapi.NewDocument(cacheTestSpec)
	require.NoError(t, err, "Failed to create document")

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs, "Failed to build v3 model")

	// Create options without cache
	opts := config.NewValidationOptions(config.WithSchemaCache(nil))
	require.Nil(t, opts.SchemaCache, "Schema cache should be nil")

	validator := NewParameterValidator(&v3Model.Model, config.WithExistingOpts(opts))

	// Validation should still work without cache
	req, _ := http.NewRequest("GET", "/items/abc123?limit=50", nil)
	isSuccess, errors := validator.ValidateQueryParams(req)
	assert.True(t, isSuccess, "Validation should succeed without cache")
	assert.Empty(t, errors, "Validation should have no errors")

	// Validation with invalid value should fail
	req2, _ := http.NewRequest("GET", "/items/abc123?limit=999", nil)
	isSuccess2, errors2 := validator.ValidateQueryParams(req2)
	assert.False(t, isSuccess2, "Validation should fail for invalid value")
	assert.NotEmpty(t, errors2, "Validation should report errors")
}

// Test_ParameterValidation_CacheConsistency verifies that cached schemas produce
// the same validation results as freshly compiled schemas.
func Test_ParameterValidation_CacheConsistency(t *testing.T) {
	doc, err := libopenapi.NewDocument(cacheTestSpec)
	require.NoError(t, err, "Failed to create document")

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs, "Failed to build v3 model")

	// Run the same validations with and without cache
	testCases := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid_limit", "/items/abc?limit=50", true},
		{"limit_at_max", "/items/abc?limit=100", true},
		{"limit_at_min", "/items/abc?limit=1", true},
		{"limit_too_high", "/items/abc?limit=101", false},
		{"limit_too_low", "/items/abc?limit=0", false},
	}

	// First run with cache
	optsWithCache := config.NewValidationOptions()
	validatorWithCache := NewParameterValidator(&v3Model.Model, config.WithExistingOpts(optsWithCache))

	// Second run without cache
	optsNoCache := config.NewValidationOptions(config.WithSchemaCache(nil))
	validatorNoCache := NewParameterValidator(&v3Model.Model, config.WithExistingOpts(optsNoCache))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.url, nil)

			successWithCache, errorsWithCache := validatorWithCache.ValidateQueryParams(req)
			successNoCache, errorsNoCache := validatorNoCache.ValidateQueryParams(req)

			assert.Equal(t, tc.expected, successWithCache, "Cached validation result mismatch for %s", tc.name)
			assert.Equal(t, successWithCache, successNoCache, "Cache vs no-cache results should match for %s", tc.name)
			assert.Equal(t, len(errorsWithCache), len(errorsNoCache), "Error count should match for %s", tc.name)
		})
	}
}

// Test_GetRenderedSchema_NilSchema verifies GetRenderedSchema handles nil schema gracefully.
func Test_GetRenderedSchema_NilSchema(t *testing.T) {
	opts := config.NewValidationOptions()
	result := GetRenderedSchema(nil, opts)
	assert.Empty(t, result, "GetRenderedSchema should return empty string for nil schema")
}

// Test_GetRenderedSchema_NilOptions verifies GetRenderedSchema works without options.
func Test_GetRenderedSchema_NilOptions(t *testing.T) {
	// Parse a document to get a properly initialized schema
	spec := []byte(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"paths": {
			"/test": {
				"get": {
					"parameters": [{
						"name": "id",
						"in": "query",
						"schema": {"type": "string", "minLength": 1}
					}],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	// Get the parameter schema
	pathItem := v3Model.Model.Paths.PathItems.GetOrZero("/test")
	param := pathItem.Get.Parameters[0]
	schema := param.Schema.Schema()

	// Call with nil options - should still render the schema (returns some representation)
	result := GetRenderedSchema(schema, nil)
	assert.NotEmpty(t, result, "GetRenderedSchema should render schema even with nil options")
}

// Test_GetRenderedSchema_CacheHit verifies GetRenderedSchema uses cached data when available.
func Test_GetRenderedSchema_CacheHit(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"paths": {
			"/test": {
				"get": {
					"parameters": [{
						"name": "id",
						"in": "query",
						"schema": {"type": "integer", "minimum": 1}
					}],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	// Get the parameter schema
	pathItem := v3Model.Model.Paths.PathItems.GetOrZero("/test")
	param := pathItem.Get.Parameters[0]
	schema := param.Schema.Schema()

	// Create options with cache and pre-populate with known value
	opts := config.NewValidationOptions()
	hash := schema.GoLow().Hash()
	testCachedJSON := []byte(`{"type":"integer","minimum":1,"cached":true}`)
	opts.SchemaCache.Store(hash, &cache.SchemaCacheEntry{
		Schema:       schema,
		RenderedJSON: testCachedJSON,
	})

	// GetRenderedSchema should return the cached value
	result := GetRenderedSchema(schema, opts)
	assert.Equal(t, string(testCachedJSON), result, "GetRenderedSchema should return cached JSON")
}

// Test_GetRenderedSchema_NilCache verifies GetRenderedSchema works when cache is disabled.
func Test_GetRenderedSchema_NilCache(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"paths": {
			"/test": {
				"get": {
					"parameters": [{
						"name": "id",
						"in": "query",
						"schema": {"type": "boolean"}
					}],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	// Get the parameter schema
	pathItem := v3Model.Model.Paths.PathItems.GetOrZero("/test")
	param := pathItem.Get.Parameters[0]
	schema := param.Schema.Schema()

	// Create options with cache disabled
	opts := config.NewValidationOptions(config.WithSchemaCache(nil))
	require.Nil(t, opts.SchemaCache)

	// GetRenderedSchema should still work by rendering fresh (returns some representation)
	result := GetRenderedSchema(schema, opts)
	assert.NotEmpty(t, result, "GetRenderedSchema should render schema even with nil cache")
}

// Test_GetRenderedSchema_CacheMiss verifies GetRenderedSchema renders fresh when cache entry has empty RenderedJSON.
// This tests the code path where cache lookup succeeds but RenderedJSON is empty.
func Test_GetRenderedSchema_CacheMiss(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"paths": {
			"/test": {
				"get": {
					"parameters": [{
						"name": "id",
						"in": "query",
						"schema": {"type": "integer"}
					}],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	// Get the parameter schema
	pathItem := v3Model.Model.Paths.PathItems.GetOrZero("/test")
	param := pathItem.Get.Parameters[0]
	schema := param.Schema.Schema()

	// Create options with cache enabled
	opts := config.NewValidationOptions()
	require.NotNil(t, opts.SchemaCache)

	// Store an entry with empty RenderedJSON to simulate cache miss scenario
	hash := schema.GoLow().Hash()
	opts.SchemaCache.Store(hash, &cache.SchemaCacheEntry{
		Schema:       schema,
		RenderedJSON: nil, // Empty - should trigger fresh rendering
	})

	// GetRenderedSchema should render fresh since RenderedJSON is empty
	result := GetRenderedSchema(schema, opts)
	assert.NotEmpty(t, result, "GetRenderedSchema should render fresh when cached RenderedJSON is empty")
}

// Test_ValidateSingleParameterSchema_CacheMissCompiledSchema tests the path where cache entry
// exists but CompiledSchema is nil, forcing recompilation.
func Test_ValidateSingleParameterSchema_CacheMissCompiledSchema(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"paths": {
			"/test/{id}": {
				"get": {
					"parameters": [{
						"name": "id",
						"in": "path",
						"required": true,
						"schema": {"type": "integer", "minimum": 1}
					}],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	// Get the parameter schema
	pathItem := v3Model.Model.Paths.PathItems.GetOrZero("/test/{id}")
	param := pathItem.Get.Parameters[0]
	schema := param.Schema.Schema()

	// Create options with cache enabled
	opts := config.NewValidationOptions()
	require.NotNil(t, opts.SchemaCache)

	// Store an entry with nil CompiledSchema to force recompilation
	hash := schema.GoLow().Hash()
	opts.SchemaCache.Store(hash, &cache.SchemaCacheEntry{
		Schema:         schema,
		CompiledSchema: nil, // nil - should trigger recompilation
	})

	// Validate should still work by recompiling the schema
	result := ValidateSingleParameterSchema(
		schema,
		int64(5), // valid integer
		"Path parameter",
		"The path parameter",
		"id",
		helpers.ParameterValidation,
		helpers.ParameterValidationPath,
		opts,
		"/test/{id}",
		"get",
	)
	assert.Empty(t, result, "Validation should pass for valid integer")

	// Now verify the cache was populated with the compiled schema
	cached, ok := opts.SchemaCache.Load(hash)
	assert.True(t, ok, "Cache entry should exist")
	assert.NotNil(t, cached.CompiledSchema, "CompiledSchema should be populated after validation")
}

// arrayValidationSpec is used to test array parameter validation with the updated function signatures
var arrayValidationSpec = []byte(`{
	"openapi": "3.1.0",
	"info": {"title": "Array Test", "version": "1.0.0"},
	"paths": {
		"/test": {
			"get": {
				"parameters": [{
					"name": "ids",
					"in": "query",
					"schema": {
						"type": "array",
						"items": {"type": "integer", "minimum": 1}
					}
				}],
				"responses": {"200": {"description": "OK"}}
			}
		}
	}
}`)

// Test_ArrayValidation_ErrorContainsRenderedSchema verifies that array validation errors
// still contain the rendered schema after the rendering optimization.
func Test_ArrayValidation_ErrorContainsRenderedSchema(t *testing.T) {
	doc, err := libopenapi.NewDocument(arrayValidationSpec)
	require.NoError(t, err)

	v3Model, errs := doc.BuildV3Model()
	require.Nil(t, errs)

	validator := NewParameterValidator(&v3Model.Model)

	// Request with invalid array values (strings instead of integers)
	req, _ := http.NewRequest("GET", "/test?ids=abc,def", nil)

	success, validationErrors := validator.ValidateQueryParams(req)
	assert.False(t, success, "Validation should fail for non-integer array values")
	assert.NotEmpty(t, validationErrors, "Should have validation errors")

	// Verify error message is properly formatted
	assert.Contains(t, validationErrors[0].Message, "ids", "Error should reference parameter name")
}
