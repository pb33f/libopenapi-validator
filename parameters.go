// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "fmt"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "net/url"
    "regexp"
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

type queryParam struct {
    key      string
    values   []string
    property string
}

func (v *validator) validateParamStyle(param *v3.Parameter, as []*queryParam) []*ValidationError {

    var errors []*ValidationError
stopValidation:

    for _, qp := range as {

        for i := range qp.values {

            switch param.Style {

            case MatrixStyle:
                // check if explode is false, but we have used an array style
                if param.Explode == nil || !*param.Explode {
                    if len(qp.values) > 1 {
                        errors = append(errors, &ValidationError{
                            ValidationType:    ParameterValidation,
                            ValidationSubType: ParameterValidationQuery,
                            Message:           fmt.Sprintf("Query parameter '%s' is not encoded correctly", param.Name),
                            Reason: fmt.Sprintf("The query parameter '%s' has the 'matrix' style defined, "+
                                "and has explode set to 'false'. There are multiple values (%d) supplied, instead of a single "+
                                "value", param.Name, len(qp.values)),
                            SpecLine: param.GoLow().Style.ValueNode.Line,
                            SpecCol:  param.GoLow().Style.ValueNode.Column,
                            Context:  param,
                            HowToFix: fmt.Sprintf(HowToFixParamInvalidMatrixMultipleValues,
                                collapseParamsIntoMatrixArrayStyle(param.Name, qp.values)),
                        })
                        break stopValidation
                    }
                }

            case DeepObject:
                if len(qp.values) > 1 {
                    errors = append(errors, &ValidationError{
                        ValidationType:    ParameterValidation,
                        ValidationSubType: ParameterValidationQuery,
                        Message:           fmt.Sprintf("Query parameter '%s' is not a valid deepObject", param.Name),
                        Reason: fmt.Sprintf("The query parameter '%s' has the 'deepObject' style defined, "+
                            "There are multiple values (%d) supplied, instead of a single "+
                            "value", param.Name, len(qp.values)),
                        SpecLine: param.GoLow().Style.ValueNode.Line,
                        SpecCol:  param.GoLow().Style.ValueNode.Column,
                        Context:  param,
                        HowToFix: fmt.Sprintf(HowToFixParamInvalidDeepObjectMultipleValues,
                            collapseCSVIntoPipeDelimitedStyle(param.Name, qp.values)),
                    })
                    break stopValidation
                }

            case PipeDelimited:
                // check for a comma separated list
                if !doesParamContainDelimiter(qp.values[i], PipeDelimited) {
                    errors = append(errors, &ValidationError{
                        ValidationType:    ParameterValidation,
                        ValidationSubType: ParameterValidationQuery,
                        Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
                        Reason: fmt.Sprintf("The query parameter '%s' has 'pipeDelimited' style defined, "+
                            "however the value '%s' contains comma separated values that indicates an object. "+
                            "Unfortunately, objects cannot be encoded with this style.", param.Name, qp.values[i]),
                        SpecLine: param.GoLow().Style.ValueNode.Line,
                        SpecCol:  param.GoLow().Style.ValueNode.Column,
                        Context:  param,
                        HowToFix: fmt.Sprintf(HowToFixParamInvalidPipeDelimitedObject, collapseCSVIntoFormStyle(param.Name, qp.values[i])),
                    })
                    break stopValidation
                }

                // check if explode is false, but we have used an array style
                if param.Explode == nil || !*param.Explode {
                    if len(qp.values) > 1 {
                        errors = append(errors, &ValidationError{
                            ValidationType:    ParameterValidation,
                            ValidationSubType: ParameterValidationQuery,
                            Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
                            Reason: fmt.Sprintf("The query parameter '%s' has 'pipeDelimited' style defined, "+
                                "and explode is defined as false. There are multiple values (%d) supplied, instead of a single"+
                                " space delimited value", param.Name, len(qp.values)),
                            SpecLine: param.GoLow().Style.ValueNode.Line,
                            SpecCol:  param.GoLow().Style.ValueNode.Column,
                            Context:  param,
                            HowToFix: fmt.Sprintf(HowToFixParamInvalidPipeDelimitedObjectExplode,
                                collapseCSVIntoPipeDelimitedStyle(param.Name, qp.values)),
                        })
                        break stopValidation
                    }
                }

            case SpaceDelimited:
                // check for a comma separated list
                if strings.Contains(qp.values[i], Comma) || strings.Contains(qp.values[i], Pipe) {

                    errors = append(errors, &ValidationError{
                        ValidationType:    ParameterValidation,
                        ValidationSubType: ParameterValidationQuery,
                        Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
                        Reason: fmt.Sprintf("The query parameter '%s' has 'spaceDelimited' style defined, "+
                            "however the value '%s' contains separated values that indicates an object. "+
                            "Unfortunately, objects cannot be encoded with this style.", param.Name, qp.values[i]),
                        SpecLine: param.GoLow().Style.ValueNode.Line,
                        SpecCol:  param.GoLow().Style.ValueNode.Column,
                        Context:  param,
                        HowToFix: fmt.Sprintf(HowToFixParamInvalidSpaceDelimitedObject,
                            collapseCSVIntoFormStyle(param.Name, qp.values[i])),
                    })
                    break stopValidation
                }

                // check if explode is false, but we have used an array style
                if param.Explode == nil || !*param.Explode {
                    if len(qp.values) > 1 {
                        errors = append(errors, &ValidationError{
                            ValidationType:    ParameterValidation,
                            ValidationSubType: ParameterValidationQuery,
                            Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
                            Reason: fmt.Sprintf("The query parameter '%s' has 'spaceDelimited' style defined, "+
                                "and explode is defined as false. There are multiple values (%d) supplied, instead of a single"+
                                " space delimited value", param.Name, len(qp.values)),
                            SpecLine: param.GoLow().Style.ValueNode.Line,
                            SpecCol:  param.GoLow().Style.ValueNode.Column,
                            Context:  param,
                            HowToFix: fmt.Sprintf(HowToFixParamInvalidSpaceDelimitedObjectExplode,
                                collapseCSVIntoSpaceDelimitedStyle(param.Name, qp.values)),
                        })
                        break stopValidation
                    }
                }

            default:

                // check for a comma separated list
                if doesParamContainDelimiter(qp.values[i], param.Style) {

                    if param.Explode != nil && *param.Explode {
                        errors = append(errors, &ValidationError{
                            ValidationType:    ParameterValidation,
                            ValidationSubType: ParameterValidationQuery,
                            Message:           fmt.Sprintf("Query parameter '%s' is not exploded correctly", param.Name),
                            Reason: fmt.Sprintf("The query parameter '%s' has a default or 'form' encoding defined, "+
                                "however the value '%s' is encoded as an object using commas with an explode value to set to 'true'", param.Name, qp.values[i]),
                            SpecLine: param.GoLow().Explode.ValueNode.Line,
                            SpecCol:  param.GoLow().Explode.ValueNode.Column,
                            Context:  param,
                            HowToFix: fmt.Sprintf(HowToFixParamInvalidFormEncode,
                                collapseCSVIntoFormStyle(param.Name, qp.values[i])),
                        })
                        break stopValidation
                    }
                }
            }
        }
    }
    return errors // defaults to true if no style is set.
}

