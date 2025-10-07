package responses

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func TestValidateResponseSchema(t *testing.T) {
	for name, tc := range map[string]struct {
		request                    *http.Request
		response                   *http.Response
		schema                     *base.Schema
		renderedSchema, jsonSchema []byte
		version                    float32
		assertValidResponseSchema  assert.BoolAssertionFunc
		expectedErrorsCount        int
	}{
		"FailOnBooleanExclusiveMinimum": {
			request:  postRequest(),
			response: responseWithBody(`{"exclusiveNumber": 13}`),
			schema: &base.Schema{
				Type: []string{"object"},
			},
			renderedSchema: []byte(`type: object
properties:
    exclusiveNumber:
        type: number
        description: This number starts its journey where most numbers are too scared to begin!
        exclusiveMinimum: true
        minimum: !!float 10`),
			jsonSchema:                []byte(`{"properties":{"exclusiveNumber":{"description":"This number starts its journey where most numbers are too scared to begin!","exclusiveMinimum":true,"minimum":10,"type":"number"}},"type":"object"}`),
			version:                   3.1,
			assertValidResponseSchema: assert.False,
			expectedErrorsCount:       1,
		},
		"PassWithCorrectExclusiveMinimum": {
			request:  postRequest(),
			response: responseWithBody(`{"exclusiveNumber": 15}`),
			schema: &base.Schema{
				Type: []string{"object"},
			},
			renderedSchema: []byte(`type: object
properties:
    exclusiveNumber:
        type: number
        description: This number is properly constrained by a numeric exclusive minimum.
        exclusiveMinimum: 12
        minimum: 12`),
			jsonSchema:                []byte(`{"properties":{"exclusiveNumber":{"type":"number","description":"This number is properly constrained by a numeric exclusive minimum.","exclusiveMinimum":12,"minimum":12}},"type":"object"}`),
			version:                   3.1,
			assertValidResponseSchema: assert.True,
			expectedErrorsCount:       0,
		},
		"PassWithValidStringType": {
			request:  postRequest(),
			response: responseWithBody(`{"greeting": "Hello, world!"}`),
			schema: &base.Schema{
				Type: []string{"object"},
			},
			renderedSchema: []byte(`type: object
properties:
	  greeting:
	      type: string
	      description: A simple greeting
	      example: "Hello, world!"`),
			jsonSchema:                []byte(`{"properties":{"greeting":{"type":"string","description":"A simple greeting","example":"Hello, world!"}},"type":"object"}`),
			version:                   3.1,
			assertValidResponseSchema: assert.True,
			expectedErrorsCount:       0,
		},
		"PassWithNullablePropertyInOpenAPI30": {
			request:  postRequest(),
			response: responseWithBody(`{"name": "John", "middleName": null}`),
			schema: &base.Schema{
				Type: []string{"object"},
			},
			renderedSchema: []byte(`type: object
properties:
	  name:
	      type: string
	      description: User's first name
	  middleName:
	      type: string
	      nullable: true
	      description: User's middle name (optional)`),
			jsonSchema:                []byte(`{"properties":{"name":{"type":"string","description":"User's first name"},"middleName":{"type":"string","nullable":true,"description":"User's middle name (optional)"}},"type":"object"}`),
			version:                   3.0,
			assertValidResponseSchema: assert.True,
			expectedErrorsCount:       0,
		},
		"PassWithNullablePropertyInOpenAPI31": {
			request:  postRequest(),
			response: responseWithBody(`{"name": "John", "middleName": null}`),
			schema: &base.Schema{
				Type: []string{"object"},
			},
			renderedSchema: []byte(`type: object
properties:
	 name:
	     type: string
	     description: User's first name
	 middleName:
	     type: string
	     nullable: true
	     description: User's middle name (optional)`),
			jsonSchema:                []byte(`{"properties":{"name":{"type":"string","description":"User's first name"},"middleName":{"type":"string","nullable":true,"description":"User's middle name (optional)"}},"type":"object"}`),
			version:                   3.1,
			assertValidResponseSchema: assert.False,
			expectedErrorsCount:       1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			valid, errors := ValidateResponseSchema(tc.request, tc.response, tc.schema, tc.renderedSchema, tc.jsonSchema, tc.version, nil)

			tc.assertValidResponseSchema(t, valid)
			assert.Len(t, errors, tc.expectedErrorsCount)
		})
	}
}

func postRequest() *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/test", io.NopCloser(strings.NewReader("")))
	return req
}

func responseWithBody(payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(payload))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestInvalidMin(t *testing.T) {
	renderedSchema := []byte(`type: object
properties:
    exclusiveNumber:
        type: number
        description: This number starts its journey where most numbers are too scared to begin!
        exclusiveMinimum: true
        minimum: !!float 10`)

	jsonSchema := []byte(`{"properties":{"exclusiveNumber":{"description":"This number starts its journey where most numbers are too scared to begin!","exclusiveMinimum":true,"minimum":10,"type":"number"}},"type":"object"}`)

	valid, errors := ValidateResponseSchema(
		postRequest(),
		responseWithBody(`{"exclusiveNumber": 13}`),
		&base.Schema{
			Type: []string{"object"},
		},
		renderedSchema,
		jsonSchema,
		3.1,
		nil,
	)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
}
