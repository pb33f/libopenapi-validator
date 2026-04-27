// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/require"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// Test ExtractParamsForOperation with various HTTP methods
func TestExtractParamsForOperation(t *testing.T) {
	pathItem := &v3.PathItem{
		// Parameters: []*v3.Parameter{{Name: "param"}},
		Get:     &v3.Operation{Parameters: []*v3.Parameter{{Name: "getParam"}}},
		Post:    &v3.Operation{Parameters: []*v3.Parameter{{Name: "postParam"}}},
		Put:     &v3.Operation{Parameters: []*v3.Parameter{{Name: "putParam"}}},
		Delete:  &v3.Operation{Parameters: []*v3.Parameter{{Name: "deleteParam"}}},
		Options: &v3.Operation{Parameters: []*v3.Parameter{{Name: "optionsParam"}}},
		Head:    &v3.Operation{Parameters: []*v3.Parameter{{Name: "headParam"}}},
		Patch:   &v3.Operation{Parameters: []*v3.Parameter{{Name: "patchParam"}}},
		Trace:   &v3.Operation{Parameters: []*v3.Parameter{{Name: "traceParam"}}},
	}

	// Test all HTTP methods
	tests := []struct {
		method   string
		expected []string // Expected parameter names
	}{
		{http.MethodGet, []string{"getParam"}},
		{http.MethodPost, []string{"postParam"}},
		{http.MethodPut, []string{"putParam"}},
		{http.MethodDelete, []string{"deleteParam"}},
		{http.MethodOptions, []string{"optionsParam"}},
		{http.MethodHead, []string{"headParam"}},
		{http.MethodPatch, []string{"patchParam"}},
		{http.MethodTrace, []string{"traceParam"}},
	}

	for _, tt := range tests {
		// Create a new request with the specified method
		request, _ := http.NewRequest(tt.method, "/", nil)

		// Extract the parameters for the current request method
		params := ExtractParamsForOperation(request, pathItem)

		// Check if the number of parameters matches the expected count
		require.Len(t, params, len(tt.expected))

		// Verify that the extracted parameter names match the expected ones
		for i, param := range params {
			require.Equal(t, tt.expected[i], param.Name)
		}
	}
}

// Test cast with different values (bool, int, float, string)
func TestCast(t *testing.T) {
	require.Equal(t, true, cast("true"))
	require.Equal(t, int64(123), cast("123"))
	require.Equal(t, 123.45, cast("123.45"))
	require.Equal(t, "test", cast("test"))
}

// Test ExtractSecurityForOperation with various HTTP methods
func TestExtractSecurityForOperation(t *testing.T) {
	// Create a PathItem with security requirements for each method
	pathItem := &v3.PathItem{
		Get:    &v3.Operation{Security: []*base.SecurityRequirement{{}}},
		Post:   &v3.Operation{Security: []*base.SecurityRequirement{{}}},
		Put:    &v3.Operation{Security: []*base.SecurityRequirement{{}}},
		Delete: &v3.Operation{Security: []*base.SecurityRequirement{{}}},
		Options: &v3.Operation{
			Security: []*base.SecurityRequirement{{}},
		},
		Head: &v3.Operation{
			Security: []*base.SecurityRequirement{{}},
		},
		Patch: &v3.Operation{
			Security: []*base.SecurityRequirement{{}},
		},
		Trace: &v3.Operation{
			Security: []*base.SecurityRequirement{{}},
		},
	}

	// Test all HTTP methods
	tests := []struct {
		method string
	}{
		{http.MethodGet},
		{http.MethodPost},
		{http.MethodPut},
		{http.MethodDelete},
		{http.MethodOptions},
		{http.MethodHead},
		{http.MethodPatch},
		{http.MethodTrace},
	}

	for _, tt := range tests {
		// Create a new request with the specified method
		request, _ := http.NewRequest(tt.method, "/", nil)

		// Extract the security requirements for the current request method
		security := ExtractSecurityForOperation(request, pathItem)

		// Check if the number of security requirements matches the expected count (1 in all cases)
		require.Len(t, security, 1, "Failed for method: "+tt.method)
	}
}

