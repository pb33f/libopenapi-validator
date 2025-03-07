// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

// FindPath will find the path in the document that matches the request path. If a successful match was found, then
// the first return value will be a pointer to the PathItem. The second return value will contain any validation errors
// that were picked up when locating the path.
// The third return value will be the path that was found in the document, as it pertains to the contract, so all path
// parameters will not have been replaced with their values from the request - allowing model lookups.
func FindPath(request *http.Request, document *v3.Document) (*v3.PathItem, []*errors.ValidationError, string) {
	basePaths := getBasePaths(document)
	stripped := StripRequestPath(request, document)

	reqPathSegments := strings.Split(stripped, "/")
	if reqPathSegments[0] == "" {
		reqPathSegments = reqPathSegments[1:]
	}

	var pItem *v3.PathItem
	var foundPath string
	for pair := orderedmap.First(document.Paths.PathItems); pair != nil; pair = pair.Next() {
		path := pair.Key()
		pathItem := pair.Value()

		// if the stripped path has a fragment, then use that as part of the lookup
		// if not, then strip off any fragments from the pathItem
		if !strings.Contains(stripped, "#") {
			if strings.Contains(path, "#") {
				path = strings.Split(path, "#")[0]
			}
		}

		segs := strings.Split(path, "/")
		if segs[0] == "" {
			segs = segs[1:]
		}

		ok := comparePaths(segs, reqPathSegments, basePaths)
		if !ok {
			continue
		}
		pItem = pathItem
		foundPath = path
		switch request.Method {
		case http.MethodGet:
			if pathItem.Get != nil {
				return pathItem, nil, path
			}
		case http.MethodPost:
			if pathItem.Post != nil {
				return pathItem, nil, path
			}
		case http.MethodPut:
			if pathItem.Put != nil {
				return pathItem, nil, path
			}
		case http.MethodDelete:
			if pathItem.Delete != nil {
				return pathItem, nil, path
			}
		case http.MethodOptions:
			if pathItem.Options != nil {
				return pathItem, nil, path
			}
		case http.MethodHead:
			if pathItem.Head != nil {
				return pathItem, nil, path
			}
		case http.MethodPatch:
			if pathItem.Patch != nil {
				return pathItem, nil, path
			}
		case http.MethodTrace:
			if pathItem.Trace != nil {
				return pathItem, nil, path
			}
		}
	}
	if pItem != nil {
		validationErrors := []*errors.ValidationError{{
			ValidationType:    helpers.ParameterValidationPath,
			ValidationSubType: "missingOperation",
			Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
			Reason: fmt.Sprintf("The %s method for that path does not exist in the specification",
				request.Method),
			SpecLine: -1,
			SpecCol:  -1,
			HowToFix: errors.HowToFixPath,
		}}
		errors.PopulateValidationErrors(validationErrors, request, foundPath)
		return pItem, validationErrors, foundPath
	}
	validationErrors := []*errors.ValidationError{
		{
			ValidationType:    helpers.ParameterValidationPath,
			ValidationSubType: "missing",
			Message:           fmt.Sprintf("%s Path '%s' not found", request.Method, request.URL.Path),
			Reason: fmt.Sprintf("The %s request contains a path of '%s' "+
				"however that path, or the %s method for that path does not exist in the specification",
				request.Method, request.URL.Path, request.Method),
			SpecLine: -1,
			SpecCol:  -1,
			HowToFix: errors.HowToFixPath,
		},
	}
	errors.PopulateValidationErrors(validationErrors, request, "")
	return nil, validationErrors, ""
}

func getBasePaths(document *v3.Document) []string {
	// extract base path from document to check against paths.
	var basePaths []string
	for _, s := range document.Servers {
		u, err := url.Parse(s.URL)
		// if the host contains special characters, we should attempt to split and parse only the relative path
		if err != nil {
			// split at first occurrence
			_, serverPath, _ := strings.Cut(strings.Replace(s.URL, "//", "", 1), "/")

			if !strings.HasPrefix(serverPath, "/") {
				serverPath = "/" + serverPath
			}

			u, _ = url.Parse(serverPath)
		}

		if u != nil && u.Path != "" {
			basePaths = append(basePaths, u.Path)
		}
	}

	return basePaths
}

