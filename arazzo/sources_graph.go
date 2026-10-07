// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"fmt"
	"strings"
)

func (s *sourceSession) externalStep(value, path string) (dependencyID, bool) {
	name, rest, ok := s.qualified(value)
	if !ok {
		return dependencyID{}, false
	}
	wf, step, ok := strings.Cut(rest, ".steps.")
	if !ok || wf == "" || step == "" {
		s.v.add(CodeReference, path, "invalid external step dependency reference")
		return dependencyID{}, false
	}
	target := s.workflow("$sourceDescriptions."+name+"."+wf, path)
	if target == nil {
		return dependencyID{}, false
	}
	if target.node == nil {
		s.v.incomplete("step-dependency", path, name, "adapter does not expose workflow steps")
		return dependencyID{}, false
	}
	for _, entry := range array(target.node["steps"]) {
		if text(object(entry)["stepId"]) == step {
			s.graphSource(target.source, path, 1)
			return dependencyID{document: target.source.identity, workflow: wf, step: step}, true
		}
	}
	s.v.add(CodeReference, path, fmt.Sprintf("step %q does not exist in workflow %q", step, wf))
	return dependencyID{}, false
}

// graphSource adds prerequisites only. Merely importing another document is not
// an execution dependency and does not load that document's unused imports.
func (s *sourceSession) graphSource(source *linkedSource, path string, depth int) {
	if s.graphVisited[source] || source.kind != "arazzo" {
		return
	}
	s.graphVisited[source] = true
	if depth > s.v.opts.limits.MaxDepth {
		s.fail(ErrorLimit, "source dependency depth limit exceeded")
		return
	}
	if source.root == nil {
		s.v.incomplete("external-prerequisites", path, source.identity, "adapter does not expose workflow dependency metadata")
		return
	}
	child := *s
	child.base = source.base
	child.scopePath = path
	child.scopeSource = source
	child.descriptionPaths = make(map[string]string)
	child.byName = make(map[string]map[string]any)
	for i, value := range array(source.root["sourceDescriptions"]) {
		description := object(value)
		child.byName[text(description["name"])] = description
		child.descriptionPaths[text(description["name"])] = s.sourcePath(source, fmt.Sprintf("/sourceDescriptions/%d", i))
	}
	for wi, value := range array(source.root["workflows"]) {
		if !s.v.check() {
			return
		}
		workflow := object(value)
		wf := text(workflow["workflowId"])
		wid := dependencyID{document: source.identity, workflow: wf}
		s.v.graph.addNode(wid)
		steps := make(map[string]bool)
		for _, entry := range array(workflow["steps"]) {
			id := text(object(entry)["stepId"])
			steps[id] = true
			s.v.graph.addEdge(wid, dependencyID{document: source.identity, workflow: wf, step: id}, "")
			if len(array(workflow["dependsOn"])) > 0 {
				s.v.graph.addEdge(dependencyID{document: source.identity, workflow: wf, step: id}, dependencyID{document: source.identity, workflow: wf, prerequisites: true}, "")
			}
		}
		for di, raw := range array(workflow["dependsOn"]) {
			path := s.sourcePath(source, fmt.Sprintf("/workflows/%d/dependsOn/%d", wi, di))
			ref := text(raw)
			to := dependencyID{document: source.identity, workflow: ref}
			if strings.HasPrefix(ref, "$sourceDescriptions.") {
				target := child.workflow(ref, path)
				if target == nil {
					continue
				}
				_, id, _ := child.qualified(ref)
				to = dependencyID{document: target.source.identity, workflow: id}
				child.graphSource(target.source, path, depth+1)
			} else if source.workflows[ref] == nil {
				s.v.add(CodeReference, path, fmt.Sprintf("external document %q references missing workflow %q", source.identity, ref))
				continue
			}
			s.v.graph.addEdge(dependencyID{document: source.identity, workflow: wf, prerequisites: true}, to, path)
		}
		for si, entry := range array(workflow["steps"]) {
			step := object(entry)
			from := dependencyID{document: source.identity, workflow: wf, step: text(step["stepId"])}
			for di, raw := range array(step["dependsOn"]) {
				path := s.sourcePath(source, fmt.Sprintf("/workflows/%d/steps/%d/dependsOn/%d", wi, si, di))
				ref := text(raw)
				to := dependencyID{document: source.identity, workflow: wf, step: ref}
				if strings.HasPrefix(ref, "$sourceDescriptions.") {
					name, rest, ok := child.qualified(ref)
					other, otherStep, valid := strings.Cut(rest, ".steps.")
					if !ok || !valid {
						s.v.add(CodeReference, path, "invalid external step dependency reference")
						continue
					}
					target := child.workflow("$sourceDescriptions."+name+"."+other, path)
					if target == nil {
						continue
					}
					found := false
					for _, candidate := range array(target.node["steps"]) {
						if text(object(candidate)["stepId"]) == otherStep {
							found = true
							break
						}
					}
					if !found {
						if target.node == nil {
							s.v.incomplete("external-prerequisites", path, target.source.identity, "workflow step metadata unavailable")
						} else {
							s.v.add(CodeReference, path, "external step prerequisite does not exist")
						}
						continue
					}
					to = dependencyID{document: target.source.identity, workflow: other, step: otherStep}
					child.graphSource(target.source, path, depth+1)
				} else if strings.HasPrefix(ref, "$workflows.") {
					rest := strings.TrimPrefix(ref, "$workflows.")
					other, otherStep, ok := strings.Cut(rest, ".steps.")
					if !ok {
						s.v.add(CodeReference, path, "invalid cross-workflow step dependency")
						continue
					}
					found := false
					for _, candidate := range array(source.workflows[other]["steps"]) {
						if text(object(candidate)["stepId"]) == otherStep {
							found = true
							break
						}
					}
					if !found {
						s.v.add(CodeReference, path, "external cross-workflow step prerequisite does not exist")
						continue
					}
					to = dependencyID{document: source.identity, workflow: other, step: otherStep}
				} else if !steps[ref] {
					s.v.add(CodeReference, path, fmt.Sprintf("external step prerequisite %q does not exist", ref))
					continue
				}
				s.v.graph.addEdge(from, to, path)
			}
		}
	}
	child.graphExpressions(source)
	s.complete("external-prerequisites", path, source.identity)
	s.count = child.count
	s.bytes = child.bytes
	s.nodes = child.nodes
}

