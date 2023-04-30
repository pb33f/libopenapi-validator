// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
    "fmt"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "strconv"
    "strings"
)

func (v *paramValidator) ValidateHeaderParams(request *http.Request) (bool, []*errors.ValidationError) {

    // find path
    var pathItem *v3.PathItem
    var errs []*errors.ValidationError
    if v.pathItem == nil {
        pathItem, errs, _ = paths.FindPath(request, v.document)
        if pathItem == nil || errs != nil {
            v.errors = errs
            return false, errs
        }
    } else {
        pathItem = v.pathItem
    }

    // extract params for the operation
    var params = helpers.ExtractParamsForOperation(request, pathItem)

    var validationErrors []*errors.ValidationError
    var seenHeaders = make(map[string]bool)
    for _, p := range params {
        if p.In == helpers.Header {

            seenHeaders[strings.ToLower(p.Name)] = true
            if param := request.Header.Get(p.Name); param != "" {

                var sch *base.Schema
                if p.Schema != nil {
                    sch = p.Schema.Schema()
                }
                pType := sch.Type

                for _, ty := range pType {
                    switch ty {
                    case helpers.Integer, helpers.Number:
                        if _, err := strconv.ParseFloat(param, 64); err != nil {
                            validationErrors = append(validationErrors,
                                errors.InvalidHeaderParamNumber(p, strings.ToLower(param), sch))
                            break
                        }
                        // check if the param is within the enum
                        if sch.Enum != nil {
                            matchFound := false
                            for _, enumVal := range sch.Enum {
                                if strings.TrimSpace(param) == fmt.Sprint(enumVal) {
                                    matchFound = true
                                    break
                                }
                            }
                            if !matchFound {
                                validationErrors = append(validationErrors,
                                    errors.IncorrectCookieParamEnum(p, strings.ToLower(param), sch))
                            }
                        }

                    case helpers.Boolean:
                        if _, err := strconv.ParseBool(param); err != nil {
                            validationErrors = append(validationErrors,
                                errors.IncorrectHeaderParamBool(p, strings.ToLower(param), sch))
                        }

                    case helpers.Object:

                        // check if the header is default encoded or not
                        var encodedObj map[string]interface{}
                        // we have found our header, check the explode type.
                        if p.IsDefaultHeaderEncoding() {
                            encodedObj = helpers.ConstructMapFromCSV(param)
                        } else {
                            if p.IsExploded() { // only option is to be exploded for KV extraction.
                                encodedObj = helpers.ConstructKVFromCSV(param)
                            }
                        }

                        if len(encodedObj) == 0 {
                            validationErrors = append(validationErrors,
                                errors.HeaderParameterCannotBeDecoded(p, strings.ToLower(param)))
                            break
                        }

                        // if a schema was extracted
                        if sch != nil {
                            validationErrors = append(validationErrors,
                                ValidateParameterSchema(sch,
                                    encodedObj,
                                    "",
                                    "Header parameter",
                                    "The header parameter",
                                    p.Name,
                                    helpers.ParameterValidation,
                                    helpers.ParameterValidationQuery)...)
                        }

                    case helpers.Array:
                        if !p.IsExploded() { // only unexploded arrays are supported for cookie params
                            if sch.Items.IsA() {
                                validationErrors = append(validationErrors,
                                    ValidateHeaderArray(sch, p, param)...)
                            }
                        }

                    case helpers.String:

                        // check if the schema has an enum, and if so, match the value against one of
                        // the defined enum values.
                        if sch.Enum != nil {
                            matchFound := false
                            for _, enumVal := range sch.Enum {
                                if strings.TrimSpace(param) == fmt.Sprint(enumVal) {
                                    matchFound = true
                                    break
                                }
                            }
                            if !matchFound {
                                validationErrors = append(validationErrors,
                                    errors.IncorrectHeaderParamEnum(p, strings.ToLower(param), sch))
                            }
                        }
                    }
                }
            } else {
                if p.Required {
                    validationErrors = append(validationErrors, errors.HeaderParameterMissing(p))
                }
            }
        }
    }

    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