// Test ExtractSecurityHeaderNames with various security scheme types
func TestExtractSecurityHeaderNames(t *testing.T) {
	t.Run("nil inputs", func(t *testing.T) {
		require.Nil(t, ExtractSecurityHeaderNames(nil, nil))
		require.Nil(t, ExtractSecurityHeaderNames([]*base.SecurityRequirement{}, nil))
		require.Nil(t, ExtractSecurityHeaderNames(nil, map[string]*v3.SecurityScheme{}))
	})

	t.Run("apiKey with in:header", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyAuth": {
				Type: "apiKey",
				In:   "header",
				Name: "X-API-Key",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"ApiKeyAuth": {"read"},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Equal(t, []string{"X-API-Key"}, headers)
	})

	t.Run("apiKey with in:query should not add header", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyQuery": {
				Type: "apiKey",
				In:   "query",
				Name: "api_key",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"ApiKeyQuery": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})

	t.Run("apiKey with in:cookie should not add header", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyCookie": {
				Type: "apiKey",
				In:   "cookie",
				Name: "session_id",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"ApiKeyCookie": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})

	t.Run("http bearer scheme", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"BearerAuth": {
				Type:   "http",
				Scheme: "bearer",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"BearerAuth": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Equal(t, []string{"Authorization"}, headers)
	})

	t.Run("http basic scheme", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"BasicAuth": {
				Type:   "http",
				Scheme: "basic",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"BasicAuth": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Equal(t, []string{"Authorization"}, headers)
	})

	t.Run("oauth2 scheme", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"OAuth2": {
				Type: "oauth2",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"OAuth2": {"read:users"},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Equal(t, []string{"Authorization"}, headers)
	})

	t.Run("openIdConnect scheme", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"OpenID": {
				Type:             "openIdConnect",
				OpenIdConnectUrl: "https://example.com/.well-known/openid-configuration",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"OpenID": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Equal(t, []string{"Authorization"}, headers)
	})

	t.Run("empty security requirement (ContainsEmptyRequirement)", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyAuth": {
				Type: "apiKey",
				In:   "header",
				Name: "X-API-Key",
			},
		}
		security := []*base.SecurityRequirement{
			{
				ContainsEmptyRequirement: true,
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})

	t.Run("nil security requirement in slice", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyAuth": {
				Type: "apiKey",
				In:   "header",
				Name: "X-API-Key",
			},
		}
		security := []*base.SecurityRequirement{nil}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})

	t.Run("security requirement with nil Requirements map", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyAuth": {
				Type: "apiKey",
				In:   "header",
				Name: "X-API-Key",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: nil,
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})

	t.Run("multiple security options OR - different headers", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyAuth": {
				Type: "apiKey",
				In:   "header",
				Name: "X-API-Key",
			},
			"BearerAuth": {
				Type:   "http",
				Scheme: "bearer",
			},
		}
		// OR logic: separate security requirements
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"ApiKeyAuth": {},
				}),
			},
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"BearerAuth": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Len(t, headers, 2)
		require.Contains(t, headers, "X-API-Key")
		require.Contains(t, headers, "Authorization")
	})

	t.Run("combined requirements AND - both headers", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyAuth": {
				Type: "apiKey",
				In:   "header",
				Name: "X-API-Key",
			},
			"BearerAuth": {
				Type:   "http",
				Scheme: "bearer",
			},
		}
		// AND logic: multiple schemes in one requirement
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"ApiKeyAuth": {},
					"BearerAuth": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Len(t, headers, 2)
		require.Contains(t, headers, "X-API-Key")
		require.Contains(t, headers, "Authorization")
	})

	t.Run("security scheme not found in schemes map", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"SomeOtherScheme": {
				Type: "apiKey",
				In:   "header",
				Name: "X-Other",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"NonExistent": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})

	t.Run("nil scheme in schemes map", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"NilScheme": nil,
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"NilScheme": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})

	t.Run("deduplication of Authorization header", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"BearerAuth": {
				Type:   "http",
				Scheme: "bearer",
			},
			"OAuth2": {
				Type: "oauth2",
			},
		}
		// Both use Authorization header
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"BearerAuth": {},
				}),
			},
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"OAuth2": {"read"},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Equal(t, []string{"Authorization"}, headers)
	})

	t.Run("case insensitive type matching", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"ApiKeyAuth": {
				Type: "APIKEY", // uppercase
				In:   "HEADER", // uppercase
				Name: "X-API-Key",
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"ApiKeyAuth": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Equal(t, []string{"X-API-Key"}, headers)
	})

	t.Run("unknown security type is ignored", func(t *testing.T) {
		schemes := map[string]*v3.SecurityScheme{
			"Unknown": {
				Type: "mutualTLS", // valid OpenAPI type but doesn't use headers
			},
		}
		security := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"Unknown": {},
				}),
			},
		}
		headers := ExtractSecurityHeaderNames(security, schemes)
		require.Nil(t, headers)
	})
}

func TestConstructParamMapFromDeepObjectEncoding(t *testing.T) {
	// Define mock values for testing
	values := []*QueryParam{
		{Key: "key1", Values: []string{"value1"}, Property: "prop1"},
		{Key: "key2", Values: []string{"123"}, Property: "prop2"},
		{Key: "key3", Values: []string{"456", "789"}, Property: "prop3"},
	}

	// Test case 1: Schema is nil
	decoded := ConstructParamMapFromDeepObjectEncoding(values, nil)
	require.NotNil(t, decoded)
	require.Equal(t, "value1", decoded["key1"].(map[string]interface{})["prop1"])
	require.Equal(t, int64(123), decoded["key2"].(map[string]interface{})["prop2"])
	require.Equal(t, int64(456), decoded["key3"].(map[string]interface{})["prop3"])

	// Test case 2: Schema type contains array for the first param (Array handling)
	schema := &base.Schema{Type: []string{"array"}}
	decoded = ConstructParamMapFromDeepObjectEncoding(values, schema)
	require.NotNil(t, decoded)
	require.Equal(t, []interface{}{"value1"}, decoded["key1"].(map[string]interface{})["prop1"])
	require.Equal(t, []interface{}{int64(123)}, decoded["key2"].(map[string]interface{})["prop2"])
	require.Equal(t, []interface{}{int64(456), int64(789)}, decoded["key3"].(map[string]interface{})["prop3"])

	// Test case 3: Schema with additional properties that is an array
	proxy := base.CreateSchemaProxy(&base.Schema{
		Type: []string{"array"},
	})

	schema = &base.Schema{
		AdditionalProperties: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: proxy,
		},
	}
	decoded = ConstructParamMapFromDeepObjectEncoding(values, schema)
	require.NotNil(t, decoded)
	require.Equal(t, []interface{}{"value1"}, decoded["key1"].(map[string]interface{})["prop1"])
	require.Equal(t, []interface{}{int64(123)}, decoded["key2"].(map[string]interface{})["prop2"])
	require.Equal(t, []interface{}{int64(456), int64(789)}, decoded["key3"].(map[string]interface{})["prop3"])

	// Test case 4: Adding a value to an existing key in the decoded map
	valuesWithDup := []*QueryParam{
		{Key: "key1", Values: []string{"value2"}, Property: "prop1"},
		{Key: "key2", Values: []string{"456"}, Property: "prop2"},
	}
	decoded = ConstructParamMapFromDeepObjectEncoding(valuesWithDup, nil)
	require.NotNil(t, decoded)
	require.Equal(t, "value2", decoded["key1"].(map[string]interface{})["prop1"])
	require.Equal(t, int64(456), decoded["key2"].(map[string]interface{})["prop2"])

	// Test case 5: Schema is not an array (standard object)
	nonArraySchema := &base.Schema{Type: []string{"object"}}
	decoded = ConstructParamMapFromDeepObjectEncoding(values, nonArraySchema)
	require.NotNil(t, decoded)
	require.Equal(t, "value1", decoded["key1"].(map[string]interface{})["prop1"])
	require.Equal(t, int64(123), decoded["key2"].(map[string]interface{})["prop2"])
	require.Equal(t, int64(456), decoded["key3"].(map[string]interface{})["prop3"])
}

