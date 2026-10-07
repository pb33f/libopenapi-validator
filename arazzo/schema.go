// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pb33f/jsonschema/v6"
	"github.com/pb33f/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed schemas/*.json
var schemaFiles embed.FS

var officialSchemas = sync.OnceValues(func() (map[string]*jsonschema.Schema, error) {
	return compileOfficialSchemas(schemaFiles)
})

func compileOfficialSchemas(files fs.ReadFileFS) (map[string]*jsonschema.Schema, error) {
	compiled := make(map[string]*jsonschema.Schema, 2)
	for _, entry := range []struct{ version, file string }{
		{"1.0", "schemas/arazzo-1.0-2025-10-15.json"},
		{"1.1", "schemas/arazzo-1.1-2026-04-15.json"},
	} {
		data, err := files.ReadFile(entry.file)
		if err != nil {
			return nil, fmt.Errorf("read embedded schema %s: %w", entry.file, err)
		}
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode embedded schema %s: %w", entry.file, err)
		}
		compiler := jsonschema.NewCompiler()
		compiler.UseLoader(denyLoader{})
		compiler.UseRegexpEngine(compileSchemaPattern)
		compiler.AssertFormat()
		// Source-name conventions are SHOULD in both specifications, not MUST.
		// The semantic advice pass applies this pattern only when requested.
		delete(object(object(object(object(value)["$defs"])["source-description-object"])["properties"])["name"].(map[string]any), "pattern")
		if entry.version == "1.1" {
			defs := object(object(value)["$defs"])
			// Source names are advisory conventions. Keep the same decision for
			// qualified step prerequisites; the semantic pass resolves exact names.
			dependencies := object(object(object(defs["step-object-base"])["properties"])["dependsOn"])
			alternatives := array(object(dependencies["items"])["oneOf"])
			object(alternatives[2])["pattern"] = `^\$sourceDescriptions\.[\s\S]+\.[^.]+\.steps\.[^.]+$`
			// Parameter.value is Any in the normative field table (5.8.6.1).
			// The dated schema omits literal objects from this union. Correct only
			// this slot in the compilation view; preserve the embedded snapshot.
			object(object(defs["parameter-object"])["properties"])["value"] = true
			// Actions accept the same Parameter/Reusable union (5.8.7/5.8.8).
			// The dated schema's items:true otherwise accepts numbers and null.
			for _, name := range []string{"success-action-object", "failure-action-object"} {
				object(object(object(defs[name])["properties"])["parameters"])["items"] = map[string]any{"oneOf": []any{
					map[string]any{"$ref": "#/$defs/parameter-object"}, map[string]any{"$ref": "#/$defs/reusable-object"},
				}}
			}
		}
		id := text(object(value)["$id"])
		if err := compiler.AddResource(id, value); err != nil {
			return nil, fmt.Errorf("register embedded schema %s: %w", entry.file, err)
		}
		schema, err := compiler.Compile(id)
		if err != nil {
			return nil, fmt.Errorf("compile embedded schema %s: %w", entry.file, err)
		}
		compiled[entry.version] = schema
	}
	return compiled, nil
}

type externalResourceError struct{ URI string }

func (e *externalResourceError) Error() string {
	return "external schema resource is not supplied: " + e.URI
}

type denyLoader struct{}

func (denyLoader) Load(uri string) (any, error) { return nil, &externalResourceError{URI: uri} }

// The fixed lookahead has a linear implementation equivalent to the pinned
// regexp2 ECMAScript engine. The exhaustive boundary test includes every line
// separator, empty strings, dollar positions and non-ASCII characters.
type nonDollarPattern struct{}

func (nonDollarPattern) String() string { return `^(?!\$).+$` }
func (nonDollarPattern) MatchString(s string) bool {
	return s != "" && s[0] != '$' && !strings.ContainsAny(s, "\r\n")
}

func compileSchemaPattern(pattern string) (jsonschema.Regexp, error) {
	if len(pattern) > DefaultLimits().MaxPatternBytes {
		return nil, fmt.Errorf("pattern exceeds compile limit")
	}
	if pattern == `^(?!\$).+$` {
		return nonDollarPattern{}, nil
	}
	return regexp.Compile(pattern)
}

