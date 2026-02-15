// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"net/http"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/paths"
)

func TestParamValidator_ValidateSecurity_APIKeyHeader_NotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "API Key X-API-Key not found in header", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/products", errors[0].SpecPath)
}

func TestParamValidator_ValidateSecurity_APIKeyHeader(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("X-API-Key", "1234")

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_APIKeyQuery_NotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: query
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "API Key X-API-Key not found in query", errors[0].Message)
	assert.Equal(t, "Add an API Key via 'X-API-Key' to the query string of the URL, "+
		"for example 'https://things.com/products?X-API-Key=your-api-key'", errors[0].HowToFix)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/products", errors[0].SpecPath)
}

func TestParamValidator_ValidateSecurity_APIKeyQuery(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: query
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products?X-API-Key=12345", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_APIKeyCookie_NotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: cookie
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "API Key X-API-Key not found in cookies", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/products", errors[0].SpecPath)
}

func TestParamValidator_ValidateSecurity_APIKeyCookie(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: cookie
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	request.AddCookie(&http.Cookie{
		Name:  "X-API-Key",
		Value: "1234",
	})

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_Basic_NotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "Authorization header for 'basic' scheme", errors[0].Message)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/products", errors[0].SpecPath)
}

func TestParamValidator_ValidateSecurity_Basic(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("Authorization", "Basic 1234")

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_BadPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/blimpo", nil)
	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "", errors[0].SpecPath)
}

func TestParamValidator_ValidateSecurity_MissingSecuritySchemes(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components: {}
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
}

func TestParamValidator_ValidateSecurity_NoComponents(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
}

func TestParamValidator_ValidateSecurity_PresetPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	pathItem, errs, pv := paths.FindPath(request, &m.Model, nil)
	assert.Nil(t, errs)

	valid, errors := v.ValidateSecurityWithPathItem(request, pathItem, pv)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_PresetPath_notfound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/beef", nil)
	pathItem, _, pv := paths.FindPath(request, &m.Model, &sync.Map{})

	valid, errors := v.ValidateSecurityWithPathItem(request, pathItem, pv)
	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "POST Path '/beef' not found", errors[0].Message)
}

func TestParamValidator_ValidateSecurity_MultipleSecurity(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthQuery:
          - write:products
        - ApiKeyAuthHeader:
          - write:products
components:
  securitySchemes:
    ApiKeyAuthQuery:
      type: apiKey
      in: query
      name: X-API-Key
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("X-API-Key", "1234")

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_MultipleSecurity_EmptyOption(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
        - {}
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_MultipleSecurity_NotFound(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthQuery:
          - write:products
        - ApiKeyAuthHeader:
          - write:products
components:
  securitySchemes:
    ApiKeyAuthQuery:
      type: apiKey
      in: query
      name: X-API-Key
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 2, len(errors))

	assert.Equal(t, "API Key X-API-Key not found in query", errors[0].Message)
	assert.Equal(t, "Add an API Key via 'X-API-Key' to the query string of the URL, "+
		"for example 'https://things.com/products?X-API-Key=your-api-key'", errors[0].HowToFix)
	assert.Equal(t, request.Method, errors[0].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[0].RequestPath)
	assert.Equal(t, "/products", errors[0].SpecPath)

	assert.Equal(t, "API Key X-API-Key not found in header", errors[1].Message)
	assert.Equal(t, request.Method, errors[1].RequestMethod)
	assert.Equal(t, request.URL.Path, errors[1].RequestPath)
	assert.Equal(t, "/products", errors[1].SpecPath)
}

func TestParamValidator_ValidateSecurity_SecurityValidationDisabled_APIKeyHeader(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model, config.WithoutSecurityValidation())

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	// No API key header provided

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_SecurityValidationDisabled_APIKeyQuery(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: query
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model, config.WithoutSecurityValidation())

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	// No API key query param provided

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_SecurityValidationDisabled_APIKeyCookie(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: cookie
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model, config.WithoutSecurityValidation())

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	// No API key cookie provided

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_SecurityValidationDisabled_BasicAuth(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model, config.WithoutSecurityValidation())

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	// No Authorization header provided

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_SecurityValidationDisabled_WithPathItem(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model, config.WithoutSecurityValidation())

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	pathItem, errs, pv := paths.FindPath(request, &m.Model, &sync.Map{})
	assert.Nil(t, errs)

	valid, errors := v.ValidateSecurityWithPathItem(request, pathItem, pv)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_SecurityValidationDisabled_MissingPath(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	m, _ := doc.BuildV3Model()

	v := NewParameterValidator(&m.Model, config.WithoutSecurityValidation())

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/nonexistent", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid) // Should still fail for invalid paths
	assert.Equal(t, 1, len(errors))
	assert.Contains(t, errors[0].Message, "Path '/nonexistent' not found")
}

func TestParamValidator_ValidateSecurity_SecurityValidationEnabled_vs_Disabled(t *testing.T) {
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuth:
          - write:products
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()

	// Test with security validation enabled (default)
	vEnabled := NewParameterValidator(&m.Model)
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := vEnabled.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "API Key X-API-Key not found in header", errors[0].Message)

	// Test with security validation disabled
	vDisabled := NewParameterValidator(&m.Model, config.WithoutSecurityValidation())

	valid, errors = vDisabled.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Equal(t, 0, len(errors))
}

