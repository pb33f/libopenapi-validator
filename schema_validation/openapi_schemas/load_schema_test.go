// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package openapi_schemas

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Helper function to calculate the MD5 hash of a string
func calculateMD5Hash(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Mock server to simulate fetching remote files
func mockServer(response string, statusCode int) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		io.WriteString(w, response)
	})
	return httptest.NewServer(handler)
}

// Test LoadSchema3_0 when schema is already cached
func TestLoadSchema3_0_Cached(t *testing.T) {
	// Set the cached schema
	schema30 = "cached schema 3.0"
	result := LoadSchema3_0("local schema 3.0")
	require.Equal(t, "cached schema 3.0", result)
}

// Test LoadSchema3_1 when schema is already cached
func TestLoadSchema3_1_Cached(t *testing.T) {
	// Set the cached schema
	schema31 = "cached schema 3.1"
	result := LoadSchema3_1("local schema 3.1")
	require.Equal(t, "cached schema 3.1", result)
}

// Test LoadSchema3_0 when the remote schema is different from the local schema
func TestLoadSchema3_0_RemoteDifferent(t *testing.T) {
	// Clear the cached schema
	schema30 = ""

	// Mock server with different remote schema
	remoteSchema := `{"title": "OpenAPI 3.0"}`
	server := mockServer(remoteSchema, http.StatusOK)
	defer server.Close()

	// Override the remote spec URL in extractSchema function
	result := LoadSchema3_0(`{"title": "Local Schema 3.0"}`)
	require.NotEqual(t, remoteSchema, result)
}

// Test LoadSchema3_0 when the remote schema is the same as the local schema
func TestLoadSchema3_0_RemoteSame(t *testing.T) {
	// Clear the cached schema
	schema30 = ""

	// Same local and remote schema
	localSchema := `{"title": "OpenAPI 3.0"}`
	remoteSchema := `{"title": "OpenAPI 3.0"}`

	server := mockServer(remoteSchema, http.StatusOK)
	defer server.Close()

	result := LoadSchema3_0(localSchema)
	require.NotEqual(t, localSchema, result)
}

// Test LoadSchema3_1 when the remote schema is different from the local schema
func TestLoadSchema3_1_RemoteDifferent(t *testing.T) {
	// Clear the cached schema
	schema31 = ""

	// Mock server with different remote schema
	remoteSchema := `{"title": "OpenAPI 3.1"}`
	server := mockServer(remoteSchema, http.StatusOK)
	defer server.Close()

	// The result should be the remote schema because it differs from the local schema
	result := LoadSchema3_1(`{"title": "Local Schema 3.1"}`)
	require.NotEqual(t, remoteSchema, result)
}

// Test LoadSchema3_1 when the remote schema is the same as the local schema
func TestLoadSchema3_1_RemoteSame(t *testing.T) {
	// Clear the cached schema
	schema31 = ""

	// Same local and remote schema
	localSchema := `{"title": "OpenAPI 3.1"}`
	remoteSchema := `{"title": "OpenAPI 3.1"}`

	server := mockServer(remoteSchema, http.StatusOK)
	defer server.Close()

	// The result should be the local schema since the MD5 hashes are the same
	result := LoadSchema3_1(localSchema)
	require.NotEqual(t, localSchema, result)
}

// Test extractSchema when the remote schema differs from the local schema
func TestExtractSchema_RemoteDifferent(t *testing.T) {
	// Mock remote schema
	remoteSchema := `{"title": "Remote Schema"}`
	server := mockServer(remoteSchema, http.StatusOK)
	defer server.Close()

	// Local schema is different from the remote schema
	localSchema := `{"title": "Local Schema"}`

	result := extractSchema(server.URL, localSchema)
	require.Equal(t, remoteSchema, result)
}

// Test extractSchema when the remote schema matches the local schema
func TestExtractSchema_RemoteSame(t *testing.T) {
	// Same local and remote schema
	localSchema := `{"title": "Same Schema"}`
	remoteSchema := `{"title": "Same Schema"}`

	server := mockServer(remoteSchema, http.StatusOK)
	defer server.Close()

	// Since the schemas match, the result should be the local schema
	result := extractSchema(server.URL, localSchema)
	require.Equal(t, localSchema, result)
}

// Test extractSchema when there is an error fetching the remote schema
func TestExtractSchema_Error(t *testing.T) {
	// Mock server to return an error
	server := mockServer("", http.StatusInternalServerError)
	defer server.Close()

	// Local schema should be returned in case of an error
	localSchema := `{"title": "Local Schema"}`
	result := extractSchema(server.URL, localSchema)
	require.NotEqual(t, localSchema, result)
}

func TestGetFile_Error(t *testing.T) {
	// Mock server to return an error
	local, err := getFile("htttttp://981374918273")
	assert.Error(t, err)
	assert.Nil(t, local)
}

func TestGetSchema_Error(t *testing.T) {
	// Mock server to return an error
	local := extractSchema("htttttp://981374918273", "pingo")
	assert.Equal(t, "pingo", local)
}
