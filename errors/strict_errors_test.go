// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUndeclaredPropertyError(t *testing.T) {
	err := UndeclaredPropertyError(
		"$.body.user.extra",
		"extra",
		"some value",
		[]string{"name", "email"},
		"request",
		"/users",
		"POST",
		42,
		10,
	)

	assert.NotNil(t, err)
	assert.Equal(t, StrictValidationType, err.ValidationType)
	assert.Equal(t, StrictSubTypeProperty, err.ValidationSubType)
	assert.Contains(t, err.Message, "request property 'extra' at '$.body.user.extra'")
	assert.Contains(t, err.Reason, "name, email")
	assert.Contains(t, err.HowToFix, "extra")
	assert.Contains(t, err.HowToFix, "$.body.user.extra")
	assert.Equal(t, "/users", err.RequestPath)
	assert.Equal(t, "POST", err.RequestMethod)
	assert.Equal(t, "extra", err.ParameterName)
	assert.Equal(t, 42, err.SpecLine)
	assert.Equal(t, 10, err.SpecCol)
}

func TestUndeclaredPropertyError_Response(t *testing.T) {
	err := UndeclaredPropertyError(
		"$.body.data.undeclared",
		"undeclared",
		map[string]any{"nested": "value"},
		[]string{"id", "name"},
		"response",
		"/items/123",
		"GET",
		100,
		5,
	)

	assert.NotNil(t, err)
	assert.Contains(t, err.Message, "response property 'undeclared'")
	assert.Contains(t, err.Reason, "id, name")
	assert.Equal(t, "{...}", err.Context) // Map truncated
	assert.Equal(t, 100, err.SpecLine)
	assert.Equal(t, 5, err.SpecCol)
}

func TestUndeclaredPropertyError_EmptyDirection(t *testing.T) {
	err := UndeclaredPropertyError(
		"$.body.prop",
		"prop",
		"value",
		nil,
		"", // Empty direction defaults to "request"
		"/test",
		"POST",
		0, // Zero values for unknown location
		0,
	)

	assert.Contains(t, err.Message, "request property")
	assert.Equal(t, 0, err.SpecLine)
	assert.Equal(t, 0, err.SpecCol)
}

func TestUndeclaredHeaderError(t *testing.T) {
	err := UndeclaredHeaderError(
		"X-Custom-Header",
		"header-value",
		[]string{"Content-Type", "Authorization"},
		"request",
		"/api/endpoint",
		"GET",
	)

	assert.NotNil(t, err)
	assert.Equal(t, StrictValidationType, err.ValidationType)
	assert.Equal(t, StrictSubTypeHeader, err.ValidationSubType)
	assert.Contains(t, err.Message, "request header 'X-Custom-Header'")
	assert.Contains(t, err.Reason, "Content-Type, Authorization")
	assert.Contains(t, err.HowToFix, "X-Custom-Header")
	assert.Equal(t, "/api/endpoint", err.RequestPath)
	assert.Equal(t, "GET", err.RequestMethod)
	assert.Equal(t, "X-Custom-Header", err.ParameterName)
	assert.Equal(t, "header-value", err.Context)
}

func TestUndeclaredHeaderError_Response(t *testing.T) {
	err := UndeclaredHeaderError(
		"X-Response-Header",
		"value",
		nil,
		"response",
		"/test",
		"POST",
	)

	assert.Contains(t, err.Message, "response header")
}

func TestUndeclaredHeaderError_EmptyDirection(t *testing.T) {
	err := UndeclaredHeaderError(
		"X-Header",
		"value",
		nil,
		"",
		"/test",
		"GET",
	)

	assert.Contains(t, err.Message, "request header")
}

func TestUndeclaredQueryParamError(t *testing.T) {
	err := UndeclaredQueryParamError(
		"$.query.debug",
		"debug",
		"true",
		[]string{"page", "limit"},
		"/items",
		"GET",
	)

	assert.NotNil(t, err)
	assert.Equal(t, StrictValidationType, err.ValidationType)
	assert.Equal(t, StrictSubTypeQuery, err.ValidationSubType)
	assert.Contains(t, err.Message, "query parameter 'debug' at '$.query.debug'")
	assert.Contains(t, err.Reason, "page, limit")
	assert.Contains(t, err.HowToFix, "debug")
	assert.Contains(t, err.HowToFix, "$.query.debug")
	assert.Equal(t, "/items", err.RequestPath)
	assert.Equal(t, "GET", err.RequestMethod)
	assert.Equal(t, "debug", err.ParameterName)
}

func TestUndeclaredCookieError(t *testing.T) {
	err := UndeclaredCookieError(
		"$.cookies.tracking",
		"tracking",
		"abc123",
		[]string{"session", "csrf"},
		"/dashboard",
		"GET",
	)

	assert.NotNil(t, err)
	assert.Equal(t, StrictValidationType, err.ValidationType)
	assert.Equal(t, StrictSubTypeCookie, err.ValidationSubType)
	assert.Contains(t, err.Message, "cookie 'tracking' at '$.cookies.tracking'")
	assert.Contains(t, err.Reason, "session, csrf")
	assert.Contains(t, err.HowToFix, "tracking")
	assert.Contains(t, err.HowToFix, "$.cookies.tracking")
	assert.Equal(t, "/dashboard", err.RequestPath)
	assert.Equal(t, "GET", err.RequestMethod)
	assert.Equal(t, "tracking", err.ParameterName)
}

func TestTruncateForContext_String(t *testing.T) {
	// Short string should not be truncated
	short := truncateForContext("short")
	assert.Equal(t, "short", short)

	// Long string should be truncated
	long := truncateForContext("this is a very long string that exceeds fifty characters and should be truncated")
	assert.Len(t, long, 50)
	assert.True(t, len(long) <= 50)
	assert.Contains(t, long, "...")
}

func TestTruncateForContext_Map(t *testing.T) {
	m := map[string]any{"key": "value"}
	result := truncateForContext(m)
	assert.Equal(t, "{...}", result)
}

func TestTruncateForContext_Slice(t *testing.T) {
	s := []any{1, 2, 3}
	result := truncateForContext(s)
	assert.Equal(t, "[...]", result)
}

func TestTruncateForContext_Other(t *testing.T) {
	// Integer
	i := truncateForContext(12345)
	assert.Equal(t, "12345", i)

	// Boolean
	b := truncateForContext(true)
	assert.Equal(t, "true", b)

	// Long formatted value
	type customType struct {
		Field1 string
		Field2 string
		Field3 string
	}
	longValue := customType{
		Field1: "this is a long value",
		Field2: "that will exceed fifty",
		Field3: "characters when formatted",
	}
	result := truncateForContext(longValue)
	assert.True(t, len(result) <= 50)
	assert.Contains(t, result, "...")
}
