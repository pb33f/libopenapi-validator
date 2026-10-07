// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/pb33f/go-yaml"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.([0-9]+)(-.+)?$`)

func configure(ctx context.Context, opts []Option) (options, error) {
	o := options{limits: DefaultLimits()}
	if ctx == nil {
		return o, &Error{Kind: ErrorConfiguration, Cause: fmt.Errorf("context must not be nil")}
	}
	for _, opt := range opts {
		if opt == nil {
			return o, &Error{Kind: ErrorConfiguration, Cause: fmt.Errorf("option must not be nil")}
		}
		if err := opt(&o); err != nil {
			return o, &Error{Kind: ErrorConfiguration, Cause: err}
		}
	}
	if len(o.sources) > o.limits.MaxSources {
		return o, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("source limit exceeded")}
	}
	if _, err := url.Parse(o.baseURI); err != nil {
		return o, &Error{Kind: ErrorConfiguration, Cause: err}
	}
	return o, nil
}

// ValidateBytes parses exactly one JSON or YAML document and validates it through
// the same path as Validate. Parse failures and multiple documents are input errors.
// Every failure returns a non-nil incomplete result.
func ValidateBytes(ctx context.Context, data []byte, uri string, opts ...Option) (*Result, error) {
	o, err := configure(ctx, opts)
	if err != nil {
		return &Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return &Result{}, &Error{Kind: ErrorOperational, Location: Location{URI: uri}, Cause: err}
	}
	if len(data) > o.limits.MaxBytes {
		return &Result{}, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("input byte limit exceeded")}
	}
	var root, extra yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		return &Result{}, &Error{Kind: ErrorInput, Location: Location{URI: uri}, Cause: err}
	}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple YAML documents are not permitted")
		}
		return &Result{}, &Error{Kind: ErrorInput, Location: Location{URI: uri, Line: extra.Line, Column: extra.Column}, Cause: err}
	}
	// The YAML resolver falls back to string when a plain number overflows
	// float64. Keep numeric syntax and precision on this parser-owned tree.
	stack := []*yaml.Node{&root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.Kind == yaml.ScalarNode && n.Style == 0 && n.ShortTag() == "!!str" && len(n.Value) > 0 && (n.Value[0] == '-' || n.Value[0] >= '0' && n.Value[0] <= '9') && json.Valid([]byte(n.Value)) {
			n.Tag = "!!int"
			if strings.ContainsAny(n.Value, ".eE") {
				n.Tag = "!!float"
			}
		}
		stack = append(stack, n.Content...)
		if len(stack) > o.limits.MaxNodes {
			return &Result{}, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("parsed node limit exceeded")}
		}
	}
	return validate(ctx, Document{Root: &root, URI: uri}, o)
}

// Validate checks an existing immutable node tree. Document errors are findings;
// unsupported versions, cancellation, limits and resolver failures are tool errors.
// A partial result accompanies operational failure and always has Complete=false.
func Validate(ctx context.Context, doc Document, opts ...Option) (*Result, error) {
	o, err := configure(ctx, opts)
	if err != nil {
		return &Result{}, err
	}
	return validate(ctx, doc, o)
}

func validate(ctx context.Context, doc Document, opts options) (result *Result, err error) {
	result = &Result{}
	v := &validation{ctx: ctx, doc: doc, opts: opts, result: result, budget: &workBudget{}}
	defer func() {
		if v.err != nil {
			var toolError *Error
			if !errors.As(v.err, &toolError) {
				v.err = &Error{Kind: ErrorOperational, Location: v.location(""), Cause: v.err}
			}
			v.incomplete("operation", "", "", v.err.Error())
			err = v.err
		}
		v.finish()
	}()
	if !v.check() {
		return result, v.err
	}
	if doc.Root == nil {
		v.err = &Error{Kind: ErrorInput, Location: Location{URI: doc.URI}, Cause: fmt.Errorf("root must not be nil")}
		return result, v.err
	}
	if _, err := url.Parse(doc.URI); err != nil {
		v.err = &Error{Kind: ErrorConfiguration, Location: Location{URI: doc.URI}, Cause: err}
		return result, v.err
	}
	value, nodes, diags, stats, normalizeErr := normalizeWithStats(ctx, doc, opts.limits)
	v.nodes = nodes
	v.nodeCount, v.byteCount = stats.nodes, stats.bytes
	result.Diagnostics = diags
	if normalizeErr != nil {
		v.err = normalizeErr
		return result, v.err
	}
	v.root = object(value)
	if v.root == nil {
		v.add(CodeStructure, "", "Arazzo document must be an object")
		v.incomplete("semantics", "", "", "structural validation failed")
		return result, nil
	}
	authored, exists := v.root["arazzo"]
	v.version = "1.0"
	if version, ok := authored.(string); ok {
		parts := versionPattern.FindStringSubmatch(version)
		if parts == nil {
			v.err = &Error{Kind: ErrorInput, Location: v.location("/arazzo"), Cause: fmt.Errorf("malformed Arazzo version %q", version)}
			return result, v.err
		}
		v.version = parts[1] + "." + parts[2]
		result.Version = v.version
		if v.version != "1.0" && v.version != "1.1" {
			v.err = &Error{Kind: ErrorUnsupported, Location: v.location("/arazzo"), Cause: fmt.Errorf("unsupported Arazzo feature version %s", v.version)}
			return result, v.err
		}
	} else if exists {
		v.add(CodeStructure, "/arazzo", "arazzo must be a version string")
	}
	v.identity = resolveLinkedURI(v.doc.URI, v.opts.baseURI)
	if v.version == "1.1" && text(v.root["$self"]) != "" {
		v.identity = resolveLinkedURI(text(v.root["$self"]), v.identity)
	}
	if len(result.Diagnostics) == 0 {
		checkInputLimits(v)
	}
	if len(result.Diagnostics) == 0 {
		if v.check() {
			checkStructure(v)
		}
	}
	v.complete("structure", "")
	if !v.check() {
		return result, v.err
	}
	if !result.Valid() {
		v.incomplete("semantics", "", "", "structural validation failed")
		return result, nil
	}
	checkSemantics(v)
	if v.check() {
		checkExpressions(v)
	}
	if v.check() {
		checkInputs(v)
	}
	if v.check() {
		checkSources(v)
	}
	if v.check() {
		finalizeExpressionUses(v)
	}
	if v.check() {
		finalizeLinkedExpressionUses(v)
	}
	if v.check() {
		checkDependencyGraph(v)
	}
	if v.check() {
		v.complete("local-semantics", "")
	}
	if v.check() {
		checkAdvice(v)
	}
	return result, v.err
}

