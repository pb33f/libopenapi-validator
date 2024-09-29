// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package helpers

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test the Load function for a successful case
func TestHTTPURLLoader_Load_Success(t *testing.T) {
	// Create a mock HTTP server that returns a 200 response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"success": true}`)
	}))
	defer server.Close()

	loader := NewHTTPURLLoader(false)

	// Test the Load function
	_, err := loader.Load(server.URL)
	require.NoError(t, err)
}

// Test the Load function when the server returns a non-200 response
func TestHTTPURLLoader_Load_NonOKStatus(t *testing.T) {
	// Create a mock HTTP server that returns a 404 response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	loader := NewHTTPURLLoader(false)

	// Test the Load function
	_, err := loader.Load(server.URL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned status code 404")
}

// Test the Load function when the server returns an error
func TestHTTPURLLoader_Load_Error(t *testing.T) {
	loader := NewHTTPURLLoader(false)

	// Test the Load function with an invalid URL
	_, err := loader.Load("http://invalid-url")
	require.Error(t, err)
}

// Test the Load function with an insecure TLS config
func TestHTTPURLLoader_Load_Insecure(t *testing.T) {
	// Create a mock HTTPS server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"secure": true}`)
	}))
	defer server.Close()

	loader := NewHTTPURLLoader(true)

	// Test the Load function
	_, err := loader.Load(server.URL)
	require.NoError(t, err)
}

// Test the NewHTTPURLLoader function with insecure set to false
func TestNewHTTPURLLoader_Secure(t *testing.T) {
	loader := NewHTTPURLLoader(false)
	require.NotNil(t, loader)

	// Assert that the loader has the correct timeout and secure transport
	client := (*http.Client)(loader)
	require.Equal(t, 15*time.Second, client.Timeout)
	require.Nil(t, client.Transport) // Transport should be nil when secure
}

// Test the NewHTTPURLLoader function with insecure set to true
func TestNewHTTPURLLoader_Insecure(t *testing.T) {
	loader := NewHTTPURLLoader(true)
	require.NotNil(t, loader)

	// Assert that the loader has an insecure transport configuration
	client := (*http.Client)(loader)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

// Test the NewCompilerLoader function
func TestNewCompilerLoader(t *testing.T) {
	loader := NewCompilerLoader()
	require.NotNil(t, loader)

	// Assert that the loader contains the correct schemes
	require.NotNil(t, loader["http"])
	require.NotNil(t, loader["https"])
	require.NotNil(t, loader["file"])
}
