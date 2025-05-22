// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package responses

import (
	"fmt"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/parameters"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	"net/http"
	"strings"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
)

// ValidateResponseHeaders validates the response headers against the OpenAPI spec.
func ValidateResponseHeaders(
	request *http.Request,
	response *http.Response,
	headers *orderedmap.Map[string, *v3.Header],
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
		for k, header := range headers.FromOldest() {
			if strings.EqualFold(k, name) {
				locatedHeaders[strings.ToLower(name)] = headerPair{
					name:  k,
					value: v,
					model: header,
				}
			}
		}
	}

	// determine if any required headers are missing from the response
	for name, header := range headers.FromOldest() {
		if header.Required {
			if _, ok := locatedHeaders[strings.ToLower(name)]; !ok {
				validationErrors = append(validationErrors, &errors.ValidationError{
					ValidationType:    helpers.ResponseBodyValidation,
					ValidationSubType: helpers.ParameterValidationHeader,
					Message:           "Missing required model",
					Reason:            fmt.Sprintf("Required model '%s' was not found in response", name),
					SpecLine:          header.GoLow().KeyNode.Line,
					SpecCol:           header.GoLow().KeyNode.Column,
					HowToFix:          errors.HowToFixMissingHeader,
					RequestPath:       request.URL.Path,
					RequestMethod:     request.Method,
				})
			}
		}
	}

	// validate the model schemas if they are set.
	for h, header := range locatedHeaders {
		if header.model.Schema != nil {
			schema := header.model.Schema.Schema()
			if schema != nil && header.model.Required {

				for _, headerValue := range header.value {
					validationErrors = append(validationErrors,
						parameters.ValidateParameterSchema(schema, nil, headerValue, "header",
							"response header", h, helpers.ResponseBodyValidation, lowv3.HeadersLabel, options)...)

				}
			}
		}
	}
	if len(validationErrors) == 0 {
		return true, nil
	}
	return false, validationErrors
}
