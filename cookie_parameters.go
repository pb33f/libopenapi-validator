// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "strconv"
    "strings"
)

// ValidateCookieParams validates the cookie parameters contained within *http.Request. It returns a boolean stating true
// if validation passed (false for failed), and a slice of errors if validation failed.
func (v *validator) ValidateCookieParams(request *http.Request) (bool, []*ValidationError) {

    // find path
    pathItem, errs, _ := v.FindPath(request)
    if pathItem == nil || errs != nil {
        return false, errs
    }

    // extract params for the operation
    var params = extractParamsForOperation(request, pathItem)
    var errors []*ValidationError
    for _, p := range params {
        if p.In == Cookie {
            for _, cookie := range request.Cookies() {
                if cookie.Name == p.Name { // cookies are case-sensitive, an exact match is required

                    var sch *base.Schema

                    if p.Schema != nil {
                        sch = p.Schema.Schema()
                    }

                    pType := sch.Type

                    for _, ty := range pType {
                        switch ty {
                        case Integer, Number:
                            if _, err := strconv.ParseFloat(cookie.Value, 64); err != nil {
                                errors = append(errors, v.invalidCookieParamNumber(p, strings.ToLower(cookie.Value), sch))
                            }
                        case Boolean:
                            if _, err := strconv.ParseBool(cookie.Value); err != nil {
                                errors = append(errors, v.incorrectCookieParamBool(p, strings.ToLower(cookie.Value), sch))
                            }
                        case Object:
                            if !p.IsExploded() {
                                var encodedObj interface{}
                                encodedObj = constructMapFromCSV(cookie.Value)

                                // if a schema was extracted
                                if sch != nil {
                                    errors = append(errors, v.validateSchema(sch, encodedObj, "",
                                        "Cookie parameter",
                                        "The cookie parameter",
                                        p.Name,
                                        ParameterValidation,
                                        ParameterValidationQuery)...)
                                }

                            }

                        case Array:

                            if !p.IsExploded() {
                                // well we're already in an array, so we need to check the items schema
                                // to ensure this array items matches the type
                                // only check if items is a schema, not a boolean
                                if sch.Items.IsA() {
                                    errors = append(errors, v.validateCookieArray(sch, p, cookie.Value)...)
                                }
                            }
                        }
                    }

                }
            }
        }
    }
    if len(errors) > 0 {
        return false, errors
    }
    return true, nil
}

func (v *validator) validateCookieArray(
    sch *base.Schema, param *v3.Parameter, value string) []*ValidationError {

    var errors []*ValidationError
    itemsSchema := sch.Items.A.Schema()

    // header arrays can only be encoded as CSV
    items := explodeQueryValue(value, DefaultDelimited)

    // now check each item in the array
    for _, item := range items {
        // for each type defined in the item's schema, check the item
        for _, itemType := range itemsSchema.Type {
            switch itemType {
            case Integer, Number:
                if _, err := strconv.ParseFloat(item, 64); err != nil {
                    errors = append(errors,
                        v.incorrectCookieParamArrayNumber(param, item, sch, itemsSchema))
                }
            case Boolean:
                if _, err := strconv.ParseBool(item); err != nil {
                    errors = append(errors,
                        v.incorrectCookieParamArrayBoolean(param, item, sch, itemsSchema))
                    break
                }
                // check for edge-cases "0" and "1" which can also be parsed into valid booleans
                if item == "0" || item == "1" {
                    errors = append(errors,
                        v.incorrectCookieParamArrayBoolean(param, item, sch, itemsSchema))
                }
            case String:
                // do nothing for now.
                continue
            }
        }
    }
    return errors
}
