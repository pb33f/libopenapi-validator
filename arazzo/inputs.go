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
	v       *validation
	err     error
	trusted bool
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
	if !r.trusted && len(pattern) > r.v.opts.limits.MaxPatternBytes {
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
	regex := &inputRegexpState{v: v, trusted: true}
	compiler.UseRegexpEngine(regex.compile)
	meta, err := compiler.Compile("https://json-schema.org/draft/2020-12/schema")
	regex.trusted = false
	if err != nil {
		v.err = &Error{Kind: ErrorOperational, Cause: fmt.Errorf("compile input metaschema: %w", err)}
		return
	}
	valid := true
	for _, slot := range slots {
		if !v.check() {
			return
		}
		if err := meta.Validate(slot.value); err != nil {
			var ve *jsonschema.ValidationError
			if errors.As(err, &ve) {
				inputSchemaFindings(v, ve, slot.path)
			} else {
				v.err = &Error{Kind: ErrorOperational, Location: v.location(slot.path), Cause: err}
				return
			}
			valid = false
		}
		if regex.err != nil {
			v.err = &Error{Kind: ErrorOperational, Location: v.location(slot.path), Cause: regex.err}
			return
		}
	}
	if !valid {
		return
	}
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

func inputSchemaFindings(v *validation, e *jsonschema.ValidationError, base string) {
	if !v.check() {
		return
	}
	if len(e.Causes) > 0 {
		for _, cause := range e.Causes {
			inputSchemaFindings(v, cause, base)
		}
		return
	}
	path := base
	for _, token := range e.InstanceLocation {
		path = joinPtr(path, token)
	}
	v.add(CodeInputSchema, path, e.Error())
}

func schemaReferencePath(value any, path string) string {
	m := object(value)
	for _, keyword := range []string{"$ref", "$dynamicRef", "$id", "$anchor", "$dynamicAnchor", "$schema"} {
		if _, ok := m[keyword]; ok {
			return joinPtr(path, keyword)
		}
	}
	// Only schema-bearing keywords count as schema references. In particular,
	// objects inside const/default/examples are data, not subschema declarations.
	for _, keyword := range []string{"$defs", "properties", "patternProperties", "dependentSchemas"} {
		children := object(m[keyword])
		keys := make([]string, 0, len(children))
		for key := range children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := joinPtr(joinPtr(path, keyword), key)
			if candidate := schemaReferencePath(children[key], childPath); candidate != childPath {
				return candidate
			}
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		for i, child := range array(m[keyword]) {
			childPath := joinPtr(joinPtr(path, keyword), strconv.Itoa(i))
			if candidate := schemaReferencePath(child, childPath); candidate != childPath {
				return candidate
			}
		}
	}
	for _, keyword := range []string{"items", "contains", "not", "if", "then", "else", "additionalProperties", "unevaluatedProperties", "unevaluatedItems", "propertyNames", "contentSchema"} {
		if child, ok := m[keyword]; ok {
			childPath := joinPtr(path, keyword)
			if candidate := schemaReferencePath(child, childPath); candidate != childPath {
				return candidate
			}
		}
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
	var invalid *jsonschema.SchemaValidationError
	if errors.As(err, &invalid) {
		var ve *jsonschema.ValidationError
		if errors.As(invalid.Err, &ve) {
			inputSchemaFindings(v, ve, "")
			return
		}
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
	for _, keyword := range []string{"$defs", "properties", "patternProperties", "dependentSchemas"} {
		children := object(m[keyword])
		keys := make([]string, 0, len(children))
		for key := range children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			collectInputReferences(children[key], joinPtr(joinPtr(path, keyword), key), base, out, index)
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		for i, child := range array(m[keyword]) {
			collectInputReferences(child, joinPtr(joinPtr(path, keyword), strconv.Itoa(i)), base, out, index)
		}
	}
	for _, keyword := range []string{"items", "contains", "not", "if", "then", "else", "additionalProperties", "unevaluatedProperties", "unevaluatedItems", "propertyNames", "contentSchema"} {
		if child, ok := m[keyword]; ok {
			collectInputReferences(child, joinPtr(path, keyword), base, out, index)
		}
	}
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
			for _, key := range semanticKeys(object(schema["patternProperties"])) {
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
	for _, keyword := range []string{"$defs", "properties", "patternProperties", "dependentSchemas"} {
		children := object(m[keyword])
		keys := make([]string, 0, len(children))
		for key := range children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			walkInputSchemas(v, children[key], joinPtr(joinPtr(path, keyword), key), visit)
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		for i, child := range array(m[keyword]) {
			walkInputSchemas(v, child, joinPtr(joinPtr(path, keyword), strconv.Itoa(i)), visit)
		}
	}
	for _, keyword := range []string{"items", "contains", "not", "if", "then", "else", "additionalProperties", "unevaluatedProperties", "unevaluatedItems", "propertyNames", "contentSchema"} {
		if child, ok := m[keyword]; ok {
			walkInputSchemas(v, child, joinPtr(path, keyword), visit)
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
