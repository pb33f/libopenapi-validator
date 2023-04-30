// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
    "encoding/json"
    "fmt"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/paths"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "regexp"
    "strconv"
    "strings"
)

func (v *paramValidator) ValidateQueryParams(request *http.Request) (bool, []*errors.ValidationError) {

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
    queryParams := make(map[string][]*helpers.QueryParam)
    var validationErrors []*errors.ValidationError

    for qKey, qVal := range request.URL.Query() {
        // check if the param is encoded as a property / deepObject
        if strings.IndexRune(qKey, '[') > 0 && strings.IndexRune(qKey, ']') > 0 {
            stripped := qKey[:strings.IndexRune(qKey, '[')]
            value := qKey[strings.IndexRune(qKey, '[')+1 : strings.IndexRune(qKey, ']')]
            queryParams[stripped] = append(queryParams[stripped], &helpers.QueryParam{
                Key:      stripped,
                Values:   qVal,
                Property: value,
            })
        } else {
            queryParams[qKey] = append(queryParams[qKey], &helpers.QueryParam{
                Key:    qKey,
                Values: qVal,
            })
        }
    }

    // look through the params for the query key
doneLooking:
    for p := range params {
        if params[p].In == helpers.Query {

            contentWrapped := false
            var contentType string
            // check if this param is found as a set of query strings
            if jk, ok := queryParams[params[p].Name]; ok {
            skipValues:
                for _, fp := range jk {
                    // let's check styles first.
                    validationErrors = append(validationErrors, ValidateQueryParamStyle(params[p], jk)...)

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
                    for _, ef := range fp.Values {

                        // check allowReserved values. If this is set to true, then we can allow the
                        // following characters
                        //  :/?#[]@!$&'()*+,;=
                        // to be present as they are, without being URLEncoded.
                        if !params[p].AllowReserved {
                            rx := `[:\/\?#\[\]\@!\$&'\(\)\*\+,;=]`
                            regexp.MustCompile(rx)
                            if regexp.MustCompile(rx).MatchString(ef) && params[p].IsExploded() {
                                validationErrors = append(validationErrors,
                                    errors.IncorrectReservedValues(params[p], ef, sch))
                            }
                        }
                        for _, ty := range pType {
                            switch ty {

                            case helpers.String:

                                // check if the param is within an enum
                                if sch.Enum != nil {
                                    matchFound := false
                                    for _, enumVal := range sch.Enum {
                                        if strings.TrimSpace(ef) == fmt.Sprint(enumVal) {
                                            matchFound = true
                                            break
                                        }
                                    }
                                    if !matchFound {
                                        validationErrors = append(validationErrors,
                                            errors.IncorrectQueryParamEnum(params[p], ef, sch))
                                    }
                                }

                            case helpers.Integer, helpers.Number:
                                if _, err := strconv.ParseFloat(ef, 64); err != nil {
                                    validationErrors = append(validationErrors,
                                        errors.InvalidQueryParamNumber(params[p], ef, sch))
                                    break
                                }
                                // check if the param is within an enum
                                if sch.Enum != nil {
                                    matchFound := false
                                    for _, enumVal := range sch.Enum {
                                        if strings.TrimSpace(ef) == fmt.Sprint(enumVal) {
                                            matchFound = true
                                            break
                                        }
                                    }
                                    if !matchFound {
                                        validationErrors = append(validationErrors,
                                            errors.IncorrectQueryParamEnum(params[p], ef, sch))
                                    }
                                }

                            case helpers.Boolean:
                                if _, err := strconv.ParseBool(ef); err != nil {
                                    validationErrors = append(validationErrors,
                                        errors.IncorrectQueryParamBool(params[p], ef, sch))
                                }
                            case helpers.Object:

                                // check what style of encoding was used and then construct a map[string]interface{}
                                // and pass that in as encoded JSON.
                                var encodedObj map[string]interface{}

                                switch params[p].Style {
                                case helpers.DeepObject:
                                    encodedObj = helpers.ConstructParamMapFromDeepObjectEncoding(jk)
                                case helpers.PipeDelimited:
                                    encodedObj = helpers.ConstructParamMapFromPipeEncoding(jk)
                                case helpers.SpaceDelimited:
                                    encodedObj = helpers.ConstructParamMapFromSpaceEncoding(jk)
                                default:
                                    // form encoding is default.
                                    if contentWrapped {
                                        switch contentType {
                                        case helpers.JSONContentType:
                                            // we need to unmarshal the JSON into a map[string]interface{}
                                            encodedParams := make(map[string]interface{})
                                            encodedObj = make(map[string]interface{})
                                            if err := json.Unmarshal([]byte(ef), &encodedParams); err != nil {
                                                validationErrors = append(validationErrors,
                                                    errors.IncorrectParamEncodingJSON(params[p], ef, sch))
                                                break skipValues
                                            }
                                            encodedObj[params[p].Name] = encodedParams
                                        }
                                    } else {
                                        encodedObj = helpers.ConstructParamMapFromFormEncodingArray(jk)
                                    }
                                }

                                numErrors := len(validationErrors)
                                validationErrors = append(validationErrors,
                                    ValidateParameterSchema(sch, encodedObj[params[p].Name].(map[string]interface{}),
                                        ef,
                                        "Query parameter",
                                        "The query parameter",
                                        params[p].Name,
                                        helpers.ParameterValidation,
                                        helpers.ParameterValidationQuery)...)
                                if len(validationErrors) > numErrors {
                                    // we've already added an error for this, so we can skip the rest of the values
                                    break skipValues
                                }

                            case helpers.Array:
                                // well we're already in an array, so we need to check the items schema
                                // to ensure this array items matches the type
                                // only check if items is a schema, not a boolean
                                if sch.Items.IsA() {
                                    validationErrors = append(validationErrors,
                                        ValidateQueryArray(sch, params[p], ef, contentWrapped)...)
                                }
                            }
                        }
                    }
                }

            } else {
                // if the param is not in the requests, so let's check if this param is an
                // object, and if we should use default encoding and explode values.
                if params[p].Schema != nil {
                    sch := params[p].Schema.Schema()

                    if sch.Type[0] == helpers.Object && params[p].IsDefaultFormEncoding() {
                        // if the param is an object, and we're using default encoding, then we need to
                        // validate the schema.
                        decoded := helpers.ConstructParamMapFromQueryParamInput(queryParams)
                        validationErrors = append(validationErrors,
                            ValidateParameterSchema(sch,
                                decoded,
                                "",
                                "Query array parameter",
                                "The query parameter (which is an array)",
                                params[p].Name,
                                helpers.ParameterValidation,
                                helpers.ParameterValidationQuery)...)
                        break doneLooking
                    }
                }
                // if there is no match, check if the param is required or not.
                if params[p].Required {
                    validationErrors = append(validationErrors, errors.QueryParameterMissing(params[p]))
                }
            }
        }
    }

    v.errors = validationErrors
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