func checkStructure(v *validation) {
	invalidTargets := make(map[string]bool)
	for wi, raw := range array(v.root["workflows"]) {
		for si, raw := range array(object(raw)["steps"]) {
			count := 0
			for _, name := range []string{"operationId", "operationPath", "workflowId", "channelPath"} {
				if _, exists := object(raw)[name]; exists {
					count++
				}
			}
			if count != 1 {
				path := "/workflows/" + strconv.Itoa(wi) + "/steps/" + strconv.Itoa(si)
				invalidTargets[path] = true
				v.add(CodeStructure, path, "step must specify exactly one operation, workflow or channel target")
			}
		}
	}
	schemas, err := officialSchemas()
	if err != nil {
		v.err = &Error{Kind: ErrorOperational, Cause: err}
		return
	}
	err = schemas[v.version].Validate(v.root)
	if err == nil {
		return
	}
	// Schema.Validate returns only *jsonschema.ValidationError on failure.
	validationError := err.(*jsonschema.ValidationError)
	printer := message.NewPrinter(language.English)
	seen := make(map[struct{ path, message string }]bool)
	var collect func(*jsonschema.ValidationError)
	collect = func(failure *jsonschema.ValidationError) {
		if !v.check() {
			return
		}
		path := ""
		for _, token := range failure.InstanceLocation {
			path = joinPtr(path, token)
		}
		schemaPath := failure.SchemaURL
		for _, token := range failure.ErrorKind.KeywordPath() {
			schemaPath = joinPtr(schemaPath, token)
		}
		emit := func(path, msg string, key bool) {
			// Suppress only the target alternatives' fallout. Common step fields
			// still need diagnostics even when the step selects several targets.
			for parent := path; parent != ""; parent = parent[:strings.LastIndex(parent, "/")] {
				if !invalidTargets[parent] {
					continue
				}
				field, _, _ := strings.Cut(strings.TrimPrefix(path[len(parent):], "/"), "/")
				switch field {
				case "", "operationId", "operationPath", "workflowId", "channelPath", "action":
					return
				case "stepId", "description", "timeout", "dependsOn", "parameters", "requestBody", "successCriteria", "onSuccess", "onFailure", "outputs", "correlationId":
					// Parameter item shape depends on the selected target. The base
					// array contract still applies while that selection is invalid.
					if field == "parameters" && path != joinPtr(parent, field) {
						return
					}
					// Failed property validation leaves a known field unevaluated.
					// Keep its type/shape error, without also calling it unknown.
					if _, ok := failure.ErrorKind.(*kind.FalseSchema); ok && strings.Contains(failure.SchemaURL, "/unevaluatedProperties") && path == joinPtr(parent, field) {
						return
					}
				}
				break
			}
			identity := struct{ path, message string }{path, msg}
			if seen[identity] {
				return
			}
			seen[identity] = true
			before := len(v.result.Diagnostics)
			if key {
				v.addKey(CodeStructure, path, msg)
			} else {
				v.add(CodeStructure, path, msg)
			}
			if len(v.result.Diagnostics) > before {
				v.result.Diagnostics[before].SchemaPointer = schemaPath
			}
		}
		switch detail := failure.ErrorKind.(type) {
		case *kind.Required:
			for _, name := range detail.Missing {
				emit(joinPtr(path, name), "missing required property "+name, false)
			}
			return
		case *kind.AdditionalProperties:
			names := append([]string(nil), detail.Properties...)
			sort.Strings(names)
			for _, name := range names {
				emit(joinPtr(path, name), "property is not permitted", true)
			}
			return
		case *kind.PropertyNames:
			emit(joinPtr(path, detail.Property), detail.LocalizedString(printer), true)
			return
		}
		if len(failure.Causes) > 0 {
			children := append([]*jsonschema.ValidationError(nil), failure.Causes...)
			sort.Slice(children, func(i, j int) bool {
				a, b := children[i], children[j]
				pa, pb := strings.Join(a.InstanceLocation, "\x00"), strings.Join(b.InstanceLocation, "\x00")
				if pa != pb {
					return pa < pb
				}
				if a.SchemaURL != b.SchemaURL {
					return a.SchemaURL < b.SchemaURL
				}
				return a.ErrorKind.LocalizedString(printer) < b.ErrorKind.LocalizedString(printer)
			})
			for _, child := range children {
				collect(child)
			}
			return
		}
		switch failure.ErrorKind.(type) {
		case *kind.FalseSchema:
			if strings.Contains(failure.SchemaURL, "/unevaluatedProperties") {
				emit(path, "property is not permitted", true)
				return
			}
		}
		emit(path, failure.ErrorKind.LocalizedString(printer), false)
	}
	collect(validationError)
}
