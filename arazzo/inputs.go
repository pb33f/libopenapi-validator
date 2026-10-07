// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/pb33f/jsonschema/v6"
)

type inputSchemaIndex struct{ resources, anchors map[string]string }

type inputReference struct{ uri, path string }

type inputSlot struct {
	value any
	path  string
}
type inputResourceUnavailable struct{ uri string }

func (e *inputResourceUnavailable) Error() string {
	return "input schema resource is not supplied: " + e.uri
}

type inputSchemaLoader struct{}

func (inputSchemaLoader) Load(uri string) (any, error) { return nil, &inputResourceUnavailable{uri} }

type inputRegexpState struct {
	v   *validation
	err error
}
type inputRegexp struct {
	re    *regexp2.Regexp
	state *inputRegexpState
}

func (r *inputRegexp) String() string { return r.re.String() }

func (r *inputRegexp) MatchString(s string) bool {
	if !r.state.v.check() {
		r.state.err = r.state.v.err
		return false
	}
	ok, err := r.re.MatchString(s)
	if err != nil {
		r.state.err = err
	}
	return ok
}

func (r *inputRegexpState) compile(pattern string) (jsonschema.Regexp, error) {
	if len(pattern) > r.v.opts.limits.MaxPatternBytes {
		r.v.err = &Error{Kind: ErrorLimit, Cause: fmt.Errorf("input schema pattern exceeds %d bytes", r.v.opts.limits.MaxPatternBytes)}
		return nil, r.v.err
	}
	re, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = 50 * time.Millisecond
	return &inputRegexp{re: re, state: r}, nil
}

func checkInputs(v *validation) {
	slots := inputSlots(v)
	if len(slots) == 0 {
		return
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(inputSchemaLoader{})
	regex := &inputRegexpState{v: v}
	compiler.UseRegexpEngine(regex.compile)
	base := inputSchemaBase(v)
	if err := compiler.AddResource(base, v.root); err != nil {
		v.err = &Error{Kind: ErrorOperational, Cause: err}
		return
	}
	// Register all schema resources and anchors before reporting any missing
	// reference. Compile first collects the targeted subschema, even when a
	// reference in its body cannot yet resolve. No custom vocabulary is used:
	// arbitrary annotations named workflows/components must remain annotations.
	for _, slot := range slots {
		if !v.check() {
			return
		}
		_, _ = compiler.Compile(base + "#" + url.PathEscape(slot.path))
		if regex.err != nil {
			v.err = &Error{Kind: ErrorOperational, Location: v.location(slot.path), Cause: regex.err}
			return
		}
	}
	// Resolve each authored reference separately. An unavailable external sibling
	// must not prevent diagnostics for a definite missing local target.
	var refs []inputReference
	index := inputSchemaIndex{resources: map[string]string{base: ""}, anchors: make(map[string]string)}
	for _, slot := range slots {
		collectInputReferences(slot.value, slot.path, base, &refs, index)
	}
	for _, ref := range refs {
		if !v.check() {
			return
		}
		_, err := compiler.Compile(inputReferenceLocation(ref.uri, base, index))
		if err != nil {
			reportInputCompile(v, regex, err, ref.path)
		}
	}
	for _, slot := range slots {
		if !v.check() {
			return
		}
		_, err := compiler.Compile(base + "#" + url.PathEscape(slot.path))
		if regex.err != nil {
			v.err = &Error{Kind: ErrorOperational, Location: v.location(slot.path), Cause: regex.err}
			return
		}
		if err == nil {
			continue
		}
		if v.err != nil {
			return
		}
		reportInputCompile(v, regex, err, schemaReferencePath(slot.value, slot.path))
	}
}

func inputSchemaBase(v *validation) string {
	base := v.doc.URI
	if v.opts.baseURI != "" {
		if b, err := url.Parse(v.opts.baseURI); err == nil {
			if u, err := url.Parse(base); err == nil {
				base = b.ResolveReference(u).String()
			}
		}
	}
	if v.version == "1.1" {
		if self := text(v.root["$self"]); self != "" {
			if b, err := url.Parse(base); err == nil {
				if s, err := url.Parse(self); err == nil {
					base = b.ResolveReference(s).String()
				}
			}
		}
	}
	u, err := url.Parse(base)
	if err != nil || !u.IsAbs() {
		return "https://arazzo.invalid/document"
	}
	u.Fragment = ""
	return u.String()
}

func schemaReferencePath(value any, path string) string {
	m := object(value)
	for _, keyword := range []string{"$ref", "$dynamicRef", "$id", "$anchor", "$dynamicAnchor", "$schema"} {
		if _, ok := m[keyword]; ok {
			return joinPtr(path, keyword)
		}
	}
	found := ""
	forEachSubschema(m, path, func(child any, childPath string) bool {
		if candidate := schemaReferencePath(child, childPath); candidate != childPath {
			found = candidate
			return false
		}
		return true
	})
	if found != "" {
		return found
	}
	return strings.TrimSuffix(path, "/")
}

func reportInputCompile(v *validation, regex *inputRegexpState, err error, path string) {
	if regex.err != nil {
		v.err = &Error{Kind: ErrorOperational, Location: v.location(path), Cause: regex.err}
		return
	}
	if v.err != nil {
		return
	}
	var unavailable *inputResourceUnavailable
	// LoadURLError deliberately has no Unwrap; inspect its public cause.
	var load *jsonschema.LoadURLError
	if errors.As(err, &load) {
		errors.As(load.Err, &unavailable)
	}
	if errors.As(err, &unavailable) || unavailable != nil {
		v.incomplete("input-schema-reference", path, unavailable.uri, "external JSON Schema resource is not supplied")
		return
	}
	v.add(CodeInputSchema, path, err.Error())
}

func collectInputReferences(value any, path, base string, out *[]inputReference, index inputSchemaIndex) {
	m := object(value)
	if id, ok := m["$id"].(string); ok {
		if b, err := url.Parse(base); err == nil {
			if u, err := url.Parse(id); err == nil {
				base = b.ResolveReference(u).String()
				index.resources[base] = path
			}
		}
	}
	for _, keyword := range []string{"$anchor", "$dynamicAnchor"} {
		if anchor, ok := m[keyword].(string); ok {
			index.anchors[base+"#"+anchor] = path
		}
	}
	for _, keyword := range []string{"$ref", "$dynamicRef"} {
		if ref, ok := m[keyword].(string); ok {
			if b, err := url.Parse(base); err == nil {
				if u, err := url.Parse(ref); err == nil {
					*out = append(*out, inputReference{b.ResolveReference(u).String(), joinPtr(path, keyword)})
				}
			}
		}
	}
	forEachSubschema(m, path, func(child any, childPath string) bool {
		collectInputReferences(child, childPath, base, out, index)
		return true
	})
}

// checkInputLimits runs before Arazzo structural validation invokes its embedded
// JSON Schema metaschema. Data annotations are deliberately outside this walk.
func checkInputLimits(v *validation) {
	for _, slot := range inputSlots(v) {
		walkInputSchemas(v, slot.value, slot.path, func(schema map[string]any, path string) {
			if pattern, ok := schema["pattern"].(string); ok && len(pattern) > v.opts.limits.MaxPatternBytes {
				v.err = &Error{Kind: ErrorLimit, Location: v.location(joinPtr(path, "pattern")), Cause: fmt.Errorf("input schema pattern exceeds %d bytes", v.opts.limits.MaxPatternBytes)}
				return
			}
			for _, key := range sortedKeys(object(schema["patternProperties"])) {
				if len(key) > v.opts.limits.MaxPatternBytes {
					v.err = &Error{Kind: ErrorLimit, Location: v.location(joinPtr(joinPtr(path, "patternProperties"), key)), Cause: fmt.Errorf("input schema property pattern exceeds %d bytes", v.opts.limits.MaxPatternBytes)}
					return
				}
			}
		})
		if v.err != nil {
			return
		}
	}
}

func inputSlots(v *validation) []inputSlot {
	var slots []inputSlot
	for i, raw := range array(v.root["workflows"]) {
		m := object(raw)
		if schema, ok := m["inputs"]; ok {
			slots = append(slots, inputSlot{schema, "/workflows/" + strconv.Itoa(i) + "/inputs"})
		}
	}
	for key, schema := range object(object(v.root["components"])["inputs"]) {
		slots = append(slots, inputSlot{schema, joinPtr("/components/inputs", key)})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].path < slots[j].path })
	return slots
}

