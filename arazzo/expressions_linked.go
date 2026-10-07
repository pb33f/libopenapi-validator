// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"strings"

	"github.com/pb33f/libopenapi/arazzo/expression"
)

// finalizeLinkedExpressionUses checks supplied external workflow symbols. Source
// description fields resolve locally; unavailable linked metadata is coverage,
// never evidence that the authored expression names a nonexistent member.
func finalizeLinkedExpressionUses(v *validation) {
	if v.sources == nil {
		return
	}
	for _, use := range v.expressionUses {
		if !v.check() {
			return
		}
		if use.Expression.Type != expression.SourceDescriptions {
			continue
		}
		name, rest, ok := v.sources.qualified(use.Expression.Raw)
		if !ok {
			continue
		}
		description := v.sources.byName[name]
		if description == nil {
			continue
		} // local pass reports unknown names
		if _, field := description[rest]; field {
			continue
		}
		source := v.sources.lookup(name, use.Path)
		if source == nil {
			continue
		}
		if source.kind != "arazzo" {
			if len(source.operations[rest]) > 0 {
				continue
			}
			if source.root == nil {
				v.incomplete("expression-symbol", use.Path, name, "adapter does not expose source member metadata")
			} else if !source.unresolvedRefs && (v.version == "1.1" || !strings.Contains(rest, ".")) {
				v.add(CodeReference, use.Path, "runtime expression references an unknown source member")
			} else {
				v.incomplete("expression-symbol", use.Path, name, "source member property traversal is not statically available")
			}
			continue
		}
		id, tail := semanticReferenceParts(rest, func(id string) bool { _, ok := source.workflows[id]; return ok })
		if id == "" {
			// Presence-only adapters can identify a workflow before its member suffix.
			for _, field := range []string{".outputs.", ".inputs.", ".steps."} {
				if position := strings.Index(rest, field); position >= 0 {
					id, tail = rest[:position], rest[position+1:]
					break
				}
			}
			if id == "" {
				id = rest
			}
		}
		target := v.sources.workflow("$sourceDescriptions."+name+"."+id, use.Path)
		if target == nil {
			continue
		}
		if target.node == nil {
			v.incomplete("expression-symbol", use.Path, name, "adapter does not expose workflow input, step, or output declarations")
			continue
		}
		to := dependencyID{document: source.identity, workflow: id}
		output := false
		switch {
		case strings.HasPrefix(tail, "outputs."):
			if !semanticOutputExists(object(target.node["outputs"]), strings.TrimPrefix(tail, "outputs."), v.version) {
				v.add(CodeReference, use.Path, "runtime expression references an unknown external workflow output")
				continue
			}
			output = true
		case strings.HasPrefix(tail, "inputs."):
			if semanticInputAbsent(object(target.node["inputs"]), strings.TrimPrefix(tail, "inputs.")) {
				v.add(CodeReference, use.Path, "runtime expression references an input excluded by the external workflow schema")
				continue
			}
		case strings.HasPrefix(tail, "steps."):
			steps := source.steps[id]
			sid, member := semanticReferenceParts(strings.TrimPrefix(tail, "steps."), func(id string) bool { _, ok := steps[id]; return ok })
			if sid == "" {
				v.add(CodeReference, use.Path, "runtime expression references an unknown external workflow step")
				continue
			}
			if strings.HasPrefix(member, "outputs.") {
				if !semanticOutputExists(object(steps[sid]["outputs"]), strings.TrimPrefix(member, "outputs."), v.version) {
					v.add(CodeReference, use.Path, "runtime expression references an unknown external step output")
					continue
				}
				to.step = sid
				output = true
			}
		}
		if output && v.version == "1.1" && use.StepID != "" {
			v.sources.graphSource(source, use.Path, 1)
			v.graph.addEdge(dependencyID{document: v.documentID(), workflow: use.WorkflowID, step: use.StepID}, to, use.Path)
		}
	}
}
