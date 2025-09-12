// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package errors

import (
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/low"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"

	"github.com/pb33f/libopenapi-validator/helpers"
)

// Helper to create a mock v3.Operation object
func createMockOperation() *v3.Operation {
	content := orderedmap.New[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]()
	content.Set(low.KeyReference[string]{
		Value: "application/json",
	}, low.ValueReference[*lowv3.MediaType]{
		Value: &lowv3.MediaType{},
	})

	r := &lowv3.Response{
		Content: low.NodeReference[*orderedmap.Map[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]]{
			Value:     content,
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}
	//	resp := v3.NewResponse(r)

	// create a lowv3.Responses object
	responses := &lowv3.Responses{
		Default: low.NodeReference[*lowv3.Response]{
			Value:     r,
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
		Codes:   orderedmap.New[low.KeyReference[string], low.ValueReference[*lowv3.Response]](),
		KeyNode: &yaml.Node{},
	}

	// create a lowv3.Operation object
	op := &lowv3.Operation{
		Responses: low.NodeReference[*lowv3.Responses]{
			Value:     responses,
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}

	// create a new v3.Operation object from the low
	highOp := v3.NewOperation(op)
	return highOp
}

func TestResponseContentTypeNotFound_Default(t *testing.T) {
	// Create a mock operation with a default response and content type
	op := createMockOperation()
	op.Responses.Default.Content.Set("application/json", &v3.MediaType{})
	op.Responses.Default.GoLow().Content.KeyNode.Line = 12
	op.Responses.Default.GoLow().Content.KeyNode.Column = 34

	// Create a mock request and response
	request, _ := http.NewRequest(http.MethodGet, "/test", nil)
	response := &http.Response{
		Header: http.Header{
			helpers.ContentTypeHeader: {"application/xml"},
		},
	}

	// Call the function
	err := ResponseContentTypeNotFound(op, request, response, "200", true)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ResponseBodyValidation, err.ValidationType)
	require.Equal(t, helpers.RequestBodyContentType, err.ValidationSubType)
	require.Contains(t, err.Message, "'application/xml' does not exist")
	require.Contains(t, err.Reason, "The content type 'application/xml' of the GET response received has not been defined")
	require.Equal(t, 12, err.SpecLine)
	require.Equal(t, 34, err.SpecCol)
	require.Contains(t, err.HowToFix, "application/json")
}

func TestResponseContentTypeNotFound_SpecificCode(t *testing.T) {
	// Create a mock operation with a response code and content type
	op := createMockOperation()
	responseContent := orderedmap.New[string, *v3.MediaType]()
	responseContent.Set("application/json", &v3.MediaType{})

	content := orderedmap.New[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]()
	content.Set(low.KeyReference[string]{
		Value: "application/json",
	}, low.ValueReference[*lowv3.MediaType]{
		Value: &lowv3.MediaType{},
	})
	r := &lowv3.Response{
		Content: low.NodeReference[*orderedmap.Map[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]]{
			Value:     content,
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}
	resp := v3.NewResponse(r)

	op.Responses.Codes.Set("200", resp)
	op.Responses.Codes.GetOrZero("200").GoLow().Content.KeyNode.Line = 15
	op.Responses.Codes.GetOrZero("200").GoLow().Content.KeyNode.Column = 42

	// Create a mock request and response
	request, _ := http.NewRequest(http.MethodPost, "/test", nil)
	response := &http.Response{
		Header: http.Header{
			helpers.ContentTypeHeader: {"application/xml"},
		},
	}

	// Call the function
	err := ResponseContentTypeNotFound(op, request, response, "200", false)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ResponseBodyValidation, err.ValidationType)
	require.Equal(t, helpers.RequestBodyContentType, err.ValidationSubType)
	require.Contains(t, err.Message, "'application/xml' does not exist")
	require.Contains(t, err.Reason, "The content type 'application/xml' of the POST response received has not been defined")
	require.Equal(t, 15, err.SpecLine)
	require.Equal(t, 42, err.SpecCol)
	require.Contains(t, err.HowToFix, "application/json")
}

func TestResponseCodeNotFound(t *testing.T) {
	// Create a mock operation with responses
	op := createMockOperation()
	op.GoLow().Responses.KeyNode.Line = 22
	op.GoLow().Responses.KeyNode.Column = 56

	// Create a mock request
	request, _ := http.NewRequest(http.MethodDelete, "/test", nil)

	// Call the function with a response code that doesn't exist
	err := ResponseCodeNotFound(op, request, 404)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ResponseBodyValidation, err.ValidationType)
	require.Equal(t, helpers.ResponseBodyResponseCode, err.ValidationSubType)
	require.Contains(t, err.Message, "response code '404' does not exist")
	require.Contains(t, err.Reason, "The response code '404' of the DELETE request submitted has not been defined")
	require.Equal(t, 22, err.SpecLine)
	require.Equal(t, 56, err.SpecCol)
	require.Equal(t, HowToFixInvalidResponseCode, err.HowToFix)
}
