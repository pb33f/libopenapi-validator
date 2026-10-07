// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package arazzo validates Arazzo 1.0 and 1.1 descriptions without executing
// workflows. Validation performs no retrieval unless a caller supplies a resolver.
package arazzo

import (
	"context"
	"fmt"

	"github.com/pb33f/go-yaml"
	source "github.com/pb33f/libopenapi/arazzo"
)

// Document is an immutable source tree and its retrieval URI. Root can be a
// YAML document node or its object node. URI supplies the base for relative URLs.
type Document struct {
	Root *yaml.Node
	URI  string
}

// Code is a stable diagnostic identifier, suitable for rule suppression.
type Code string

// Diagnostic codes distinguish structural, identity, reference and syntax checks.
const (
	CodeStructure   Code = "arazzo-structure"
	CodeDuplicateID Code = "arazzo-duplicate-id"
	CodeReference   Code = "arazzo-reference"
	CodeParameter   Code = "arazzo-parameter"
	CodeExpression  Code = "arazzo-expression"
	CodeSelector    Code = "arazzo-selector"
	CodeDependency  Code = "arazzo-dependency"
	CodeInputSchema Code = "arazzo-input-schema"
	CodeSourceType  Code = "arazzo-source-type"
	CodeAdvisory    Code = "arazzo-advisory"
)

// Severity indicates whether a diagnostic violates a requirement or gives advice.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Location contains detached source coordinates and an RFC 6901 instance pointer.
// Line and column are one-based, or zero when the caller's nodes have no position.
type Location struct {
	URI     string `json:"uri"`
	Pointer string `json:"pointer"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

// Diagnostic describes a document error without retaining source nodes or models.
type Diagnostic struct {
	Code     Code     `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Location
	SchemaPointer string     `json:"schemaPointer,omitempty"`
	Related       []Location `json:"related,omitempty"`
}

// CheckStatus records whether an applicable check ran.
type CheckStatus string

const (
	CheckComplete   CheckStatus = "complete"
	CheckIncomplete CheckStatus = "incomplete"
)

// Check identifies a completed or unavailable check at its affected source slot.
// Incomplete checks do not imply a malformed document.
type Check struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	URI     string      `json:"uri"`
	Pointer string      `json:"pointer,omitempty"`
	Source  string      `json:"source,omitempty"`
	Reason  string      `json:"reason,omitempty"`
}

// Result contains findings and coverage for the detected major.minor feature set.
// Complete is false if any applicable check was skipped or an operation failed.
type Result struct {
	Version     string       `json:"version"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Checks      []Check      `json:"checks"`
	Complete    bool         `json:"complete"`
}

// Valid reports whether validation found no conformance errors. Use Complete to
// distinguish fully checked documents from documents with unavailable checks.
func (r *Result) Valid() bool {
	if r == nil {
		return false
	}
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			return false
		}
	}
	return true
}

// ErrorKind distinguishes input, feature, configuration and operational failures.
type ErrorKind string

const (
	ErrorInput         ErrorKind = "input"
	ErrorUnsupported   ErrorKind = "unsupported-version"
	ErrorConfiguration ErrorKind = "configuration"
	ErrorLimit         ErrorKind = "resource-limit"
	ErrorOperational   ErrorKind = "operational"
)

// Error is a typed tool error. Invalid document fields normally produce diagnostics.
type Error struct {
	Kind     ErrorKind
	Location Location
	Cause    error
}

// Error returns a printable description of the tool failure.
func (e *Error) Error() string { return fmt.Sprintf("arazzo %s: %v", e.Kind, e.Cause) }

// Unwrap exposes the cause for errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// Limits bounds each validation call, including alias expansion and source loads.
// Every field must be positive when passed to WithLimits.
type Limits struct {
	MaxBytes        int
	MaxNodes        int
	MaxDepth        int
	MaxSources      int
	MaxDiagnostics  int
	MaxPatternBytes int
}

// DefaultLimits returns the default per-call limits.
func DefaultLimits() Limits {
	return Limits{
		MaxBytes:        16 << 20,
		MaxNodes:        200000,
		MaxDepth:        128,
		MaxSources:      50,
		MaxDiagnostics:  1000,
		MaxPatternBytes: 65536,
	}
}

// Option configures one validation call. Options do not modify shared state.
type (
	Option  func(*options) error
	options struct {
		limits   Limits
		sources  []source.CandidateDocument
		resolver source.SourceDocumentResolver
		baseURI  string
		advisory bool
	}
)

// WithLimits replaces all resource limits for this call.
func WithLimits(limits Limits) Option {
	return func(o *options) error {
		if limits.MaxBytes <= 0 || limits.MaxNodes <= 0 || limits.MaxDepth <= 0 || limits.MaxSources <= 0 || limits.MaxDiagnostics <= 0 || limits.MaxPatternBytes <= 0 {
			return fmt.Errorf("all limits must be positive")
		}
		o.limits = limits
		return nil
	}
}

// WithSources supplies application-owned candidate documents. They must remain
// immutable during validation. Their roots and models are never modified.
func WithSources(candidates ...source.CandidateDocument) Option {
	return func(o *options) error {
		o.sources = append(o.sources, candidates...)
		return nil
	}
}

// WithResolver enables explicit caller-owned retrieval. The resolver must honor
// context cancellation and enforce its own network and file access policy.
func WithResolver(resolver source.SourceDocumentResolver) Option {
	return func(o *options) error {
		if resolver == nil {
			return fmt.Errorf("resolver must not be nil")
		}
		o.resolver = resolver
		return nil
	}
}

// WithBaseURI provides an application base when the document URI is relative.
func WithBaseURI(uri string) Option { return func(o *options) error { o.baseURI = uri; return nil } }

// WithAdvisories enables optional authoring recommendations.
func WithAdvisories() Option { return func(o *options) error { o.advisory = true; return nil } }

type (
	nodeLocation struct{ key, value *yaml.Node }
	validation   struct {
		ctx                  context.Context
		doc                  Document
		opts                 options
		result               *Result
		root                 map[string]any
		nodes                map[string]nodeLocation
		version              string
		err                  error
		local                *semanticIndex
		graph                *dependencyGraph
		sources              *sourceSession
		expressionUses       []expressionUse
		foreignLocations     map[string]Location
		nodeCount, byteCount int
		budget               *workBudget
		identity             string
	}
)

type workBudget struct {
	used  int
	bytes int
	err   error
}
