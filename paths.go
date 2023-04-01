// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

func (v *validator) FindPath(request *http.Request) (*v3.PathItem, []*ValidationError) {

	reqPathSegments := strings.Split(request.URL.Path, "/")
	if reqPathSegments[0] == "" {
		reqPathSegments = reqPathSegments[1:]
	}
	var pItem *v3.PathItem
	for path, pathItem := range v.document.Paths.PathItems {
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
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		case http.MethodPost:
			if pathItem.Post != nil {
				p := append(params, pathItem.Post.Parameters...)
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		case http.MethodPut:
			if pathItem.Put != nil {
				p := append(params, pathItem.Put.Parameters...)
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		case http.MethodDelete:
			if pathItem.Delete != nil {
				p := append(params, pathItem.Delete.Parameters...)
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		case http.MethodOptions:
			if pathItem.Options != nil {
				p := append(params, pathItem.Options.Parameters...)
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		case http.MethodHead:
			if pathItem.Head != nil {
				p := append(params, pathItem.Head.Parameters...)
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		case http.MethodPatch:
			if pathItem.Patch != nil {
				p := append(params, pathItem.Patch.Parameters...)
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		case http.MethodTrace:
			if pathItem.Trace != nil {
				p := append(params, pathItem.Trace.Parameters...)
				if ok, _ := v.comparePaths(segs, reqPathSegments, p, request.URL.Path); ok {
					pItem = pathItem
					break
				}
			}
		}
	}
	if pItem == nil {
		errs := []*ValidationError{
			{
				ValidationType:    ParameterValidationPath,
				ValidationSubType: "missing",
				Message:           fmt.Sprintf("Path '%s' not found", request.URL.Path),
				Reason: fmt.Sprintf("The request contains a path of '%s' "+
					"however that path does not exist in the specification", request.URL.Path),
				SpecLine: -1,
				SpecCol:  -1,
			}}
		v.errors = errs
		return pItem, errs
	} else {
		return pItem, nil
	}
}

func (v *validator) comparePaths(mapped, requested []string,
	params []*v3.Parameter, path string) (bool, []*ValidationError) {

	// check lengths first
	var errors []*ValidationError

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
			if params[p].In == "path" {
				h := seg[1 : len(seg)-1]
				if params[p].Name == h {
					schema := params[p].Schema.Schema()
					for t := range schema.Type {
						if schema.Type[t] == "number" || schema.Type[t] == "integer" {
							notaNumber := false
							// will return no error on floats or int
							if _, err := strconv.ParseFloat(s, 64); err != nil {
								notaNumber = true
							} else {
								continue
							}
							if notaNumber {
								errors = append(errors, &ValidationError{
									ValidationType:    ParameterValidationPath,
									ValidationSubType: "number",
									Message: fmt.Sprintf("Match for path '%s', but the parameter "+
										"'%s' is not a number", path, seg),
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
	v.errors = append(v.errors, errors...)
	if l == r {
		return true, errors
	}
	return false, errors
}
