// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi/arazzo/expression"
)

// CodeAction identifies invalid contextual action targets or action fields.
const CodeAction Code = "arazzo-action"

// CodeForwardReference identifies optional guidance for sequential workflows.
const CodeForwardReference Code = "arazzo-forward-reference"

type semanticStep struct {
	id       string
	value    map[string]any
	path     string
	position int
}

type semanticWorkflow struct {
	id                  string
	value               map[string]any
	path                string
	steps               map[string]*semanticStep
	order               []*semanticStep
	hasStepDependencies bool
}

type semanticIndex struct {
	workflows  map[string]*semanticWorkflow
	order      []*semanticWorkflow
	components map[string]map[string]any
	sources    map[string]string
}

// effectiveObject retains the consuming slot independently from its component
// definition. Overrides copy only the object envelope, leaving values immutable.
type effectiveObject struct {
	value         map[string]any
	path          string
	definition    string
	authoredValue bool
}

func checkSemantics(v *validation) {
	v.local = &semanticIndex{workflows: make(map[string]*semanticWorkflow), components: make(map[string]map[string]any)}
	v.graph = newDependencyGraph()
	v.graph.run = v
	sources := make(map[string]string)
	v.local.sources = sources
	for i, value := range array(v.root["sourceDescriptions"]) {
		m := object(value)
		path := fmt.Sprintf("/sourceDescriptions/%d/name", i)
		id := text(m["name"])
		if previous, ok := sources[id]; ok {
			v.semanticDuplicate(CodeDuplicateID, path, "source description name is not unique", previous)
		} else {
			sources[id] = path
		}
	}
	for i, value := range array(v.root["workflows"]) {
		if !v.check() {
			return
		}
		m := object(value)
		w := &semanticWorkflow{id: text(m["workflowId"]), value: m, path: fmt.Sprintf("/workflows/%d", i), steps: make(map[string]*semanticStep)}
		if previous, ok := v.local.workflows[w.id]; ok {
			v.semanticDuplicate(CodeDuplicateID, w.path+"/workflowId", "workflowId is not unique", previous.path+"/workflowId")
		} else {
			v.local.workflows[w.id] = w
		}
		v.local.order = append(v.local.order, w)
		for j, value := range array(m["steps"]) {
			sm := object(value)
			s := &semanticStep{id: text(sm["stepId"]), value: sm, path: fmt.Sprintf("%s/steps/%d", w.path, j), position: j}
			if previous, ok := w.steps[s.id]; ok {
				v.semanticDuplicate(CodeDuplicateID, s.path+"/stepId", "stepId is not unique within its workflow", previous.path+"/stepId")
			} else {
				w.steps[s.id] = s
			}
			w.order = append(w.order, s)
			w.hasStepDependencies = w.hasStepDependencies || len(array(sm["dependsOn"])) != 0
		}
	}
	components := object(v.root["components"])
	for _, kind := range []string{"parameters", "successActions", "failureActions"} {
		v.local.components[kind] = object(components[kind])
	}
	// Every component body is checked, including definitions with no uses.
	for _, kind := range []string{"successActions", "failureActions"} {
		for _, key := range sortedKeys(v.local.components[kind]) {
			item := effectiveObject{value: object(v.local.components[kind][key]), path: joinPtr("/components/"+kind, key)}
			v.checkAction(item, nil)
		}
	}
	for _, w := range v.local.order {
		if !v.check() {
			return
		}
		wid := dependencyID{document: v.documentID(), workflow: w.id}
		v.graph.addNode(wid)
		start := dependencyID{document: v.documentID(), workflow: w.id, prerequisites: true}
		for _, s := range w.order {
			step := dependencyID{document: v.documentID(), workflow: w.id, step: s.id}
			v.graph.addEdge(wid, step, "")
			if len(array(w.value["dependsOn"])) > 0 {
				v.graph.addEdge(step, start, "")
			}
		}
		for i, raw := range array(w.value["dependsOn"]) {
			path := fmt.Sprintf("%s/dependsOn/%d", w.path, i)
			target := text(raw)
			if strings.HasPrefix(target, "$sourceDescriptions.") {
				continue
			}
			if v.checkLocalWorkflow(target, path) {
				to := dependencyID{document: v.documentID(), workflow: target}
				v.graph.addEdge(start, to, path)
			}
		}
		defaults := v.resolveList(w.value["parameters"], w.path+"/parameters", "parameters")
		success := v.resolveList(w.value["successActions"], w.path+"/successActions", "successActions")
		failure := v.resolveList(w.value["failureActions"], w.path+"/failureActions", "failureActions")
		v.uniqueParameters(defaults)
		v.uniqueActions(success)
		v.uniqueActions(failure)
		for _, group := range [][]effectiveObject{success, failure} {
			for _, action := range group {
				v.checkAction(action, w)
			}
		}
		for _, s := range w.order {
			if !v.check() {
				return
			}
			if target, ok := s.value["workflowId"]; ok {
				v.checkLocalWorkflow(text(target), s.path+"/workflowId")
			}
			params := v.resolveList(s.value["parameters"], s.path+"/parameters", "parameters")
			v.uniqueParameters(params)
			effective := v.mergeEffective(defaults, params, parameterIdentity, s.path+"/parameters")
			_, workflowCall := s.value["workflowId"]
			v.checkParameterContext(effective, workflowCall, false)
			if workflowCall {
				v.checkWorkflowInputParameters(effective, text(s.value["workflowId"]))
			}
			for _, p := range effective {
				// The expression pass owns authored inline values. Only inherited
				// or reusable values need an additional check in the consuming scope.
				if p.definition != "" && !p.authoredValue || !strings.HasPrefix(p.path, s.path+"/parameters/") {
					v.checkParameterExpressions(p.value, p.path, exprScope{workflowID: w.id, stepID: s.id})
				}
			}
			for _, group := range []struct {
				field, kind string
				defaults    []effectiveObject
			}{{"onSuccess", "successActions", success}, {"onFailure", "failureActions", failure}} {
				inline := v.resolveList(s.value[group.field], s.path+"/"+group.field, group.kind)
				v.uniqueActions(inline)
				for _, action := range v.mergeEffective(group.defaults, inline, actionIdentity, s.path+"/"+group.field) {
					v.checkAction(action, w)
					scope := exprScope{workflowID: w.id, stepID: s.id}
					inherited := action.definition != "" || !strings.HasPrefix(action.path, s.path+"/"+group.field+"/")
					if inherited {
						v.checkCriteria(action.value["criteria"], action.path+"/criteria", scope)
					}
					for _, p := range v.resolveList(action.value["parameters"], action.path+"/parameters", "parameters") {
						if inherited || p.definition != "" && !p.authoredValue {
							v.checkParameterExpressions(p.value, p.path, scope)
						}
					}
				}
			}
			if v.version == "1.1" {
				for i, raw := range array(s.value["dependsOn"]) {
					path := fmt.Sprintf("%s/dependsOn/%d", s.path, i)
					if target, ok := v.localStepDependency(w, text(raw), path); ok {
						v.graph.addEdge(dependencyID{document: v.documentID(), workflow: w.id, step: s.id}, target, path)
					}
				}
			}
		}
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (v *validation) checkLocalWorkflow(id, path string) bool {
	if strings.HasPrefix(id, "$sourceDescriptions.") {
		return false
	}
	if _, ok := v.local.workflows[id]; !ok {
		v.add(CodeReference, path, fmt.Sprintf("workflow %q does not exist in this document", id))
		return false
	}
	return true
}

// Reusable references select an exact collection and key. The remaining suffix
// is a key, rather than dot-separated path segments.
func (v *validation) resolveList(raw any, path, kind string) []effectiveObject {
	items := array(raw)
	if !v.work(len(items), path) {
		return nil
	}
	result := make([]effectiveObject, 0, len(items))
	for i, item := range items {
		if !v.check() {
			return result
		}
		p := fmt.Sprintf("%s/%d", path, i)
		m := object(item)
		if _, ok := m["reference"]; !ok {
			result = append(result, effectiveObject{value: m, path: p})
			continue
		}
		resolved, ok := v.resolveReusable(m, p, kind)
		if ok {
			result = append(result, resolved)
		}
	}
	return result
}

func (v *validation) resolveReusable(m map[string]any, path, kind string) (effectiveObject, bool) {
	_, authoredValue := m["value"]
	if !v.work(1, path+"/reference") {
		return effectiveObject{}, false
	}
	seen := make(map[string]bool)
	var overlays []map[string]any
	definition := ""
	for {
		if !v.check() {
			return effectiveObject{}, false
		}
		ref, present := m["reference"]
		if !present {
			break
		}
		if !v.work(1, path+"/reference") {
			return effectiveObject{}, false
		}
		name := text(ref)
		prefix := "$components." + kind + "."
		if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
			v.add(CodeReference, path+"/reference", "reusable reference must select components/"+kind)
			return effectiveObject{}, false
		}
		if seen[name] {
			v.add(CodeReference, path+"/reference", "reusable reference contains a cycle")
			return effectiveObject{}, false
		}
		seen[name] = true
		key := strings.TrimPrefix(name, prefix)
		next, exists := v.local.components[kind][key]
		if !exists {
			v.add(CodeReference, path+"/reference", fmt.Sprintf("component %q does not exist in %s", key, kind))
			return effectiveObject{}, false
		}
		overlays = append(overlays, m)
		m = object(next)
		definition = joinPtr("/components/"+kind, key)
	}
	if kind == "parameters" {
		if !v.work(len(m), path) {
			return effectiveObject{}, false
		}
		clone := make(map[string]any, len(m))
		for k, value := range m {
			clone[k] = value
		}
		for i := len(overlays) - 1; i >= 0; i-- {
			if value, ok := overlays[i]["value"]; ok {
				clone["value"] = value
			}
		}
		m = clone
	}
	return effectiveObject{value: m, path: path, definition: definition, authoredValue: authoredValue}, true
}

type parameterKey struct{ name, location string }

func parameterIdentity(item effectiveObject) any {
	return parameterKey{text(item.value["name"]), text(item.value["in"])}
}
func actionIdentity(item effectiveObject) any { return text(item.value["name"]) }

func (v *validation) mergeEffective(defaults, overrides []effectiveObject, key func(effectiveObject) any, path string) []effectiveObject {
	if !v.work(len(defaults), path) || !v.work(len(overrides), path) {
		return nil
	}
	count := len(defaults) + len(overrides)
	items := make([]effectiveObject, len(defaults), count)
	copy(items, defaults)
	positions := make(map[any]int, count)
	for i, item := range items {
		positions[key(item)] = i
	}
	for _, item := range overrides {
		id := key(item)
		if i, ok := positions[id]; ok {
			items[i] = item
		} else {
			positions[id] = len(items)
			items = append(items, item)
		}
	}
	return items
}

func (v *validation) uniqueParameters(items []effectiveObject) {
	seen := make(map[any]string, len(items))
	for _, item := range items {
		id := parameterIdentity(item)
		if previous, ok := seen[id]; ok {
			v.semanticDuplicate(CodeParameter, item.path, "parameter name and location are not unique", previous)
		}
		seen[id] = item.path
	}
}

func (v *validation) uniqueActions(items []effectiveObject) {
	seen := make(map[any]string, len(items))
	for _, item := range items {
		id := actionIdentity(item)
		if previous, ok := seen[id]; ok {
			v.semanticDuplicate(CodeDuplicateID, item.path, "effective action name is not unique", previous)
		}
		seen[id] = item.path
	}
}

func (v *validation) semanticDuplicate(code Code, path, message, previous string) {
	before := len(v.result.Diagnostics)
	v.add(code, path, message)
	if len(v.result.Diagnostics) > before {
		v.result.Diagnostics[len(v.result.Diagnostics)-1].Related = []Location{v.location(previous)}
	}
}

func (v *validation) checkParameterContext(items []effectiveObject, workflow, action bool) {
	query, querystring := false, false
	for _, item := range items {
		location, exists := item.value["in"]
		if action && exists {
			v.add(CodeParameter, item.path+"/in", "action parameters must not specify a location")
		}
		if !workflow && !action && !exists {
			v.add(CodeParameter, item.path, "operation parameters must specify in")
		}
		query = query || text(location) == "query"
		querystring = querystring || text(location) == "querystring"
		if !workflow && !action && text(location) == "querystring" {
			if _, ok := item.value["value"].(string); !ok && !selectorValue(object(item.value["value"])) {
				v.add(CodeParameter, item.path+"/value", "querystring parameter value must be a string or dynamic selector")
			}
		}
	}
	if !workflow && query && querystring {
		for _, item := range items {
			if text(item.value["in"]) == "querystring" {
				v.add(CodeParameter, item.path+"/in", "querystring and query parameters cannot coexist")
			}
		}
	}
}

func (v *validation) checkAction(item effectiveObject, w *semanticWorkflow) {
	if !v.work(1, item.path) {
		return
	}
	m := item.value
	_, workflowTarget := m["workflowId"]
	_, stepTarget := m["stepId"]
	if workflowTarget && stepTarget {
		v.add(CodeAction, item.path, "action workflowId and stepId are mutually exclusive")
	}
	if id, ok := m["workflowId"]; ok {
		v.checkLocalWorkflow(text(id), item.path+"/workflowId")
	}
	if id, ok := m["stepId"]; ok && w != nil {
		if _, exists := w.steps[text(id)]; !exists {
			v.add(CodeReference, item.path+"/stepId", fmt.Sprintf("step %q does not exist in the consuming workflow", text(id)))
		}
	}
	if _, ok := m["parameters"]; ok {
		if _, exists := m["workflowId"]; !exists {
			v.add(CodeAction, item.path+"/parameters", "action parameters require a workflowId target")
		}
		params := v.resolveList(m["parameters"], item.path+"/parameters", "parameters")
		v.checkWorkflowInputParameters(params, text(m["workflowId"]))
		v.uniqueParameters(params)
		v.checkParameterContext(params, true, true)
	}
}

func (v *validation) localStepDependency(w *semanticWorkflow, ref, path string) (dependencyID, bool) {
	if strings.HasPrefix(ref, "$sourceDescriptions.") {
		return dependencyID{}, false
	}
	target := w
	step := ref
	if strings.HasPrefix(ref, "$workflows.") {
		suffix := strings.TrimPrefix(ref, "$workflows.")
		var matches int
		for offset := 0; offset < len(suffix); {
			position := strings.Index(suffix[offset:], ".steps.")
			if position < 0 {
				break
			}
			position += offset
			candidate := v.local.workflows[suffix[:position]]
			id := suffix[position+len(".steps."):]
			if candidate != nil && candidate.steps[id] != nil {
				target = candidate
				step = id
				matches++
			}
			offset = position + len(".steps.")
		}
		if matches != 1 {
			v.add(CodeDependency, path, "step dependency must resolve to exactly one workflow step")
			return dependencyID{}, false
		}
	} else if strings.HasPrefix(ref, "$") {
		v.add(CodeDependency, path, "step dependency must use a stepId or a workflow step reference")
		return dependencyID{}, false
	}
	if _, ok := target.steps[step]; !ok {
		v.add(CodeDependency, path, fmt.Sprintf("prerequisite step %q does not exist", step))
		return dependencyID{}, false
	}
	return dependencyID{document: v.documentID(), workflow: target.id, step: step}, true
}

func checkDependencyGraph(v *validation) {
	if v.graph == nil || v.err != nil {
		return
	}
	if err := v.graph.check(v.ctx, func(path, message string) { v.add(CodeDependency, path, message) }); err != nil {
		v.err = err
	}
}

// finalizeExpressionUses resolves only statically declared symbols. Request,
// response and message properties remain runtime data. Open input schemas can
// provide undeclared properties, so they do not prove a missing input.
func finalizeExpressionUses(v *validation) {
	if v.local == nil {
		return
	}
	for _, use := range v.expressionUses {
		if !v.check() {
			return
		}
		e := use.Expression
		w := v.local.workflows[use.WorkflowID]
		switch e.Type {
		case expression.Steps:
			if w == nil {
				continue
			}
			id, tail := semanticReferenceParts(strings.TrimPrefix(e.Raw, "$steps."), func(id string) bool { return w.steps[id] != nil })
			s := w.steps[id]
			if s == nil {
				v.add(CodeReference, use.Path, "runtime expression references an unknown step")
				continue
			}
			if strings.HasPrefix(tail, "outputs.") {
				if !semanticOutputExists(object(s.value["outputs"]), strings.TrimPrefix(tail, "outputs."), v.version) {
					v.add(CodeReference, use.Path, "runtime expression references an unknown step output")
					continue
				}
				if v.version == "1.1" && use.StepID != "" && (use.StepID != s.id || semanticPreExecution(w, use)) {
					v.graph.addEdge(dependencyID{document: v.documentID(), workflow: w.id, step: use.StepID}, dependencyID{document: v.documentID(), workflow: w.id, step: s.id}, use.Path)
					if v.opts.advisory && semanticPreExecution(w, use) && s.position > w.steps[use.StepID].position && !w.hasStepDependencies {
						v.emit(Diagnostic{Code: CodeForwardReference, Severity: SeverityWarning, Message: "a sequential step references a later step's output; declare prerequisite dependencies or place the producing step first", Location: v.location(use.Path)})
					}
				}
			}
		case expression.Workflows:
			id, tail := semanticReferenceParts(strings.TrimPrefix(e.Raw, "$workflows."), func(id string) bool { return v.local.workflows[id] != nil })
			target := v.local.workflows[id]
			if target == nil {
				v.add(CodeReference, use.Path, "runtime expression references an unknown workflow")
				continue
			}
			if strings.HasPrefix(tail, "outputs.") {
				if !semanticOutputExists(object(target.value["outputs"]), strings.TrimPrefix(tail, "outputs."), v.version) {
					v.add(CodeReference, use.Path, "runtime expression references an unknown workflow output")
					continue
				}
				if v.version == "1.1" && use.StepID != "" {
					v.graph.addEdge(dependencyID{document: v.documentID(), workflow: use.WorkflowID, step: use.StepID}, dependencyID{document: v.documentID(), workflow: target.id}, use.Path)
				}
			} else if strings.HasPrefix(tail, "inputs.") && semanticInputAbsent(object(target.value["inputs"]), strings.TrimPrefix(tail, "inputs.")) {
				v.add(CodeReference, use.Path, "runtime expression references an input excluded by the workflow schema")
			}
		case expression.Outputs:
			if w != nil && !semanticOutputExists(object(w.value["outputs"]), strings.TrimPrefix(e.Raw, "$outputs."), v.version) {
				v.add(CodeReference, use.Path, "runtime expression references an unknown workflow output")
			}
		case expression.Inputs:
			if w != nil && semanticInputAbsent(object(w.value["inputs"]), e.Name) {
				v.add(CodeReference, use.Path, "runtime expression references an input excluded by the workflow schema")
			}
		case expression.ComponentParameters, expression.ComponentSuccessActions, expression.ComponentFailureActions:
			kind := "parameters"
			if e.Type == expression.ComponentSuccessActions {
				kind = "successActions"
			}
			if e.Type == expression.ComponentFailureActions {
				kind = "failureActions"
			}
			key := strings.TrimPrefix(e.Raw, "$components."+kind+".")
			if _, ok := v.local.components[kind][key]; !ok {
				v.add(CodeReference, use.Path, "runtime expression references an unknown component")
			}
		case expression.SourceDescriptions:
			// Source-qualified members are checked by the source pass. Bare
			// source references still have to name a declared source.
			id, _ := semanticReferenceParts(strings.TrimPrefix(e.Raw, "$sourceDescriptions."), func(id string) bool { _, ok := v.local.sources[id]; return ok })
			found := id != ""
			if !found {
				v.add(CodeReference, use.Path, "runtime expression references an unknown source description")
			}
		}
	}
}

func semanticReferenceParts(raw string, exists func(string) bool) (string, string) {
	for candidate := raw; candidate != ""; {
		if exists(candidate) {
			return candidate, strings.TrimPrefix(strings.TrimPrefix(raw, candidate), ".")
		}
		dot := strings.LastIndexByte(candidate, '.')
		if dot < 0 {
			break
		}
		candidate = candidate[:dot]
	}
	return "", ""
}

func semanticPreExecution(w *semanticWorkflow, use expressionUse) bool {
	s := w.steps[use.StepID]
	if s == nil {
		return false
	}
	for _, prefix := range []string{s.path + "/parameters/", s.path + "/requestBody/", s.path + "/correlationId", w.path + "/parameters/"} {
		if strings.HasPrefix(use.Path, prefix) {
			return true
		}
	}
	return false
}

func semanticOutputExists(outputs map[string]any, name, version string) bool {
	name, _, _ = strings.Cut(name, "#")
	if _, ok := outputs[name]; ok {
		return true
	}
	if version == "1.0" {
		id, _ := semanticReferenceParts(name, func(key string) bool { _, ok := outputs[key]; return ok })
		return id != ""
	}
	return false
}

func semanticInputAbsent(schema map[string]any, name string) bool {
	if schema == nil {
		return false
	}
	for _, key := range []string{"$ref", "$dynamicRef", "allOf", "anyOf", "oneOf", "if", "then", "else", "patternProperties", "unevaluatedProperties"} {
		if _, ok := schema[key]; ok {
			return false
		}
	}
	closed, ok := schema["additionalProperties"].(bool)
	if !ok || closed {
		return false
	}
	name, _, _ = strings.Cut(name, "#")
	if _, ok := object(schema["properties"])[name]; ok {
		return false
	}
	return true
}

func (v *validation) checkWorkflowInputParameters(parameters []effectiveObject, targetID string) {
	if target := v.local.workflows[targetID]; target != nil {
		for _, parameter := range parameters {
			if !v.work(1, parameter.path) {
				return
			}
			if semanticInputAbsent(object(target.value["inputs"]), text(parameter.value["name"])) {
				v.add(CodeParameter, parameter.path+"/name", "parameter is excluded by the target workflow input schema")
			}
		}
	}
}
