// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "encoding/json"
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
                                "however the value '%s' is encoded as an object or an array using commas. The contract defines "+
                                "the explode value to set to 'true'", param.Name, qp.values[i]),
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

func collapseCSVIntoSpaceDelimitedStyle(key string, values []string) string {
    return fmt.Sprintf("%s=%s", key, strings.Join(values, "%20"))
}

func collapseCSVIntoPipeDelimitedStyle(key string, values []string) string {
    return fmt.Sprintf("%s=%s", key, strings.Join(values, Pipe))
}

func (v *validator) ValidateQueryParams(request *http.Request) (bool, []*ValidationError) {

    // find path
    pathItem, errs := v.FindPath(request)
    if pathItem == nil || errs != nil {
        return false, errs
    }

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
                                                errors = append(errors, &ValidationError{
                                                    ValidationType:    ParameterValidation,
                                                    ValidationSubType: ParameterValidationQuery,
                                                    Message:           fmt.Sprintf("Query parameter '%s' is not valid JSON", params[p].Name),
                                                    Reason: fmt.Sprintf("The query parameter '%s' is defined as being a JSON object, "+
                                                        "however the value '%s' is not valid JSON", params[p].Name, ef),
                                                    SpecLine: params[p].GoLow().Schema.KeyNode.Line,
                                                    SpecCol:  params[p].GoLow().Schema.KeyNode.Column,
                                                    Context:  sch,
                                                    HowToFix: HowToFixInvalidJSON,
                                                })
                                                break skipValues
                                            }

                                            encodedObj[params[p].Name] = encodedParams
                                        }
                                    } else {
                                        encodedObj = constructParamMapFromFormEncodingArray(jk)
                                    }

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
                                        //check if the item has a delimiter and there are multiple items
                                        if doesParamContainDelimiter(ef, params[p].Style) && len(fp.values) > 1 {
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
                                        // check for a style of form (or no style) and if so, explode the value
                                        if params[p].Style == "" || params[p].Style == Form {
                                            if !contentWrapped {
                                                items = explodeQueryValue(ef, params[p].Style)
                                            } else {
                                                items = []string{ef}
                                            }
                                        } else {
                                            items = []string{ef}
                                        }
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

                // if the param is not in the request, so let's check if this param is an
                // object, and if we should use default encoding and explode values.

                if params[p].Schema != nil {
                    sch := params[p].Schema.Schema()

                    if sch.Type[0] == Object && params[p].IsDefaultEncoding() {
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
        if !strings.Contains(v, Period) {
            iv, _ := strconv.ParseInt(v, 10, 64)
            return iv
        }
        return i
    }
    return v
}

// deepObject encoding is a technique used to encode objects into query parameters. Kinda nuts.
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

func constructParamMapFromQueryParamInput(values map[string][]*queryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, q := range values {
        for _, v := range q {
            decoded[v.key] = cast(v.values[0])
        }
    }
    return decoded
}

// Pipes are always a good alternative to commas, personally I think they're better, if I were encoding, I would
// use pipes instead of commas, so much can go wrong with a comma, but a pipe? hardly ever.
func constructParamMapFromPipeEncoding(values []*queryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        props := make(map[string]interface{})
        // explode PSV into array
        exploded := strings.Split(v.values[0], Pipe)
        for i := range exploded {
            if i%2 == 0 {
                props[exploded[i]] = cast(exploded[i+1])
            }
        }
        decoded[v.key] = props
    }
    return decoded
}

// Don't use spaces to delimit anything unless you really know what the hell you're doing. Perhaps the
// easiest way to blow something up, unless you're tokenizing strings... don't do this.
func constructParamMapFromSpaceEncoding(values []*queryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        props := make(map[string]interface{})
        // explode SSV into array
        exploded := strings.Split(v.values[0], Space)
        for i := range exploded {
            if i%2 == 0 {
                props[exploded[i]] = cast(exploded[i+1])
            }
        }
        decoded[v.key] = props
    }
    return decoded
}

func constructParamMapFromFormEncodingArray(values []*queryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        props := make(map[string]interface{})
        // explode SSV into array
        exploded := strings.Split(v.values[0], Comma)
        for i := range exploded {
            if i%2 == 0 {
                props[exploded[i]] = cast(exploded[i+1])
            }
        }
        decoded[v.key] = props
    }
    return decoded
}
