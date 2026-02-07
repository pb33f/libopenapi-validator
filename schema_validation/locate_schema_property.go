// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"github.com/pb33f/jsonpath/pkg/jsonpath"
	"github.com/pb33f/libopenapi/utils"
	"go.yaml.in/yaml/v4"
)

// LocateSchemaPropertyNodeByJSONPath will locate a schema property node by a JSONPath. It converts something like
// #/components/schemas/MySchema/properties/MyProperty to something like $.components.schemas.MySchema.properties.MyProperty
func LocateSchemaPropertyNodeByJSONPath(doc *yaml.Node, JSONPath string) (result *yaml.Node) {
	defer func() { _ = recover() }()
	_, path := utils.ConvertComponentIdIntoFriendlyPathSearch(JSONPath)
	if path == "" {
		return nil
	}
	jp, _ := jsonpath.NewPath(path)
	nodes := jp.Query(doc)
	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}
