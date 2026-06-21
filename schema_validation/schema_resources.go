// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/utils"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
)

const syntheticSchemaResourceBase = "https://libopenapi-validator.local/schema/"

// renderSchemaWithRefs keeps fallback rendering testable without depending on libopenapi internals failing.
var renderSchemaWithRefs = func(schema *base.Schema) ([]byte, error) {
	return schema.Render()
}

// CompiledValidationSchema contains a schema compiled for validation plus the rendered schema context.
type CompiledValidationSchema struct {
	RenderedInline  []byte
	ReferenceSchema string
	RenderedJSON    []byte
	RenderedNode    *yaml.Node
	ResourceNodes   map[string]*yaml.Node
	CompiledSchema  *jsonschema.Schema
}

// schemaDocumentResourceSet is the in-memory set of JSON Schema resources passed to jsonschema.
type schemaDocumentResourceSet struct {
	resources     map[string][]byte
	resourceNodes map[string]*yaml.Node
	entryName     string
	entryNode     *yaml.Node
}

// schemaResourceBuildState tracks resource identity and recursion guards while walking reachable refs.
type schemaResourceBuildState struct {
	resourceNames map[*index.SpecIndex]string
	seenRefs      map[string]struct{}
	seenNodes     map[*yaml.Node]struct{}
}

// CompileSchemaForValidation compiles a schema for validation while preserving reachable references.
func CompileSchemaForValidation(
	schema *base.Schema,
	purpose SchemaValidationPurpose,
	options *config.ValidationOptions,
	version float32,
) (*CompiledValidationSchema, error) {
	if schema == nil {
		return nil, nil
	}

	rendered, err := renderRootSchemaForValidation(schema, purpose)
	if err != nil {
		return nil, err
	}

	if singleSchemaCompilePreferred(schema, rendered) {
		return compileSingleValidationSchema(schema, rendered, options, version)
	}

	resourceSet, err := buildSchemaDocumentResources(schema, purpose)
	if err != nil {
		return nil, err
	}

	if resourceSet == nil || len(resourceSet.resources) == 0 || resourceSet.entryName == "" {
		return compileSingleValidationSchema(schema, rendered, options, version)
	}

	compiled, compileErr := helpers.NewCompiledSchemaResourcesWithVersion(
		resourceSet.entryName,
		resourceSet.resources,
		options,
		version,
	)
	if compileErr != nil {
		return nil, compileErr
	}

	return &CompiledValidationSchema{
		RenderedInline:  rendered.RenderedInline,
		ReferenceSchema: rendered.ReferenceSchema,
		RenderedJSON:    rendered.RenderedJSON,
		RenderedNode:    resourceSet.entryNode,
		ResourceNodes:   resourceSet.resourceNodes,
		CompiledSchema:  compiled,
	}, nil
}

// ToCacheEntry converts a compiled validation schema into the shared schema cache shape.
func (c *CompiledValidationSchema) ToCacheEntry(schema *base.Schema) *cache.SchemaCacheEntry {
	if c == nil {
		return nil
	}
	return &cache.SchemaCacheEntry{
		Schema:          schema,
		RenderedInline:  c.RenderedInline,
		ReferenceSchema: c.ReferenceSchema,
		RenderedJSON:    c.RenderedJSON,
		CompiledSchema:  c.CompiledSchema,
		RenderedNode:    c.RenderedNode,
		ResourceNodes:   c.ResourceNodes,
	}
}

