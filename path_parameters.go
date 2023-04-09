// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "fmt"
    "net/http"
    "strconv"
    "strings"
)

// ValidatePathParams validates the path parameters contained within *http.Request. It returns a boolean stating true
// if validation passed (false for failed), and a slice of errors if validation failed.
func (v *validator) ValidatePathParams(request *http.Request) (bool, []*ValidationError) {

    // find path
    pathItem, errs, foundPath := v.FindPath(request)
    if pathItem == nil || errs != nil {
        return false, errs
    }

    // extract params for the operation
    var params = extractParamsForOperation(request, pathItem)
    var errors []*ValidationError
    for _, p := range params {
        if p.In == Path {

            // split the path into segments
            submittedSegments := strings.Split(request.URL.Path, Slash)
            pathSegments := strings.Split(foundPath, Slash)

            //var paramTemplate string
            for x := range pathSegments {
                if pathSegments[x] == "" { // skip empty segments
                    continue
                }
                i := strings.IndexRune(pathSegments[x], '{')
                if i > -1 {
                    isMatrix := false
                    isLabel := false
                    //isExplode := false
                    isSimple := true
                    paramTemplate := pathSegments[x][i+1 : len(pathSegments[x])-1]
                    paramName := paramTemplate
                    // check for an asterisk on the end of the parameter (explode)
                    if strings.HasSuffix(paramTemplate, Asterisk) {
                        //isExplode = true
                        paramName = paramTemplate[:len(paramTemplate)-1]
                    }
                    if strings.HasPrefix(paramTemplate, Period) {
                        isLabel = true
                        isSimple = false
                        paramName = paramName[1:]
                    }
                    if strings.HasPrefix(paramTemplate, SemiColon) {
                        isMatrix = true
                        isSimple = false
                        paramName = paramName[1:]
                    }

                    // does this param name match the current path segment param name
                    if paramName != p.Name {
                        continue
                    }

                    // extract the parameter value from the path.
                    paramValue := submittedSegments[x]

                    // extract the schema from the parameter
                    sch := p.Schema.Schema()

                    for typ := range sch.Type {

                        switch sch.Type[typ] {
                        case Integer, Number:
                            if isSimple {
                                if _, err := strconv.ParseFloat(paramValue, 64); err != nil {
                                    errors = append(errors, v.incorrectPathParamNumber(p, paramValue, sch))
                                }
                            }
                            if isLabel && p.Style == LabelStyle {
                                if _, err := strconv.ParseFloat(paramValue[1:], 64); err != nil {
                                    errors = append(errors, v.incorrectPathParamNumber(p, paramValue[1:], sch))
                                }
                            }
                            if isMatrix && p.Style == MatrixStyle {
                                // strip off the colon and the parameter name
                                paramValue = strings.Replace(paramValue[1:], fmt.Sprintf("%s=", p.Name), "", 1)
                                if _, err := strconv.ParseFloat(paramValue[1:], 64); err != nil {
                                    errors = append(errors, v.incorrectPathParamNumber(p, paramValue[1:], sch))
                                }
                            }

                        case Boolean:
                            if isLabel && p.Style == LabelStyle {
                                if _, err := strconv.ParseFloat(paramValue[1:], 64); err != nil {
                                    errors = append(errors, v.incorrectPathParamNumber(p, paramValue[1:], sch))
                                }
                            }
                            if isSimple {
                                if _, err := strconv.ParseBool(paramValue); err != nil {
                                    errors = append(errors, v.incorrectPathParamBool(p, paramValue, sch))
                                }
                            }
                            if isMatrix && p.Style == MatrixStyle {
                                // strip off the colon and the parameter name
                                paramValue = strings.Replace(paramValue[1:], fmt.Sprintf("%s=", p.Name), "", 1)
                                if _, err := strconv.ParseBool(paramValue); err != nil {
                                    errors = append(errors, v.incorrectPathParamBool(p, paramValue, sch))
                                }
                            }
                        case Object:
                            var encodedObject interface{}

                            if p.IsDefaultPathEncoding() {
                                encodedObject = constructMapFromCSV(paramValue)
                            } else {
                                switch p.Style {
                                case LabelStyle:
                                    if !p.IsExploded() {
                                        encodedObject = constructMapFromCSV(paramValue[1:])
                                    } else {
                                        encodedObject = constructKVFromLabelEncoding(paramValue)
                                    }
                                case MatrixStyle:
                                    fmt.Print(paramValue[1:])

                                default:
                                    if p.IsExploded() {
                                        encodedObject = constructKVFromCSV(paramValue)
                                    }
                                }
                            }
                            // if a schema was extracted
                            if sch != nil {
                                errors = append(errors,
                                    v.validateSchema(sch, encodedObject, "",
                                        "Path parameter",
                                        "The path parameter",
                                        p.Name,
                                        ParameterValidation,
                                        ParameterValidationPath)...)
                            }

                        case Array:

                            // extract the items schema in order to validate the array items.
                            if sch.Items != nil && sch.Items.IsA() {
                                iSch := sch.Items.A.Schema()
                                for n := range iSch.Type {
                                    // determine how to explode the array
                                    var arrayValues []string
                                    if isSimple {
                                        arrayValues = strings.Split(paramValue, Comma)
                                    }
                                    if isLabel {
                                        arrayValues = strings.Split(paramValue, Period)
                                    }
                                    if isMatrix {
                                        panic("oh my stars")
                                    }
                                    switch iSch.Type[n] {
                                    case Integer, Number:
                                        for pv := range arrayValues {
                                            if _, err := strconv.ParseFloat(arrayValues[pv], 64); err != nil {
                                                errors = append(errors,
                                                    v.incorrectPathParamArrayNumber(p, arrayValues[pv], sch, iSch))
                                            }
                                        }
                                    case Boolean:
                                        for pv := range arrayValues {
                                            bc := len(errors)
                                            if _, err := strconv.ParseBool(arrayValues[pv]); err != nil {
                                                errors = append(errors,
                                                    v.incorrectPathParamArrayBoolean(p, arrayValues[pv], sch, iSch))
                                                continue
                                            }
                                            if len(errors) == bc {
                                                // ParseBool will parse 0 or 1 as false/true to we
                                                // need to catch this edge case.
                                                if arrayValues[pv] == "0" || arrayValues[pv] == "1" {
                                                    errors = append(errors,
                                                        v.incorrectPathParamArrayBoolean(p, arrayValues[pv], sch, iSch))
                                                    continue
                                                }
                                            }
                                        }
                                    }
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
