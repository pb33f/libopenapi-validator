// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package responses

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/parameters"
	"github.com/pb33f/libopenapi-validator/strict"
)

// ValidateResponseHeaders validates the response headers against the OpenAPI spec.
func ValidateResponseHeaders(
	request *http.Request,
	response *http.Response,
	headers *orderedmap.Map[string, *v3.Header],
	pathTemplate string,
	statusCode string,
	opts ...config.Option,
) (bool, []*errors.ValidationError) {
	options := config.NewValidationOptions(opts...)

	// locate headers
	type headerPair struct {
		name  string
		value []string
		model *v3.Header
	}
	locatedHeaders := make(map[string]headerPair)
	var validationErrors []*errors.ValidationError
	// iterate through the response headers
	for name, v := range response.Header {
		// check if the model is in the spec
		for pair := headers.First(); pair != nil; pair = pair.Next() {
			k := pair.Key()
			header := pair.Value()
			if strings.EqualFold(k, name) && !ignoredResponseHeader(k) {
				locatedHeaders[strings.ToLower(name)] = headerPair{
					name:  k,
					value: v,
					model: header,
				}
			}
		}
	}

	// determine if any required headers are missing from the response
	for pair := headers.First(); pair != nil; pair = pair.Next() {
		name := pair.Key()
		header := pair.Value()
		if header.Required && !ignoredResponseHeader(name) {
			if _, ok := locatedHeaders[strings.ToLower(name)]; !ok {
				keywordLocation := helpers.ConstructResponseHeaderJSONPointer(pathTemplate, request.Method, statusCode, name, "required")

				specLine, specCol := 1, 0
				if low := header.GoLow(); low != nil && low.KeyNode != nil {
					specLine = low.KeyNode.Line
					specCol = low.KeyNode.Column
				}

				validationErrors = append(validationErrors, &errors.ValidationError{
					ValidationType:    helpers.ResponseBodyValidation,
					ValidationSubType: helpers.ParameterValidationHeader,
					Message:           "Missing required header",
					Reason:            fmt.Sprintf("Required header '%s' was not found in response", name),
					SpecLine:          specLine,
					SpecCol:           specCol,
					HowToFix:          errors.HowToFixMissingHeader,
					RequestPath:       request.URL.Path,
					RequestMethod:     request.Method,
					SchemaValidationErrors: []*errors.SchemaValidationFailure{{
						Reason:          fmt.Sprintf("Required header '%s' is missing", name),
						FieldName:       name,
						InstancePath:    []string{name},
						KeywordLocation: keywordLocation,
					}},
				})
			}
		}
	}

	// validate every header that is present against its schema, whether it is required or not.
	for h, header := range locatedHeaders {
		if header.model.Schema != nil {
			if schema := header.model.Schema.Schema(); schema != nil {
				for _, headerValue := range header.value {
					validationErrors = append(validationErrors,
						validateHeaderValue(headerValue, schema, header.model.Explode, h, options)...)
				}
			}
		}
	}

	if len(validationErrors) > 0 {
		return false, validationErrors
	}

	// strict mode: check for undeclared response headers
	if options.StrictMode {
		// convert orderedmap to regular map for strict validation
		declaredMap := make(map[string]*v3.Header)
		for name, header := range headers.FromOldest() {
			declaredMap[name] = header
		}

		undeclaredHeaders := strict.ValidateResponseHeaders(response.Header, &declaredMap, options)
		for _, undeclared := range undeclaredHeaders {
			validationErrors = append(validationErrors,
				errors.UndeclaredHeaderError(
					undeclared.Name,
					undeclared.Value.(string),
					undeclared.DeclaredProperties,
					undeclared.Direction.String(),
					request.URL.Path,
					request.Method,
				))
		}
	}

	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}

// ignoredResponseHeader reports whether a declared response header is ignored: OpenAPI says a
// header named Content-Type SHALL be ignored, because the response's content map describes it.
func ignoredResponseHeader(name string) bool {
	return strings.EqualFold(name, helpers.ContentTypeHeader)
}

// validateHeaderValue validates a header value against its schema. Header values are text that can
// be read more than one way ("5" is the number 5 and the string "5"), so every reading the schema
// allows is tried, and the value is valid when any of them is. Otherwise the errors of the first
// reading are returned.
func validateHeaderValue(value string, schema *base.Schema, explode bool, name string, options *config.ValidationOptions) []*errors.ValidationError {
	var firstErrors []*errors.ValidationError
	for i, reading := range headerValueReadings(value, schema, explode) {
		readingErrors := parameters.ValidateSingleParameterSchema(schema, reading, "header", "response header",
			name, helpers.ResponseBodyValidation, lowv3.HeadersLabel, options, "", "")
		if len(readingErrors) == 0 {
			return nil
		}
		if i == 0 {
			firstErrors = readingErrors
		}
	}
	return firstErrors
}

// headerValueReadings returns the ways a header value can be read: as each type the schema declares,
// in order (an array or object is split as the simple style serializes it), then as JSON, then as the
// string it was sent as.
func headerValueReadings(value string, schema *base.Schema, explode bool) []any {
	var readings []any
	readAsString := false
	for _, schemaType := range schema.Type {
		switch schemaType {
		case helpers.String:
			readings = append(readings, value)
			readAsString = true
		case helpers.Integer:
			if parsed, err := helpers.ParseInteger(value); err == nil {
				readings = append(readings, parsed)
			}
		case helpers.Number:
			if parsed, err := helpers.ParseNumber(value); err == nil {
				readings = append(readings, parsed)
			}
		case helpers.Boolean:
			if parsed, err := strconv.ParseBool(value); err == nil {
				readings = append(readings, parsed)
			}
		case helpers.Array:
			readings = append(readings, headerArrayItems(value, schema))
		case helpers.Object:
			if explode {
				readings = append(readings, helpers.ConstructKVFromCSVWithSchema(value, schema))
			} else {
				readings = append(readings, helpers.ConstructMapFromCSVWithSchema(value, schema))
			}
		}
	}
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err == nil {
		readings = append(readings, decoded)
	}
	if !readAsString {
		readings = append(readings, value)
	}
	return readings
}

// headerArrayItems splits a simple style array header into its items, each read as the first
// reading its items schema allows.
func headerArrayItems(value string, schema *base.Schema) []any {
	var itemSchema *base.Schema
	if schema.Items != nil && schema.Items.IsA() && schema.Items.A != nil {
		itemSchema = schema.Items.A.Schema()
	}
	items := strings.Split(value, helpers.Comma)
	decoded := make([]any, len(items))
	for i, item := range items {
		item = strings.TrimSpace(item)
		decoded[i] = item
		if itemSchema != nil {
			decoded[i] = headerValueReadings(item, itemSchema, false)[0]
		}
	}
	return decoded
}
