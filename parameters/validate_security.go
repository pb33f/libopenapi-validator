// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
)

func (v *paramValidator) ValidateSecurity(request *http.Request) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.document, v.options.RegexCache)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateSecurityWithPathItem(request, pathItem, foundPath)
}

func (v *paramValidator) ValidateSecurityWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
	if pathItem == nil {
		return false, []*errors.ValidationError{{
			ValidationType:    helpers.ParameterValidationPath,
			ValidationSubType: "missing",
			Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
			Reason: fmt.Sprintf("The %s request contains a path of '%s' "+
				"however that path, or the %s method for that path does not exist in the specification",
				request.Method, request.URL.Path, request.Method),
			SpecLine: -1,
			SpecCol:  -1,
			HowToFix: errors.HowToFixPath,
		}}
	}
	if !v.options.SecurityValidation {
		return true, nil
	}
	// extract security for the operation
	security := helpers.ExtractSecurityForOperation(request, pathItem)

	if security == nil {
		return true, nil
	}

	allErrors := []*errors.ValidationError{}

	for _, sec := range security {
		if sec.ContainsEmptyRequirement {
			return true, nil
		}

		for pair := orderedmap.First(sec.Requirements); pair != nil; pair = pair.Next() {
			secName := pair.Key()

			// look up security from components
			if v.document.Components == nil || v.document.Components.SecuritySchemes.GetOrZero(secName) == nil {
				validationErrors := []*errors.ValidationError{
					{
						Message: fmt.Sprintf("Security scheme '%s' is missing", secName),
						Reason: fmt.Sprintf("The security scheme '%s' is defined as being required, "+
							"however it's missing from the components", secName),
						ValidationType: "security",
						SpecLine:       sec.GoLow().Requirements.ValueNode.Line,
						SpecCol:        sec.GoLow().Requirements.ValueNode.Column,
						HowToFix:       "Add the missing security scheme to the components",
					},
				}
				errors.PopulateValidationErrors(validationErrors, request, pathValue)

				return false, validationErrors
			}
			secScheme := v.document.Components.SecuritySchemes.GetOrZero(secName)
			switch strings.ToLower(secScheme.Type) {
			case "http":
				switch strings.ToLower(secScheme.Scheme) {
				case "basic", "bearer", "digest":
					// check for an authorization header
					if request.Header.Get("Authorization") == "" {
						validationErrors := []*errors.ValidationError{
							{
								Message:           fmt.Sprintf("Authorization header for '%s' scheme", secScheme.Scheme),
								Reason:            "Authorization header was not found",
								ValidationType:    "security",
								ValidationSubType: secScheme.Scheme,
								SpecLine:          sec.GoLow().Requirements.ValueNode.Line,
								SpecCol:           sec.GoLow().Requirements.ValueNode.Column,
								HowToFix:          "Add an 'Authorization' header to this request",
							},
						}

						errors.PopulateValidationErrors(validationErrors, request, pathValue)
						allErrors = append(allErrors, validationErrors...)
					} else {
						return true, nil
					}
				}

			case "apikey":
				// check if the api key is in the request
				if secScheme.In == "header" {
					if request.Header.Get(secScheme.Name) == "" {
						validationErrors := []*errors.ValidationError{
							{
								Message:           fmt.Sprintf("API Key %s not found in header", secScheme.Name),
								Reason:            "API Key not found in http header for security scheme 'apiKey' with type 'header'",
								ValidationType:    "security",
								ValidationSubType: "apiKey",
								SpecLine:          sec.GoLow().Requirements.ValueNode.Line,
								SpecCol:           sec.GoLow().Requirements.ValueNode.Column,
								HowToFix:          fmt.Sprintf("Add the API Key via '%s' as a header of the request", secScheme.Name),
							},
						}

						errors.PopulateValidationErrors(validationErrors, request, pathValue)
						allErrors = append(allErrors, validationErrors...)
					} else {
						return true, nil
					}
				}
				if secScheme.In == "query" {
					if request.URL.Query().Get(secScheme.Name) == "" {
						copyUrl := *request.URL
						fixed := &copyUrl
						q := fixed.Query()
						q.Add(secScheme.Name, "your-api-key")
						fixed.RawQuery = q.Encode()

						validationErrors := []*errors.ValidationError{
							{
								Message:           fmt.Sprintf("API Key %s not found in query", secScheme.Name),
								Reason:            "API Key not found in URL query for security scheme 'apiKey' with type 'query'",
								ValidationType:    "security",
								ValidationSubType: "apiKey",
								SpecLine:          sec.GoLow().Requirements.ValueNode.Line,
								SpecCol:           sec.GoLow().Requirements.ValueNode.Column,
								HowToFix: fmt.Sprintf("Add an API Key via '%s' to the query string "+
									"of the URL, for example '%s'", secScheme.Name, fixed.String()),
							},
						}

						errors.PopulateValidationErrors(validationErrors, request, pathValue)
						allErrors = append(allErrors, validationErrors...)
					} else {
						return true, nil
					}
				}
				if secScheme.In == "cookie" {
					cookies := request.Cookies()
					cookieFound := false
					for _, cookie := range cookies {
						if cookie.Name == secScheme.Name {
							cookieFound = true
							break
						}
					}
					if !cookieFound {
						validationErrors := []*errors.ValidationError{
							{
								Message:           fmt.Sprintf("API Key %s not found in cookies", secScheme.Name),
								Reason:            "API Key not found in http request cookies for security scheme 'apiKey' with type 'cookie'",
								ValidationType:    "security",
								ValidationSubType: "apiKey",
								SpecLine:          sec.GoLow().Requirements.ValueNode.Line,
								SpecCol:           sec.GoLow().Requirements.ValueNode.Column,
								HowToFix:          fmt.Sprintf("Submit an API Key '%s' as a cookie with the request", secScheme.Name),
							},
						}

						errors.PopulateValidationErrors(validationErrors, request, pathValue)
						allErrors = append(allErrors, validationErrors...)
					} else {
						return true, nil
					}
				}
			}
		}
	}

	return false, allErrors
}