func (v *validation) location(path string) Location {
	if loc, ok := v.foreignLocations[path]; ok {
		return loc
	}
	loc := Location{URI: v.doc.URI, Pointer: path}
	for p := path; ; {
		if n := v.nodes[p].value; n != nil {
			loc.Line, loc.Column = n.Line, n.Column
			return loc
		}
		if p == "" {
			return loc
		}
		p = p[:strings.LastIndex(p, "/")]
	}
}

func (v *validation) add(code Code, path, msg string) {
	v.emit(Diagnostic{Code: code, Severity: SeverityError, Message: msg, Location: v.location(path)})
}

func (v *validation) emit(diagnostic Diagnostic) bool {
	if !v.check() {
		return false
	}
	if len(v.result.Diagnostics) >= v.opts.limits.MaxDiagnostics {
		v.err = &Error{Kind: ErrorLimit, Location: diagnostic.Location, Cause: fmt.Errorf("diagnostic limit exceeded")}
		return false
	}
	v.result.Diagnostics = append(v.result.Diagnostics, diagnostic)
	return true
}

func (v *validation) addKey(code Code, path, msg string) {
	v.add(code, path, msg)
	if v.err != nil {
		return
	}
	if key := v.nodes[path].key; key != nil {
		d := &v.result.Diagnostics[len(v.result.Diagnostics)-1]
		d.Line, d.Column = key.Line, key.Column
	}
}

func (v *validation) check() bool {
	if v.budget != nil && v.budget.err != nil {
		v.err = v.budget.err
	}
	if v.err != nil {
		return false
	}
	if err := v.ctx.Err(); err != nil {
		v.err = err
		return false
	}
	return true
}

func (v *validation) documentID() string {
	if v.identity != "" {
		return v.identity
	}
	return v.doc.URI
}

func (v *validation) work(count int, path string) bool {
	if !v.check() {
		return false
	}
	if v.budget == nil {
		v.budget = &workBudget{}
	}
	if count > v.opts.limits.MaxNodes-v.budget.used {
		v.err = &Error{Kind: ErrorLimit, Location: v.location(path), Cause: fmt.Errorf("derived validation work limit exceeded")}
		v.budget.err = v.err
		return false
	}
	v.budget.used += count
	return true
}

func (v *validation) workBytes(count int, path string) bool {
	if !v.check() {
		return false
	}
	if v.budget == nil {
		v.budget = &workBudget{}
	}
	if count > v.opts.limits.MaxBytes-v.budget.bytes {
		v.err = &Error{Kind: ErrorLimit, Location: v.location(path), Cause: fmt.Errorf("derived validation byte limit exceeded")}
		v.budget.err = v.err
		return false
	}
	v.budget.bytes += count
	return true
}

func (v *validation) incomplete(name, path, source, reason string) {
	loc := v.location(path)
	v.recordCheck(Check{Name: name, Status: CheckIncomplete, URI: loc.URI, Pointer: loc.Pointer, Source: source, Reason: reason})
}

func (v *validation) complete(name, path string) {
	loc := v.location(path)
	v.recordCheck(Check{Name: name, Status: CheckComplete, URI: loc.URI, Pointer: loc.Pointer})
}

func (v *validation) recordCheck(check Check) {
	if check.Name != "operation" && !v.work(1, check.Pointer) {
		return
	}
	v.result.Checks = append(v.result.Checks, check)
}

func (v *validation) finish() {
	sort.Slice(v.result.Diagnostics, func(i, j int) bool {
		a, b := v.result.Diagnostics[i], v.result.Diagnostics[j]
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.Pointer != b.Pointer {
			return a.Pointer < b.Pointer
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Message != b.Message {
			return a.Message < b.Message
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
	diags := v.result.Diagnostics[:0]
	for _, d := range v.result.Diagnostics {
		if len(diags) > 0 {
			prev := diags[len(diags)-1]
			if prev.URI == d.URI && prev.Pointer == d.Pointer && prev.Code == d.Code && prev.Message == d.Message {
				continue
			}
		}
		diags = append(diags, d)
	}
	v.result.Diagnostics = diags
	sort.Slice(v.result.Checks, func(i, j int) bool {
		a, b := v.result.Checks[i], v.result.Checks[j]
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.Pointer != b.Pointer {
			return a.Pointer < b.Pointer
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Reason < b.Reason
	})
	checks := v.result.Checks[:0]
	for _, c := range v.result.Checks {
		if len(checks) > 0 && checks[len(checks)-1] == c {
			continue
		}
		checks = append(checks, c)
	}
	v.result.Checks = checks
	v.result.Complete = v.err == nil
	for _, c := range checks {
		if c.Status != CheckComplete {
			v.result.Complete = false
		}
	}
}
