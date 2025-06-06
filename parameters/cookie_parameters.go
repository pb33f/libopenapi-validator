// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
)

func (v *paramValidator) ValidateCookieParams(request *http.Request) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.document)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateCookieParamsWithPathItem(request, pathItem, foundPath)
}

func (v *paramValidator) ValidateCookieParamsWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
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
	// extract params for the operation
	params := helpers.ExtractParamsForOperation(request, pathItem)
	var validationErrors []*errors.ValidationError
	for _, p := range params {
		if p.In == helpers.Cookie {
			for _, cookie := range request.Cookies() {
				if cookie.Name == p.Name { // cookies are case-sensitive, an exact match is required

					var sch *base.Schema
					if p.Schema != nil {
						sch = p.Schema.Schema()
					}
					pType := sch.Type

					for _, ty := range pType {
						switch ty {
						case helpers.Integer:
							if _, err := strconv.ParseInt(cookie.Value, 10, 64); err != nil {
								validationErrors = append(validationErrors,
									errors.InvalidCookieParamInteger(p, strings.ToLower(cookie.Value), sch))
								break
							}
							// check if enum is in range
							if sch.Enum != nil {
								matchFound := false
								for _, enumVal := range sch.Enum {
									if strings.TrimSpace(cookie.Value) == fmt.Sprint(enumVal.Value) {
										matchFound = true
										break
									}
								}
								if !matchFound {
									validationErrors = append(validationErrors,
										errors.IncorrectCookieParamEnum(p, strings.ToLower(cookie.Value), sch))
								}
							}
						case helpers.Number:
							if _, err := strconv.ParseFloat(cookie.Value, 64); err != nil {
								validationErrors = append(validationErrors,
									errors.InvalidCookieParamNumber(p, strings.ToLower(cookie.Value), sch))
								break
							}
							// check if enum is in range
							if sch.Enum != nil {
								matchFound := false
								for _, enumVal := range sch.Enum {
									if strings.TrimSpace(cookie.Value) == fmt.Sprint(enumVal.Value) {
										matchFound = true
										break
									}
								}
								if !matchFound {
									validationErrors = append(validationErrors,
										errors.IncorrectCookieParamEnum(p, strings.ToLower(cookie.Value), sch))
								}
							}
						case helpers.Boolean:
							if _, err := strconv.ParseBool(cookie.Value); err != nil {
								validationErrors = append(validationErrors,
									errors.IncorrectCookieParamBool(p, strings.ToLower(cookie.Value), sch))
							}
						case helpers.Object:
							if !p.IsExploded() {
								encodedObj := helpers.ConstructMapFromCSV(cookie.Value)

								// if a schema was extracted
								if sch != nil {
									validationErrors = append(validationErrors,
										ValidateParameterSchema(sch, encodedObj, "",
											"Cookie parameter",
											"The cookie parameter",
											p.Name,
											helpers.ParameterValidation,
											helpers.ParameterValidationQuery,
											v.options)...)
								}
							}
						case helpers.Array:

							if !p.IsExploded() {
								// well we're already in an array, so we need to check the items schema
								// to ensure this array items matches the type
								// only check if items is a schema, not a boolean
								if sch.Items.IsA() {
									validationErrors = append(validationErrors,
										ValidateCookieArray(sch, p, cookie.Value)...)
								}
							}

						case helpers.String:

							// check if the schema has an enum, and if so, match the value against one of
							// the defined enum values.
							if sch.Enum != nil {
								matchFound := false
								for _, enumVal := range sch.Enum {
									if strings.TrimSpace(cookie.Value) == fmt.Sprint(enumVal.Value) {
										matchFound = true
										break
									}
								}
								if !matchFound {
									validationErrors = append(validationErrors,
										errors.IncorrectCookieParamEnum(p, strings.ToLower(cookie.Value), sch))
								}
							}
						}
					}
				}
			}
		}
	}

	errors.PopulateValidationErrors(validationErrors, request, pathValue)

	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}
