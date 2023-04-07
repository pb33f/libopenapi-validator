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

// ValidateHeaderParams validates the header parameters contained within *http.Request. It returns a boolean stating true
// if validation passed (false for failed), and a slice of errors if validation failed.
func (v *validator) ValidateHeaderParams(request *http.Request) (bool, []*ValidationError) {

    // find path
    pathItem, errs := v.FindPath(request)
    if pathItem == nil || errs != nil {
        return false, errs
    }

    // extract params for the operation
    var params = extractParamsForOperation(request, pathItem)
    // headerParams := make(map[string][]*headerParam)
    var errors []*ValidationError
    var seenHeaders = make(map[string]bool)
    for _, p := range params {
        if p.In == Header {

            seenHeaders[strings.ToLower(p.Name)] = true
            if param := request.Header.Get(p.Name); param != "" {

                var sch *base.Schema

                if p.Schema != nil {
                    sch = p.Schema.Schema()
                }

                pType := sch.Type

                for _, ty := range pType {
                    switch ty {
                    case Integer, Number:
                        if _, err := strconv.ParseFloat(param, 64); err != nil {
                            errors = append(errors, v.invalidHeaderParamNumber(p, strings.ToLower(param), sch))
                        }
                    case Boolean:
                        if _, err := strconv.ParseBool(param); err != nil {
                            errors = append(errors, v.incorrectHeaderParamBool(p, strings.ToLower(param), sch))
                        }
                    case Object:

                        // check if the header is default encoded or not
                        var encodedObj interface{}
                        // we have found our header, check the explode type.
                        if p.IsDefaultHeaderEncoding() {
                            encodedObj = constructMapFromCSV(param)
                        } else {
                            if p.IsExploded() { // only option is to be exploded for KV extraction.
                                encodedObj = constructKVFromCSV(param)
                            }
                        }

                        // if a schema was extracted
                        if sch != nil {
                            errors = append(errors, v.validateSchema(sch, encodedObj, "",
                                "Header parameter",
                                "The header parameter",
                                p.Name,
                                ParameterValidation,
                                ParameterValidationQuery)...)
                        }

                    case Array:
                        // well we're already in an array, so we need to check the items schema
                        // to ensure this array items matches the type
                        // only check if items is a schema, not a boolean
                        if sch.Items.IsA() {
                            errors = append(errors, v.validateHeaderArray(sch, p, param)...)
                        }
                    }
                }
            } else {
                if p.Required {
                    errors = append(errors, v.headerParameterMissing(p))
                }
            }
        }
    }

    // check for any headers that are not defined in the spec
    for k := range request.Header {
        if _, ok := seenHeaders[strings.ToLower(k)]; !ok {
            ps := pathItem.GetOperations()[strings.ToLower(request.Method)].GoLow().Parameters
            if ps.KeyNode != nil {
                errors = append(errors, v.headerParameterNotDefined(k, ps.KeyNode))
            }
        }
    }

    if len(errors) > 0 {
        return false, errors
    }
    return true, nil

}

func (v *validator) validateHeaderArray(
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
                        v.incorrectQueryParamArrayNumber(param, item, sch, itemsSchema))
                }
            case Boolean:
                if _, err := strconv.ParseBool(item); err != nil {
                    errors = append(errors,
                        v.incorrectQueryParamArrayBoolean(param, item, sch, itemsSchema))
                    break
                }
                // check for edge-cases "0" and "1" which can also be parsed into valid booleans
                if item == "0" || item == "1" {
                    errors = append(errors,
                        v.incorrectQueryParamArrayBoolean(param, item, sch, itemsSchema))
                }
            case String:
                // do nothing for now.
                continue
            }
        }
    }
    return errors
}
