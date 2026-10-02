// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"fmt"
	"regexp"

	"github.com/pb33f/go-yaml"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

var pathTemplateVarRegex = regexp.MustCompile(`\{([^}]+)\}`)

func validatePathParameters(model *v3.Document) []*liberrors.ValidationError {
	if model == nil || model.Paths == nil || model.Paths.PathItems == nil {
		return nil
	}

	keyNodes := pathKeyNodes(model)

	var validationErrors []*liberrors.ValidationError
	for pair := model.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		validationErrors = append(validationErrors,
			validatePathItemParameters(pair.Key(), pair.Value(), keyNodes[pair.Key()])...)
	}
	return validationErrors
}

func validatePathItemParameters(pathKey string, pathItem *v3.PathItem, keyNode *yaml.Node) []*liberrors.ValidationError {
	if pathItem == nil {
		return nil
	}

	operations := pathItem.GetOperations()
	hasOperations := operations != nil && operations.Len() > 0
	hasPathLevelParams := len(pathItem.Parameters) > 0

	if !hasOperations && !hasPathLevelParams {
		return nil
	}

	templateVars := extractTemplateVars(pathKey)

	var errs []*liberrors.ValidationError

	reportedExtra := make(map[string]bool)
	checkExtra := func(params []*v3.Parameter) {
		for _, p := range params {
			if p == nil || p.In != helpers.Path || p.Name == "" {
				continue
			}
			if !templateVars[p.Name] && !reportedExtra[p.Name] {
				reportedExtra[p.Name] = true
				errs = append(errs, unmatchedParameterError(pathKey, p, keyNode))
			}
		}
	}

	pathLevelParams := pathParamNames(pathItem.Parameters)
	checkExtra(pathItem.Parameters)

	if hasOperations {
		for op := operations.First(); op != nil; op = op.Next() {
			operation := op.Value()
			checkExtra(operation.Parameters)

			effective := pathParamNames(operation.Parameters)
			for name := range pathLevelParams {
				effective[name] = true
			}
			for name := range templateVars {
				if !effective[name] {
					errs = append(errs, missingParameterError(pathKey, op.Key(), name, keyNode))
				}
			}
		}
	} else {
		for name := range templateVars {
			if !pathLevelParams[name] {
				errs = append(errs, missingParameterError(pathKey, "", name, keyNode))
			}
		}
	}

	return errs
}

func extractTemplateVars(pathKey string) map[string]bool {
	vars := make(map[string]bool)
	for _, match := range pathTemplateVarRegex.FindAllStringSubmatch(pathKey, -1) {
		if len(match) > 1 && match[1] != "" {
			vars[match[1]] = true
		}
	}
	return vars
}

func pathParamNames(params []*v3.Parameter) map[string]bool {
	names := make(map[string]bool)
	for _, p := range params {
		if p != nil && p.In == helpers.Path && p.Name != "" {
			names[p.Name] = true
		}
	}
	return names
}

func pathKeyNodes(model *v3.Document) map[string]*yaml.Node {
	nodes := make(map[string]*yaml.Node)
	low := model.GoLow()
	if low == nil || low.Paths.Value == nil || low.Paths.Value.PathItems == nil {
		return nodes
	}
	for pair := low.Paths.Value.PathItems.First(); pair != nil; pair = pair.Next() {
		key := pair.Key()
		nodes[key.Value] = key.KeyNode
	}
	return nodes
}

func missingParameterError(pathKey, method, varName string, keyNode *yaml.Node) *liberrors.ValidationError {
	reason := fmt.Sprintf("Path %q declares template variable %q but no corresponding 'path' parameter is defined",
		pathKey, varName)
	if method != "" {
		reason = fmt.Sprintf("Path %q declares template variable %q but the %s operation defines no corresponding 'path' parameter",
			pathKey, varName, method)
	}
	return newPathParameterError(pathKey, reason,
		fmt.Sprintf("Add a parameter with name: %s, in: path, required: true to the path item or operation", varName),
		keyNode)
}

func unmatchedParameterError(pathKey string, param *v3.Parameter, keyNode *yaml.Node) *liberrors.ValidationError {
	node := keyNode
	if param != nil && param.GoLow() != nil && param.GoLow().GetKeyNode() != nil {
		node = param.GoLow().GetKeyNode()
	}
	return newPathParameterError(pathKey,
		fmt.Sprintf("The 'path' parameter %q defined in %q does not match any template variable in the path",
			param.Name, pathKey),
		fmt.Sprintf("Remove the %q parameter, or add a matching {%s} template variable to the path", param.Name, param.Name),
		node)
}

func newPathParameterError(pathKey, reason, howToFix string, node *yaml.Node) *liberrors.ValidationError {
	err := &liberrors.ValidationError{
		ValidationType:    helpers.Schema,
		ValidationSubType: helpers.DocumentValidation,
		Message:           "OpenAPI document validation failed",
		Reason:            reason,
		HowToFix:          howToFix,
		Context:           buildJSONPointer([]string{"paths", pathKey}),
	}
	if node != nil {
		err.SpecLine = node.Line
		err.SpecCol = node.Column
	}
	return err
}