func TestConstructParamMapFromDeepObjectEncoding_ElseCase(t *testing.T) {
	arraySchema := &base.Schema{Type: []string{"array"}, AdditionalProperties: &base.DynamicValue[*base.SchemaProxy, bool]{
		A: base.CreateSchemaProxy(&base.Schema{Type: []string{"array"}}),
	}}
	newValues := []*QueryParam{
		{Key: "key1", Values: []string{"456", "789"}, Property: "prop3"},
		{Key: "key1", Values: []string{"999", "888"}, Property: "prop3"},
	}
	decoded := ConstructParamMapFromDeepObjectEncoding(newValues, arraySchema)
	require.Equal(t, []interface{}{int64(999), int64(888)}, decoded["key1"].(map[string]interface{})["prop3"])

	arraySchema = &base.Schema{Type: []string{"integer"}, AdditionalProperties: &base.DynamicValue[*base.SchemaProxy, bool]{
		A: base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}}),
	}}
	newValues = []*QueryParam{
		{Key: "key1", Values: []string{"456", "789"}, Property: "prop3"},
		{Key: "key1", Values: []string{"999", "888"}, Property: "prop3"},
	}
	decoded = ConstructParamMapFromDeepObjectEncoding(newValues, arraySchema)
	require.Equal(t, int64(999), decoded["key1"].(map[string]interface{})["prop3"])
}

func TestParseDeepObjectKey(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		expectedBase string
		expectedPath []string
		expectedOK   bool
	}{
		{
			name:         "flat",
			key:          "obj[root]",
			expectedBase: "obj",
			expectedPath: []string{"root"},
			expectedOK:   true,
		},
		{
			name:         "nested",
			key:          "obj[nested][child]",
			expectedBase: "obj",
			expectedPath: []string{"nested", "child"},
			expectedOK:   true,
		},
		{
			name:       "plain key",
			key:        "obj",
			expectedOK: false,
		},
		{
			name:       "empty segment",
			key:        "obj[]",
			expectedOK: false,
		},
		{
			name:       "trailing text",
			key:        "obj[root]extra",
			expectedOK: false,
		},
		{
			name:       "missing closing bracket",
			key:        "obj[root",
			expectedOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseName, propertyPath, ok := ParseDeepObjectKey(tc.key)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expectedBase, baseName)
			require.Equal(t, tc.expectedPath, propertyPath)
		})
	}
}

func TestConstructParamMapFromDeepObjectEncoding_NestedObject(t *testing.T) {
	sch := &base.Schema{
		Type: []string{"object"},
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"root": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			"nested": base.CreateSchemaProxy(&base.Schema{
				Type: []string{"object"},
				Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
					"child": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
					"count": base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}}),
				}),
			}),
		}),
	}
	values := []*QueryParam{
		{Key: "obj", Values: []string{"test1"}, Property: "root", PropertyPath: []string{"root"}},
		{Key: "obj", Values: []string{"10"}, Property: "nested", PropertyPath: []string{"nested", "child"}},
		{Key: "obj", Values: []string{"42"}, Property: "nested", PropertyPath: []string{"nested", "count"}},
	}

	decoded := ConstructParamMapFromDeepObjectEncoding(values, sch)
	obj := decoded["obj"].(map[string]interface{})
	nested := obj["nested"].(map[string]interface{})

	require.Equal(t, "test1", obj["root"])
	require.Equal(t, "10", nested["child"])
	require.Equal(t, int64(42), nested["count"])
}

func TestConstructParamMapFromDeepObjectEncoding_NestedArray(t *testing.T) {
	sch := &base.Schema{
		Type: []string{"object"},
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"nested": base.CreateSchemaProxy(&base.Schema{
				Type: []string{"object"},
				Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
					"tags": base.CreateSchemaProxy(&base.Schema{
						Type: []string{"array"},
						Items: &base.DynamicValue[*base.SchemaProxy, bool]{
							A: base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
						},
					}),
				}),
			}),
		}),
	}
	values := []*QueryParam{
		{Key: "obj", Values: []string{"123", "456"}, Property: "nested", PropertyPath: []string{"nested", "tags"}},
	}

	decoded := ConstructParamMapFromDeepObjectEncoding(values, sch)
	obj := decoded["obj"].(map[string]interface{})
	nested := obj["nested"].(map[string]interface{})

	require.Equal(t, []interface{}{"123", "456"}, nested["tags"])
	require.True(t, DeepObjectAllowsMultipleValues(sch, values[0]))
}