// graphExpressions reuses the expression traversal and symbol checks on supplied
// documents. It creates local indexes but shares the prerequisite graph and
// bounded source cache; it does not execute workflows or rebuild source models.
func (s *sourceSession) graphExpressions(source *linkedSource) {
	version := strings.Split(text(source.root["arazzo"]), ".")
	if len(version) < 2 {
		return
	}
	feature := version[0] + "." + version[1]
	if feature != "1.0" && feature != "1.1" {
		s.v.incomplete("external-expression-grammar", "", source.identity, "source has an unsupported expression feature version")
		return
	}
	local := &semanticIndex{workflows: make(map[string]*semanticWorkflow), components: make(map[string]map[string]any), sources: make(map[string]string)}
	for name, description := range s.byName {
		local.sources[name] = text(description["url"])
	}
	for _, collection := range []string{"parameters", "successActions", "failureActions"} {
		local.components[collection] = object(object(source.root["components"])[collection])
	}
	for wi, raw := range array(source.root["workflows"]) {
		value := object(raw)
		workflow := &semanticWorkflow{id: text(value["workflowId"]), value: value, path: s.sourcePath(source, fmt.Sprintf("/workflows/%d", wi)), steps: make(map[string]*semanticStep)}
		for si, raw := range array(value["steps"]) {
			value := object(raw)
			step := &semanticStep{id: text(value["stepId"]), value: value, path: s.sourcePath(source, fmt.Sprintf("/workflows/%d/steps/%d", wi, si)), position: si}
			workflow.steps[step.id] = step
			workflow.order = append(workflow.order, step)
			if len(array(value["dependsOn"])) > 0 {
				workflow.hasStepDependencies = true
			}
		}
		local.workflows[workflow.id] = workflow
		local.order = append(local.order, workflow)
	}
	locations := make(map[string]Location, len(source.nodes))
	for pointer := range source.nodes {
		key := s.sourcePath(source, pointer)
		locations[pointer] = s.v.foreignLocations[key]
	}
	uri := source.retrieval
	if uri == "" {
		uri = source.identity
	}
	nested := &validation{identity: source.identity, budget: s.v.budget, ctx: s.v.ctx, doc: Document{URI: uri}, opts: s.v.opts, result: s.v.result, root: source.root, nodes: source.nodes, version: feature, local: local, graph: s.v.graph, foreignLocations: locations}
	child := *s
	child.v = nested
	nested.sources = &child
	checkExpressions(nested)
	for i := range nested.expressionUses {
		nested.expressionUses[i].Path = s.sourcePath(source, nested.expressionUses[i].Path)
	}
	nested.foreignLocations = s.v.foreignLocations
	if nested.err == nil {
		child.requestExpressions(nested.expressionUses)
		finalizeExpressionUses(nested)
	}
	if nested.err != nil {
		s.v.err = nested.err
	}
	s.count, s.bytes, s.nodes = child.count, child.bytes, child.nodes
}
