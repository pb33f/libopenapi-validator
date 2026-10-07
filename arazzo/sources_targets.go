// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	upstream "github.com/pb33f/libopenapi/arazzo"
	"github.com/pb33f/libopenapi/arazzo/expression"
)

func (s *sourceSession) workflow(value, path string) *linkedTarget {
	name, id, ok := s.qualified(value)
	if !ok {
		return nil
	}
	source := s.lookup(name, path)
	if source == nil {
		return nil
	}
	if source.kind != "arazzo" {
		s.v.add(CodeSourceType, path, "workflow reference requires an Arazzo source")
		return nil
	}
	workflow := source.workflows[id]
	if workflow == nil && source.root == nil && source.adapter != nil {
		adapter, ok := source.adapter.(upstream.ArazzoSourceAdapter)
		if !ok {
			s.v.incomplete("workflow-target", path, name, "adapter does not expose workflow lookup")
			return nil
		}
		if adapter.HasWorkflow(id) {
			s.complete("workflow-target", path, source.identity)
			s.v.incomplete("workflow-metadata", path, name, "adapter exposes workflow presence only")
			return &linkedTarget{source: source, kind: "workflow"}
		}
	}
	if workflow == nil {
		s.v.add(CodeReference, path, fmt.Sprintf("workflow %q does not exist in source %q", id, name))
		return nil
	}
	s.complete("workflow-target", path, source.identity)
	return &linkedTarget{source: source, kind: "workflow", node: workflow}
}

func (s *sourceSession) step(step map[string]any, path string) *linkedTarget {
	if value := text(step["workflowId"]); strings.HasPrefix(value, "$sourceDescriptions.") {
		target := s.workflow(value, path+"/workflowId")
		if target != nil {
			s.graphSource(target.source, path+"/workflowId", 1)
			s.async(step, path, target)
		}
		return target
	}
	if value := text(step["operationId"]); value != "" {
		return s.operation(value, path+"/operationId", step, path)
	}
	for _, field := range []string{"operationPath", "channelPath"} {
		if value := text(step[field]); value != "" {
			return s.pointer(value, path+"/"+field, field, step, path)
		}
	}
	return nil
}