// StripRequestPath strips the base path from the request path, based on the server paths provided in the specification
func StripRequestPath(request *http.Request, document *v3.Document) string {
	basePaths := getBasePaths(document)

	// strip any base path
	stripped := stripBaseFromPath(request.URL.EscapedPath(), basePaths)
	if request.URL.Fragment != "" {
		stripped = fmt.Sprintf("%s#%s", stripped, request.URL.Fragment)
	}
	if len(stripped) > 0 && !strings.HasPrefix(stripped, "/") {
		stripped = "/" + stripped
	}
	return stripped
}

func checkPathAgainstBase(docPath, urlPath string, basePaths []string) bool {
	if docPath == urlPath {
		return true
	}
	for _, basePath := range basePaths {
		if basePath[len(basePath)-1] == '/' {
			basePath = basePath[:len(basePath)-1]
		}
		merged := fmt.Sprintf("%s%s", basePath, urlPath)
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

func comparePaths(mapped, requested, basePaths []string) bool {
	if len(mapped) != len(requested) {
		return false // short circuit out
	}
	var imploded []string
	for i, seg := range mapped {
		s := seg
		r, err := getRegexForPath(seg)
		if err != nil {
			return false
		}
		if r.MatchString(requested[i]) {
			s = requested[i]
		}
		imploded = append(imploded, s)
	}
	l := filepath.Join(imploded...)
	r := filepath.Join(requested...)
	return checkPathAgainstBase(l, r, basePaths)
}

func getRegexForPath(tpl string) (*regexp.Regexp, error) {

	// Check if it is well-formed.
	idxs, errBraces := braceIndices(tpl)
	if errBraces != nil {
		return nil, errBraces
	}

	// Backup the original.
	template := tpl

	// Now let's parse it.
	defaultPattern := "[^/]+"

	pattern := bytes.NewBufferString("^")
	var end int

	for i := 0; i < len(idxs); i += 2 {

		// Set all values we are interested in.
		raw := tpl[end:idxs[i]]
		end = idxs[i+1]
		parts := strings.SplitN(tpl[idxs[i]+1:end-1], ":", 2)
		name := parts[0]
		patt := defaultPattern
		if len(parts) == 2 {
			patt = parts[1]
		}

		// Name or pattern can't be empty.
		if name == "" || patt == "" {
			return nil, fmt.Errorf("mux: missing name or pattern in %q", tpl[idxs[i]:end])
		}

		// Build the regexp pattern.
		_, err := fmt.Fprintf(pattern, "%s(%s)", regexp.QuoteMeta(raw), patt)
		if err != nil {
			return nil, err
		}

	}

	// Add the remaining.
	raw := tpl[end:]
	pattern.WriteString(regexp.QuoteMeta(raw))

	// Compile full regexp.
	reg, errCompile := regexp.Compile(pattern.String())
	if errCompile != nil {
		return nil, errCompile
	}

	// Check for capturing groups which used to work in older versions
	if reg.NumSubexp() != len(idxs)/2 {
		return nil, fmt.Errorf(fmt.Sprintf("route %s contains capture groups in its regexp. ", template) + "Only non-capturing groups are accepted: e.g. (?:pattern) instead of (pattern)")
	}

	// Done!
	return reg, nil
}

func braceIndices(s string) ([]int, error) {
	var level, idx int
	var idxs []int
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			if level++; level == 1 {
				idx = i
			}
		case '}':
			if level--; level == 0 {
				idxs = append(idxs, idx, i+1)
			} else if level < 0 {
				return nil, fmt.Errorf("mux: unbalanced braces in %q", s)
			}
		}
	}
	if level != 0 {
		return nil, fmt.Errorf("mux: unbalanced braces in %q", s)
	}
	return idxs, nil
}
