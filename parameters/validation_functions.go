// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"fmt"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"strconv"
	"strings"
)

// ValidateCookieArray will validate a cookie parameter that is an array
func ValidateCookieArray(
	sch *base.Schema, param *v3.Parameter, value string) []*errors.ValidationError {

	var validationErrors []*errors.ValidationError
	itemsSchema := sch.Items.A.Schema()

	// header arrays can only be encoded as CSV
	items := helpers.ExplodeQueryValue(value, helpers.DefaultDelimited)

	// now check each item in the array
	for _, item := range items {
		// for each type defined in the item's schema, check the item
		for _, itemType := range itemsSchema.Type {
			switch itemType {
			case helpers.Integer, helpers.Number:
				if _, err := strconv.ParseFloat(item, 64); err != nil {
					validationErrors = append(validationErrors,
						errors.IncorrectCookieParamArrayNumber(param, item, sch, itemsSchema))
				}
			case helpers.Boolean:
				if _, err := strconv.ParseBool(item); err != nil {
					validationErrors = append(validationErrors,
						errors.IncorrectCookieParamArrayBoolean(param, item, sch, itemsSchema))
					break
				}
				// check for edge-cases "0" and "1" which can also be parsed into valid booleans
				if item == "0" || item == "1" {
					validationErrors = append(validationErrors,
						errors.IncorrectCookieParamArrayBoolean(param, item, sch, itemsSchema))
				}
			case helpers.String:
				// do nothing for now.
				continue
			}
		}
	}
	return validationErrors
}

// ValidateHeaderArray will validate a header parameter that is an array
func ValidateHeaderArray(
	sch *base.Schema, param *v3.Parameter, value string) []*errors.ValidationError {

	var validationErrors []*errors.ValidationError
	itemsSchema := sch.Items.A.Schema()

	// header arrays can only be encoded as CSV
	items := helpers.ExplodeQueryValue(value, helpers.DefaultDelimited)

	// now check each item in the array
	for _, item := range items {
		// for each type defined in the item's schema, check the item
		for _, itemType := range itemsSchema.Type {
			switch itemType {
			case helpers.Integer, helpers.Number:
				if _, err := strconv.ParseFloat(item, 64); err != nil {
					validationErrors = append(validationErrors,
						errors.IncorrectHeaderParamArrayNumber(param, item, sch, itemsSchema))
				}
			case helpers.Boolean:
				if _, err := strconv.ParseBool(item); err != nil {
					validationErrors = append(validationErrors,
						errors.IncorrectHeaderParamArrayBoolean(param, item, sch, itemsSchema))
					break
				}
				// check for edge-cases "0" and "1" which can also be parsed into valid booleans
				if item == "0" || item == "1" {
					validationErrors = append(validationErrors,
						errors.IncorrectHeaderParamArrayBoolean(param, item, sch, itemsSchema))
				}
			case helpers.String:
				// do nothing for now.
				continue
			}
		}
	}
	return validationErrors
}

// ValidateQueryArray will validate a query parameter that is an array
func ValidateQueryArray(
	sch *base.Schema, param *v3.Parameter, ef string, contentWrapped bool) []*errors.ValidationError {

	var validationErrors []*errors.ValidationError
	itemsSchema := sch.Items.A.Schema()

	// check for an exploded bit on the schema.
	// if it's exploded, then we need to check each item in the array
	// if it's not exploded, then we need to check the whole array as a string
	var items []string
	if param.IsExploded() {
		items = helpers.ExplodeQueryValue(ef, param.Style)
	} else {
		// check for a style of form (or no style) and if so, explode the value
		if param.Style == "" || param.Style == helpers.Form {
			if !contentWrapped {
				items = helpers.ExplodeQueryValue(ef, param.Style)
			} else {
				items = []string{ef}
			}
		} else {
			switch param.Style {
			case helpers.PipeDelimited, helpers.SpaceDelimited:
				items = helpers.ExplodeQueryValue(ef, param.Style)
			}
		}
	}

	// check if the param is within an enum
	checkEnum := func(enumCheck, item string) {
		// check if the array param is within an enum
		if sch.Items.IsA() {
			itemsSch := sch.Items.A.Schema()
			if itemsSch.Enum != nil {
				matchFound := false
				for _, enumVal := range itemsSch.Enum {
					if strings.TrimSpace(enumCheck) == fmt.Sprint(enumVal) {
						matchFound = true
						break
					}
				}
				if !matchFound {
					validationErrors = append(validationErrors,
						errors.IncorrectQueryParamEnumArray(param, item, sch))
				}
			}
		}
	}

	// now check each item in the array
	for _, item := range items {
		// for each type defined in the item's schema, check the item
		for _, itemType := range itemsSchema.Type {
			switch itemType {
			case helpers.Integer, helpers.Number:
				if _, err := strconv.ParseFloat(item, 64); err != nil {
					validationErrors = append(validationErrors,
						errors.IncorrectQueryParamArrayNumber(param, item, sch, itemsSchema))
					break
				}
				// will it blend?
				checkEnum(ef, item)

			case helpers.Boolean:
				if _, err := strconv.ParseBool(item); err != nil {
					validationErrors = append(validationErrors,
						errors.IncorrectQueryParamArrayBoolean(param, item, sch, itemsSchema))
				}
			case helpers.Object:
				validationErrors = append(validationErrors,
					ValidateParameterSchema(itemsSchema,
						nil,
						item,
						"Query array parameter",
						"The query parameter (which is an array)",
						param.Name,
						helpers.ParameterValidation,
						helpers.ParameterValidationQuery)...)

			case helpers.String:

				// will it float?
				checkEnum(ef, item)
			}
		}
	}
	return validationErrors
}

// ValidateQueryParamStyle will validate a query parameter by style
func ValidateQueryParamStyle(param *v3.Parameter, as []*helpers.QueryParam) []*errors.ValidationError {

	var validationErrors []*errors.ValidationError
stopValidation:
	for _, qp := range as {
		for i := range qp.Values {
			switch param.Style {
			case helpers.DeepObject:
				if len(qp.Values) > 1 {
					validationErrors = append(validationErrors, errors.InvalidDeepObject(param, qp))
					break stopValidation
				}

			case helpers.PipeDelimited:
				// check if explode is false, but we have used an array style
				if !param.IsExploded() {
					if len(qp.Values) > 1 {
						validationErrors = append(validationErrors, errors.IncorrectPipeDelimiting(param, qp))
						break stopValidation
					}
				}
			case helpers.SpaceDelimited:
				// check if explode is false, but we have used an array style
				if !param.IsExploded() {
					if len(qp.Values) > 1 {
						validationErrors = append(validationErrors, errors.IncorrectSpaceDelimiting(param, qp))
						break stopValidation
					}
				}
			default:
				// check for a delimited list.
				if helpers.DoesFormParamContainDelimiter(qp.Values[i], param.Style) {
					if param.Explode != nil && *param.Explode {
						validationErrors = append(validationErrors, errors.IncorrectFormEncoding(param, qp, i))
						break stopValidation
					}
				}
			}
		}
	}
	return validationErrors // defaults to true if no style is set.
}
