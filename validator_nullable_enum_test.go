// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNullableEnum_ResponseValidation_NullValue tests that nullable enum fields
// accept null values even when null is not explicitly in the enum definition
func TestNullableEnum_ResponseValidation_NullValue(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test response with null status (enum doesn't explicitly contain null)
	responseBody := map[string]interface{}{
		"id":     1,
		"status": nil, // null value for nullable enum
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/status", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.True(t, valid, "Response should be valid with null enum value")
	assert.Empty(t, validationErrs, "Should have no validation errors")
}

// TestNullableEnum_ResponseValidation_EnumValue tests that nullable enum fields
// accept valid enum values
func TestNullableEnum_ResponseValidation_EnumValue(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test response with valid enum value
	responseBody := map[string]interface{}{
		"id":     1,
		"status": "active", // valid enum value
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/status", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.True(t, valid, "Response should be valid with enum value")
	assert.Empty(t, validationErrs, "Should have no validation errors")
}

// TestNullableEnum_ResponseValidation_InvalidEnumValue tests that nullable enum fields
// reject invalid enum values
func TestNullableEnum_ResponseValidation_InvalidEnumValue(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test response with invalid enum value
	responseBody := map[string]interface{}{
		"id":     1,
		"status": "invalid_status", // invalid enum value
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/status", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.False(t, valid, "Response should be invalid with non-enum value")
	assert.NotEmpty(t, validationErrs, "Should have validation errors")
}

// TestNullableEnum_ResponseValidation_PriorityWithNullInEnum tests enum that
// already has null in the enum definition
func TestNullableEnum_ResponseValidation_PriorityWithNullInEnum(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test response with null priority (enum explicitly contains null)
	responseBody := map[string]interface{}{
		"id":       1,
		"priority": nil, // null value for nullable enum (null already in enum)
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/status", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.True(t, valid, "Response should be valid with null enum value")
	assert.Empty(t, validationErrs, "Should have no validation errors")
}

// TestNullableEnum_ResponseValidation_NonNullableEnum tests that non-nullable
// enum fields reject null values
func TestNullableEnum_ResponseValidation_NonNullableEnum(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test response with null category (non-nullable enum)
	responseBody := map[string]interface{}{
		"id":       1,
		"category": nil, // null value for NON-nullable enum
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/status", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.False(t, valid, "Response should be invalid with null for non-nullable enum")
	assert.NotEmpty(t, validationErrs, "Should have validation errors")
}

// TestNullableEnum_RequestValidation_NullValue tests that nullable enum fields
// accept null values in request body
func TestNullableEnum_RequestValidation_NullValue(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test request with null status
	requestBody := map[string]interface{}{
		"status": nil, // null value for nullable enum
	}

	body, _ := json.Marshal(requestBody)

	request, _ := http.NewRequest(http.MethodPost, "https://example.com/status", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")

	valid, validationErrs := v.ValidateHttpRequest(request)

	assert.True(t, valid, "Request should be valid with null enum value")
	assert.Empty(t, validationErrs, "Should have no validation errors")
}

// TestNullableEnum_ArrayResponse tests nullable enum in array items
func TestNullableEnum_ArrayResponse(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test array response with nullable enum
	responseBody := []map[string]interface{}{
		{
			"id":     1,
			"name":   "Item 1",
			"status": "available",
		},
		{
			"id":     2,
			"name":   "Item 2",
			"status": nil, // null value for nullable enum in array
		},
		{
			"id":     3,
			"name":   "Item 3",
			"status": "sold",
		},
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/items", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.True(t, valid, "Response should be valid with null enum value in array")
	assert.Empty(t, validationErrs, "Should have no validation errors")
}

// TestNullableEnum_NestedObject tests nullable enum in nested object
func TestNullableEnum_NestedObject(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test response with deeply nested nullable enum
	responseBody := []map[string]interface{}{
		{
			"id":   1,
			"name": "Item 1",
			"metadata": map[string]interface{}{
				"visibility": nil, // null value for deeply nested nullable enum
			},
		},
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/items", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.True(t, valid, "Response should be valid with null enum value in nested object")
	assert.Empty(t, validationErrs, "Should have no validation errors")
}

// TestNullableEnum_MultipleNullableFields tests response with multiple nullable enum fields
func TestNullableEnum_MultipleNullableFields(t *testing.T) {
	spec, err := os.ReadFile("test_specs/nullable_enum.yaml")
	require.NoError(t, err)

	doc, err := libopenapi.NewDocument(spec)
	require.NoError(t, err)

	v, errs := NewValidator(doc)
	require.Empty(t, errs)

	// Test response with multiple nullable fields set to null
	responseBody := map[string]interface{}{
		"id":       1,
		"status":   nil, // null for status (enum doesn't have null)
		"priority": nil, // null for priority (enum has null)
	}

	body, _ := json.Marshal(responseBody)

	request, _ := http.NewRequest(http.MethodGet, "https://example.com/status", nil)
	response := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		Request:    request,
	}

	valid, validationErrs := v.ValidateHttpResponse(request, response)

	assert.True(t, valid, "Response should be valid with multiple null enum values")
	assert.Empty(t, validationErrs, "Should have no validation errors")
}