// compileSingleValidationSchema compiles one rendered schema document without building a resource graph.
//
// This keeps the no-ref path close to the legacy behavior and avoids the extra
// resource rendering work needed only when reachable refs must stay addressable.
func compileSingleValidationSchema(
	schema *base.Schema,
	rendered *RenderedValidationSchema,
	options *config.ValidationOptions,
	version float32,
) (*CompiledValidationSchema, error) {
	schemaName := "schema"
	if schema != nil && schema.GoLow() != nil {
		schemaName = fmt.Sprintf("%x", schema.GoLow().Hash())
	}
	compiled, compileErr := helpers.NewCompiledSchemaWithVersion(
		schemaName,
		rendered.RenderedJSON,
		options,
		version,
	)
	if compileErr != nil {
		return nil, compileErr
	}
	return &CompiledValidationSchema{
		RenderedInline:  rendered.RenderedInline,
		ReferenceSchema: rendered.ReferenceSchema,
		RenderedJSON:    rendered.RenderedJSON,
		RenderedNode:    rendered.RenderedNode,
		ResourceNodes:   sourceNodesForRenderedSchema(schemaName, rendered.RenderedNode),
		CompiledSchema:  compiled,
	}, nil
}

// sourceNodesForRenderedSchema builds the source-node lookup for the single-resource compiler path.
//
// The same rendered node is indexed by both the empty resource name and the
// compiler resource name because jsonschema diagnostics can identify the entry
// schema either way depending on the reported location shape.
func sourceNodesForRenderedSchema(resourceName string, renderedNode *yaml.Node) map[string]*yaml.Node {
	if renderedNode == nil {
		return nil
	}

	resourceNodes := map[string]*yaml.Node{
		"": renderedNode,
	}
	if resourceName != "" {
		resourceNodes[resourceName] = renderedNode
	}
	return resourceNodes
}

// renderRootSchemaForValidation renders the entry schema in the best form available for validation.
//
// The normal renderer is preferred because it applies request/response pruning
// consistently. If inline rendering cannot resolve a circular graph, this falls
// back to rendering the schema node with refs intact so the resource compiler can
// preserve those refs.
func renderRootSchemaForValidation(schema *base.Schema, purpose SchemaValidationPurpose) (*RenderedValidationSchema, error) {
	if schema == nil {
		return nil, nil
	}
	if schema.GoLow() == nil || schema.GoLow().GetRootNode() == nil {
		return nil, fmt.Errorf("schema does not have low-level information and cannot be rendered")
	}

	rendered, err := RenderSchemaForValidation(schema, purpose)
	if err == nil {
		return rendered, nil
	}

	renderedInline, err := renderSchemaWithRefs(schema)
	if err != nil {
		return nil, err
	}

	return renderSchemaBytesForValidation(renderedInline, purpose)
}

// singleSchemaCompilePreferred reports whether the schema can safely use the one-resource compiler path.
//
// The decision is based on reachable schema-keyword refs in the source tree, not
// a rendered-output scan, so examples, property names, enum values, or const
// values that literally contain "$ref" do not force the slower resource graph.
func singleSchemaCompilePreferred(schema *base.Schema, rendered *RenderedValidationSchema) bool {
	return rendered != nil && !schemaHasReachableRefs(schema)
}

// schemaHasReachableRefs checks whether the entry schema tree contains refs that jsonschema must resolve.
func schemaHasReachableRefs(schema *base.Schema) bool {
	if schema == nil || schema.GoLow() == nil {
		return false
	}
	return len(collectSchemaRefValues(schema.GoLow().GetRootNode())) > 0
}

// renderYAMLNodeForValidation renders a YAML node to the YAML and JSON forms used by jsonschema.
//
// Request and response validation prune directional "required" markers, so they
// clone before mutating. Generic validation does not prune, which keeps the
// whole-document resource path allocation-light for ordinary document schemas.
func renderYAMLNodeForValidation(node *yaml.Node, purpose SchemaValidationPurpose) (*RenderedValidationSchema, error) {
	if node == nil {
		return nil, nil
	}

	renderedNode := node
	if purpose != SchemaValidationPurposeGeneric {
		renderedNode = cloneYAMLNode(node)
		pruneDirectionalRequiredEverywhere(renderedNode, purpose)
	}

	renderedInline, err := yaml.Marshal(renderedNode)
	if err != nil {
		return nil, fmt.Errorf("schema render encode failed: %w", err)
	}

	renderedJSON, err := utils.ConvertYAMLtoJSON(renderedInline)
	if err != nil {
		return nil, fmt.Errorf("schema render JSON conversion failed: %w", err)
	}

	return &RenderedValidationSchema{
		RenderedInline:  renderedInline,
		ReferenceSchema: string(renderedInline),
		RenderedJSON:    renderedJSON,
		RenderedNode:    renderedNode,
	}, nil
}

