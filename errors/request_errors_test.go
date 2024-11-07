// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package errors

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/low"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"

	"github.com/pb33f/libopenapi-validator/helpers"
)

// Helper to create a mock v3.Operation object with a RequestBody
func createMockOperationWithRequestBody() *v3.Operation {
	content := orderedmap.New[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]()
	content.Set(low.KeyReference[string]{
		Value: "application/json",
	}, low.ValueReference[*lowv3.MediaType]{
		Value: &lowv3.MediaType{},
	})

	reqBody := &lowv3.RequestBody{
		Content: low.NodeReference[*orderedmap.Map[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]]{
			Value:     content,
			KeyNode:   &yaml.Node{Line: 10, Column: 20},
			ValueNode: &yaml.Node{},
		},
	}

	// Create a lowv3.Operation object
	op := &lowv3.Operation{
		RequestBody: low.NodeReference[*lowv3.RequestBody]{
			Value:     reqBody,
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}

	// Create a new v3.Operation object from the low
	return v3.NewOperation(op)
}

// Helper to create a mock v3.PathItem object
func createMockPathItem() *v3.PathItem {
	pathItem := &lowv3.PathItem{
		KeyNode: &yaml.Node{Line: 15, Column: 25},
	}
	return v3.NewPathItem(pathItem)
}

func TestRequestContentTypeNotFound(t *testing.T) {
	// Create a mock operation with request body content types
	op := createMockOperationWithRequestBody()

	// Create a mock request with an invalid content type
	request, _ := http.NewRequest(http.MethodPost, "/test", nil)
	request.Header.Set(helpers.ContentTypeHeader, "application/xml")

	// Call the function
	err := RequestContentTypeNotFound(op, request, "/test")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.RequestBodyValidation, err.ValidationType)
	require.Equal(t, helpers.RequestBodyContentType, err.ValidationSubType)
	require.Contains(t, err.Message, "'application/xml' does not exist")
	require.Contains(t, err.Reason, "The content type 'application/xml' of the POST request submitted has not been defined")
	require.Equal(t, 10, err.SpecLine)
	require.Equal(t, 20, err.SpecCol)
	require.Contains(t, err.HowToFix, "application/json")
}

func TestOperationNotFound(t *testing.T) {
	// Create a mock path item
	pathItem := createMockPathItem()

	// Create a mock request
	request, _ := http.NewRequest(http.MethodPatch, "/test", nil)

	// Call the function
	err := OperationNotFound(pathItem, request, http.MethodPatch, "/test")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.RequestValidation, err.ValidationType)
	require.Equal(t, helpers.RequestMissingOperation, err.ValidationSubType)
	require.Contains(t, err.Message, "'PATCH' does not exist")
	require.Contains(t, err.Reason, "there was no 'PATCH' method found in the spec")
	require.Equal(t, 15, err.SpecLine)
	require.Equal(t, 25, err.SpecCol)
	require.Equal(t, HowToFixPathMethod, err.HowToFix)
}
