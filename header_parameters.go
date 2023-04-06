// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "fmt"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "net/http"
    "strconv"
    "strings"
)

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
                contentWrapped := false
                var contentType string
                // skipValues:

                // for _, ef := range param {

                var sch *base.Schema

                if p.Schema != nil {
                    sch = p.Schema.Schema()
                } else {
                    // ok, no schema, check for a content type
                    // currently we only support one content type if used.
                    if p.Content != nil {
                        for k, ct := range p.Content {
                            sch = ct.Schema.Schema()
                            contentWrapped = true
                            contentType = k
                            break
                        }
                    }
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

                        // check what style of encoding was used and then construct a map[string]interface{}
                        // and pass that in as encoded JSON.
                        var encodedObj map[string]interface{}
                        // we have found our header, check the explode type.
                        if p.IsDefaultHeaderEncoding() {
                            encodedObj = constructMapFromCSV(param)
                        } else {
                            fmt.Print(contentType)
                            panic("oh my stars")
                        }

                        errors = append(errors, v.validateSchema(sch, encodedObj, param,
                            "Header parameter",
                            "The header parameter",
                            p.Name,
                            ParameterValidation,
                            ParameterValidationQuery)...)

                    case Array:
                        // well we're already in an array, so we need to check the items schema
                        // to ensure this array items matches the type
                        // only check if items is a schema, not a boolean
                        if sch.Items.IsA() {
                            errors = append(errors, v.validateQueryArray(sch, p, param, contentWrapped)...)
                        }
                    }
                }

                // }
            } else {
                if p.Required {
                    errors = append(errors, v.headerParameterMissing(p))
                }
            }
        }
    }

    // check for any headers that are not defined in the spec
    for k, _ := range request.Header {
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

type headerParam struct {
    key    string
    values []string
}
