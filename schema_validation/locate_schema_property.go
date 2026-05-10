// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"github.com/pb33f/jsonpath/pkg/jsonpath"
	"github.com/pb33f/jsonpath/pkg/jsonpath/config"
	"github.com/pb33f/libopenapi/utils"
	"go.yaml.in/yaml/v4"
)

// LocateSchemaPropertyNodeByJSONPath will locate a schema property node by a JSONPath. It converts something like
// #/components/schemas/MySchema/properties/MyProperty to something like $.components.schemas.MySchema.properties.MyProperty
func LocateSchemaPropertyNodeByJSONPath(doc *yaml.Node, JSONPath string) *yaml.Node {
	_, path := utils.ConvertComponentIdIntoFriendlyPathSearch(JSONPath)
	return locateSchemaPropertyNode(doc, path)
}

func locateSchemaPropertyNode(doc *yaml.Node, path string) *yaml.Node {
	if path == "" {
		return nil
	}
	var locatedNode *yaml.Node
	doneChan := make(chan bool)
	locatedNodeChan := make(chan *yaml.Node)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				doneChan <- true
			}
		}()
		jsonPath, _ := jsonpath.NewPath(path, config.WithLazyContextTracking())
		locatedNodes := jsonPath.Query(doc)
		if len(locatedNodes) > 0 {
			locatedNode = locatedNodes[0]
		}
		locatedNodeChan <- locatedNode
	}()
	select {
	case locatedNode = <-locatedNodeChan:
		return locatedNode
	case <-doneChan:
		return nil
	}
}