// buildSchemaDocumentResources builds the resource graph needed for a schema with reachable refs.
//
// The root OpenAPI document is registered as a resource, then only documents
// reachable from schema-keyword refs are added. The entry name points at the
// specific schema node inside the root resource, not necessarily the document root.
func buildSchemaDocumentResources(
	schema *base.Schema,
	purpose SchemaValidationPurpose,
) (*schemaDocumentResourceSet, error) {
	if schema == nil || schema.GoLow() == nil {
		return nil, nil
	}

	schemaIndex := schema.GoLow().GetIndex()
	if schemaIndex == nil || schemaIndex.GetRootNode() == nil {
		return nil, nil
	}
	if !schemaHasReachableRefs(schema) {
		return nil, nil
	}

	schemaPointer, ok := jsonPointerForNode(schemaIndex.GetRootNode(), schema.GoLow().GetRootNode())
	if !ok {
		return nil, fmt.Errorf("schema node was not found in its root document")
	}

	state := &schemaResourceBuildState{
		resourceNames: make(map[*index.SpecIndex]string),
		seenRefs:      make(map[string]struct{}),
		seenNodes:     make(map[*yaml.Node]struct{}),
	}
	rootResourceName := resourceNameForIndex(schemaIndex, schema.GoLow().Hash())
	state.resourceNames[schemaIndex] = rootResourceName
	resourceSet := &schemaDocumentResourceSet{
		resources:     make(map[string][]byte),
		resourceNodes: make(map[string]*yaml.Node),
	}
	rootRendered, err := addSchemaDocumentResource(
		resourceSet.resources,
		resourceSet.resourceNodes,
		rootResourceName,
		schemaIndex.GetRootNode(),
		purpose,
	)
	if err != nil {
		return nil, err
	}

	if err := addReachableSchemaResources(
		resourceSet,
		state,
		schemaIndex,
		schema.GoLow().GetRootNode(),
		purpose,
	); err != nil {
		return nil, err
	}

	resourceSet.entryName = rootResourceName + "#" + schemaPointer
	if rootRendered != nil {
		resourceSet.entryNode = rootRendered.RenderedNode
	}
	return resourceSet, nil
}

// addReachableSchemaResources recursively adds resources referenced by schema-keyword refs.
//
// seenNodes prevents circular schema graphs from recursing forever, while
// seenRefs prevents repeatedly visiting the same resolved definition through
// different local paths.
func addReachableSchemaResources(
	resourceSet *schemaDocumentResourceSet,
	state *schemaResourceBuildState,
	currentIndex *index.SpecIndex,
	node *yaml.Node,
	purpose SchemaValidationPurpose,
) error {
	if resourceSet == nil || state == nil || currentIndex == nil || node == nil {
		return nil
	}
	if _, seen := state.seenNodes[node]; seen {
		return nil
	}
	state.seenNodes[node] = struct{}{}

	for _, refValue := range collectSchemaRefValues(node) {
		foundRef, foundIndex := currentIndex.SearchIndexForReference(refValue)
		if foundRef == nil {
			continue
		}
		foundIndex = schemaResourceIndex(foundRef, foundIndex)
		if foundIndex != nil && foundIndex.GetRootNode() != nil {
			resourceName := ensureSchemaResourceName(state, foundIndex, uint64(len(resourceSet.resources)+1))
			refKey := resourceName + "|" + foundRef.FullDefinition
			if _, seen := state.seenRefs[refKey]; seen {
				continue
			}
			state.seenRefs[refKey] = struct{}{}

			if _, exists := resourceSet.resources[resourceName]; !exists {
				if _, err := addSchemaDocumentResource(
					resourceSet.resources,
					resourceSet.resourceNodes,
					resourceName,
					foundIndex.GetRootNode(),
					purpose,
				); err != nil {
					return err
				}
			}

			if err := addReachableSchemaResources(resourceSet, state, foundIndex, foundRef.Node, purpose); err != nil {
				return err
			}
		}
	}
	return nil
}

