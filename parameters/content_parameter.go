// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/internal/requeststate"
)

func (v *paramValidator) validateContentParameters(request *http.Request, pathItem *v3.PathItem, pathValue, location string) []*errors.ValidationError {
	if v.options == nil || (location == helpers.Query && v.options.ContentParameterDecoder == nil) ||
		(location != helpers.Query && !v.options.ValidateContentParameters) {
		return nil
	}
	params := helpers.ExtractParamsForOperation(request, pathItem)
	var validationErrors []*errors.ValidationError
	var pathParams, serverParams map[string]string
	routeResolved := false
	for _, parameter := range params {
		if parameter == nil || parameter.In != location || parameter.Schema != nil || parameter.Content == nil || orderedmap.Len(parameter.Content) == 0 {
			continue
		}
		if v.options.ContentParameterDecoder != nil && !routeResolved {
			pathParams, serverParams = v.routeParameters(request)
			routeResolved = true
		}
		raw := rawParameterValues(request, parameter, pathValue, pathParams)
		if len(raw) == 0 {
			if parameter.Required != nil && *parameter.Required {
				validationErrors = append(validationErrors, contentParameterError(parameter, location, "required parameter is missing"))
			}
			continue
		}
		entry := orderedmap.First(parameter.Content)
		mediaType := entry.Key()
		media := entry.Value()
		var schema *base.Schema
		if media != nil && media.Schema != nil {
			schema = media.Schema.Schema()
		}
		input := &config.ContentParameterInput{
			Parameter: parameter, RawValues: raw, MediaType: mediaType, DefaultSchema: schema,
			Request: request, PathParams: pathParams, ServerVariables: serverParams,
		}
		var value any
		var decodeErr error
		if v.options.ContentParameterDecoder != nil {
			value, schema, decodeErr = v.options.ContentParameterDecoder(request.Context(), input)
		} else if strings.EqualFold(mediaType, helpers.JSONContentType) || strings.HasSuffix(strings.ToLower(mediaType), "+json") {
			decodeErr = json.Unmarshal([]byte(raw[0]), &value)
		} else {
			decodeErr = fmt.Errorf("no content-parameter decoder is registered for %s", mediaType)
		}
		if decodeErr != nil {
			validationErrors = append(validationErrors, contentParameterError(parameter, location, decodeErr.Error()))
			continue
		}
		if schema == nil || schema.GoLow() == nil {
			validationErrors = append(validationErrors, contentParameterError(parameter, location, "decoded parameter schema has no low-level render identity"))
			continue
		}
		validationErrors = append(validationErrors, ValidateSingleParameterSchema(
			schema, value, parameterLocationLabel(location)+" parameter", "The "+location+" parameter", parameter.Name,
			helpers.ParameterValidation, parameterSubtype(location), v.options, pathValue, strings.ToLower(request.Method),
		)...)
	}
	errors.PopulateValidationErrors(validationErrors, request, pathValue)
	return validationErrors
}

func parameterLocationLabel(location string) string {
	if location == "" {
		return ""
	}
	return strings.ToUpper(location[:1]) + location[1:]
}

func (v *paramValidator) routeParameters(request *http.Request) (map[string]string, map[string]string) {
	if route := requeststate.Route(request); route != nil {
		return route.PathParams, route.ServerParams
	}
	if v.options != nil && v.options.Router != nil {
		if route, err := v.options.Router.FindRoute(request); err == nil && route != nil {
			return route.PathParams, route.ServerParams
		}
	}
	return nil, nil
}

func rawParameterValues(request *http.Request, parameter *v3.Parameter, pathValue string, routeParams map[string]string) []string {
	switch parameter.In {
	case helpers.Query:
		return request.URL.Query()[parameter.Name]
	case helpers.Header:
		return request.Header.Values(parameter.Name)
	case helpers.Cookie:
		if cookie, err := request.Cookie(parameter.Name); err == nil {
			return []string{cookie.Value}
		}
	case helpers.Path:
		if value, ok := routeParams[parameter.Name]; ok {
			return []string{value}
		}
		return pathParameterValues(request.URL.EscapedPath(), pathValue, parameter.Name)
	}
	return nil
}

func pathParameterValues(requestPath, template, target string) []string {
	expression, err := helpers.GetRegexForPath(template)
	if err != nil {
		return nil
	}
	matches := expression.FindStringSubmatch(requestPath)
	indices, err := helpers.BraceIndices(template)
	if err != nil || len(matches) != len(indices)/2+1 {
		return nil
	}
	for i := 0; i < len(indices); i += 2 {
		name := template[indices[i]+1 : indices[i+1]-1]
		name = strings.TrimSuffix(strings.TrimLeft(strings.SplitN(name, ":", 2)[0], ".;"), "*")
		if name == target {
			value, err := url.PathUnescape(matches[i/2+1])
			if err != nil {
				value = matches[i/2+1]
			}
			return []string{value}
		}
	}
	return nil
}

func contentParameterError(parameter *v3.Parameter, location, reason string) *errors.ValidationError {
	return &errors.ValidationError{
		ValidationType: helpers.ParameterValidation, ValidationSubType: parameterSubtype(location),
		ParameterName: parameter.Name, Message: fmt.Sprintf("%s parameter '%s' could not be decoded", location, parameter.Name),
		Reason: reason, HowToFix: errors.HowToFixDecodingError, Context: parameter,
	}
}

func parameterSubtype(location string) string {
	switch location {
	case helpers.Query:
		return helpers.ParameterValidationQuery
	case helpers.Header:
		return helpers.ParameterValidationHeader
	case helpers.Cookie:
		return helpers.ParameterValidationCookie
	case helpers.Path:
		return helpers.ParameterValidationPath
	}
	return helpers.ParameterValidation
}