func TestConstructParamMapFromDeepObjectEncoding_NestedAdditionalPropertiesArray(t *testing.T) {
	sch := &base.Schema{
		Type: []string{"object"},
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"filters": base.CreateSchemaProxy(&base.Schema{
				Type: []string{"object"},
				AdditionalProperties: &base.DynamicValue[*base.SchemaProxy, bool]{
					A: base.CreateSchemaProxy(&base.Schema{
						Type: []string{"array"},
						Items: &base.DynamicValue[*base.SchemaProxy, bool]{
							A: base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
						},
					}),
				},
			}),
		}),
	}
	values := []*QueryParam{
		{Key: "obj", Values: []string{"123", "456"}, Property: "filters", PropertyPath: []string{"filters", "tag"}},
	}

	decoded := ConstructParamMapFromDeepObjectEncoding(values, sch)
	obj := decoded["obj"].(map[string]interface{})
	filters := obj["filters"].(map[string]interface{})

	require.Equal(t, []interface{}{"123", "456"}, filters["tag"])
	require.True(t, DeepObjectAllowsMultipleValues(sch, values[0]))
}

func TestDeepObjectPathConflict(t *testing.T) {
	tests := []struct {
		name       string
		values     []*QueryParam
		expect     bool
		prefixPath []string
		nestedPath []string
	}{
		{
			name: "scalar before nested",
			values: []*QueryParam{
				{Key: "obj", Values: []string{"bad"}, Property: "nested", PropertyPath: []string{"nested"}},
				{Key: "obj", Values: []string{"ok"}, Property: "nested", PropertyPath: []string{"nested", "child"}},
			},
			expect:     true,
			prefixPath: []string{"nested"},
			nestedPath: []string{"nested", "child"},
		},
		{
			name: "nested before scalar",
			values: []*QueryParam{
				{Key: "obj", Values: []string{"ok"}, Property: "nested", PropertyPath: []string{"nested", "child"}},
				{Key: "obj", Values: []string{"bad"}, Property: "nested", PropertyPath: []string{"nested"}},
			},
			expect:     true,
			prefixPath: []string{"nested"},
			nestedPath: []string{"nested", "child"},
		},
		{
			name: "same array path",
			values: []*QueryParam{
				{Key: "obj", Values: []string{"alpha"}, Property: "nested", PropertyPath: []string{"nested", "tags"}},
				{Key: "obj", Values: []string{"beta"}, Property: "nested", PropertyPath: []string{"nested", "tags"}},
			},
		},
		{
			name: "sibling nested paths",
			values: []*QueryParam{
				{Key: "obj", Values: []string{"ok"}, Property: "nested", PropertyPath: []string{"nested", "child"}},
				{Key: "obj", Values: []string{"ok"}, Property: "nested", PropertyPath: []string{"nested", "other"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prefixParam, nestedParam, ok := DeepObjectPathConflict(tc.values)
			require.Equal(t, tc.expect, ok)
			if !tc.expect {
				return
			}
			require.Equal(t, tc.prefixPath, prefixParam.PropertyPath)
			require.Equal(t, tc.nestedPath, nestedParam.PropertyPath)
		})
	}
}

func TestSetNestedDeepObjectValue_PreservesConflicts(t *testing.T) {
	t.Run("scalar before nested", func(t *testing.T) {
		target := make(map[string]interface{})

		require.True(t, setNestedDeepObjectValue(target, []string{"nested"}, "bad"))
		require.False(t, setNestedDeepObjectValue(target, []string{"nested", "child"}, "ok"))
		require.IsType(t, []interface{}{}, target["nested"])
	})

	t.Run("nested before scalar", func(t *testing.T) {
		target := make(map[string]interface{})

		require.True(t, setNestedDeepObjectValue(target, []string{"nested", "child"}, "ok"))
		require.False(t, setNestedDeepObjectValue(target, []string{"nested"}, "bad"))
		require.IsType(t, []interface{}{}, target["nested"])
	})
}

func TestConstructParamMapFromDeepObjectEncoding_NestedPathConflict(t *testing.T) {
	tests := []struct {
		name   string
		values []*QueryParam
	}{
		{
			name: "scalar before nested",
			values: []*QueryParam{
				{Key: "obj", Values: []string{"bad"}, Property: "nested", PropertyPath: []string{"nested"}},
				{Key: "obj", Values: []string{"ok"}, Property: "nested", PropertyPath: []string{"nested", "child"}},
			},
		},
		{
			name: "nested before scalar",
			values: []*QueryParam{
				{Key: "obj", Values: []string{"ok"}, Property: "nested", PropertyPath: []string{"nested", "child"}},
				{Key: "obj", Values: []string{"bad"}, Property: "nested", PropertyPath: []string{"nested"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decoded := ConstructParamMapFromDeepObjectEncoding(tc.values, nil)
			obj := decoded["obj"].(map[string]interface{})

			require.IsType(t, []interface{}{}, obj["nested"])
		})
	}
}

func TestConstructKVFromLabelEncoding(t *testing.T) {
	// Test case 1: Empty input string
	values := ""
	props := ConstructKVFromLabelEncoding(values)
	require.NotNil(t, props)
	require.Empty(t, props)

	// Test case 2: Single valid key-value pair
	values = "key1=value1"
	props = ConstructKVFromLabelEncoding(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])

	// Test case 3: Multiple valid key-value pairs
	values = "key1=value1.key2=value2"
	props = ConstructKVFromLabelEncoding(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])
	require.Equal(t, "value2", props["key2"])

	// Test case 4: Invalid key-value pair (missing equals)
	values = "key1=value1.key2"
	props = ConstructKVFromLabelEncoding(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])
	require.NotContains(t, props, "key2") // key2 should be ignored due to invalid format

	// Test case 5: Key-value pair where value needs to be cast to int and bool
	values = "key1=123.key2=true"
	props = ConstructKVFromLabelEncoding(values)
	require.NotNil(t, props)
	require.Equal(t, int64(123), props["key1"]) // cast to int
	require.Equal(t, true, props["key2"])       // cast to bool

	// Test case 6: Handle multiple valid and invalid key-value pairs
	values = "key1=value1.key2.key3=123.key4=true"
	props = ConstructKVFromLabelEncoding(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])   // valid
	require.Equal(t, int64(123), props["key3"]) // valid
	require.Equal(t, true, props["key4"])       // valid
	require.NotContains(t, props, "key2")       // invalid, missing value
}

