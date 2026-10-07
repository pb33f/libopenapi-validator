// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/pb33f/go-yaml"
)

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func text(v any) string           { s, _ := v.(string); return s }
func joinPtr(path, token string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
}

type normalizer struct {
	ctx          context.Context
	doc          Document
	limits       Limits
	nodes        map[string]nodeLocation
	active       map[*yaml.Node]bool
	diagnostics  []Diagnostic
	count, bytes int
}

// normalizeWithStats converts the JSON-compatible node graph once, indexing each instance
// pointer. Alias visits count against the budget, so a small graph cannot expand
// into an unbounded JSON tree. Merge keys are expanded without editing any node.
type nodeStats struct{ nodes, bytes int }

func normalizeWithStats(ctx context.Context, doc Document, limits Limits) (any, map[string]nodeLocation, []Diagnostic, nodeStats, error) {
	n := normalizer{ctx: ctx, doc: doc, limits: limits, nodes: make(map[string]nodeLocation), active: make(map[*yaml.Node]bool)}
	value, err := n.value(doc.Root, "", 0)
	return value, n.nodes, n.diagnostics, nodeStats{n.count, n.bytes}, err
}

func (n *normalizer) issue(path, message string, node *yaml.Node) error {
	if len(n.diagnostics) >= n.limits.MaxDiagnostics {
		return &Error{Kind: ErrorLimit, Cause: fmt.Errorf("diagnostic limit exceeded")}
	}
	loc := Location{URI: n.doc.URI, Pointer: path}
	if node != nil {
		loc.Line, loc.Column = node.Line, node.Column
	}
	n.diagnostics = append(n.diagnostics, Diagnostic{Code: CodeStructure, Severity: SeverityError, Message: message, Location: loc})
	return nil
}

func (n *normalizer) value(node *yaml.Node, path string, depth int) (any, error) {
	if err := n.ctx.Err(); err != nil {
		return nil, err
	}
	if node == nil {
		return nil, &Error{Kind: ErrorInput, Cause: fmt.Errorf("source contains a nil node")}
	}
	n.count++
	n.bytes += len(node.Value)
	if depth > n.limits.MaxDepth || n.count > n.limits.MaxNodes || n.bytes > n.limits.MaxBytes {
		return nil, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("node, depth or expanded byte limit exceeded")}
	}
	if n.active[node] {
		return nil, n.issue(path, "recursive YAML alias or node graph is not a JSON value", node)
	}
	n.active[node] = true
	defer delete(n.active, node)
	loc := n.nodes[path]
	loc.value = node
	n.nodes[path] = loc
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return nil, &Error{Kind: ErrorInput, Cause: fmt.Errorf("expected one document root")}
		}
		return n.value(node.Content[0], path, depth)
	case yaml.AliasNode:
		value, err := n.value(node.Alias, path, depth+1)
		// Findings at the alias use point refer to the authored alias itself.
		loc.value = node
		n.nodes[path] = loc
		return value, err
	case yaml.MappingNode:
		if node.ShortTag() != "!!map" && node.Tag != "" {
			return nil, n.issue(path, "YAML mapping tag is not JSON-compatible", node)
		}
		return n.mapping(node, path, depth)
	case yaml.SequenceNode:
		if node.ShortTag() != "!!seq" && node.Tag != "" {
			return nil, n.issue(path, "YAML sequence tag is not JSON-compatible", node)
		}
		if len(node.Content) > n.limits.MaxNodes-n.count {
			return nil, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("node limit exceeded")}
		}
		items := make([]any, len(node.Content))
		for i, child := range node.Content {
			v, err := n.value(child, joinPtr(path, strconv.Itoa(i)), depth+1)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return items, nil
	case yaml.ScalarNode:
		switch node.ShortTag() {
		case "!!str":
			return node.Value, nil
		case "!!null":
			return nil, nil
		case "!!bool":
			if node.Value == "true" || node.Value == "True" || node.Value == "TRUE" {
				return true, nil
			}
			if node.Value == "false" || node.Value == "False" || node.Value == "FALSE" {
				return false, nil
			}
		case "!!int":
			clean := strings.ReplaceAll(node.Value, "_", "")
			base := 10
			unsigned := strings.TrimLeft(clean, "+-")
			if strings.HasPrefix(unsigned, "0x") || strings.HasPrefix(unsigned, "0o") || strings.HasPrefix(unsigned, "0b") {
				base = 0
			}
			if integer, ok := new(big.Int).SetString(clean, base); ok {
				return json.Number(integer.String()), nil
			}
		case "!!float":
			clean := strings.ReplaceAll(node.Value, "_", "")
			clean = strings.TrimPrefix(clean, "+")
			if strings.HasPrefix(clean, ".") {
				clean = "0" + clean
			}
			if strings.HasPrefix(clean, "-.") {
				clean = "-0" + clean[1:]
			}
			if strings.HasSuffix(clean, ".") {
				clean += "0"
			}
			// JSON's number grammar rejects NaN, infinity and YAML-only spellings.
			if json.Valid([]byte(clean)) && len(clean) > 0 && (clean[0] == '-' || clean[0] >= '0' && clean[0] <= '9') {
				// Schema integer and numeric checks use arbitrary-precision numbers.
				// Charge exponent expansion before those checks allocate 10^exponent.
				if i := strings.IndexAny(clean, "eE"); i >= 0 {
					exponent := strings.TrimLeft(clean[i+1:], "+-")
					growth, err := strconv.Atoi(exponent)
					if err != nil || growth > n.limits.MaxBytes-n.bytes {
						return nil, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("numeric exponent exceeds expanded byte limit")}
					}
					n.bytes += growth
				}
				return json.Number(clean), nil
			}
		}
		return nil, n.issue(path, "YAML scalar is not a JSON-compatible value", node)
	default:
		return nil, n.issue(path, "unsupported YAML node kind", node)
	}
}