func doesParamContainDelimiter(value, style string) bool {
    if strings.Contains(value, Comma) && style == "" {
        return true
    }
    if strings.Contains(value, Pipe) && style == PipeDelimited {
        return true
    }
    if strings.Contains(value, SpaceDelimited) && style == SpaceDelimited {
        return true
    }
    return false
}

func extractDelimiterChar(style string) string {
    switch style {
    case PipeDelimited:
        return Pipe
    case SpaceDelimited:
        return Space
    default:
        return Comma
    }
}

func explodeQueryValue(value, style string) []string {
    switch style {
    case SpaceDelimited:
        return strings.Split(value, Space)
    case PipeDelimited:
        return strings.Split(value, Pipe)
    }
    return strings.Split(value, Comma)
}

func collapseCSVIntoFormStyle(key string, value string) string {
    return fmt.Sprintf("&%s=%s", key,
        strings.Join(strings.Split(value, ","), fmt.Sprintf("&%s=", key)))
}

func collapseParamsIntoMatrixArrayStyle(key string, values []string) string {
    return fmt.Sprintf(";%s=%s", key, strings.Join(values, ","))
}

func collapseCSVIntoSpaceDelimitedStyle(key string, values []string) string {
    return fmt.Sprintf("%s=%s", key, strings.Join(values, "%20"))
}