func walkInputSchemas(v *validation, value any, path string, visit func(map[string]any, string)) {
	if !v.check() {
		return
	}
	m := object(value)
	if m == nil {
		return
	}
	visit(m, path)
	if v.err != nil {
		return
	}
	forEachSubschema(m, path, func(child any, childPath string) bool {
		walkInputSchemas(v, child, childPath, visit)
		return v.err == nil
	})
}

// forEachSubschema visits only schema-bearing keywords. Annotation data such as
// const, default and examples must not become schemas or source references.
func forEachSubschema(m map[string]any, path string, visit func(any, string) bool) {
	for _, keyword := range []string{"$defs", "properties", "patternProperties", "dependentSchemas"} {
		children := object(m[keyword])
		for _, key := range sortedKeys(children) {
			if !visit(children[key], joinPtr(joinPtr(path, keyword), key)) {
				return
			}
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		for i, child := range array(m[keyword]) {
			if !visit(child, joinPtr(joinPtr(path, keyword), strconv.Itoa(i))) {
				return
			}
		}
	}
	for _, keyword := range []string{"items", "contains", "not", "if", "then", "else", "additionalProperties", "unevaluatedProperties", "unevaluatedItems", "propertyNames", "contentSchema"} {
		if child, ok := m[keyword]; ok {
			if !visit(child, joinPtr(path, keyword)) {
				return
			}
		}
	}
}

func inputReferenceLocation(uri, base string, index inputSchemaIndex) string {
	if pointer, ok := index.anchors[uri]; ok {
		return base + "#" + url.PathEscape(pointer)
	}
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	fragment := u.Fragment
	u.Fragment = ""
	if pointer, ok := index.resources[u.String()]; ok && (fragment == "" || strings.HasPrefix(fragment, "/")) {
		return base + "#" + url.PathEscape(pointer+fragment)
	}
	return uri
}