func schemaResourceIndex(foundRef *index.Reference, foundIndex *index.SpecIndex) *index.SpecIndex {
	if foundIndex != nil {
		return foundIndex
	}
	if foundRef == nil {
		return nil
	}
	return foundRef.Index
}

// ensureSchemaResourceName returns the stable compiler resource name for a parsed document.
func ensureSchemaResourceName(state *schemaResourceBuildState, schemaIndex *index.SpecIndex, fallback uint64) string {
	if state == nil || schemaIndex == nil {
		return ""
	}
	if resourceName, exists := state.resourceNames[schemaIndex]; exists {
		return resourceName
	}
	resourceName := resourceNameForIndex(schemaIndex, fallback)
	state.resourceNames[schemaIndex] = resourceName
	return resourceName
}

// addSchemaDocumentResource renders and registers a document-level JSON Schema resource.
func addSchemaDocumentResource(
	resources map[string][]byte,
	resourceNodes map[string]*yaml.Node,
	resourceName string,
	rootNode *yaml.Node,
	purpose SchemaValidationPurpose,
) (*RenderedValidationSchema, error) {
	if resourceName == "" || rootNode == nil {
		return nil, nil
	}

	rendered, err := renderYAMLNodeForValidation(rootNode, purpose)
	if err != nil {
		return nil, fmt.Errorf("schema resource %q render failed: %w", resourceName, err)
	}

	if resources != nil {
		resources[resourceName] = rendered.RenderedJSON
	}
	if resourceNodes != nil {
		resourceNodes[resourceName] = rendered.RenderedNode
	}
	return rendered, nil
}

// resourceNameForIndex returns the canonical resource URI for an indexed document.
//
// File-backed specs use a canonical file URI so jsonschema absolute keyword
// locations line up with source-node lookup. Memory-only specs get deterministic
// synthetic HTTPS names scoped to this validator package.
func resourceNameForIndex(schemaIndex *index.SpecIndex, fallback uint64) string {
	if schemaIndex != nil && schemaIndex.GetSpecAbsolutePath() != "" {
		return canonicalResourceName(schemaIndex.GetSpecAbsolutePath())
	}
	return syntheticSchemaResourceBase + strconv.FormatUint(fallback, 16) + ".json"
}

// canonicalResourceName normalizes resource names into the URI form used by jsonschema diagnostics.
func canonicalResourceName(resourceName string) string {
	if resourceName == "" {
		return ""
	}
	parsed, err := url.Parse(resourceName)
	if err == nil && parsed.Scheme != "" {
		return parsed.String()
	}
	absPath := resourceName
	if !filepath.IsAbs(absPath) {
		if resolved, err := filepath.Abs(absPath); err == nil {
			absPath = resolved
		}
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}).String()
}

// collectSchemaRefValues returns refs from schema-keyword positions in a YAML tree.
func collectSchemaRefValues(node *yaml.Node) []string {
	var refs []string
	collectSchemaRefValuesInto(node, "", &refs)
	return refs
}

// collectSchemaRefValuesInto walks a YAML tree and records only real schema $ref keywords.
//
// OpenAPI schema maps can legally contain schema names or property names called
// "$ref"; those names are not references and must not pull unrelated resources
// into the compiler graph.
func collectSchemaRefValuesInto(node *yaml.Node, parentKey string, refs *[]string) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			collectSchemaRefValuesInto(child, parentKey, refs)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			key := ""
			if keyNode != nil {
				key = keyNode.Value
			}
			if key == "$ref" && !isSchemaNameMap(parentKey) && valueNode != nil && valueNode.Kind == yaml.ScalarNode {
				*refs = append(*refs, valueNode.Value)
				continue
			}
			collectSchemaRefValuesInto(valueNode, key, refs)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			collectSchemaRefValuesInto(child, parentKey, refs)
		}
	}
}

