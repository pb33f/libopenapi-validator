// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"net/url"
	"strings"

	"github.com/pb33f/jsonpath/pkg/jsonpath"
	"github.com/pb33f/jsonpath/pkg/jsonpath/config"
	"github.com/pb33f/libopenapi/utils"
	"go.yaml.in/yaml/v4"
)

// LocateSchemaPropertyNodeByJSONPath will locate a schema property node by a JSONPath. It converts something like
// #/components/schemas/MySchema/properties/MyProperty to something like $.components.schemas.MySchema.properties.MyProperty
func LocateSchemaPropertyNodeByJSONPath(doc *yaml.Node, JSONPath string) *yaml.Node {
	JSONPath = normalizeKeywordLocation(JSONPath)
	_, path := utils.ConvertComponentIdIntoFriendlyPathSearch(JSONPath)
	return locateSchemaPropertyNode(doc, path)
}

// LocateSchemaPropertyNodeByJSONPathFallback locates a schema node using a primary and fallback keyword location.
func LocateSchemaPropertyNodeByJSONPathFallback(doc *yaml.Node, primaryLocation, fallbackLocation string) *yaml.Node {
	return LocateSchemaPropertyNodeByJSONPathWithResources(doc, nil, primaryLocation, fallbackLocation)
}

// LocateSchemaPropertyNodeByJSONPathWithResources locates a schema node, selecting external resource nodes when available.
func LocateSchemaPropertyNodeByJSONPathWithResources(
	doc *yaml.Node,
	resourceNodes map[string]*yaml.Node,
	primaryLocation, fallbackLocation string,
) *yaml.Node {
	located := locateSchemaPropertyNodeByKeywordLocation(doc, resourceNodes, primaryLocation)
	if located != nil || fallbackLocation == "" {
		return located
	}
	return locateSchemaPropertyNodeByKeywordLocation(doc, resourceNodes, fallbackLocation)
}

func normalizeKeywordLocation(location string) string {
	_, pointer := splitKeywordLocation(location)
	return pointer
}

func locateSchemaPropertyNodeByKeywordLocation(
	doc *yaml.Node,
	resourceNodes map[string]*yaml.Node,
	location string,
) *yaml.Node {
	resourceName, pointer := splitKeywordLocation(location)
	sourceNode := doc
	if resourceName != "" && resourceNodes != nil {
		if resourceNode := lookupResourceNode(resourceNodes, resourceName); resourceNode != nil {
			sourceNode = resourceNode
		}
	}
	return LocateSchemaPropertyNodeByJSONPath(rootContentNode(sourceNode), pointer)
}

func lookupResourceNode(resourceNodes map[string]*yaml.Node, resourceName string) *yaml.Node {
	if resourceNode := resourceNodes[resourceName]; resourceNode != nil {
		return resourceNode
	}
	if !strings.HasPrefix(resourceName, "file:") {
		return resourceNodes[canonicalResourceName(resourceName)]
	}

	parsedURL, err := url.Parse(resourceName)
	if err != nil || parsedURL.Scheme != "file" || parsedURL.Path == "" {
		return nil
	}

	if resourceNode := resourceNodes[parsedURL.String()]; resourceNode != nil {
		return resourceNode
	}
	filePath, err := url.PathUnescape(parsedURL.Path)
	if err != nil {
		filePath = parsedURL.Path
	}
	if resourceNode := resourceNodes[filePath]; resourceNode != nil {
		return resourceNode
	}
	return resourceNodes[canonicalResourceName(filePath)]
}

func splitKeywordLocation(location string) (string, string) {
	if location == "" || strings.HasPrefix(location, "#") || strings.HasPrefix(location, "/") {
		return "", location
	}

	hashIndex := strings.Index(location, "#")
	if hashIndex < 0 {
		return "", location
	}
	return location[:hashIndex], location[hashIndex:]
}

func rootContentNode(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
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