func TestConstructParamMapFromQueryParamInput(t *testing.T) {
	// Test case 1: Empty input map
	values := map[string][]*QueryParam{}
	decoded := ConstructParamMapFromQueryParamInput(values)
	require.NotNil(t, decoded)
	require.Empty(t, decoded)

	// Test case 2: Single entry in the input map
	values = map[string][]*QueryParam{
		"param1": {
			{Key: "param1", Values: []string{"value1"}},
		},
	}
	decoded = ConstructParamMapFromQueryParamInput(values)
	require.NotNil(t, decoded)
	require.Equal(t, "value1", decoded["param1"])

	// Test case 3: Multiple entries in the input map
	values = map[string][]*QueryParam{
		"param1": {
			{Key: "param1", Values: []string{"value1"}},
		},
		"param2": {
			{Key: "param2", Values: []string{"123"}},
		},
		"param3": {
			{Key: "param3", Values: []string{"true"}},
		},
	}
	decoded = ConstructParamMapFromQueryParamInput(values)
	require.NotNil(t, decoded)
	require.Equal(t, "value1", decoded["param1"])
	require.Equal(t, int64(123), decoded["param2"]) // cast to int
	require.Equal(t, true, decoded["param3"])       // cast to bool

	// Test case 4: Handle multiple values but only the first value is used
	values = map[string][]*QueryParam{
		"param1": {
			{Key: "param1", Values: []string{"first", "second"}},
		},
	}
	decoded = ConstructParamMapFromQueryParamInput(values)
	require.NotNil(t, decoded)
	require.Equal(t, "first", decoded["param1"]) // Only the first value is used

	// Test case 5: Handle different types of values
	values = map[string][]*QueryParam{
		"intParam": {
			{Key: "intParam", Values: []string{"42"}},
		},
		"boolParam": {
			{Key: "boolParam", Values: []string{"false"}},
		},
		"stringParam": {
			{Key: "stringParam", Values: []string{"hello"}},
		},
	}
	decoded = ConstructParamMapFromQueryParamInput(values)
	require.NotNil(t, decoded)
	require.Equal(t, int64(42), decoded["intParam"])
	require.Equal(t, false, decoded["boolParam"])
	require.Equal(t, "hello", decoded["stringParam"])
}

// Test ConstructParamMapFromPipeEncoding
func TestConstructParamMapFromPipeEncoding(t *testing.T) {
	params := []*QueryParam{
		{Key: "key1", Values: []string{"name|value"}},
	}
	result := ConstructParamMapFromPipeEncoding(params)
	require.Equal(t, "value", result["key1"].(map[string]interface{})["name"])
}

// Test ConstructParamMapFromSpaceEncoding
func TestConstructParamMapFromSpaceEncoding(t *testing.T) {
	params := []*QueryParam{
		{Key: "key1", Values: []string{"name value"}},
	}
	result := ConstructParamMapFromSpaceEncoding(params)
	require.Equal(t, "value", result["key1"].(map[string]interface{})["name"])
}

// Test ConstructMapFromCSV
func TestConstructMapFromCSV(t *testing.T) {
	result := ConstructMapFromCSV("key1,value1,key2,value2")
	require.Equal(t, "value1", result["key1"])
	require.Equal(t, "value2", result["key2"])

	// add odd number of keys/values
	result = ConstructMapFromCSV("key1,value1,key2")
	require.Equal(t, "value1", result["key1"])
}

// Test ConstructKVFromCSV
func TestConstructKVFromCSV(t *testing.T) {
	result := ConstructKVFromCSV("key1=value1,key2=value2")
	require.Equal(t, "value1", result["key1"])
	require.Equal(t, "value2", result["key2"])
}

// Test CollapseCSVIntoFormStyle
func TestCollapseCSVIntoFormStyle(t *testing.T) {
	result := CollapseCSVIntoFormStyle("key", "value1,value2")
	require.Equal(t, "&key=value1&key=value2", result)
}

