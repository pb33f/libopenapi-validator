// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"github.com/pb33f/go-yaml"
	"github.com/pb33f/jsonschema/v6"
	"github.com/pb33f/jsonschema/v6/kind"

	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

// Index the original property-name errors by their typed kind. Flattened output
// retains these kinds, but its children use locations relative to the key string.
func documentPropertyNameErrors(root *jsonschema.ValidationError) map[*kind.PropertyNames]*jsonschema.ValidationError {
	var found map[*kind.PropertyNames]*jsonschema.ValidationError
	var visit func(*jsonschema.ValidationError)
	visit = func(err *jsonschema.ValidationError) {
		if name, ok := err.ErrorKind.(*kind.PropertyNames); ok {
			if found == nil {
				found = make(map[*kind.PropertyNames]*jsonschema.ValidationError)
			}
			found[name] = err
			return
		}
		for _, cause := range err.Causes {
			visit(cause)
		}
	}
	visit(root)
	return found
}

func documentPropertyNamePattern(err *jsonschema.ValidationError) string {
	if _, ok := err.ErrorKind.(*kind.ContentSchema); ok {
		return "" // Patterns in decoded content do not constrain the property name itself.
	}
	if pattern, ok := err.ErrorKind.(*kind.Pattern); ok {
		return pattern.Want
	}
	for _, cause := range err.Causes {
		if pattern := documentPropertyNamePattern(cause); pattern != "" {
			return pattern
		}
	}
	return ""
}

func applyDocumentPropertyNameError(root *yaml.Node, err *jsonschema.ValidationError, name string, failure *liberrors.SchemaValidationFailure) {
	failure.FieldName = name
	failure.InstancePath = appendPathSegment(err.InstanceLocation, name)
	failure.FieldPath = helpers.ExtractJSONPathFromInstanceLocation(failure.InstancePath)
	if pattern := documentPropertyNamePattern(err); pattern != "" {
		failure.Reason = buildEnhancedReason(name, pattern)
	} else {
		failure.Reason = err.Error()
	}
	parent := root
	if len(err.InstanceLocation) > 0 {
		parent = LocateSchemaPropertyNodeByJSONPath(root, buildJSONPointer(err.InstanceLocation))
	}
	if key := findMapKeyNode(parent, name); key != nil {
		failure.Line = key.Line
		failure.Column = key.Column
		failure.ReferenceObject = key.Value
	}
}
