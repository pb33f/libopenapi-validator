// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
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
                    var paramName string
                    paramTemplate := pathSegments[x][i+1 : len(pathSegments[x])-1]
                    // check for an asterisk on the end of the parameter (explode)
                    if strings.HasSuffix(paramTemplate, Asterisk) {
                        //isExplode = true
                        paramName = paramTemplate[:len(paramTemplate)-1]
                    }
                    if strings.HasPrefix(paramTemplate, Period) {
                        isLabel = true
                        isSimple = false
                        paramName = paramTemplate[:len(paramTemplate)-1]
                    }
                    if strings.HasPrefix(paramTemplate, SemiColon) {
                        isMatrix = true
                        isSimple = false
                        paramName = paramTemplate[:len(paramTemplate)-1]
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
                            // primitives were checked already in FindPath,
                            // now we need to validate arrays and objects
                            // with interesting and strange encoding schemes
                            break
                        case Boolean:
                            if _, err := strconv.ParseBool(paramValue); err != nil {
                                errors = append(errors, v.incorrectPathParamBool(p, paramValue, sch))
                            }
                        case Array:

                            // extract the items schema in order to validate the array items.
                            if sch.Items != nil && sch.Items.IsA() {
                                iSch := sch.Items.A.Schema()
                                for n := range iSch.Type {
                                    switch iSch.Type[n] {
                                    case Integer, Number:

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
                                        for pv := range arrayValues {
                                            if _, err := strconv.ParseFloat(arrayValues[pv], 64); err != nil {
                                                errors = append(errors, v.incorrectPathParamArrayNumber(p, arrayValues[pv], sch, iSch))
                                            }
                                        }
                                    case Boolean:
                                        if _, err := strconv.ParseBool(paramValue); err != nil {
                                            errors = append(errors, v.incorrectPathParamBool(p, paramValue, sch))
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