// Test CollapseCSVIntoSpaceDelimitedStyle
func TestCollapseCSVIntoSpaceDelimitedStyle(t *testing.T) {
	result := CollapseCSVIntoSpaceDelimitedStyle("key", []string{"value1", "value2"})
	require.Equal(t, "key=value1%20value2", result)
}

// Test CollapseCSVIntoPipeDelimitedStyle
func TestCollapseCSVIntoPipeDelimitedStyle(t *testing.T) {
	result := CollapseCSVIntoPipeDelimitedStyle("key", []string{"value1", "value2"})
	require.Equal(t, "key=value1|value2", result)
}

// Test DoesFormParamContainDelimiter
func TestDoesFormParamContainDelimiter(t *testing.T) {
	require.True(t, DoesFormParamContainDelimiter("value1,value2", ""))
	require.False(t, DoesFormParamContainDelimiter("value1 value2", ""))
}

// Test ExplodeQueryValue
func TestExplodeQueryValue(t *testing.T) {
	require.Equal(t, []string{"value1", "value2"}, ExplodeQueryValue("value1,value2", ""))
	require.Equal(t, []string{"value1", "value2"}, ExplodeQueryValue("value1 value2", "spaceDelimited"))
	require.Equal(t, []string{"value1", "value2"}, ExplodeQueryValue("value1|value2", "pipeDelimited"))
}

func TestConstructKVFromMatrixCSV(t *testing.T) {
	// Test case 1: Empty input string
	values := ""
	props := ConstructKVFromMatrixCSV(values)
	require.NotNil(t, props)
	require.Empty(t, props)

	// Test case 2: Single valid key-value pair
	values = "key1=value1"
	props = ConstructKVFromMatrixCSV(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])

	// Test case 3: Multiple valid key-value pairs
	values = "key1=value1;key2=value2"
	props = ConstructKVFromMatrixCSV(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])
	require.Equal(t, "value2", props["key2"])

	// Test case 4: Invalid key-value pair (missing equals)
	values = "key1=value1;key2"
	props = ConstructKVFromMatrixCSV(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])
	require.NotContains(t, props, "key2") // key2 should be ignored due to invalid format

	// Test case 5: Key-value pair where value needs to be cast to int and bool
	values = "key1=123;key2=true"
	props = ConstructKVFromMatrixCSV(values)
	require.NotNil(t, props)
	require.Equal(t, int64(123), props["key1"]) // cast to int
	require.Equal(t, true, props["key2"])       // cast to bool

	// Test case 6: Handle multiple valid and invalid key-value pairs
	values = "key1=value1;key2;key3=456;key4=false"
	props = ConstructKVFromMatrixCSV(values)
	require.NotNil(t, props)
	require.Equal(t, "value1", props["key1"])   // valid
	require.Equal(t, int64(456), props["key3"]) // valid
	require.Equal(t, false, props["key4"])      // valid
	require.NotContains(t, props, "key2")       // invalid, missing value
}

func TestConstructParamMapFromFormEncodingArray(t *testing.T) {
	// Test case 1: Empty input
	values := []*QueryParam{}
	decoded := ConstructParamMapFromFormEncodingArray(values)
	require.NotNil(t, decoded)
	require.Empty(t, decoded)

	// Test case 2: Single QueryParam with valid key-value pairs
	values = []*QueryParam{
		{
			Key:    "param1",
			Values: []string{"key1,value1,key2,value2"},
		},
	}
	decoded = ConstructParamMapFromFormEncodingArray(values)
	require.NotNil(t, decoded)
	require.Contains(t, decoded, "param1")
	require.Equal(t, "value1", decoded["param1"].(map[string]interface{})["key1"])
	require.Equal(t, "value2", decoded["param1"].(map[string]interface{})["key2"])

	// Test case 3: Multiple QueryParam entries
	values = []*QueryParam{
		{
			Key:    "param1",
			Values: []string{"key1,value1"},
		},
		{
			Key:    "param2",
			Values: []string{"key3,value3,key4,value4"},
		},
	}
	decoded = ConstructParamMapFromFormEncodingArray(values)
	require.NotNil(t, decoded)
	require.Contains(t, decoded, "param1")
	require.Equal(t, "value1", decoded["param1"].(map[string]interface{})["key1"])
	require.Equal(t, "value3", decoded["param2"].(map[string]interface{})["key3"])
	require.Equal(t, "value4", decoded["param2"].(map[string]interface{})["key4"])

	// Test case 4: Odd number of values (incomplete key-value pair)
	values = []*QueryParam{
		{
			Key:    "param1",
			Values: []string{"key1,value1,key2"},
		},
	}
	decoded = ConstructParamMapFromFormEncodingArray(values)
	require.NotNil(t, decoded)
	require.Contains(t, decoded, "param1")
	require.Equal(t, "value1", decoded["param1"].(map[string]interface{})["key1"])
	require.NotContains(t, decoded["param1"].(map[string]interface{}), "key2") // Invalid, no value for key2

	// Test case 5: Casting different types (int, bool, string)
	values = []*QueryParam{
		{
			Key:    "param1",
			Values: []string{"key1,123,key2,true,key3,hello"},
		},
	}
	decoded = ConstructParamMapFromFormEncodingArray(values)
	require.NotNil(t, decoded)
	require.Contains(t, decoded, "param1")
	require.Equal(t, int64(123), decoded["param1"].(map[string]interface{})["key1"]) // cast to int
	require.Equal(t, true, decoded["param1"].(map[string]interface{})["key2"])       // cast to bool
	require.Equal(t, "hello", decoded["param1"].(map[string]interface{})["key3"])    // string remains string
}