func (n *normalizer) mapping(node *yaml.Node, path string, depth int) (any, error) {
	if len(node.Content)%2 != 0 {
		return nil, &Error{Kind: ErrorInput, Cause: fmt.Errorf("mapping has an unpaired key")}
	}
	if len(node.Content) > n.limits.MaxNodes-n.count {
		return nil, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("node limit exceeded")}
	}
	result := make(map[string]any, len(node.Content)/2)
	explicit := make(map[string]bool, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key, child := node.Content[i], node.Content[i+1]
		if key == nil {
			return nil, &Error{Kind: ErrorInput, Cause: fmt.Errorf("nil mapping key")}
		}
		n.count++
		n.bytes += len(key.Value)
		if n.count > n.limits.MaxNodes || n.bytes > n.limits.MaxBytes {
			return nil, &Error{Kind: ErrorLimit, Cause: fmt.Errorf("key node or byte limit exceeded")}
		}
		if key.ShortTag() == "!!merge" {
			continue
		}
		if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
			if err := n.issue(path, "mapping keys must be scalar strings", key); err != nil {
				return nil, err
			}
			continue
		}
		p := joinPtr(path, key.Value)
		if explicit[key.Value] {
			if err := n.issue(p, "duplicate mapping key", key); err != nil {
				return nil, err
			}
			continue
		}
		explicit[key.Value] = true
		n.nodes[p] = nodeLocation{key: key}
		v, err := n.value(child, p, depth+1)
		if err != nil {
			return nil, err
		}
		result[key.Value] = v
	}
	// Explicit fields take precedence. Earlier merge maps take precedence over
	// later maps, as defined by YAML merge semantics.
	mergeSeen := false
	for i := 0; i < len(node.Content); i += 2 {
		key, child := node.Content[i], node.Content[i+1]
		if key.ShortTag() != "!!merge" {
			continue
		}
		if mergeSeen {
			if err := n.issue(path, "duplicate merge key", key); err != nil {
				return nil, err
			}
			continue
		}
		mergeSeen = true
		v, err := n.value(child, joinPtr(path, "<<"), depth+1)
		if err != nil {
			return nil, err
		}
		maps := []any{v}
		if seq, ok := v.([]any); ok {
			maps = seq
		}
		for j, item := range maps {
			m, ok := item.(map[string]any)
			if !ok {
				if err := n.issue(path, "merge value must be a mapping or sequence of mappings", child); err != nil {
					return nil, err
				}
				continue
			}
			origin := joinPtr(path, "<<")
			if _, seq := v.([]any); seq {
				origin = joinPtr(origin, strconv.Itoa(j))
			}
			for name, value := range m {
				if _, exists := result[name]; exists {
					continue
				}
				result[name] = value
				n.copyLocations(value, joinPtr(origin, name), joinPtr(path, name))
			}
		}
	}
	return result, nil
}

func (n *normalizer) copyLocations(value any, from, to string) {
	// Visit this expanded subtree only. Scanning the whole document per merged
	// field would make repeated small merges quadratic.
	n.nodes[to] = n.nodes[from]
	if m, ok := value.(map[string]any); ok {
		for key, child := range m {
			n.copyLocations(child, joinPtr(from, key), joinPtr(to, key))
		}
	}
	if a, ok := value.([]any); ok {
		for i, child := range a {
			token := strconv.Itoa(i)
			n.copyLocations(child, joinPtr(from, token), joinPtr(to, token))
		}
	}
}
