// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	highArazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

func (source *linkedSource) indexRaw() {
	switch source.kind {
	case "arazzo":
		for _, value := range array(source.root["workflows"]) {
			workflow := object(value)
			if id := text(workflow["workflowId"]); id != "" {
				source.workflows[id] = workflow
			}
		}
		source.indexSteps()
	case "openapi":
		for _, collection := range []string{"paths", "webhooks"} {
			values := object(source.root[collection])
			for _, key := range semanticKeys(values) {
				value := values[key]
				source.indexPathItem(object(value), joinPtr("/"+collection, key), true, make(map[string]bool))
			}
		}
		items := object(object(source.root["components"])["pathItems"])
		for _, key := range semanticKeys(items) {
			value := items[key]
			source.indexPathItem(object(value), joinPtr("/components/pathItems", key), false, make(map[string]bool))
		}
		callbacks := object(object(source.root["components"])["callbacks"])
		for _, key := range semanticKeys(callbacks) {
			value := callbacks[key]
			callback := source.deref(object(value))
			for _, expression := range semanticKeys(callback) {
				item := callback[expression]
				if !strings.HasPrefix(expression, "x-") {
					source.indexPathItem(object(item), joinPtr(joinPtr("/components/callbacks", key), expression), false, make(map[string]bool))
				}
			}
		}

	case "asyncapi":
		channels := object(source.root["channels"])
		for _, name := range semanticKeys(channels) {
			value := channels[name]
			channel := source.deref(object(value))
			source.pointers[joinPtr("/channels", name)] = &linkedTarget{source: source, kind: "channel", node: channel}
			for _, action := range []string{"publish", "subscribe"} {
				operation := source.deref(object(channel[action]))
				if operation != nil {
					source.addOperation(operation, joinPtr(joinPtr("/channels", name), action), nil)
				}
			}
		}
		operations := object(source.root["operations"])
		for _, name := range semanticKeys(operations) {
			value := operations[name]
			operation := source.deref(object(value))
			target := source.addOperation(operation, joinPtr("/operations", name), nil)
			if text(operation["operationId"]) == "" && target != nil {
				source.operations[name] = append(source.operations[name], target)
			}
		}
	}
}

// indexSteps gives every source-qualified dependency and expression the same
// scoped lookup. Repeated references never rescan a workflow's step array.
func (source *linkedSource) indexSteps() {
	source.steps = make(map[string]map[string]map[string]any)
	for wi, raw := range array(source.root["workflows"]) {
		workflow := object(raw)
		items := array(workflow["steps"])
		path := source.owner.sourcePath(source, fmt.Sprintf("/workflows/%d/steps", wi))
		if !source.owner.v.work(len(items)+1, path) {
			return
		}
		steps := make(map[string]map[string]any, len(items))
		for _, raw := range items {
			if !source.owner.v.check() {
				return
			}
			step := object(raw)
			steps[text(step["stepId"])] = step
		}
		source.steps[text(workflow["workflowId"])] = steps
	}
}

// deref memoizes complete local-reference chains within this validation call.
// Every uncached hop consumes bounded work; external refs never trigger I/O.
func (source *linkedSource) deref(node map[string]any) map[string]any {
	if source.refMemo == nil {
		source.refMemo = make(map[string]map[string]any)
	}
	visited := map[string]bool{}
	chain := []string{}
	finish := func(result map[string]any) map[string]any {
		for _, ref := range chain {
			source.refMemo[ref] = result
		}
		return result
	}
	for node != nil {
		ref := text(node["$ref"])
		if ref == "" {
			return finish(node)
		}
		if source.owner != nil && !source.owner.v.work(1, source.owner.sourcePath(source, linkedRefLocation(ref))) {
			return nil
		}
		if cached, ok := source.refMemo[ref]; ok {
			return finish(cached)
		}
		if visited[ref] {
			return finish(nil)
		}
		visited[ref] = true
		chain = append(chain, ref)
		if !strings.HasPrefix(ref, "#") {
			source.unresolvedRefs = true
			return finish(nil)
		}
		tokens, ok := linkedPointerTokens(strings.TrimPrefix(ref, "#"))
		if !ok {
			return finish(nil)
		}
		var value any = source.root
		for _, token := range tokens {
			switch item := value.(type) {
			case map[string]any:
				value = item[token]
			case []any:
				index, err := strconv.Atoi(token)
				if err != nil || index < 0 || index >= len(item) {
					return finish(nil)
				}
				value = item[index]
			default:
				return finish(nil)
			}
		}
		node = object(value)
	}
	return finish(nil)
}

func (source *linkedSource) addOperation(operation map[string]any, pointer string, inherited any) *linkedTarget {
	return source.addOperationMetadata(operation, pointer, inherited, true)
}

func (source *linkedSource) addOperationMetadata(operation map[string]any, pointer string, inherited any, registerID bool) *linkedTarget {
	if operation == nil {
		return nil
	}
	if source.owner != nil && !source.owner.charge(1, len(pointer)) {
		return nil
	}
	target := &linkedTarget{source: source, kind: source.kind, node: operation, parameters: make(map[linkedParameterKey]bool), parametersComplete: true}
	for _, list := range []any{inherited, operation["parameters"]} {
		for _, value := range array(list) {
			if source.owner != nil && !source.owner.v.work(1, source.owner.sourcePath(source, pointer)) {
				return nil
			}
			parameter := source.deref(object(value))
			if parameter == nil {
				target.parametersComplete = false
				continue
			}
			target.parameters[linkedParameterIdentity(text(parameter["name"]), text(parameter["in"]))] = true
		}
	}
	source.pointers[pointer] = target
	if id := text(operation["operationId"]); registerID && id != "" {
		source.operations[id] = append(source.operations[id], target)
	}
	return target
}

