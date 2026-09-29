// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"net/http"
	"sort"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/internal/requeststate"
	"github.com/pb33f/libopenapi-validator/router"
)

// JSON Schema defines an integer as any number with a zero fractional part, so "1.0" is an integer.
func TestIntegerParameters_AcceptZeroFraction(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Integers
  version: 1.0.0
paths:
  /things/{id}/{.label}/{;matrix}/{ids}:
    get:
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer, enum: [1, 2]}}
        - {name: label, in: path, required: true, style: label, schema: {type: integer}}
        - {name: matrix, in: path, required: true, style: matrix, schema: {type: integer}}
        - {name: ids, in: path, required: true, schema: {type: array, items: {type: integer}}}
        - {name: limit, in: query, schema: {type: integer, maximum: 10}}
        - {name: state, in: query, schema: {type: integer, enum: [1, 2]}}
        - {name: pages, in: query, explode: false, schema: {type: array, items: {type: integer, enum: [1, 2]}}}
        - {name: X-Count, in: header, schema: {type: integer, enum: [1, 2]}}
        - {name: session, in: cookie, schema: {type: integer, enum: [1, 2]}}
      responses:
        '200':
          description: ok`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, errs := doc.BuildV3Model()
	require.NoError(t, errs)
	v := NewParameterValidator(&m.Model)

	// failing returns the sorted names of the parameters that fail validation
	failing := func(url, header, cookie string) []string {
		request, _ := http.NewRequest(http.MethodGet, url, nil)
		request.Header.Set("X-Count", header)
		request.AddCookie(&http.Cookie{Name: "session", Value: cookie})

		var names []string
		for _, validate := range []func(*http.Request) (bool, []*errors.ValidationError){
			v.ValidatePathParams, v.ValidateQueryParams, v.ValidateHeaderParams, v.ValidateCookieParams,
		} {
			_, validationErrors := validate(request)
			for _, validationError := range validationErrors {
				names = append(names, validationError.ParameterName)
			}
		}
		sort.Strings(names)
		return names
	}

	assert.Empty(t, failing(
		"https://things.com/things/1.0/.2.0/;matrix=3.0/1.0,2e1?limit=5.0&state=2.0&pages=1.0,2", "1.0", "2.0"))

	assert.Equal(t, []string{"X-Count", "id", "ids", "label", "limit", "matrix", "pages", "session", "state"}, failing(
		"https://things.com/things/1.5/.2.5/;matrix=3.5/1.5?limit=5.5&state=2.5&pages=1.5", "1.5", "2.5"))

	// a whole number still has to satisfy the rest of its schema, including enums
	assert.Equal(t, []string{"X-Count", "id", "limit", "pages", "session", "state"}, failing(
		"https://things.com/things/3.0/.1/;matrix=1/1?limit=11.0&state=3.0&pages=3.0", "3.0", "3.0"))
}

// The OpenAPI integer formats are enforced once format assertions are enabled.
func TestIntegerParameters_OpenAPIFormats(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Formats
  version: 1.0.0
paths:
  /things:
    get:
      parameters:
        - {name: offset, in: query, schema: {type: integer, format: int32, minimum: 0}}
      responses:
        '200':
          description: ok`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, errs := doc.BuildV3Model()
	require.NoError(t, errs)

	asserting := NewParameterValidator(&m.Model, config.WithFormatAssertions())
	annotating := NewParameterValidator(&m.Model)

	for value, valid := range map[string]bool{"2147483647": true, "2147483648": false, "2147483647.0": true} {
		request, _ := http.NewRequest(http.MethodGet, "https://things.com/things?offset="+value, nil)
		ok, _ := asserting.ValidateQueryParams(request)
		assert.Equal(t, valid, ok, value)

		ok, _ = annotating.ValidateQueryParams(request)
		assert.True(t, ok, value)
	}
}

