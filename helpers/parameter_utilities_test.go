// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
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
