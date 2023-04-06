// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "encoding/json"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "regexp"
    "strconv"
    "strings"
)

// ValidateQueryParams accepts an *http.Request and validates the query parameters against the OpenAPI specification.
// The method will locate the correct path, and operation, based on the verb. The parameters for the operation
// will be matched and validated against what has been supplied in the http.Request query string.
func (v *validator) ValidateQueryParams(request *http.Request) (bool, []*ValidationError) {
    // find path
    pathItem, errs := v.FindPath(request)
    if pathItem == nil || errs != nil {
        return false, errs
    }

    // extract params for the operation
    var params = extractParamsForOperation(request, pathItem)
    queryParams := make(map[string][]*queryParam)
    var errors []*ValidationError

    for qKey, qVal := range request.URL.Query() {
        // check if the param is encoded as a property / deepObject
        if strings.IndexRune(qKey, '[') > 0 && strings.IndexRune(qKey, ']') > 0 {
            stripped := qKey[:strings.IndexRune(qKey, '[')]
            value := qKey[strings.IndexRune(qKey, '[')+1 : strings.IndexRune(qKey, ']')]
            queryParams[stripped] = append(queryParams[stripped], &queryParam{
                key:      stripped,
                values:   qVal,
                property: value,
            })
        } else {
            queryParams[qKey] = append(queryParams[qKey], &queryParam{
                key:    qKey,
                values: qVal,
            })
        }
    }

    // look through the params for the query key
doneLooking:
    for p := range params {
        if params[p].In == Query {

            contentWrapped := false
            var contentType string

            // check if this param is found as a set of query strings
            if jk, ok := queryParams[params[p].Name]; ok {
            skipValues:
                for _, fp := range jk {
                    // let's check styles first.
                    errors = append(errors, v.validateQueryParamStyle(params[p], jk)...)

                    // there is a match, is the type correct
                    // this context is extracted from the 3.1 spec to explain what is going on here:
                    // For more complex scenarios, the content property can define the media type and schema of the
                    // parameter. A parameter MUST contain either a schema property, or a content property, but not both.
                    // The map MUST only contain one entry. (for content)
                    var sch *base.Schema
                    if params[p].Schema != nil {
                        sch = params[p].Schema.Schema()
                    } else {
                        // ok, no schema, check for a content type
                        if params[p].Content != nil {
                            for k, ct := range params[p].Content {
                                sch = ct.Schema.Schema()
                                contentWrapped = true
                                contentType = k
                                break
                            }
                        }
                    }
                    pType := sch.Type

                    // for each param, check each type
                    for _, ef := range fp.values {

                        // check allowReserved values. If this is set to true, then we can allow the
                        // following characters
                        //  :/?#[]@!$&'()*+,;=
                        // to be present as they are, without being URLEncoded.
                        if !params[p].AllowReserved {
                            rx := `[:\/\?#\[\]\@!\$&'\(\)\*\+,;=]`
                            regexp.MustCompile(rx)
                            if regexp.MustCompile(rx).MatchString(ef) && params[p].Explode != nil && *params[p].Explode {
                                errors = append(errors, v.incorrectReservedValues(params[p], ef, sch))
                            }
                        }

                        for _, ty := range pType {
                            switch ty {
                            case Integer, Number:
                                if _, err := strconv.ParseFloat(ef, 64); err != nil {
                                    errors = append(errors, v.invalidQueryParamNumber(params[p], ef, sch))
                                }
                            case Boolean:
                                if _, err := strconv.ParseBool(ef); err != nil {
                                    errors = append(errors, v.incorrectQueryParamBool(params[p], ef, sch))
                                }
                            case Object:

                                // check what style of encoding was used and then construct a map[string]interface{}
                                // and pass that in as encoded JSON.
                                var encodedObj map[string]interface{}

                                switch params[p].Style {
                                case DeepObject:
                                    encodedObj = constructParamMapFromDeepObjectEncoding(jk)
                                case PipeDelimited:
                                    encodedObj = constructParamMapFromPipeEncoding(jk)
                                case SpaceDelimited:
                                    encodedObj = constructParamMapFromSpaceEncoding(jk)
                                default:
                                    // form encoding is default.
                                    if contentWrapped {
                                        switch contentType {
                                        case JSONContentType:
                                            // we need to unmarshal the JSON into a map[string]interface{}
                                            encodedParams := make(map[string]interface{})
                                            encodedObj = make(map[string]interface{})
                                            if err := json.Unmarshal([]byte(ef), &encodedParams); err != nil {
                                                errors = append(errors, v.incorrectParamEncodingJSON(params[p], ef, sch))
                                                break skipValues
                                            }
                                            encodedObj[params[p].Name] = encodedParams
                                        }
                                    } else {
                                        encodedObj = constructParamMapFromFormEncodingArray(jk)
                                    }
                                }

                                numErrors := len(errors)
                                errors = append(errors, v.validateSchema(sch, encodedObj[params[p].Name].(map[string]interface{}), ef,
                                    "Query parameter",
                                    "The query parameter",
                                    params[p].Name,
                                    ParameterValidation,
                                    ParameterValidationQuery)...)
                                if len(errors) > numErrors {
                                    // we've already added an error for this, so we can skip the rest of the values
                                    break skipValues
                                }

                            case Array:
                                // well we're already in an array, so we need to check the items schema
                                // to ensure this array items matches the type
                                // only check if items is a schema, not a boolean
                                if sch.Items.IsA() {
                                    errors = append(errors, v.validateQueryArray(sch, params[p], ef, contentWrapped)...)
                                }
                            }
                        }
                    }
                }

            } else {
                // if the param is not in the request, so let's check if this param is an
                // object, and if we should use default encoding and explode values.
                if params[p].Schema != nil {
                    sch := params[p].Schema.Schema()

                    if sch.Type[0] == Object && params[p].IsDefaultFormEncoding() {
                        // if the param is an object, and we're using default encoding, then we need to
                        // validate the schema.
                        decoded := constructParamMapFromQueryParamInput(queryParams)
                        errors = append(errors, v.validateSchema(sch, decoded, "",
                            "Query array parameter",
                            "The query parameter (which is an array)",
                            params[p].Name,
                            ParameterValidation,
                            ParameterValidationQuery)...)
                        break doneLooking
                    }
                }
                // if there is no match, check if the param is required or not.
                if params[p].Required {
                    errors = append(errors, v.queryParameterMissing(params[p]))
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

func (v *validator) validateQueryArray(
    sch *base.Schema, param *v3.Parameter, ef string, contentWrapped bool) []*ValidationError {

    var errors []*ValidationError
    itemsSchema := sch.Items.A.Schema()

    // check for an exploded bit on the schema.
    // if it's exploded, then we need to check each item in the array
    // if it's not exploded, then we need to check the whole array as a string
    var items []string
    if param.Explode != nil && *param.Explode {
        items = explodeQueryValue(ef, param.Style)
    } else {
        // check for a style of form (or no style) and if so, explode the value
        if param.Style == "" || param.Style == Form {
            if !contentWrapped {
                items = explodeQueryValue(ef, param.Style)
            } else {
                items = []string{ef}
            }
        } else {
            switch param.Style {
            case PipeDelimited, SpaceDelimited:
                items = explodeQueryValue(ef, param.Style)
            }
        }
    }

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
                }
            case Object:
                errors = append(errors, v.validateSchema(itemsSchema, nil, item,
                    "Query array parameter",
                    "The query parameter (which is an array)",
                    param.Name,
                    ParameterValidation,
                    ParameterValidationQuery)...)
            case String:
                // do nothing for now.
                continue
            }
        }
    }
    return errors
}

func (v *validator) validateQueryParamStyle(param *v3.Parameter, as []*queryParam) []*ValidationError {
    var errors []*ValidationError
stopValidation:
    for _, qp := range as {
        for i := range qp.values {
            switch param.Style {
            case DeepObject:
                if len(qp.values) > 1 {
                    errors = append(errors, v.invalidDeepObject(param, qp))
                    break stopValidation
                }

            case PipeDelimited:
                // check if explode is false, but we have used an array style
                if param.Explode == nil || !*param.Explode {
                    if len(qp.values) > 1 {
                        errors = append(errors, v.incorrectPipeDelimiting(param, qp))
                        break stopValidation
                    }
                }
            case SpaceDelimited:
                // check if explode is false, but we have used an array style
                if param.Explode == nil || !*param.Explode {
                    if len(qp.values) > 1 {
                        errors = append(errors, v.incorrectSpaceDelimiting(param, qp))
                        break stopValidation
                    }
                }
            default:
                // check for a delimited list.
                if doesFormParamContainDelimiter(qp.values[i], param.Style) {
                    if param.Explode != nil && *param.Explode {
                        errors = append(errors, v.incorrectFormEncoding(param, qp, i))
                        break stopValidation
                    }
                }
            }
        }
    }
    return errors // defaults to true if no style is set.
}