func TestCastWithSchema(t *testing.T) {
	t.Run("returns string unchanged when schema property type is string", func(t *testing.T) {
		sch := &base.Schema{
			Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
				"item_count": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			}),
		}
		result := castWithSchema("10", sch, "item_count")
		require.Equal(t, "10", result)
	})

	t.Run("casts to int64 when schema property type is integer", func(t *testing.T) {
		sch := &base.Schema{
			Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
				"count": base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}}),
			}),
		}
		result := castWithSchema("10", sch, "count")
		require.Equal(t, int64(10), result)
	})

	t.Run("falls back to cast when no schema provided", func(t *testing.T) {
		result := castWithSchema("10", nil, "anything")
		require.Equal(t, int64(10), result)
	})

	t.Run("falls back to cast when property not found in schema", func(t *testing.T) {
		sch := &base.Schema{
			Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
				"other": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			}),
		}
		result := castWithSchema("10", sch, "missing")
		require.Equal(t, int64(10), result)
	})

	t.Run("falls back to cast when schema has no properties", func(t *testing.T) {
		sch := &base.Schema{Type: []string{"object"}}
		result := castWithSchema("10", sch, "anything")
		require.Equal(t, int64(10), result)
	})
}

func TestConstructParamMapFromQueryParamInputWithSchema(t *testing.T) {
	t.Run("preserves string '10' when schema says string", func(t *testing.T) {
		sch := &base.Schema{
			Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
				"item_count":  base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
				"search_term": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			}),
		}
		values := map[string][]*QueryParam{
			"item_count": {
				{Key: "item_count", Values: []string{"10"}},
			},
			"search_term": {
				{Key: "search_term", Values: []string{"foo"}},
			},
		}
		decoded := ConstructParamMapFromQueryParamInputWithSchema(values, sch)
		require.Equal(t, "10", decoded["item_count"])
		require.Equal(t, "foo", decoded["search_term"])
	})

	t.Run("casts numeric values when schema says integer", func(t *testing.T) {
		sch := &base.Schema{
			Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
				"count": base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}}),
			}),
		}
		values := map[string][]*QueryParam{
			"count": {
				{Key: "count", Values: []string{"42"}},
			},
		}
		decoded := ConstructParamMapFromQueryParamInputWithSchema(values, sch)
		require.Equal(t, int64(42), decoded["count"])
	})

	t.Run("falls back to heuristic when no schema", func(t *testing.T) {
		values := map[string][]*QueryParam{
			"count": {
				{Key: "count", Values: []string{"42"}},
			},
		}
		decoded := ConstructParamMapFromQueryParamInputWithSchema(values, nil)
		require.Equal(t, int64(42), decoded["count"])
	})
}

func TestConstructParamMapFromDeepObjectEncoding_WithSchema(t *testing.T) {
	t.Run("preserves string values when schema property type is string", func(t *testing.T) {
		sch := &base.Schema{
			Type: []string{"object"},
			Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
				"prop1": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			}),
		}
		values := []*QueryParam{
			{Key: "key1", Values: []string{"123"}, Property: "prop1"},
		}
		decoded := ConstructParamMapFromDeepObjectEncoding(values, sch)
		require.Equal(t, "123", decoded["key1"].(map[string]interface{})["prop1"])
	})
}

func TestConstructParamMapFromPipeEncodingWithSchema(t *testing.T) {
	sch := &base.Schema{
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"name":  base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			"count": base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}}),
		}),
	}
	params := []*QueryParam{
		{Key: "key1", Values: []string{"name|123|count|42"}},
	}
	result := ConstructParamMapFromPipeEncodingWithSchema(params, sch)
	props := result["key1"].(map[string]interface{})
	require.Equal(t, "123", props["name"])      // string because schema says string
	require.Equal(t, int64(42), props["count"]) // int because schema says integer
}

func TestConstructParamMapFromSpaceEncodingWithSchema(t *testing.T) {
	sch := &base.Schema{
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"name": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
		}),
	}
	params := []*QueryParam{
		{Key: "key1", Values: []string{"name 456"}},
	}
	result := ConstructParamMapFromSpaceEncodingWithSchema(params, sch)
	props := result["key1"].(map[string]interface{})
	require.Equal(t, "456", props["name"]) // string because schema says string
}

func TestConstructMapFromCSVWithSchema(t *testing.T) {
	sch := &base.Schema{
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"id":   base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			"rank": base.CreateSchemaProxy(&base.Schema{Type: []string{"number"}}),
		}),
	}
	result := ConstructMapFromCSVWithSchema("id,99,rank,3.5", sch)
	require.Equal(t, "99", result["id"])  // string
	require.Equal(t, 3.5, result["rank"]) // number

	// odd number of values
	result = ConstructMapFromCSVWithSchema("id,99,rank", sch)
	require.Equal(t, "99", result["id"])
	require.NotContains(t, result, "rank")
}

func TestConstructKVFromCSVWithSchema(t *testing.T) {
	sch := &base.Schema{
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"key1": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			"key2": base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}}),
		}),
	}
	result := ConstructKVFromCSVWithSchema("key1=100,key2=200", sch)
	require.Equal(t, "100", result["key1"])      // string
	require.Equal(t, int64(200), result["key2"]) // integer
}