func TestParamValidator_ValidateSecurity_ANDRequirement_BothPresent(t *testing.T) {
	// Test AND security requirement: both schemes in same requirement must pass
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthHeader: []
          BasicAuth: []
components:
  securitySchemes:
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
    BasicAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with BOTH api key AND authorization header - should pass
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("X-API-Key", "1234")
	request.Header.Add("Authorization", "Basic dXNlcjpwYXNz")

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestParamValidator_ValidateSecurity_ANDRequirement_OnlyApiKey(t *testing.T) {
	// Test AND security requirement: missing one scheme should fail
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthHeader: []
          BasicAuth: []
components:
  securitySchemes:
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
    BasicAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with ONLY api key - should fail because BasicAuth is also required
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("X-API-Key", "1234")

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "Authorization header")
}

func TestParamValidator_ValidateSecurity_ANDRequirement_OnlyBasicAuth(t *testing.T) {
	// Test AND security requirement: missing one scheme should fail
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthHeader: []
          BasicAuth: []
components:
  securitySchemes:
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
    BasicAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with ONLY authorization header - should fail because ApiKeyAuthHeader is also required
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("Authorization", "Basic dXNlcjpwYXNz")

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "API Key")
}

func TestParamValidator_ValidateSecurity_ANDRequirement_NeitherPresent(t *testing.T) {
	// Test AND security requirement: missing both schemes should return errors for both
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthHeader: []
          BasicAuth: []
components:
  securitySchemes:
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
    BasicAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with neither - should fail with errors for both
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Len(t, errors, 2)
}

func TestParamValidator_ValidateSecurity_ORWithAND_FirstOROptionPasses(t *testing.T) {
	// Test mixed OR and AND: first option is single scheme, second is AND requirement
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthHeader: []
        - BasicAuth: []
          BearerAuth: []
components:
  securitySchemes:
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
    BasicAuth:
      type: http
      scheme: basic
    BearerAuth:
      type: http
      scheme: bearer
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with only API key - should pass (first OR option)
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("X-API-Key", "1234")

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestParamValidator_ValidateSecurity_ORWithAND_SecondOROptionPasses(t *testing.T) {
	// Test mixed OR and AND: second option (AND requirement) passes
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthHeader: []
        - BasicAuth: []
          ApiKeyAuthQuery: []
components:
  securitySchemes:
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
    ApiKeyAuthQuery:
      type: apiKey
      in: query
      name: api_key
    BasicAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with basic auth AND query API key - should pass (second OR option, which is AND)
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products?api_key=secret", nil)
	request.Header.Add("Authorization", "Basic dXNlcjpwYXNz")

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestParamValidator_ValidateSecurity_ORWithAND_PartialSecondOption(t *testing.T) {
	// Test mixed OR and AND: partial match on second option should try both and fail
	spec := `openapi: 3.1.0
paths:
  /products:
    post:
      security:
        - ApiKeyAuthHeader: []
        - BasicAuth: []
          ApiKeyAuthQuery: []
components:
  securitySchemes:
    ApiKeyAuthHeader:
      type: apiKey
      in: header
      name: X-API-Key
    ApiKeyAuthQuery:
      type: apiKey
      in: query
      name: api_key
    BasicAuth:
      type: http
      scheme: basic
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with only basic auth - should fail (first option needs X-API-Key header,
	// second option needs BOTH basic auth AND api_key query param)
	request, _ := http.NewRequest(http.MethodPost, "https://things.com/products", nil)
	request.Header.Add("Authorization", "Basic dXNlcjpwYXNz")

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	// Should have errors from both OR options
	assert.GreaterOrEqual(t, len(errors), 1)
}

func TestParamValidator_ValidateSecurity_UnknownSchemeType(t *testing.T) {
	// Test oauth2 type - unknown to our validator, should pass through (not fail)
	spec := `openapi: 3.1.0
paths:
  /products:
    get:
      security:
        - OAuth2: []
components:
  securitySchemes:
    OAuth2:
      type: oauth2
      flows:
        implicit:
          authorizationUrl: https://example.com/oauth
          scopes:
            read: Read access
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with no auth - should pass because oauth2 type is not validated
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestParamValidator_ValidateSecurity_CustomHTTPScheme(t *testing.T) {
	// Test custom HTTP scheme - should pass with correct scheme in header
	spec := `openapi: 3.1.0
paths:
  /products:
    get:
      security:
        - CustomAuth: []
components:
  securitySchemes:
    CustomAuth:
      type: http
      scheme: custom
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with custom auth header - should pass
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/products", nil)
	request.Header.Add("Authorization", "Custom dXNlcjpwYXNz")

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestParamValidator_ValidateSecurity_APIKey_UnknownInLocation(t *testing.T) {
	// Test apiKey with unknown "in" location - should pass through (fallback at line 221)
	spec := `openapi: 3.1.0
paths:
  /products:
    get:
      security:
        - ApiKeyAuth: []
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: body
      name: X-API-Key
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with no auth - should pass because "body" is an unknown "in" location
	// and the validator falls through to return true (line 221)
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/products", nil)

	valid, errors := v.ValidateSecurity(request)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestParamValidator_ValidateSecurity_HTTPScheme_Mismatch(t *testing.T) {
	// Test http scheme with mismatch in header: should return errors
	spec := `openapi: 3.1.0
paths:
  /products:
    get:
      security:
        - CustomAuth: []
components:
  securitySchemes:
    CustomAuth:
      type: http
      scheme: custom
`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	m, _ := doc.BuildV3Model()
	v := NewParameterValidator(&m.Model)

	// Request with auth header - should fail as header scheme is incorrect
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/products", nil)
	request.Header.Add("Authorization", "Basic dXNlcjpwYXNz")

	valid, errors := v.ValidateSecurity(request)
	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Message, "Authorization header scheme 'custom' mismatch")
}