// Integer and number headers and cookies are validated against their whole schema, not just their type.
func TestHeaderAndCookieNumbers_ValidateSchema(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Numbers
  version: 1.0.0
paths:
  /things:
    get:
      parameters:
        - {name: X-Int, in: header, schema: {type: integer, maximum: 10}}
        - {name: X-Num, in: header, schema: {type: number, maximum: 10, enum: [2.5, 20]}}
        - {name: X-Id, in: header, schema: {type: integer, format: int32}}
        - {name: int, in: cookie, schema: {type: integer, maximum: 10}}
        - {name: num, in: cookie, schema: {type: number, maximum: 10, enum: [2.5, 20]}}
      responses:
        '200':
          description: ok`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, errs := doc.BuildV3Model()
	require.NoError(t, errs)
	v := NewParameterValidator(&m.Model, config.WithFormatAssertions())

	for _, test := range []struct {
		header, cookie, value string
		valid                 bool
	}{
		{header: "X-Int", value: "10", valid: true},
		{header: "X-Int", value: "11", valid: false},
		{header: "X-Num", value: "2.5", valid: true},
		{header: "X-Num", value: "3", valid: false},  // not in the enum
		{header: "X-Num", value: "20", valid: false}, // in the enum, over the maximum
		{header: "X-Id", value: "2147483647", valid: true},
		{header: "X-Id", value: "2147483648", valid: false},
		{cookie: "int", value: "10", valid: true},
		{cookie: "int", value: "11", valid: false},
		{cookie: "num", value: "2.5", valid: true},
		{cookie: "num", value: "3", valid: false},
		{cookie: "num", value: "20", valid: false},
	} {
		request, _ := http.NewRequest(http.MethodGet, "https://things.com/things", nil)
		var valid bool
		var validationErrors []*errors.ValidationError
		if test.header != "" {
			request.Header.Set(test.header, test.value)
			valid, validationErrors = v.ValidateHeaderParams(request)
		} else {
			request.AddCookie(&http.Cookie{Name: test.cookie, Value: test.value})
			valid, validationErrors = v.ValidateCookieParams(request)
		}
		assert.Equal(t, test.valid, valid, "%s%s=%s", test.header, test.cookie, test.value)
		if !test.valid {
			assert.Len(t, validationErrors, 1, "%s%s=%s", test.header, test.cookie, test.value)
		}
	}
}

// Path parameters are read from the path the router matched, after a server variable in the base path.
func TestPathParameters_UseRouterMatch(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Router
  version: 1.0.0
servers:
  - url: https://api.example.com/{version}/api
    variables:
      version: {default: v1}
paths:
  /widgets/{id}:
    get:
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer}}
      responses:
        '200':
          description: ok
  /{kind}/{id}:
    get:
      parameters:
        - {name: kind, in: path, required: true, schema: {type: string, enum: [widgets]}}
        - {name: id, in: path, required: true, schema: {type: integer}}
      responses:
        '200':
          description: ok`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, errs := doc.BuildV3Model()
	require.NoError(t, errs)

	options := config.NewValidationOptions()
	options.Router = router.NewRouter(&m.Model) // strict server matching
	v := NewParameterValidator(&m.Model, config.WithExistingOpts(options))

	request, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v2/api/widgets/5", nil)
	valid, validationErrors := v.ValidatePathParams(request)
	assert.True(t, valid, validationErrors)
	assert.Nil(t, requeststate.Route(request), "the caller's request is not changed")

	request, _ = http.NewRequest(http.MethodGet, "https://api.example.com/v2/api/widgets/five", nil)
	valid, validationErrors = v.ValidatePathParams(request)
	assert.False(t, valid)
	require.Len(t, validationErrors, 1)
	assert.Equal(t, "Path parameter 'id' is not a valid integer", validationErrors[0].Message)

	// a path item the router did not choose is validated against the path as sent
	request, _ = http.NewRequest(http.MethodGet, "https://api.example.com/widgets/5", nil)
	valid, validationErrors = v.ValidatePathParamsWithPathItem(request,
		m.Model.Paths.PathItems.GetOrZero("/{kind}/{id}"), "/{kind}/{id}")
	assert.True(t, valid, validationErrors)
}

