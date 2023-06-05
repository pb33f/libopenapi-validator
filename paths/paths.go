// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	"fmt"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// FindPath will find the path in the document that matches the request path. If a successful match was found, then
// the first return value will be a pointer to the PathItem. The second return value will contain any validation errors
// that were picked up when locating the path. Number/Integer validation is performed in any path parameters in the request.
// The third return value will be the path that was found in the document, as it pertains to the contract, so all path
// parameters will not have been replaced with their values from the request - allowing model lookups.
func FindPath(request *http.Request, document *v3.Document) (*v3.PathItem, []*errors.ValidationError, string) {

	var validationErrors []*errors.ValidationError

	// extract base path from document to check against paths.
	var basePaths []string
	for _, s := range document.Servers {
		u, _ := url.Parse(s.URL)
		if u != nil && u.Path != "" {
			basePaths = append(basePaths, u.Path)
		}
	}

	// strip any base path
	stripped := stripBaseFromPath(request.URL.Path, basePaths)

	reqPathSegments := strings.Split(stripped, "/")
	if reqPathSegments[0] == "" {
		reqPathSegments = reqPathSegments[1:]
	}

	var pItem *v3.PathItem
	var foundPath string
pathFound:
	for path, pathItem := range document.Paths.PathItems {
		segs := strings.Split(path, "/")
		if segs[0] == "" {
			segs = segs[1:]
		}

		// collect path level params
		params := pathItem.Parameters
		var errs []*errors.ValidationError
		var ok bool
		switch request.Method {
		case http.MethodGet:
			if pathItem.Get != nil {
				p := append(params, pathItem.Get.Parameters...)
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		case http.MethodPost:
			if pathItem.Post != nil {
				p := append(params, pathItem.Post.Parameters...)
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		case http.MethodPut:
			if pathItem.Put != nil {
				p := append(params, pathItem.Put.Parameters...)
				// check for a literal match
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		case http.MethodDelete:
			if pathItem.Delete != nil {
				p := append(params, pathItem.Delete.Parameters...)
				// check for a literal match
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		case http.MethodOptions:
			if pathItem.Options != nil {
				p := append(params, pathItem.Options.Parameters...)
				// check for a literal match
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		case http.MethodHead:
			if pathItem.Head != nil {
				p := append(params, pathItem.Head.Parameters...)
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		case http.MethodPatch:
			if pathItem.Patch != nil {
				p := append(params, pathItem.Patch.Parameters...)
				// check for a literal match
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		case http.MethodTrace:
			if pathItem.Trace != nil {
				p := append(params, pathItem.Trace.Parameters...)
				if checkPathAgainstBase(request.URL.Path, path, basePaths) {
					pItem = pathItem
					foundPath = path
					break pathFound
				}
				if ok, errs = comparePaths(segs, reqPathSegments, p, basePaths); ok {
					pItem = pathItem
					foundPath = path
					validationErrors = errs
					break pathFound
				} else {
					validationErrors = errs
				}
			}
		}
	}
	if pItem == nil && len(validationErrors) == 0 {
		validationErrors = append(validationErrors, &errors.ValidationError{
			ValidationType:    helpers.ParameterValidationPath,
			ValidationSubType: "missing",
			Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
			Reason: fmt.Sprintf("The %s request contains a path of '%s' "+
				"however that path, or the %s method for that path does not exist in the specification",
				request.Method, request.URL.Path, request.Method),
			SpecLine: -1,
			SpecCol:  -1,
			HowToFix: errors.HowToFixPath,
		})
		return pItem, validationErrors, foundPath
	} else {
		return pItem, validationErrors, foundPath
	}
}

func checkPathAgainstBase(docPath, urlPath string, basePaths []string) bool {
	if docPath == urlPath {
		return true
	}
	for i := range basePaths {
		if basePaths[i][len(basePaths[i])-1] == '/' {
			basePaths[i] = basePaths[i][:len(basePaths[i])-1]
		}
		merged := fmt.Sprintf("%s%s", basePaths[i], urlPath)
		if docPath == merged {
			return true
		}
	}
	return false
}

func stripBaseFromPath(path string, basePaths []string) string {
	for i := range basePaths {
		if strings.HasPrefix(path, basePaths[i]) {
			return path[len(basePaths[i]):]
		}
	}
	return path
}

func comparePaths(mapped, requested []string,
	params []*v3.Parameter, basePaths []string) (bool, []*errors.ValidationError) {

	// check lengths first
	var pathErrors []*errors.ValidationError

	if len(mapped) != len(requested) {
		return false, nil // short circuit out
	}
	var imploded []string
	for i, seg := range mapped {
		s := seg
		//sOrig := seg
		// check for braces
		if strings.Contains(seg, "{") {
			s = requested[i]
			//sOrig = s
		}
		// check param against type, check if it's a number or not, and if it validates.
		for p := range params {
			if params[p].In == helpers.Path {
				h := seg[1 : len(seg)-1]
				if params[p].Name == h {
					schema := params[p].Schema.Schema()
					for t := range schema.Type {

						switch schema.Type[t] {
						case helpers.String, helpers.Object, helpers.Array:
							// should not be a number.
							if _, err := strconv.ParseFloat(s, 64); err == nil {
								s = helpers.FailSegment
							}
						case helpers.Number, helpers.Integer:
							// should not be a string.
							if _, err := strconv.ParseFloat(s, 64); err != nil {
								s = helpers.FailSegment
							}
							// TODO: check for encoded objects and arrays (yikes)
						}
					}
				}
			}
		}
		imploded = append(imploded, s)
	}
	l := filepath.Join(imploded...)
	r := filepath.Join(requested...)
	return checkPathAgainstBase(l, r, basePaths), pathErrors
}