func (source *linkedSource) indexOpenAPI(document *v3.Document, s *sourceSession) {
	if document.Paths == nil || document.Paths.PathItems == nil {
		return
	}
	for path, item := range document.Paths.PathItems.FromOldest() {
		if !s.charge(2, len(path)) {
			return
		}
		if item == nil {
			continue
		}
		operations := map[string]*v3.Operation{"get": item.Get, "put": item.Put, "post": item.Post, "delete": item.Delete, "options": item.Options, "head": item.Head, "patch": item.Patch, "trace": item.Trace, "query": item.Query}
		if item.AdditionalOperations != nil {
			if !s.charge(item.AdditionalOperations.Len(), 0) {
				return
			}
			for method, operation := range item.AdditionalOperations.FromOldest() {
				if !s.charge(1, len("additionalOperations/")+3*len(method)) {
					return
				}
				operations["additionalOperations/"+strings.ReplaceAll(strings.ReplaceAll(method, "~", "~0"), "/", "~1")] = operation
			}
		}
		operationNames := make([]string, 0, len(operations))
		for method := range operations {
			operationNames = append(operationNames, method)
		}
		sort.Strings(operationNames)
		for _, method := range operationNames {
			operation := operations[method]
			if operation == nil {
				continue
			}
			if !s.charge(2, len(method)+len(operation.OperationId)) {
				return
			}
			target := &linkedTarget{source: source, kind: "openapi", parameters: make(map[linkedParameterKey]bool), parametersComplete: true}
			for _, list := range [][]*v3.Parameter{item.Parameters, operation.Parameters} {
				for _, parameter := range list {
					if parameter == nil {
						continue
					}
					if !s.charge(2, len(parameter.Name)+len(parameter.In)) {
						return
					}
					if parameter.Reference != "" && parameter.Name == "" {
						target.parametersComplete = false
						continue
					}
					target.parameters[linkedParameterIdentity(parameter.Name, parameter.In)] = true
				}
			}
			source.pointers[joinPtr("/paths", path)+"/"+method] = target
			if operation.OperationId != "" {
				source.operations[operation.OperationId] = append(source.operations[operation.OperationId], target)
			}
		}
	}
}

func (s *sourceSession) linkedStrings(values []string) []any {
	totalBytes := 0
	for _, value := range values {
		totalBytes += len(value)
	}
	if !s.charge(len(values)+1, totalBytes) {
		return nil
	}
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func linkedOutputs(values *orderedmap.Map[string, *highArazzo.OutputValue], s *sourceSession) map[string]any {
	result := make(map[string]any)
	if values != nil {
		for name, value := range values.FromOldest() {
			if !s.charge(2, len(name)) {
				return nil
			}
			if value != nil {
				if expression, ok := value.GetExpression(); ok {
					if !s.charge(1, len(expression)) {
						return nil
					}
					result[name] = expression
				} else if selector, ok := value.GetSelector(); ok && selector != nil {
					if !s.charge(7, len(selector.Context)+len(selector.Selector)+len(selector.Type)) {
						return nil
					}
					node := map[string]any{"context": selector.Context, "selector": selector.Selector, "type": selector.Type}
					if selector.ExpressionType != nil {
						if !s.charge(5, len(selector.ExpressionType.Type)+len(selector.ExpressionType.Version)) {
							return nil
						}
						node["type"] = map[string]any{"type": selector.ExpressionType.Type, "version": selector.ExpressionType.Version}
					}
					result[name] = node
				}
			} else {
				result[name] = nil
			}
		}
	}
	return result
}

// indexPathItem includes callback operations, while component definitions supply
// pointer targets without inventing additional callable operation identities.
func (source *linkedSource) indexPathItem(raw map[string]any, path string, registerID bool, active map[string]bool) {
	if source.owner != nil && !source.owner.charge(1, len(path)) {
		return
	}
	ref := text(raw["$ref"])
	if ref != "" {
		if active[ref] {
			source.unresolvedRefs = true
			return
		}
		active[ref] = true
		defer delete(active, ref)
	}
	item := source.deref(raw)
	if item == nil {
		return
	}
	operations := map[string]map[string]any{}
	for _, method := range semanticKeys(item) {
		value := item[method]
		if httpMethod(method) {
			operations[method] = source.deref(object(value))
		}
	}
	additional := object(item["additionalOperations"])
	for _, method := range semanticKeys(additional) {
		value := additional[method]
		operations[joinPtr("additionalOperations", method)] = source.deref(object(value))
	}
	operationNames := make([]string, 0, len(operations))
	for method := range operations {
		operationNames = append(operationNames, method)
	}
	sort.Strings(operationNames)
	for _, method := range operationNames {
		operation := operations[method]
		opPath := path + "/" + method
		source.addOperationMetadata(operation, opPath, item["parameters"], registerID)
		callbacks := object(operation["callbacks"])
		for _, name := range semanticKeys(callbacks) {
			value := callbacks[name]
			rawCallback := object(value)
			ref := text(rawCallback["$ref"])
			if ref != "" && active[ref] {
				source.unresolvedRefs = true
				continue
			}
			if ref != "" {
				active[ref] = true
			}
			callback := source.deref(rawCallback)
			for _, expression := range semanticKeys(callback) {
				value := callback[expression]
				if !strings.HasPrefix(expression, "x-") {
					source.indexPathItem(object(value), joinPtr(joinPtr(opPath+"/callbacks", name), expression), registerID, active)
				}
			}
			if ref != "" {
				delete(active, ref)
			}
		}
	}
}

func linkedRefLocation(ref string) string {
	if strings.HasPrefix(ref, "#/") {
		return strings.TrimPrefix(ref, "#")
	}
	return ""
}
