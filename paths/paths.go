// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
    "fmt"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "path/filepath"
    "strconv"
    "strings"
)

func FindPath(request *http.Request, document *v3.Document) (*v3.PathItem, []*errors.ValidationError, string) {

    var validationErrors []*errors.ValidationError

    reqPathSegments := strings.Split(request.URL.Path, "/")
    if reqPathSegments[0] == "" {
        reqPathSegments = reqPathSegments[1:]
    }
    var pItem *v3.PathItem
    var foundPath string
    for path, pathItem := range document.Paths.PathItems {
        segs := strings.Split(path, "/")
        if segs[0] == "" {
            segs = segs[1:]
        }

        // collect path level params
        params := pathItem.Parameters

        switch request.Method {
        case http.MethodGet:
            if pathItem.Get != nil {
                p := append(params, pathItem.Get.Parameters...)
                if ok, errs := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    validationErrors = errs
                    break
                }
            }
        case http.MethodPost:
            if pathItem.Post != nil {
                p := append(params, pathItem.Post.Parameters...)
                if ok, _ := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    break
                }
            }
        case http.MethodPut:
            if pathItem.Put != nil {
                p := append(params, pathItem.Put.Parameters...)
                if ok, errs := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    validationErrors = errs
                    break
                }
            }
        case http.MethodDelete:
            if pathItem.Delete != nil {
                p := append(params, pathItem.Delete.Parameters...)
                if ok, errs := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    validationErrors = errs
                    break
                }
            }
        case http.MethodOptions:
            if pathItem.Options != nil {
                p := append(params, pathItem.Options.Parameters...)
                if ok, errs := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    validationErrors = errs
                    break
                }
            }
        case http.MethodHead:
            if pathItem.Head != nil {
                p := append(params, pathItem.Head.Parameters...)
                if ok, errs := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    validationErrors = errs
                    break
                }
            }
        case http.MethodPatch:
            if pathItem.Patch != nil {
                p := append(params, pathItem.Patch.Parameters...)
                if ok, errs := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    validationErrors = errs
                    break
                }
            }
        case http.MethodTrace:
            if pathItem.Trace != nil {
                p := append(params, pathItem.Trace.Parameters...)
                if ok, errs := comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
                    pItem = pathItem
                    foundPath = path
                    validationErrors = errs
                    break
                }
            }
        }
    }
    if pItem == nil {
        validationErrors = append(validationErrors, &errors.ValidationError{
            ValidationType:    helpers.ParameterValidationPath,
            ValidationSubType: "missing",
            Message:           fmt.Sprintf("Path '%s' not found", request.URL.Path),
            Reason: fmt.Sprintf("The requests contains a path of '%s' "+
                "however that path does not exist in the specification", request.URL.Path),
            SpecLine: -1,
            SpecCol:  -1,
        })
        return pItem, validationErrors, foundPath
    } else {
        return pItem, validationErrors, foundPath
    }
}

func comparePaths(mapped, requested []string,
    params []*v3.Parameter, path string) (bool, []*errors.ValidationError) {

    // check lengths first
    var pathErrors []*errors.ValidationError

    if len(mapped) != len(requested) {
        return false, nil // short circuit out
    }
    var imploded []string
    for i, seg := range mapped {
        s := seg
        // check for braces
        if strings.Contains(seg, "{") {
            s = requested[i]
        }
        // check param against type, check if it's a number or not, and if it validates.
        for p := range params {
            if params[p].In == helpers.Path {
                h := seg[1 : len(seg)-1]
                if params[p].Name == h {
                    schema := params[p].Schema.Schema()
                    for t := range schema.Type {
                        if schema.Type[t] == helpers.Number || schema.Type[t] == helpers.Integer {
                            notaNumber := false
                            // will return no error on floats or int
                            if _, err := strconv.ParseFloat(s, 64); err != nil {
                                notaNumber = true
                            } else {
                                continue
                            }
                            if notaNumber {
                                pathErrors = append(pathErrors, &errors.ValidationError{
                                    ValidationType:    helpers.ParameterValidationPath,
                                    ValidationSubType: "number",
                                    Message: fmt.Sprintf("Match for path '%s', but the parameter "+
                                        "'%s' is not a number", path, s),
                                    Reason: fmt.Sprintf("The parameter '%s' is defined as a number, "+
                                        "but the value '%s' is not a number", h, s),
                                    SpecLine: params[p].GoLow().Schema.Value.Schema().Type.KeyNode.Line,
                                    SpecCol:  params[p].GoLow().Schema.Value.Schema().Type.KeyNode.Column,
                                    Context:  schema,
                                })
                            }
                        }
                    }
                }
            }
        }
        imploded = append(imploded, s)
    }
    l := filepath.Join(imploded...)
    r := filepath.Join(requested...)
    if l == r {
        return true, pathErrors
    }
    return false, pathErrors
}
