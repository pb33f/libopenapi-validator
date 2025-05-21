package parameters

import (
	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"

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
	if len(errs) > 0 {
		t.Fatalf("error while building v3 model: %v", errs)
	}

	v3Model.Model.Servers = nil
	// render the document back to bytes and reload the model.
	_, doc, v3Model, errs = doc.RenderAndReload()

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
                },
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
	if len(errs) > 0 {
		t.Fatalf("error while building v3 model: %v", errs)
	}

	v3Model.Model.Servers = nil
	// render the document back to bytes and reload the model.
	_, doc, v3Model, errs = doc.RenderAndReload()

	validator := NewParameterValidator(&v3Model.Model)

	isSuccess, valErrs := validator.ValidateHeaderParams(req)

	assert.False(t, isSuccess)
	assert.Len(t, valErrs, 1)
	assert.Equal(t, "schema 'apiKey' is defined as an boolean and a integer, however it failed to pass a schema validation", valErrs[0].Reason)
	assert.Len(t, valErrs[0].SchemaValidationErrors, 3)
}