// NaN and Inf are not JSON numbers, and schema validation cannot compare them, so they are rejected.
func TestNumberParameters_RejectNonFiniteValues(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Numbers
  version: 1.0.0
paths:
  /things/{simple}/{.label}/{;matrix}:
    get:
      parameters:
        - {name: simple, in: path, required: true, schema: {type: number, maximum: 10}}
        - {name: label, in: path, required: true, style: label, schema: {type: number, maximum: 10}}
        - {name: matrix, in: path, required: true, style: matrix, schema: {type: number, maximum: 10}}
        - {name: q, in: query, schema: {type: number, maximum: 10}}
        - {name: X-Num, in: header, schema: {type: number, maximum: 10}}
        - {name: num, in: cookie, schema: {type: number, maximum: 10}}
      responses:
        '200':
          description: ok`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, errs := doc.BuildV3Model()
	require.NoError(t, errs)
	v := NewParameterValidator(&m.Model)

	for _, value := range []string{"NaN", "Inf", "-Inf"} {
		request, _ := http.NewRequest(http.MethodGet,
			"https://things.com/things/"+value+"/."+value+"/;matrix="+value+"?q="+value, nil)
		request.Header.Set("X-Num", value)
		request.AddCookie(&http.Cookie{Name: "num", Value: value})

		for name, validate := range map[string]func(*http.Request) (bool, []*errors.ValidationError){
			"path": v.ValidatePathParams, "query": v.ValidateQueryParams, "header": v.ValidateHeaderParams, "cookie": v.ValidateCookieParams,
		} {
			var valid bool
			require.NotPanics(t, func() { valid, _ = validate(request) }, "%s=%s", name, value)
			assert.False(t, valid, "%s=%s", name, value)
		}
	}

	// an empty matrix value is reported with the value that was sent, not a panic
	request, _ := http.NewRequest(http.MethodGet, "https://things.com/things/1/.1/;matrix=", nil)
	var validationErrors []*errors.ValidationError
	require.NotPanics(t, func() { _, validationErrors = v.ValidatePathParams(request) })
	require.NotEmpty(t, validationErrors)
}

// A value is checked against a multi-type schema once, and integer enums compare as numbers.
func TestHeaderParameters_MultiTypeAndIntegerEnums(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Headers
  version: 1.0.0
paths:
  /things:
    get:
      parameters:
        - {name: X-Either, in: header, schema: {type: [integer, number], maximum: 10}}
        - {name: X-Enum, in: header, schema: {type: integer, enum: [1, 2]}}
        - {name: X-Padded, in: header, schema: {type: [integer, string], enum: ["007"]}}
      responses:
        '200':
          description: ok`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	m, errs := doc.BuildV3Model()
	require.NoError(t, errs)
	v := NewParameterValidator(&m.Model)

	validate := func(header, value string) (bool, []*errors.ValidationError) {
		request, _ := http.NewRequest(http.MethodGet, "https://things.com/things", nil)
		request.Header.Set(header, value)
		return v.ValidateHeaderParams(request)
	}

	valid, validationErrors := validate("X-Either", "11")
	assert.False(t, valid)
	assert.Len(t, validationErrors, 1)

	for _, value := range []string{"1", "01", "1.0", "+2"} {
		valid, validationErrors = validate("X-Enum", value)
		assert.True(t, valid, "%s: %v", value, validationErrors)
	}
	valid, validationErrors = validate("X-Enum", "3.0")
	assert.False(t, valid)
	require.Len(t, validationErrors, 1)
	assert.Contains(t, validationErrors[0].Reason, "'3.0'", "errors report the value that was sent")

	valid, validationErrors = validate("X-Padded", "007")
	assert.True(t, valid, validationErrors)
}
