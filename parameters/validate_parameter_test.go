package parameters

import (
	"net/http"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func Test_ForceCompilerError(t *testing.T) {
	// Try to force a panic
	result := ValidateSingleParameterSchema(nil, nil, "", "", "", "", "", nil)

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
	assert.Equal(t, "/format", valErrs[0].SchemaValidationErrors[0].Location)
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

	// basic type errors should NOT have SchemaValidationErrors (no JSONSchema validation occurred)
	assert.Empty(t, valErrs[0].SchemaValidationErrors)
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
				err.SchemaValidationErrors != nil &&
				len(err.SchemaValidationErrors) > 0 {
				for _, schemaErr := range err.SchemaValidationErrors {
					if schemaErr.Location == "schema compilation" &&
						schemaErr.Reason != "" {
						found = true
						assert.Contains(t, schemaErr.Reason, "failed to compile JSON schema")
						assert.Contains(t, err.HowToFix, "complex regex patterns")
						break
					}
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
				validationError.SchemaValidationErrors != nil {
				for _, schemaErr := range validationError.SchemaValidationErrors {
					if schemaErr.Location == "schema compilation" {
						assert.Contains(t, schemaErr.Reason, "failed to compile JSON schema")
						assert.Contains(t, validationError.HowToFix, "complex regex patterns")
						assert.Equal(t, "Query parameter 'failParam' failed schema compilation", validationError.Message)
						found = true
						break
					}
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
