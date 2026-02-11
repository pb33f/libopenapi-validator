// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package helpers

import (
	"mime"
	"net/http"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/require"
)

// Test ExtractOperation for each HTTP method
func TestExtractOperation(t *testing.T) {
	pathItem := &v3.PathItem{
		Get:     &v3.Operation{Summary: "GET operation"},
		Post:    &v3.Operation{Summary: "POST operation"},
		Put:     &v3.Operation{Summary: "PUT operation"},
		Delete:  &v3.Operation{Summary: "DELETE operation"},
		Options: &v3.Operation{Summary: "OPTIONS operation"},
		Head:    &v3.Operation{Summary: "HEAD operation"},
		Patch:   &v3.Operation{Summary: "PATCH operation"},
		Trace:   &v3.Operation{Summary: "TRACE operation"},
	}

	// Test all HTTP methods
	tests := []struct {
		method string
		want   string
	}{
		{http.MethodGet, "GET operation"},
		{http.MethodPost, "POST operation"},
		{http.MethodPut, "PUT operation"},
		{http.MethodDelete, "DELETE operation"},
		{http.MethodOptions, "OPTIONS operation"},
		{http.MethodHead, "HEAD operation"},
		{http.MethodPatch, "PATCH operation"},
		{http.MethodTrace, "TRACE operation"},
	}

	for _, tt := range tests {
		req, _ := http.NewRequest(tt.method, "/", nil)
		operation := ExtractOperation(req, pathItem)
		require.NotNil(t, operation)
		require.Equal(t, tt.want, operation.Summary)
	}

	// Test an unsupported HTTP method
	req, _ := http.NewRequest("INVALID", "/", nil)
	operation := ExtractOperation(req, pathItem)
	require.Nil(t, operation)
}

// Test ExtractContentType for various input cases
func TestExtractContentType(t *testing.T) {
	// Simple content type with no charset or boundary
	contentType, charset, boundary := ExtractContentType("application/json")
	require.Equal(t, "application/json", contentType)
	require.Empty(t, charset)
	require.Empty(t, boundary)

	// Content type with charset
	contentType, charset, boundary = ExtractContentType("text/html; charset=UTF-8")
	require.Equal(t, "text/html", contentType)
	require.Equal(t, "UTF-8", charset)
	require.Empty(t, boundary)

	// Content type with boundary
	contentType, charset, boundary = ExtractContentType("multipart/form-data; boundary=----WebKitFormBoundary")
	require.Equal(t, "multipart/form-data", contentType)
	require.Empty(t, charset)
	require.Equal(t, "----WebKitFormBoundary", boundary)

	// Content type with both charset and boundary
	contentType, charset, boundary = ExtractContentType("multipart/form-data; charset=UTF-8; boundary=----WebKitFormBoundary")
	require.Equal(t, "multipart/form-data", contentType)
	require.Equal(t, "UTF-8", charset)
	require.Equal(t, "----WebKitFormBoundary", boundary)

	// Content type with leading/trailing spaces
	contentType, charset, boundary = ExtractContentType("  application/xml ; charset=ISO-8859-1 ; boundary=myBoundary  ")
	require.Equal(t, "application/xml", contentType)
	require.Equal(t, "ISO-8859-1", charset)
	require.Equal(t, "myBoundary", boundary)

	// Invalid content type (no key-value pair for charset/boundary)
	contentType, charset, boundary = ExtractContentType("application/xml; charset; boundary")
	require.Equal(t, "application/xml", contentType)
	require.Empty(t, charset)
	require.Empty(t, boundary)

	// Content type with custom parameter
	contentType, charset, boundary = ExtractContentType("text/html; version=2")
	require.Equal(t, "text/html", contentType)
	require.Empty(t, charset)
	require.Empty(t, boundary)

	// Content type with custom parameter, charset, and boundary
	contentType, charset, boundary = ExtractContentType("text/html; charset=UTF-8; version=2; boundary=myBoundary")
	require.Equal(t, "text/html", contentType)
	require.Equal(t, "UTF-8", charset)
	require.Equal(t, "myBoundary", boundary)

	// mime.ParseMediaType returns an error, but ExtractContentType still returns the content type.
	const ct = "text/plain;;"
	_, _, err := mime.ParseMediaType(ct)
	require.ErrorIs(t, err, mime.ErrInvalidMediaParameter)
	contentType, charset, boundary = ExtractContentType(ct)
	require.Equal(t, "text/plain", contentType)
	require.Empty(t, charset)
	require.Empty(t, boundary)
}
func TestExtractOperationHeadFallback(t *testing.T) {
	pathItem := &v3.PathItem{
		Get:  &v3.Operation{Summary: "GET operation"},
		Head: nil,
	}

	req, _ := http.NewRequest(http.MethodHead, "/", nil)
	operation := ExtractOperation(req, pathItem)
	require.NotNil(t, operation)
	require.Equal(t, "GET operation", operation.Summary)
}

func TestExtractOperationHeadFallbackNoGet(t *testing.T) {
	pathItem := &v3.PathItem{
		Head: nil,
		Get:  nil,
	}

	req, _ := http.NewRequest(http.MethodHead, "/", nil)
	operation := ExtractOperation(req, pathItem)
	require.Nil(t, operation)
}