// isSchemaNameMap reports whether a mapping value is keyed by user-defined schema names.
func isSchemaNameMap(parentKey string) bool {
	switch parentKey {
	case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
		return true
	default:
		return false
	}
}

// cloneYAMLNode deep-copies a YAML node tree before validation-specific pruning mutates it.
func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	cloned := *node
	if len(node.Content) > 0 {
		cloned.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			cloned.Content[i] = cloneYAMLNode(child)
		}
	}
	return &cloned
}

// pruneDirectionalRequiredEverywhere removes request-only or response-only required markers recursively.
func pruneDirectionalRequiredEverywhere(node *yaml.Node, purpose SchemaValidationPurpose) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		pruneRequiredAtSchema(node, purpose)
	}
	for _, child := range node.Content {
		pruneDirectionalRequiredEverywhere(child, purpose)
	}
}

// jsonPointerForNode returns the RFC 6901 pointer to targetNode relative to rootNode.
func jsonPointerForNode(rootNode, targetNode *yaml.Node) (string, bool) {
	if rootNode == nil || targetNode == nil {
		return "", false
	}
	if rootNode == targetNode {
		return "", true
	}
	if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
		return jsonPointerForNode(rootNode.Content[0], targetNode)
	}
	if targetNode.Kind == yaml.DocumentNode && len(targetNode.Content) > 0 {
		return jsonPointerForNode(rootNode, targetNode.Content[0])
	}

	var tokens []string
	ok := appendJSONPointerTokensForNode(rootNode, targetNode, &tokens)
	if !ok {
		return "", false
	}
	return "/" + strings.Join(tokens, "/"), true
}

// jsonPointerTokensForNode returns the unjoined pointer tokens for targetNode relative to currentNode.
func jsonPointerTokensForNode(currentNode, targetNode *yaml.Node) ([]string, bool) {
	var tokens []string
	ok := appendJSONPointerTokensForNode(currentNode, targetNode, &tokens)
	return tokens, ok
}

// appendJSONPointerTokensForNode depth-first searches a YAML tree while maintaining the current pointer path.
//
// The path is pushed and popped in place so deep schemas avoid the O(depth^2)
// allocations that come from prepending tokens during recursive unwind.
func appendJSONPointerTokensForNode(currentNode, targetNode *yaml.Node, tokens *[]string) bool {
	if currentNode == nil {
		return false
	}
	if currentNode == targetNode {
		return true
	}

	switch currentNode.Kind {
	case yaml.DocumentNode:
		if len(currentNode.Content) == 0 {
			return false
		}
		return appendJSONPointerTokensForNode(currentNode.Content[0], targetNode, tokens)
	case yaml.MappingNode:
		for i := 0; i+1 < len(currentNode.Content); i += 2 {
			keyNode := currentNode.Content[i]
			valueNode := currentNode.Content[i+1]
			token := ""
			if keyNode != nil {
				token = escapeJSONPointerToken(keyNode.Value)
			}
			*tokens = append(*tokens, token)
			if appendJSONPointerTokensForNode(valueNode, targetNode, tokens) {
				return true
			}
			*tokens = (*tokens)[:len(*tokens)-1]
		}
	case yaml.SequenceNode:
		for i, valueNode := range currentNode.Content {
			*tokens = append(*tokens, strconv.Itoa(i))
			if appendJSONPointerTokensForNode(valueNode, targetNode, tokens) {
				return true
			}
			*tokens = (*tokens)[:len(*tokens)-1]
		}
	}

	return false
}

// escapeJSONPointerToken escapes a single token according to RFC 6901.
func escapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}