func TestConstructKVFromLabelEncodingWithSchema(t *testing.T) {
	sch := &base.Schema{
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"key1": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			"key2": base.CreateSchemaProxy(&base.Schema{Type: []string{"boolean"}}),
		}),
	}
	result := ConstructKVFromLabelEncodingWithSchema("key1=true.key2=true", sch)
	require.Equal(t, "true", result["key1"]) // string because schema says string
	require.Equal(t, true, result["key2"])   // bool because schema says boolean

	// invalid pair (missing equals) is ignored
	result = ConstructKVFromLabelEncodingWithSchema("key1=val.key2", sch)
	require.Equal(t, "val", result["key1"])
	require.NotContains(t, result, "key2")
}

func TestConstructKVFromMatrixCSVWithSchema(t *testing.T) {
	sch := &base.Schema{
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"key1": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
		}),
	}
	result := ConstructKVFromMatrixCSVWithSchema("key1=456;key2=789", sch)
	require.Equal(t, "456", result["key1"])      // string
	require.Equal(t, int64(789), result["key2"]) // no schema for key2, falls back to cast

	// invalid pair
	result = ConstructKVFromMatrixCSVWithSchema("key1=val;key2", sch)
	require.Equal(t, "val", result["key1"])
	require.NotContains(t, result, "key2")
}

func TestConstructParamMapFromFormEncodingArrayWithSchema(t *testing.T) {
	sch := &base.Schema{
		Properties: orderedmap.ToOrderedMap(map[string]*base.SchemaProxy{
			"key1": base.CreateSchemaProxy(&base.Schema{Type: []string{"string"}}),
			"key2": base.CreateSchemaProxy(&base.Schema{Type: []string{"integer"}}),
		}),
	}
	values := []*QueryParam{
		{Key: "param1", Values: []string{"key1,123,key2,456"}},
	}
	decoded := ConstructParamMapFromFormEncodingArrayWithSchema(values, sch)
	props := decoded["param1"].(map[string]interface{})
	require.Equal(t, "123", props["key1"])      // string
	require.Equal(t, int64(456), props["key2"]) // integer

	// odd number of values — incomplete pair ignored
	values = []*QueryParam{
		{Key: "param1", Values: []string{"key1,val,key2"}},
	}
	decoded = ConstructParamMapFromFormEncodingArrayWithSchema(values, sch)
	props = decoded["param1"].(map[string]interface{})
	require.Equal(t, "val", props["key1"])
	require.NotContains(t, props, "key2")
}

func TestEffectiveSecurityForOperation(t *testing.T) {
	globalSecurity := []*base.SecurityRequirement{
		{
			Requirements: orderedmap.ToOrderedMap(map[string][]string{
				"GlobalAuth": {},
			}),
		},
	}

	opSecurity := []*base.SecurityRequirement{
		{
			Requirements: orderedmap.ToOrderedMap(map[string][]string{
				"OpAuth": {},
			}),
		},
	}

	t.Run("operation-level security wins over global", func(t *testing.T) {
		pathItem := &v3.PathItem{
			Get: &v3.Operation{Security: opSecurity},
		}
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		result := EffectiveSecurityForOperation(request, pathItem, globalSecurity)
		require.Equal(t, opSecurity, result)
	})

	t.Run("nil operation security falls back to global", func(t *testing.T) {
		pathItem := &v3.PathItem{
			Get: &v3.Operation{}, // Security is nil
		}
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		result := EffectiveSecurityForOperation(request, pathItem, globalSecurity)
		require.Equal(t, globalSecurity, result)
	})

	t.Run("empty operation security means no security (opt-out)", func(t *testing.T) {
		pathItem := &v3.PathItem{
			Get: &v3.Operation{Security: []*base.SecurityRequirement{}},
		}
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		result := EffectiveSecurityForOperation(request, pathItem, globalSecurity)
		require.NotNil(t, result)
		require.Len(t, result, 0)
	})

	t.Run("both nil returns nil", func(t *testing.T) {
		pathItem := &v3.PathItem{
			Get: &v3.Operation{},
		}
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		result := EffectiveSecurityForOperation(request, pathItem, nil)
		require.Nil(t, result)
	})

	t.Run("nil operation falls back to global", func(t *testing.T) {
		pathItem := &v3.PathItem{} // no Get operation
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		result := EffectiveSecurityForOperation(request, pathItem, globalSecurity)
		require.Equal(t, globalSecurity, result)
	})

	t.Run("HEAD falls back to GET operation security", func(t *testing.T) {
		pathItem := &v3.PathItem{
			Get: &v3.Operation{Security: opSecurity},
		}
		request, _ := http.NewRequest(http.MethodHead, "/", nil)
		result := EffectiveSecurityForOperation(request, pathItem, globalSecurity)
		require.Equal(t, opSecurity, result)
	})

	t.Run("HEAD with explicit Head security uses Head", func(t *testing.T) {
		headSecurity := []*base.SecurityRequirement{
			{
				Requirements: orderedmap.ToOrderedMap(map[string][]string{
					"HeadAuth": {},
				}),
			},
		}
		pathItem := &v3.PathItem{
			Get:  &v3.Operation{Security: opSecurity},
			Head: &v3.Operation{Security: headSecurity},
		}
		request, _ := http.NewRequest(http.MethodHead, "/", nil)
		result := EffectiveSecurityForOperation(request, pathItem, globalSecurity)
		require.Equal(t, headSecurity, result)
	})
}
