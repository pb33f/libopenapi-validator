// Copyright 2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
)

func TestHeaderParametersRepeatedValues(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schema   string
		encoding string
		values   []string
		optional bool
		valid    bool
	}{
		{name: "single integer", schema: `{type: integer}`, values: []string{"1"}, valid: true},
		{name: "integer invalid second", schema: `{type: integer}`, values: []string{"1", "bad"}},
		{name: "integer duplicates", schema: `{type: integer}`, values: []string{"1", "2"}},
		{name: "boolean invalid second", schema: `{type: boolean}`, values: []string{"true", "bad"}},
		{name: "string enum invalid second", schema: `{type: string, enum: [allowed]}`, values: []string{"allowed", "bad"}},
		{name: "string list preserves order", schema: `{type: string, pattern: '^first,second,third$'}`, values: []string{"first", "second", "third"}, valid: true},
		{name: "string list wrong order", schema: `{type: string, pattern: '^first,second$'}`, values: []string{"second", "first"}},
		{name: "integer list", schema: `{type: array, items: {type: integer}}`, values: []string{"1", "2"}, valid: true},
		{name: "mixed list lines", schema: `{type: array, items: {type: integer}}`, values: []string{"1,2", "3,4"}, valid: true},
		{name: "list invalid second", schema: `{type: array, items: {type: integer}}`, values: []string{"1", "bad"}},
		{name: "list invalid first", schema: `{type: array, items: {type: integer}}`, values: []string{"bad", "1"}},
		{name: "single invalid list", schema: `{type: array, items: {type: integer}}`, values: []string{"1,bad"}},
		{name: "explicit unexploded list", schema: `{type: array, items: {type: integer}}`, encoding: "style: simple\n          explode: false", values: []string{"1", "2"}, valid: true},
		{name: "exploded list", schema: `{type: array, items: {type: integer}}`, encoding: "style: simple\n          explode: true", values: []string{"1", "2"}, valid: true},
		{name: "exploded list invalid second", schema: `{type: array, items: {type: integer}}`, encoding: "style: simple\n          explode: true", values: []string{"1", "bad"}},
		{name: "exploded single invalid list", schema: `{type: array, items: {type: integer}}`, encoding: "style: simple\n          explode: true", values: []string{"1,bad"}},
		{name: "exploded list without items", schema: `{type: array}`, encoding: "style: simple\n          explode: true", values: []string{"1", "two"}, valid: true},
		{name: "object split across lines", schema: `{type: object, required: [a, b], properties: {a: {type: integer}, b: {type: integer}}}`, values: []string{"a,1", "b,2"}, valid: true},
		{name: "object invalid second", schema: `{type: object, properties: {a: {type: integer}, b: {type: integer}}}`, values: []string{"a,1", "b,bad"}},
		{name: "optional empty first invalid second", schema: `{type: integer}`, values: []string{"", "bad"}, optional: true},
		{name: "missing required", schema: `{type: integer}`},
		{name: "missing optional", schema: `{type: integer}`, optional: true, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := fmt.Sprintf(`openapi: 3.1.0
info: {title: Repeated headers, version: '1'}
paths:
  /items:
    get:
      parameters:
        - name: X-Values
          in: header
          required: %t
          schema: %s
          %s
      responses: {'204': {description: OK}}
`, !tc.optional, tc.schema, tc.encoding)
			document, err := libopenapi.NewDocument([]byte(spec))
			require.NoError(t, err)
			model, err := document.BuildV3Model()
			require.NoError(t, err)
			validator := NewParameterValidator(&model.Model)
			t.Cleanup(validator.Release)

			// Parse actual field lines, including differently cased names.
			raw := "GET /items HTTP/1.1\r\nHost: example.com\r\n"
			for i, value := range tc.values {
				name := "X-Values"
				if i%2 == 1 {
					name = "x-values"
				}
				raw += name + ": " + value + "\r\n"
			}
			request, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw + "\r\n")))
			require.NoError(t, err)
			before := request.Header.Clone()
			valid, validationErrors := validator.ValidateHeaderParams(request)
			assert.Equal(t, tc.valid, valid, validationErrors)
			if tc.valid {
				assert.Empty(t, validationErrors)
			} else {
				require.NotEmpty(t, validationErrors)
				assert.Equal(t, http.MethodGet, validationErrors[0].RequestMethod)
				assert.Equal(t, "/items", validationErrors[0].RequestPath)
			}
			assert.Equal(t, before, request.Header)
		})
	}
}

func TestContentHeaderParametersRepeatedValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		valid  bool
	}{
		{name: "single list", values: []string{"[1,2]"}, valid: true},
		{name: "list split across lines", values: []string{"[1", "2]"}, valid: true},
		{name: "invalid second value", values: []string{"[1]", "bad"}},
		{name: "two JSON documents", values: []string{"[1]", "[2]"}},
		{name: "invalid second item", values: []string{"[1", `"bad"]`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := contentParameterModel(t, `openapi: 3.1.0
info: {title: Repeated content headers, version: '1'}
paths:
  /items:
    get:
      parameters:
        - name: X-Data
          in: header
          content: {application/json: {schema: {type: array, items: {type: integer}}}}
      responses: {'204': {description: OK}}
`)
			validator := NewParameterValidator(model, config.WithContentParameterValidation())
			t.Cleanup(validator.Release)
			request, err := http.NewRequest(http.MethodGet, "http://example.com/items", nil)
			require.NoError(t, err)
			for _, value := range tc.values {
				request.Header.Add("X-Data", value)
			}
			valid, validationErrors := validator.ValidateHeaderParams(request)
			assert.Equal(t, tc.valid, valid, validationErrors)
			if tc.valid {
				assert.Empty(t, validationErrors)
			} else {
				require.Len(t, validationErrors, 1)
				assert.Equal(t, "X-Data", validationErrors[0].ParameterName)
			}
			assert.Equal(t, tc.values, request.Header.Values("X-Data"))
		})
	}
}
