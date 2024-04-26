package requests

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func TestValidateRequestSchema(t *testing.T) {
	for name, tc := range map[string]struct {
		request                    *http.Request
		schema                     *base.Schema
		renderedSchema, jsonSchema []byte
		assertValidRequestSchema   assert.BoolAssertionFunc
		expectedErrorsCount        int
	}{
		"FailRequestBodyValidation": {
			// KeywordLocation: /allOf/1/$ref/properties/properties/additionalProperties/$dynamicRef/allOf/3/$ref/properties/exclusiveMinimum/type
			// Message: expected number, but got boolean
			request: &http.Request{
				Method: http.MethodPost,
				Body:   io.NopCloser(strings.NewReader(`{"exclusiveNumber": 13}`)),
			},
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
			jsonSchema:               []byte(`{"properties":{"exclusiveNumber":{"description":"This number starts its journey where most numbers are too scared to begin!","exclusiveMinimum":true,"minimum":10,"type":"number"}},"type":"object"}`),
			assertValidRequestSchema: assert.False,
			expectedErrorsCount:      1,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			valid, errors := ValidateRequestSchema(tc.request, tc.schema, tc.renderedSchema, tc.jsonSchema)

			tc.assertValidRequestSchema(t, valid)
			assert.Len(t, errors, tc.expectedErrorsCount)
		})
	}
}