func (s *sourceSession) operation(value, path string, step map[string]any, stepPath string) *linkedTarget {
	name, id, qualified := s.qualified(value)
	if !qualified {
		names := []string{}
		for n, d := range s.byName {
			if text(d["type"]) != "arazzo" {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		if len(names) > 1 {
			s.v.add(CodeReference, path, "operationId must identify its source when multiple API sources are declared")
			return nil
		}
		if len(names) == 0 {
			s.v.add(CodeReference, path, "operationId requires an API source description")
			return nil
		}
		name = names[0]
		id = value
	}
	source := s.lookup(name, path)
	if source == nil {
		return nil
	}
	if source.kind != "openapi" && source.kind != "asyncapi" {
		s.v.add(CodeSourceType, path, "operation reference requires an OpenAPI or AsyncAPI source")
		return nil
	}
	targets := source.operations[id]
	if len(targets) > 1 {
		s.v.add(CodeReference, path, fmt.Sprintf("operationId %q is not unique in source %q", id, name))
		return nil
	}
	if len(targets) == 1 {
		s.complete("operation-target", path, source.identity)
		s.async(step, stepPath, targets[0])
		return targets[0]
	}
	if source.root == nil && source.adapter != nil {
		present := false
		capable := false
		switch adapter := source.adapter.(type) {
		case upstream.OpenAPISourceAdapter:
			capable = true
			present = adapter.HasOperationID(id)
		case upstream.AsyncAPISourceAdapter:
			capable = true
			present = adapter.HasOperation(id)
		}
		if !capable {
			s.v.incomplete("operation-presence", path, name, "adapter does not expose operation lookup")
			return nil
		}
		if present {
			s.complete("operation-target", path, source.identity)
			target := &linkedTarget{source: source, kind: source.kind}
			s.v.incomplete("operation-metadata", path, name, "adapter exposes operation presence only")
			s.async(step, stepPath, target)
			return target
		}
	}
	if source.unresolvedRefs {
		s.v.incomplete("operation-presence", path, name, "source contains unavailable external references")
		return nil
	}
	s.v.add(CodeReference, path, fmt.Sprintf("operation %q does not exist in source %q", id, name))
	return nil
}

func (s *sourceSession) pointer(value, path, field string, step map[string]any, stepPath string) *linkedTarget {
	prefix, fragment, ok := strings.Cut(value, "#")
	if !ok {
		s.v.add(CodeReference, path, "target requires a JSON Pointer fragment")
		return nil
	}
	if !strings.HasPrefix(prefix, "{") || !strings.HasSuffix(prefix, "}") {
		s.v.add(CodeReference, path, "target must use a source description runtime expression")
		return nil
	}
	name, member, ok := s.qualified(prefix[1 : len(prefix)-1])
	if !ok || member != "url" {
		s.v.add(CodeReference, path, "target must reference a source description url")
		return nil
	}
	decoded, err := url.PathUnescape(fragment)
	if err != nil {
		s.v.add(CodeReference, path, "invalid URI fragment encoding")
		return nil
	}
	tokens, ok := linkedPointerTokens(decoded)
	if !ok {
		s.v.add(CodeReference, path, "invalid JSON Pointer fragment")
		return nil
	}
	source := s.lookup(name, path)
	if source == nil {
		return nil
	}
	expected := "operation"
	if field == "channelPath" {
		expected = "channel"
	}
	if field == "channelPath" && source.kind != "asyncapi" || field == "operationPath" && source.kind != "openapi" && source.kind != "asyncapi" {
		s.v.add(CodeSourceType, path, "target source has the wrong document type")
		return nil
	}
	canonical := ""
	for _, token := range tokens {
		canonical = joinPtr(canonical, token)
	}
	if target := source.pointers[canonical]; target != nil && ((expected == "operation" && target.kind != "channel") || expected == "channel" && target.kind == "channel") {
		s.complete("pointer-target", path, source.identity)
		s.async(step, stepPath, target)
		return target
	}
	if source.root == nil && source.adapter != nil {
		// Validate the complete target shape before asking a presence-only adapter.
		shape := field == "operationPath" && source.kind == "openapi" && linkedOperationPointerShape(tokens) || source.kind == "asyncapi" && len(tokens) == 2 && (field == "channelPath" && tokens[0] == "channels" || field == "operationPath" && tokens[0] == "operations")
		present := false
		capable := false
		if shape {
			switch adapter := source.adapter.(type) {
			case upstream.OpenAPISourceAdapter:
				capable = true
				present = adapter.HasOperationPath("#" + canonical)
			case upstream.AsyncAPISourceAdapter:
				capable = true
				if expected == "channel" {
					present = adapter.HasChannel(tokens[1])
				} else {
					present = adapter.HasOperation(tokens[1])
				}
			}
		}
		if shape && !capable {
			s.v.incomplete("target-presence", path, name, "adapter does not expose target lookup")
			return nil
		}
		if present {
			s.complete("pointer-target", path, source.identity)
			target := &linkedTarget{source: source, kind: source.kind}
			s.v.incomplete("target-metadata", path, name, "adapter exposes target presence only")
			s.async(step, stepPath, target)
			return target
		}
	}
	if source.unresolvedRefs {
		s.v.incomplete("target-presence", path, name, "source contains unavailable external references")
		return nil
	}
	s.v.add(CodeReference, path, "JSON Pointer does not identify the required target object")
	return nil
}

func linkedPointerTokens(pointer string) ([]string, bool) {
	if pointer == "" {
		return nil, true
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	parts := strings.Split(pointer[1:], "/")
	for i, part := range parts {
		var b strings.Builder
		for j := 0; j < len(part); j++ {
			if part[j] != '~' {
				b.WriteByte(part[j])
				continue
			}
			j++
			if j == len(part) || part[j] != '0' && part[j] != '1' {
				return nil, false
			}
			if part[j] == '0' {
				b.WriteByte('~')
			} else {
				b.WriteByte('/')
			}
		}
		parts[i] = b.String()
	}
	return parts, true
}

func httpMethod(method string) bool {
	switch method {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace", "query":
		return true
	}
	return false
}

func linkedParameterIdentity(name, in string) linkedParameterKey {
	if in == "header" {
		name = strings.ToLower(name)
	}
	return linkedParameterKey{name, in}
}

func (s *sourceSession) parameters(inherited any, inheritedPath string, own any, ownPath string, target *linkedTarget) {
	if target.kind == "workflow" {
		if target.node == nil {
			s.v.incomplete("workflow-inputs", ownPath, target.source.identity, "target workflow input metadata is unavailable")
			return
		}
		defaults := s.v.resolveList(inherited, inheritedPath, "parameters")
		ownItems := s.v.resolveList(own, ownPath, "parameters")
		for _, parameter := range s.v.mergeEffective(defaults, ownItems, parameterIdentity, ownPath) {
			if !s.v.work(1, parameter.path) {
				return
			}
			s.complete("workflow-inputs", parameter.path, target.source.identity)
			if semanticInputAbsent(object(target.node["inputs"]), text(parameter.value["name"])) {
				s.v.add(CodeParameter, parameter.path+"/name", "parameter names an input excluded by the target workflow schema")
			}
		}
		return
	}
	if target.source.kind != "openapi" {
		if len(array(inherited))+len(array(own)) > 0 {
			s.v.incomplete("asyncapi-parameters", ownPath, target.source.identity, "AsyncAPI parameter and message value compatibility is not available")
		}
		return
	}
	effective := map[linkedParameterKey]struct {
		node map[string]any
		path string
	}{}
	for li, list := range []any{inherited, own} {
		base := inheritedPath
		if li == 1 {
			base = ownPath
		}
		for i, value := range array(list) {
			parameter := object(value)
			if ref := text(parameter["reference"]); ref != "" {
				if strings.HasPrefix(ref, "$components.parameters.") {
					parameter = object(object(object(s.v.root["components"])["parameters"])[strings.TrimPrefix(ref, "$components.parameters.")])
				} else {
					continue
				}
			}
			effective[linkedParameterIdentity(text(parameter["name"]), text(parameter["in"]))] = struct {
				node map[string]any
				path string
			}{parameter, fmt.Sprintf("%s/%d", base, i)}
		}
	}
	keys := make([]linkedParameterKey, 0, len(effective))
	for key := range effective {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].in == keys[j].in {
			return keys[i].name < keys[j].name
		}
		return keys[i].in < keys[j].in
	})
	for _, key := range keys {
		parameter := effective[key]
		if !s.v.work(1, parameter.path) {
			return
		}
		if key.name == "" || key.in == "" {
			continue
		}
		if target.parameters == nil || !target.parametersComplete {
			s.v.incomplete("operation-parameters", parameter.path, target.source.identity, "operation parameter metadata is incomplete")
			continue
		}
		s.complete("operation-parameters", parameter.path, target.source.identity)
		if !target.parameters[key] {
			s.v.add(CodeParameter, parameter.path+"/name", fmt.Sprintf("parameter %q in %s is not declared by the target operation", key.name, key.in))
		}
	}
}

func (s *sourceSession) actions(value any, path string) {
	for i, value := range array(value) {
		action := object(value)
		actionPath := fmt.Sprintf("%s/%d", path, i)
		if ref := text(action["reference"]); ref != "" {
			for _, collection := range []string{"successActions", "failureActions"} {
				prefix := "$components." + collection + "."
				if strings.HasPrefix(ref, prefix) {
					action = object(object(object(s.v.root["components"])[collection])[strings.TrimPrefix(ref, prefix)])
					break
				}
			}
		}
		if ref := text(action["workflowId"]); strings.HasPrefix(ref, "$sourceDescriptions.") {
			target := s.workflow(ref, actionPath+"/workflowId")
			if target != nil {
				s.parameters(nil, "", action["parameters"], actionPath+"/parameters", target)
			}
		}
	}
}

func (s *sourceSession) async(step map[string]any, path string, target *linkedTarget) {
	if target.source.kind != "asyncapi" {
		for _, field := range []string{"action", "correlationId"} {
			if _, present := step[field]; present {
				s.v.add(CodeSourceType, path+"/"+field, field+" applies only to AsyncAPI steps")
			}
		}
		return
	}
	action := text(step["action"])
	if action == "" {
		action = text(target.node["action"])
	}
	if text(step["correlationId"]) != "" && action != "" && action != "receive" {
		s.v.add(CodeSourceType, path+"/correlationId", "correlationId applies only to receive steps")
	}
	if target.node == nil {
		s.v.incomplete("asyncapi-metadata", path, target.source.identity, "adapter does not expose action, channel or message metadata")
		return
	}
	if expected := text(target.node["action"]); expected != "" && action != "" && expected != action {
		s.v.add(CodeReference, path+"/action", "step action differs from the referenced AsyncAPI operation")
	}
	channel := target.node
	if target.kind != "channel" {
		if value := target.source.deref(object(target.node["channel"])); value != nil {
			channel = value
		}
	}
	if action == "receive" && len(array(step["successCriteria"])) == 0 {
		messages := object(channel["messages"])
		if len(messages) > 1 {
			s.v.add(CodeReference, path, "receive step without successCriteria requires a single successful message type")
		} else {
			s.v.incomplete("asyncapi-success", path, target.source.identity, "message success semantics cannot be proved from static metadata")
		}
	}
	if text(step["correlationId"]) != "" {
		s.v.incomplete("asyncapi-correlation", path+"/correlationId", target.source.identity, "correlation value compatibility requires message data")
	}
}

func linkedOperationPointerShape(tokens []string) bool {
	if len(tokens) < 3 {
		return false
	}
	index := 2
	switch tokens[0] {
	case "paths", "webhooks":
	case "components":
		if len(tokens) < 4 {
			return false
		}
		switch tokens[1] {
		case "pathItems":
			index = 3
		case "callbacks":
			index = 4
		default:
			return false
		}
	default:
		return false
	}
	for index < len(tokens) {
		if tokens[index] == "additionalOperations" {
			index++
			if index >= len(tokens) {
				return false
			}
		} else if !httpMethod(tokens[index]) {
			return false
		}
		index++
		if index == len(tokens) {
			return true
		}
		if tokens[index] != "callbacks" || index+3 >= len(tokens) {
			return false
		}
		index += 3
	}
	return false
}

// requestExpressions checks the declared HTTP parameter boundary. Dynamic body
// members do not require static properties, but named request parameters do.
func (s *sourceSession) requestExpressions(uses []expressionUse) {
	for _, use := range uses {
		location := ""
		switch use.Expression.Type {
		case expression.RequestHeader:
			location = "header"
		case expression.RequestQuery:
			location = "query"
		case expression.RequestPath:
			location = "path"
		default:
			continue
		}
		workflow := s.v.local.workflows[use.WorkflowID]
		if workflow == nil {
			continue
		}
		steps := []string{use.StepID}
		if use.StepID == "" {
			// Inherited workflow parameters/actions are checked at every consuming step.
			if strings.Contains(use.Path, "/parameters/") || strings.Contains(use.Path, "/successActions/") || strings.Contains(use.Path, "/failureActions/") {
				steps = nil
				for _, step := range workflow.order {
					steps = append(steps, step.id)
				}
			} else {
				s.v.incomplete("request-parameter-context", use.Path, "", "request expression has no static parent operation")
				continue
			}
		}
		for _, stepID := range steps {
			step := workflow.steps[stepID]
			if step == nil {
				continue
			}
			key := dependencyID{document: s.v.documentID(), workflow: workflow.id, step: stepID}
			target, known := s.targets[key]
			if !known {
				target = s.step(step.value, step.path)
				s.targets[key] = target
			}
			if target == nil {
				continue
			}
			if target.source.kind != "openapi" || target.parameters == nil || !target.parametersComplete {
				s.v.incomplete("request-parameter-context", use.Path, target.source.identity, "source does not expose complete HTTP operation parameter metadata")
				continue
			}
			s.complete("request-parameter-context", use.Path, target.source.identity)
			if !target.parameters[linkedParameterIdentity(use.Expression.Property, location)] {
				s.v.add(CodeReference, use.Path, fmt.Sprintf("request %s parameter %q is not declared by the parent operation", location, use.Expression.Property))
			}
		}
	}
}
