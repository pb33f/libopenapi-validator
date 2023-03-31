// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "fmt"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "path/filepath"
    "strconv"
    "strings"
)

type ValidationError struct {
    Message  string `json:"message" yaml:"message"`
    Reason   string `json:"reason" yaml:"reason"`
    SpecLine int    `json:"specLine" yaml:"specLine"`
}

type Validator interface {
    ValidateHttpRequest(request *http.Request) (bool, []*ValidationError)
    FindPath(request *http.Request) *v3.PathItem
    ValidationErrors() []*ValidationError
}

type validator struct {
    document *v3.Document
    errors   []*ValidationError
}

type paramPathVariable struct {
    variableName string
    param        *v3.Parameter
}

func NewValidator(document *v3.Document) Validator {
    return &validator{document: document}
}

func (v *validator) ValidateHttpRequest(request *http.Request) (bool, []*ValidationError) {
    //pathItem :=
    return false, nil
}

func (v *validator) FindPath(request *http.Request) *v3.PathItem {

    reqPathSegments := strings.Split(request.URL.Path, "/")
    if reqPathSegments[0] == "" {
        reqPathSegments = reqPathSegments[1:]
    }

    for path, pathItem := range v.document.Paths.PathItems {
        segs := strings.Split(path, "/")
        if segs[0] == "" {
            segs = segs[1:]
        }

        switch request.Method {
        case http.MethodGet:
            if pathItem.Get != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Get.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        case http.MethodPost:
            if pathItem.Post != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Post.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        case http.MethodPut:
            if pathItem.Put != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Put.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        case http.MethodDelete:
            if pathItem.Delete != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Delete.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        case http.MethodOptions:
            if pathItem.Options != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Options.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        case http.MethodHead:
            if pathItem.Head != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Head.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        case http.MethodPatch:
            if pathItem.Patch != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Patch.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        case http.MethodTrace:
            if pathItem.Trace != nil {
                if v.comparePaths(segs, reqPathSegments, pathItem.Trace.Parameters, request.URL.Path) {
                    return pathItem
                }
            }
        }
    }
    return nil
}

func (v *validator) ValidationErrors() []*ValidationError {
    return v.errors
}

//
//func (v *validator) extractParams(request *http.Request) []*paramPathVariable {
//
//    return nil
//}

func (v *validator) comparePaths(mapped []string, requested []string, params []*v3.Parameter, path string) bool {

    // check lengths first
    if len(mapped) != len(requested) {
        return false // short circuit out
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
            if params[p].In == "path" {
                h := seg[1 : len(seg)-1]
                if params[p].Name == h {
                    schema := params[p].Schema.Schema()
                    for t := range schema.Type {
                        if schema.Type[t] == "number" || schema.Type[t] == "integer" {
                            notaNumber := false
                            if _, err := strconv.ParseFloat(s, 64); err != nil { // will return no error on floats or int
                                notaNumber = true
                            } else {
                                continue
                            }
                            if notaNumber {
                                v.errors = append(v.errors, &ValidationError{
                                    Message: fmt.Sprintf("Match for path '%s', but the parameter "+
                                        "'%s' is not a number", path, seg),
                                    Reason: fmt.Sprintf("The parameter '%s' is defined as a number, "+
                                        "but the value '%s' is not a number", h, s),
                                    SpecLine: params[p].GoLow().Schema.Value.Schema().Type.KeyNode.Line,
                                })
                                return false
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
        return true
    }
    return false
}
