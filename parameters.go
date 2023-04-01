// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"net/http"
	"strconv"
	"strings"
)

func extractParamsForOperation(request *http.Request, item *v3.PathItem) []*v3.Parameter {

	params := item.Parameters
	switch request.Method {
	case http.MethodGet:
		if item.Get != nil {
			params = append(params, item.Get.Parameters...)
		}
	case http.MethodPost:
		if item.Post != nil {
			params = append(params, item.Post.Parameters...)
		}
	case http.MethodPut:
		if item.Put != nil {
			params = append(params, item.Put.Parameters...)
		}
	case http.MethodDelete:
		if item.Delete != nil {
			params = append(params, item.Delete.Parameters...)
		}
	case http.MethodOptions:
		if item.Options != nil {
			params = append(params, item.Options.Parameters...)
		}
	case http.MethodHead:
		if item.Head != nil {
			params = append(params, item.Head.Parameters...)
		}
	case http.MethodPatch:
		if item.Patch != nil {
			params = append(params, item.Patch.Parameters...)
		}
	case http.MethodTrace:
		if item.Trace != nil {
			params = append(params, item.Trace.Parameters...)
		}
	}
	return params
}

func (v *validator) ValidateQueryParams(request *http.Request) (bool, []*ValidationError) {

	queryParams := make(map[string][]string)
	var errors []*ValidationError

	for qKey, qVal := range request.URL.Query() {
		queryParams[qKey] = qVal
	}

	// find path
	pathItem, errs := v.FindPath(request)
	if pathItem == nil || errs != nil {
		return false, errs
	}

	var params = extractParamsForOperation(request, pathItem)

	// look through the params for the query key
	for p := range params {
		if params[p].In == "query" {
			// check if this param is found as a set of query strings
			if fp, ok := queryParams[params[p].Name]; ok {
				// there is a match, is the type correct
				sch := params[p].Schema.Schema()
				pType := sch.Type

				// for each param, check each type
				for _, ef := range fp {

					for _, ty := range pType {
						switch ty {
						case "integer", "number":
							if _, err := strconv.ParseFloat(ef, 64); err != nil {
								errors = append(v.errors, &ValidationError{
									ValidationType:    ParameterValidation,
									ValidationSubType: ParameterValidationQuery,
									Message:           fmt.Sprintf("Query parameter '%s' is not a valid number", params[p].Name),
									Reason: fmt.Sprintf("The query parameter '%s' is defined as being a number, "+
										"however the value '%s' is not a valid number", params[p].Name, ef),
									SpecLine: params[p].GoLow().Schema.KeyNode.Line,
									SpecCol:  params[p].GoLow().Schema.KeyNode.Column,
									Context:  sch,
								})
							}
						case "boolean":
							if _, err := strconv.ParseBool(ef); err != nil {
								errors = append(v.errors, &ValidationError{
									ValidationType:    ParameterValidation,
									ValidationSubType: ParameterValidationQuery,
									Message:           fmt.Sprintf("Query parameter '%s' is not a valid boolean", params[p].Name),
									Reason: fmt.Sprintf("The query parameter '%s' is defined as being a boolean, "+
										"however the value '%s' is not a valid boolean", params[p].Name, ef),
									SpecLine: params[p].GoLow().Schema.KeyNode.Line,
									SpecCol:  params[p].GoLow().Schema.KeyNode.Column,
									Context:  sch,
								})
							}
						case "object":

							errors = append(errors, v.validateSchema(sch, ef,
								"Query parameter",
								"The query parameter",
								params[p].Name,
								ParameterValidation,
								ParameterValidationQuery)...)

						case "array":
							// well we're already in an array, so we need to check the items schema
							// to ensure this array items matches the type
							// only check if items is a schema, not a boolean
							if sch.Items.IsA() {
								itemsSchema := sch.Items.A.Schema()

								// check for an exploded bit on the schema.
								// if it's exploded, then we need to check each item in the array
								// if it's not exploded, then we need to check the whole array as a string
								var items []string
								if params[p].Explode != nil && *params[p].Explode {
									//check if the item has a comma in it, if not, this is invalid use
									if !strings.Contains(ef, ",") && len(fp) >= 1 {
										errors = append(errors, &ValidationError{
											ValidationType:    ParameterValidation,
											ValidationSubType: ParameterValidationQuery,
											Message:           fmt.Sprintf("Query array parameter '%s' has not been exploded correctly", params[p].Name),
											Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being exploded, "+
												"however the value '%s' is not comma delimited. There are multiple "+
												"parameters with the same name in the request (%d)", params[p].Name, ef, len(fp)),
											SpecLine: params[p].GoLow().Explode.ValueNode.Line,
											SpecCol:  params[p].GoLow().Explode.ValueNode.Column,
											Context:  sch,
										})
										items = []string{ef}
									} else {
										items = strings.Split(ef, ",")
									}
								} else {
									items = []string{ef}
								}

								// now check each item in the array
								for _, item := range items {

									// for each type defined in the items schema, check the item
									for _, itemType := range itemsSchema.Type {
										switch itemType {
										case "integer", "number":
											if _, err := strconv.ParseFloat(item, 64); err != nil {
												errors = append(errors, &ValidationError{
													ValidationType:    ParameterValidation,
													ValidationSubType: ParameterValidationQuery,
													Message:           fmt.Sprintf("Query array parameter '%s' is not a valid number", params[p].Name),
													Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being a number, "+
														"however the value '%s' is not a valid number", params[p].Name, item),
													SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
													SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
													Context:  itemsSchema,
												})
											}
										case "boolean":
											if _, err := strconv.ParseBool(item); err != nil {
												errors = append(errors, &ValidationError{
													ValidationType:    ParameterValidation,
													ValidationSubType: ParameterValidationQuery,
													Message:           fmt.Sprintf("Query array parameter '%s' is not a valid boolean", params[p].Name),
													Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being a boolean, "+
														"however the value '%s' is not a valid true/false value", params[p].Name, item),
													SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
													SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
													Context:  itemsSchema,
												})
											}
										case "object":
											errors = append(errors, v.validateSchema(itemsSchema, item,
												"Query array parameter",
												"The query parameter (which is an array)",
												params[p].Name,
												ParameterValidation,
												ParameterValidationQuery)...)

										case "string":
											// do nothing for now.
											continue

										}
									}
								}
							}
						}
					}
				}

			} else {
				// if there is no match, check if the param is required or not.
				if params[p].Required {
					errors = append(errors, &ValidationError{
						Message: fmt.Sprintf("Query parameter '%s' is missing", params[p].Name),
						Reason: fmt.Sprintf("The query parameter '%s' is defined as being required, "+
							"however it's missing from the request", params[p].Name),
						SpecLine: params[p].GoLow().Required.KeyNode.Line,
						SpecCol:  params[p].GoLow().Required.KeyNode.Column,
					})
				}
			}
		}
	}
	v.errors = errors
	if len(errors) > 0 {
		return false, errors
	}
	return true, nil
}
