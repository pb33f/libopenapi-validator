// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"github.com/pb33f/libopenapi/utils"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
)

// LocateSchemaPropertyNodeByJSONPath will locate a schema property node by a JSONPath. It converts something like
// #/components/schemas/MySchema/properties/MyProperty to something like $.components.schemas.MySchema.properties.MyProperty
func LocateSchemaPropertyNodeByJSONPath(doc *yaml.Node, JSONPath string) *yaml.Node {
	// first convert the path to something we can use as a lookup, remove the leading slash
	_, path := utils.ConvertComponentIdIntoFriendlyPathSearch(JSONPath)
	yamlPath, _ := yamlpath.NewPath(path)
	locatedNodes, _ := yamlPath.Find(doc)
	if len(locatedNodes) > 0 {
		return locatedNodes[0]
	}
	return nil
}