func collapseCSVIntoPipeDelimitedStyle(key string, values []string) string {
    return fmt.Sprintf("%s=%s", key, strings.Join(values, Pipe))
}

func (v *validator) ValidateQueryParams(request *http.Request) (bool, []*ValidationError) {

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

    // check if there is a raw query string, but no params were extracted and then try to decode it.
    if request.URL.RawQuery != "" && len(queryParams) == 0 {
        if strings.Contains(request.URL.RawQuery, SemiColon) {
            // parse matrix style params.
            paramPairs := strings.Split(request.URL.RawQuery, SemiColon)
            for p := range paramPairs {
                if paramPairs[p] == "" {
                    continue
                }
                kvArr := strings.Split(paramPairs[p], Equals)
                if len(kvArr) == 2 {

                    if qp, ok := queryParams[kvArr[0]]; ok {
                        qp[0].values = append(qp[0].values, kvArr[1])
                    } else {
                        queryParams[kvArr[0]] = append(queryParams[kvArr[0]], &queryParam{
                            key:    kvArr[0],
                            values: []string{kvArr[1]},
                        })
                    }
                }
            }
        }
        if strings.Contains(request.URL.RawQuery, Period) {
            // TODO: this needs completing, however, I don't know why on earth anyone in their right mind would use this.
            // type of encoding for http APIs.
        }
    }

    // find path
    pathItem, errs := v.FindPath(request)
    if pathItem == nil || errs != nil {
        return false, errs
    }

    var params = extractParamsForOperation(request, pathItem)

    // look through the params for the query key
    for p := range params {
        if params[p].In == Query {
            // check if this param is found as a set of query strings
            if jk, ok := queryParams[params[p].Name]; ok {
            skipValues:
                for _, fp := range jk {
                    // let's check styles first.
                    errors = append(errors, v.validateParamStyle(params[p], jk)...)

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
                            for _, ct := range params[p].Content {
                                sch = ct.Schema.Schema()
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
                        if !params[p].AllowReserved && params[p].Style != MatrixStyle {
                            rx := `[:\/\?#\[\]\@!\$&'\(\)\*\+,;=]`
                            regexp.MustCompile(rx)
                            if regexp.MustCompile(rx).MatchString(ef) && params[p].Explode != nil && *params[p].Explode {
                                errors = append(errors, &ValidationError{
                                    ValidationType:    ParameterValidation,
                                    ValidationSubType: ParameterValidationQuery,
                                    Message:           fmt.Sprintf("Query parameter '%s' value contains reserved values", params[p].Name),
                                    Reason: fmt.Sprintf("The query parameter '%s' has 'allowReserved' set to false, "+
                                        "however the value '%s' contains one of the following characters: :/?#[]@!$&'()*+,;=", params[p].Name, ef),
                                    SpecLine: params[p].GoLow().Schema.KeyNode.Line,
                                    SpecCol:  params[p].GoLow().Schema.KeyNode.Column,
                                    Context:  sch,
                                    HowToFix: fmt.Sprintf(HowToFixReservedValues, url.QueryEscape(ef)),
                                })
                            }
                        }

                        for _, ty := range pType {
                            switch ty {
                            case Integer, Number:
                                if _, err := strconv.ParseFloat(ef, 64); err != nil {
                                    errors = append(errors, &ValidationError{
                                        ValidationType:    ParameterValidation,
                                        ValidationSubType: ParameterValidationQuery,
                                        Message:           fmt.Sprintf("Query parameter '%s' is not a valid number", params[p].Name),
                                        Reason: fmt.Sprintf("The query parameter '%s' is defined as being a number, "+
                                            "however the value '%s' is not a valid number", params[p].Name, ef),
                                        SpecLine: params[p].GoLow().Schema.KeyNode.Line,
                                        SpecCol:  params[p].GoLow().Schema.KeyNode.Column,
                                        Context:  sch,
                                        HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, ef),
                                    })
                                }
                            case Boolean:
                                if _, err := strconv.ParseBool(ef); err != nil {
                                    errors = append(errors, &ValidationError{
                                        ValidationType:    ParameterValidation,
                                        ValidationSubType: ParameterValidationQuery,
                                        Message:           fmt.Sprintf("Query parameter '%s' is not a valid boolean", params[p].Name),
                                        Reason: fmt.Sprintf("The query parameter '%s' is defined as being a boolean, "+
                                            "however the value '%s' is not a valid boolean", params[p].Name, ef),
                                        SpecLine: params[p].GoLow().Schema.KeyNode.Line,
                                        SpecCol:  params[p].GoLow().Schema.KeyNode.Column,
                                        Context:  sch,
                                        HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, ef),
                                    })
                                }
                            case Object:

                                // check if the object is encoded as a deepObject or matrix, if so, construct a map[string]interface{}
                                // and pass that in as encoded JSON.
                                var encodedObj map[string]interface{}
                                if params[p].Style == DeepObject {
                                    encodedObj = constructParamMapFromDeepObjectEncoding(jk)
                                }

                                if params[p].Style == MatrixStyle {
                                    encodedObj = constructParamMapFromMatrixEncoding(jk)
                                }

                                numErrors := len(errors)
                                errors = append(errors, v.validateSchema(sch, encodedObj[params[p].Name], ef,
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
                                    itemsSchema := sch.Items.A.Schema()

                                    // check for an exploded bit on the schema.
                                    // if it's exploded, then we need to check each item in the array
                                    // if it's not exploded, then we need to check the whole array as a string
                                    var items []string
                                    if params[p].Explode != nil && *params[p].Explode {
                                        //check if the item has a comma in it, if not, this is invalid use
                                        if !doesParamContainDelimiter(ef, params[p].Style) && len(fp.values) > 1 {
                                            delimiter := extractDelimiterChar(params[p].Style)
                                            delimitStyle := params[p].Style
                                            if delimitStyle == "" {
                                                delimitStyle = "form"
                                            }
                                            errors = append(errors, &ValidationError{
                                                ValidationType:    ParameterValidation,
                                                ValidationSubType: ParameterValidationQuery,
                                                Message:           fmt.Sprintf("Query array parameter '%s' has not been exploded correctly", params[p].Name),
                                                Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being exploded, and has a "+
                                                    "style defined as '%s', however the value '%s' is not delimited by '%s' characters. There are multiple "+
                                                    "parameters with the same name in the request (%d)", params[p].Name, delimitStyle, ef, delimiter, len(fp.values)),
                                                SpecLine: params[p].GoLow().Explode.ValueNode.Line,
                                                SpecCol:  params[p].GoLow().Explode.ValueNode.Column,
                                                Context:  sch,
                                                HowToFix: HowToFixParamInvalidExplode,
                                            })
                                            items = []string{ef}
                                        } else {
                                            items = explodeQueryValue(ef, params[p].Style)
                                        }
                                    } else {
                                        items = []string{ef}
                                    }

                                    // now check each item in the array
                                    for _, item := range items {

                                        // for each type defined in the items schema, check the item
                                        for _, itemType := range itemsSchema.Type {
                                            switch itemType {
                                            case Integer, Number:
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
                                                        HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, ef),
                                                    })
                                                }
                                            case Boolean:
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
                                                        HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, ef),
                                                    })
                                                }
                                            case Object:
                                                errors = append(errors, v.validateSchema(itemsSchema, nil, item,
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

func cast(v string) any {

    if v == "true" || v == "false" {
        b, _ := strconv.ParseBool(v)
        return b
    }
    if i, err := strconv.ParseFloat(v, 64); err == nil {
        // check if this is an int or not
        if !strings.Contains(v, ".") {
            iv, _ := strconv.ParseInt(v, 10, 64)
            return iv
        }
        return i
    }
    return v
}

// deepObject encoding is a technique used to encode objects into query parameters.
func constructParamMapFromDeepObjectEncoding(values []*queryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        if decoded[v.key] == nil {
            props := make(map[string]interface{})
            props[v.property] = cast(v.values[0])
            decoded[v.key] = props
        } else {
            decoded[v.key].(map[string]interface{})[v.property] = cast(v.values[0])
        }
    }
    return decoded
}

// deepObject encoding is a technique used to encode objects into query parameters.
func constructParamMapFromMatrixEncoding(values []*queryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        props := make(map[string]interface{})
        // explode CSV into array
        exploded := strings.Split(v.values[0], ",")
        for i := range exploded {
            if i%2 == 0 {
                props[exploded[i]] = cast(exploded[i+1])
            }
        }
        decoded[v.key] = props
    }
    return decoded
}
