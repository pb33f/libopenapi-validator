// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	validatorcache "github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/pb33f/libopenapi-validator/config"
)

type countingSchemaResourceCache struct {
	delegate validatorcache.SchemaResourceCache
	loads    int
	stores   int
}

func newCountingSchemaResourceCache() *countingSchemaResourceCache {
	return &countingSchemaResourceCache{delegate: validatorcache.NewDefaultSchemaResourceCache()}
}

func (c *countingSchemaResourceCache) Load(key string) (*validatorcache.SchemaResourceCacheEntry, bool) {
	c.loads++
	return c.delegate.Load(key)
}

func (c *countingSchemaResourceCache) Store(key string, value *validatorcache.SchemaResourceCacheEntry) {
	c.stores++
	c.delegate.Store(key, value)
}

func (c *countingSchemaResourceCache) Range(f func(key string, value *validatorcache.SchemaResourceCacheEntry) bool) {
	c.delegate.Range(f)
}

func TestJSONPointerForNode_EscapesMappingKeysAndSequences(t *testing.T) {
	spec := `paths:
  /objects:
    get:
      parameters:
        - name: filter
          in: query
          schema:
            type: object
            properties:
              tilde~name:
                type: string
              slash/name:
                type: string`

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(spec), &root))

	target := mappingValue(
		mappingValue(
			mappingValue(
				mappingValue(
					mappingValue(root.Content[0], "paths"),
					"/objects",
				),
				"get",
			),
			"parameters",
		).Content[0],
		"schema",
	)

	pointer, ok := jsonPointerForNode(&root, target)

	require.True(t, ok)
	assert.Equal(t, "/paths/~1objects/get/parameters/0/schema", pointer)

	slashProperty := mappingValue(mappingValue(target, "properties"), "slash/name")
	pointer, ok = jsonPointerForNode(&root, slashProperty)

	require.True(t, ok)
	assert.Equal(t, "/paths/~1objects/get/parameters/0/schema/properties/slash~1name", pointer)
}

func TestJSONPointerForNode_EdgeCases(t *testing.T) {
	assert.False(t, func() bool {
		_, ok := jsonPointerForNode(nil, nil)
		return ok
	}())

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`name: test`), &root))

	pointer, ok := jsonPointerForNode(&root, &root)
	require.True(t, ok)
	assert.Empty(t, pointer)

	missing := &yaml.Node{Kind: yaml.ScalarNode, Value: "missing"}
	_, ok = jsonPointerForNode(&root, missing)
	assert.False(t, ok)

	pointer, ok = jsonPointerForNode(root.Content[0], &root)
	require.True(t, ok)
	assert.Empty(t, pointer)

	emptyDocument := &yaml.Node{Kind: yaml.DocumentNode}
	_, ok = jsonPointerForNode(emptyDocument, missing)
	assert.False(t, ok)
}

func TestCompileSchemaForValidation_EdgeCases(t *testing.T) {
	compiled, err := CompileSchemaForValidation(nil, SchemaValidationPurposeGeneric, config.NewValidationOptions(), 3.1)
	require.NoError(t, err)
	assert.Nil(t, compiled)

	compiled, err = CompileSchemaForValidation(
		&base.Schema{Type: []string{"string"}},
		SchemaValidationPurposeGeneric,
		config.NewValidationOptions(),
		3.1,
	)
	assert.Nil(t, compiled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "low-level information")
}

func TestCompileSchemaForValidation_SingleSchemaCompileFailure(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Thing:
      type: not-a-real-type`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	compiled, err := CompileSchemaForValidation(
		model.Model.Components.Schemas.GetOrZero("Thing").Schema(),
		SchemaValidationPurposeGeneric,
		config.NewValidationOptions(),
		3.1,
	)

	assert.Nil(t, compiled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON schema compile failed")
}

func TestCompileSchemaForValidation_BuildResourceFailuresAndFallbacks(t *testing.T) {
	t.Run("schema root not found in indexed document", func(t *testing.T) {
		spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Name:
      type: string`

		doc, err := libopenapi.NewDocument([]byte(spec))
		require.NoError(t, err)
		model, errs := doc.BuildV3Model()
		require.Empty(t, errs)

		schema := model.Model.Components.Schemas.GetOrZero("Name").Schema()
		var detached yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(`$ref: '#/components/schemas/Name'`), &detached))
		schema.GoLow().RootNode = detached.Content[0]

		resourceSet, err := buildSchemaDocumentResources(
			schema,
			SchemaValidationPurposeGeneric,
			nil,
		)

		assert.Nil(t, resourceSet)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema node was not found")
	})

	t.Run("low schema with no index falls back to single schema compiler", func(t *testing.T) {
		var root yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(`$ref: '#/components/schemas/Name'`), &root))
		schema := base.NewSchema(&lowbase.Schema{RootNode: root.Content[0]})

		compiled, err := CompileSchemaForValidation(
			schema,
			SchemaValidationPurposeGeneric,
			config.NewValidationOptions(),
			3.1,
		)

		require.NoError(t, err)
		require.NotNil(t, compiled)
		assert.NotNil(t, compiled.CompiledSchema)
	})
}

func TestRenderRootSchemaForValidation_RenderFallbackFailure(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Node:
      type: object
      properties:
        child:
          $ref: '#/components/schemas/Node'`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)
	schema := model.Model.Components.Schemas.GetOrZero("Node").Schema()

	originalRenderSchemaWithRefs := renderSchemaWithRefs
	renderSchemaWithRefs = func(*base.Schema) ([]byte, error) {
		return nil, assert.AnError
	}
	t.Cleanup(func() {
		renderSchemaWithRefs = originalRenderSchemaWithRefs
	})

	rendered, err := renderRootSchemaForValidation(schema, SchemaValidationPurposeGeneric)

	assert.Nil(t, rendered)
	require.ErrorIs(t, err, assert.AnError)
}

func TestSingleSchemaCompilePreferred_ResolvedLocalReferenceUsesSingleSchemaCompiler(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Inline:
      type: object
      properties:
        id:
          type: string
    Referenced:
      $ref: '#/components/schemas/Inline'`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	inlineSchema := model.Model.Components.Schemas.GetOrZero("Inline").Schema()
	inlineRendered, err := renderRootSchemaForValidation(inlineSchema, SchemaValidationPurposeGeneric)
	require.NoError(t, err)
	assert.True(t, singleSchemaCompilePreferred(inlineRendered))

	referencedSchema := model.Model.Components.Schemas.GetOrZero("Referenced").Schema()
	referencedRendered, err := renderRootSchemaForValidation(referencedSchema, SchemaValidationPurposeGeneric)
	require.NoError(t, err)
	assert.True(t, singleSchemaCompilePreferred(referencedRendered))
	assert.True(t, schemaHasReachableRefs(referencedSchema))
	assert.False(t, renderedSchemaHasReachableRefs(referencedRendered))
}

func TestCompileSchemaForValidation_ReusesRenderedDocumentResourceAcrossSchemas(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    First:
      type: object
      properties:
        child:
          $ref: '#/components/schemas/Node'
    Second:
      type: object
      properties:
        child:
          $ref: '#/components/schemas/Node'
    Node:
      type: object
      required: [id]
      properties:
        id:
          type: string
        next:
          $ref: '#/components/schemas/Node'`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	resourceCache := newCountingSchemaResourceCache()
	options := config.NewValidationOptions(config.WithSchemaResourceCache(resourceCache))

	firstCompiled, err := CompileSchemaForValidation(
		model.Model.Components.Schemas.GetOrZero("First").Schema(),
		SchemaValidationPurposeGeneric,
		options,
		3.1,
	)
	require.NoError(t, err)
	require.NotNil(t, firstCompiled)
	require.NotNil(t, firstCompiled.CompiledSchema)
	assert.NoError(t, firstCompiled.CompiledSchema.Validate(map[string]any{
		"child": map[string]any{
			"id": "root",
			"next": map[string]any{
				"id": "nested",
			},
		},
	}))

	secondCompiled, err := CompileSchemaForValidation(
		model.Model.Components.Schemas.GetOrZero("Second").Schema(),
		SchemaValidationPurposeGeneric,
		options,
		3.1,
	)
	require.NoError(t, err)
	require.NotNil(t, secondCompiled)
	require.NotNil(t, secondCompiled.CompiledSchema)
	assert.NoError(t, secondCompiled.CompiledSchema.Validate(map[string]any{
		"child": map[string]any{
			"id": "root",
		},
	}))

	assert.Equal(t, 2, resourceCache.loads)
	assert.Equal(t, 1, resourceCache.stores)
}

func TestCompileSchemaForValidation_ReusesDirectionalResourcesByPurpose(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Wrapper:
      type: object
      required: [product]
      properties:
        product:
          $ref: '#/components/schemas/Product'
    Product:
      type: object
      required: [id, name, secret]
      properties:
        id:
          type: string
          readOnly: true
        name:
          type: string
        secret:
          type: string
          writeOnly: true`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	schema := model.Model.Components.Schemas.GetOrZero("Wrapper").Schema()
	resourceCache := newCountingSchemaResourceCache()
	options := config.NewValidationOptions(config.WithSchemaResourceCache(resourceCache))

	requestCompiled, err := CompileSchemaForValidation(schema, SchemaValidationPurposeRequestBody, options, 3.1)
	require.NoError(t, err)
	require.NotNil(t, requestCompiled)
	assert.NoError(t, requestCompiled.CompiledSchema.Validate(map[string]any{
		"product": map[string]any{
			"name":   "Desk",
			"secret": "internal",
		},
	}))
	assert.Error(t, requestCompiled.CompiledSchema.Validate(map[string]any{
		"product": map[string]any{
			"name": "Desk",
		},
	}))

	responseCompiled, err := CompileSchemaForValidation(schema, SchemaValidationPurposeResponseBody, options, 3.1)
	require.NoError(t, err)
	require.NotNil(t, responseCompiled)
	assert.NoError(t, responseCompiled.CompiledSchema.Validate(map[string]any{
		"product": map[string]any{
			"id":   "p1",
			"name": "Desk",
		},
	}))
	assert.Error(t, responseCompiled.CompiledSchema.Validate(map[string]any{
		"product": map[string]any{
			"name": "Desk",
		},
	}))

	requestCompiled, err = CompileSchemaForValidation(schema, SchemaValidationPurposeRequestBody, options, 3.1)
	require.NoError(t, err)
	require.NotNil(t, requestCompiled)
	assert.NoError(t, requestCompiled.CompiledSchema.Validate(map[string]any{
		"product": map[string]any{
			"name":   "Desk",
			"secret": "internal",
		},
	}))

	var cacheKeys []string
	resourceCache.Range(func(key string, value *validatorcache.SchemaResourceCacheEntry) bool {
		cacheKeys = append(cacheKeys, key)
		require.NotNil(t, value.SourceRootNode)
		return true
	})

	assert.Equal(t, 3, resourceCache.loads)
	assert.Equal(t, 2, resourceCache.stores)
	require.Len(t, cacheKeys, 2)
	assert.NotEqual(t, cacheKeys[0], cacheKeys[1])
}

func TestSingleSchemaCompilePreferred_ResolvedExternalReferenceUsesSingleSchemaCompiler(t *testing.T) {
	tempDir := t.TempDir()
	rootPath := filepath.Join(tempDir, "openapi.yaml")
	require.NoError(t, os.WriteFile(rootPath, []byte(`openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Referenced:
      $ref: './models.yaml#/components/schemas/External'`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "models.yaml"), []byte(`components:
  schemas:
    External:
      type: object
      properties:
        id:
          type: string`), 0o600))

	docConfig := datamodel.NewDocumentConfiguration()
	docConfig.AllowFileReferences = true
	docConfig.BasePath = tempDir
	docConfig.SpecFilePath = rootPath
	docConfig.FileFilter = []string{"openapi.yaml", "models.yaml"}

	rootSpec, err := os.ReadFile(rootPath)
	require.NoError(t, err)
	doc, err := libopenapi.NewDocumentWithConfiguration(rootSpec, docConfig)
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	referencedSchema := model.Model.Components.Schemas.GetOrZero("Referenced").Schema()
	referencedRendered, err := renderRootSchemaForValidation(referencedSchema, SchemaValidationPurposeGeneric)
	require.NoError(t, err)
	assert.True(t, singleSchemaCompilePreferred(referencedRendered))
	assert.False(t, renderedSchemaHasReachableRefs(referencedRendered))
	require.True(t, schemaHasReachableRefs(referencedSchema))

	resourceSet, err := buildSchemaDocumentResources(referencedSchema, SchemaValidationPurposeGeneric, nil)
	require.NoError(t, err)
	require.NotNil(t, resourceSet)
	assert.Len(t, resourceSet.resources, 2)
	assert.Len(t, resourceSet.resourceNodes, 2)
	assert.NotEmpty(t, resourceSet.entryName)

	var foundExternal bool
	for resourceName := range resourceSet.resourceNodes {
		if strings.HasSuffix(resourceName, "models.yaml") {
			foundExternal = true
			break
		}
	}
	assert.True(t, foundExternal)
}

func TestSingleSchemaCompilePreferred_UnrelatedExternalResourceStaysSingleSchema(t *testing.T) {
	tempDir := t.TempDir()
	rootPath := filepath.Join(tempDir, "openapi.yaml")
	require.NoError(t, os.WriteFile(rootPath, []byte(`openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /external:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: './models.yaml#/components/schemas/External'
      responses:
        '200':
          description: ok
components:
  schemas:
    Inline:
      type: object
      properties:
        id:
          type: string`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "models.yaml"), []byte(`components:
  schemas:
    External:
      type: object
      properties:
        name:
          type: string`), 0o600))

	docConfig := datamodel.NewDocumentConfiguration()
	docConfig.AllowFileReferences = true
	docConfig.BasePath = tempDir
	docConfig.SpecFilePath = rootPath
	docConfig.FileFilter = []string{"openapi.yaml", "models.yaml"}

	rootSpec, err := os.ReadFile(rootPath)
	require.NoError(t, err)
	doc, err := libopenapi.NewDocumentWithConfiguration(rootSpec, docConfig)
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	inlineSchema := model.Model.Components.Schemas.GetOrZero("Inline").Schema()
	inlineRendered, err := renderRootSchemaForValidation(inlineSchema, SchemaValidationPurposeGeneric)
	require.NoError(t, err)

	assert.True(t, singleSchemaCompilePreferred(inlineRendered))
	assert.False(t, schemaHasReachableRefs(inlineSchema))
	resourceSet, err := buildSchemaDocumentResources(inlineSchema, SchemaValidationPurposeGeneric, nil)
	require.NoError(t, err)
	assert.Nil(t, resourceSet)
}

func TestBuildSchemaDocumentResources_NoLowSchema(t *testing.T) {
	resourceSet, err := buildSchemaDocumentResources(
		&base.Schema{},
		SchemaValidationPurposeGeneric,
		nil,
	)

	assert.Nil(t, resourceSet)
	require.NoError(t, err)
	assert.False(t, schemaHasReachableRefs(nil))
}

func TestBuildSchemaDocumentResources_IndexWithoutRootNode(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Thing:
      type: string`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)
	schema := model.Model.Components.Schemas.GetOrZero("Thing").Schema()
	schema.GoLow().GetIndex().SetRootNode(nil)

	resourceSet, err := buildSchemaDocumentResources(schema, SchemaValidationPurposeGeneric, nil)

	require.NoError(t, err)
	assert.Nil(t, resourceSet)
}

func TestCollectSchemaRefValues_IgnoresSchemaNameMaps(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object
properties:
  $ref:
    type: string
  child:
    $ref: '#/components/schemas/Child'
$defs:
  $ref:
    type: object`), &root))

	refs := collectSchemaRefValues(root.Content[0])

	require.Len(t, refs, 1)
	assert.Equal(t, "#/components/schemas/Child", refs[0])
}

func TestSchemaResourceHelpers_EdgeCases(t *testing.T) {
	var compiled *CompiledValidationSchema
	assert.Nil(t, compiled.ToCacheEntry(nil))
	assert.Empty(t, schemaResourceCacheKey(nil, SchemaValidationPurposeGeneric))
	assert.Nil(t, resourceCacheEntryFromRenderedSchema(nil, nil))
	assert.Nil(t, renderedSchemaFromResourceCache(nil))
	assert.False(t, renderedSchemaHasReachableRefs(nil))
	assert.False(t, renderedSchemaHasReachableRefs(&RenderedValidationSchema{}))
	assert.False(t, renderedSchemaHasReachableRefs(&RenderedValidationSchema{RenderedInline: []byte(`type: string`)}))
	assert.True(t, renderedSchemaHasReachableRefs(&RenderedValidationSchema{RenderedInline: []byte(`$ref: '#/components/schemas/Thing'`)}))
	assert.True(t, renderedSchemaHasReachableRefs(&RenderedValidationSchema{RenderedInline: []byte(":\n")}))

	rendered, err := renderRootSchemaForValidation(nil, SchemaValidationPurposeGeneric)
	require.NoError(t, err)
	assert.Nil(t, rendered)

	assert.Nil(t, sourceNodesForRenderedSchema("schema", nil))
	renderedNode := &yaml.Node{Kind: yaml.MappingNode}
	sourceNodes := sourceNodesForRenderedSchema("schema", renderedNode)
	assert.Same(t, renderedNode, sourceNodes[""])
	assert.Same(t, renderedNode, sourceNodes["schema"])

	rendered, err = renderYAMLNodeForValidation(nil, SchemaValidationPurposeGeneric)
	require.NoError(t, err)
	assert.Nil(t, rendered)

	rendered, err = renderYAMLNodeForValidation(
		&yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "not-a-document-map"},
			},
		},
		SchemaValidationPurposeGeneric,
	)
	assert.Nil(t, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON conversion")

	rendered, err = renderYAMLNodeForValidation(
		&yaml.Node{
			Kind: yaml.AliasNode,
		},
		SchemaValidationPurposeGeneric,
	)
	assert.Nil(t, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema render encode failed")

	resources := make(map[string][]byte)
	resourceNodes := make(map[string]*yaml.Node)
	registerSchemaDocumentResource(resources, resourceNodes, "https://example.com/nil.json", nil)
	assert.Empty(t, resources)
	assert.Empty(t, resourceNodes)

	rendered, err = addSchemaDocumentResource(
		resources,
		resourceNodes,
		"",
		&yaml.Node{Kind: yaml.MappingNode},
		SchemaValidationPurposeGeneric,
		nil,
	)
	require.NoError(t, err)
	assert.Nil(t, rendered)
	assert.Empty(t, resources)
	assert.Empty(t, resourceNodes)

	rendered, err = addSchemaDocumentResource(
		resources,
		resourceNodes,
		"https://example.com/root.json",
		nil,
		SchemaValidationPurposeGeneric,
		nil,
	)
	require.NoError(t, err)
	assert.Nil(t, rendered)
	assert.Empty(t, resources)
	assert.Empty(t, resourceNodes)

	var resourceRoot yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object`), &resourceRoot))
	cacheEntry := resourceCacheEntryFromRenderedSchema(
		&RenderedValidationSchema{RenderedNode: resourceRoot.Content[0]},
		&resourceRoot,
	)
	require.NotNil(t, cacheEntry)
	assert.Same(t, &resourceRoot, cacheEntry.SourceRootNode)

	rendered, err = addSchemaDocumentResource(
		resources,
		resourceNodes,
		"https://example.com/root.json",
		&resourceRoot,
		SchemaValidationPurposeGeneric,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	assert.NotEmpty(t, resources["https://example.com/root.json"])
	assert.Same(t, rendered.RenderedNode, resourceNodes["https://example.com/root.json"])

	resourceCache := validatorcache.NewDefaultSchemaResourceCache()
	resourceCache.Store(schemaResourceCacheKey(&resourceRoot, SchemaValidationPurposeGeneric), &validatorcache.SchemaResourceCacheEntry{
		RenderedInline:  []byte("cached"),
		ReferenceSchema: "cached",
		RenderedJSON:    []byte(`{"type":"object"}`),
		RenderedNode:    &resourceRoot,
	})
	resources = make(map[string][]byte)
	resourceNodes = make(map[string]*yaml.Node)
	rendered, err = addSchemaDocumentResource(
		resources,
		resourceNodes,
		"https://example.com/cached.json",
		&resourceRoot,
		SchemaValidationPurposeGeneric,
		resourceCache,
	)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	assert.Equal(t, []byte("cached"), rendered.RenderedInline)
	assert.Equal(t, []byte(`{"type":"object"}`), resources["https://example.com/cached.json"])
	assert.Same(t, &resourceRoot, resourceNodes["https://example.com/cached.json"])

	rendered, err = addSchemaDocumentResource(
		resources,
		resourceNodes,
		"https://example.com/invalid.json",
		&yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "not-a-document-map"},
			},
		},
		SchemaValidationPurposeGeneric,
		nil,
	)
	assert.Nil(t, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema resource")

	pruneDirectionalRequiredEverywhere(nil, SchemaValidationPurposeRequestBody)
	assert.Nil(t, cloneYAMLNode(nil))

	tokens, ok := jsonPointerTokensForNode(nil, &yaml.Node{})
	assert.Nil(t, tokens)
	assert.False(t, ok)

	target := &yaml.Node{Kind: yaml.ScalarNode, Value: "value"}
	mappingWithNilKey := &yaml.Node{
		Kind:    yaml.MappingNode,
		Content: []*yaml.Node{nil, target},
	}
	tokens, ok = jsonPointerTokensForNode(mappingWithNilKey, target)
	require.True(t, ok)
	assert.Equal(t, []string{""}, tokens)

	assert.Empty(t, ensureSchemaResourceName(nil, nil, 1))
}

func TestResourceCacheEntryPinsSourceRootForClonedDirectionalRender(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object
required: [id, name]
properties:
  id:
    type: string
    readOnly: true
  name:
    type: string`), &root))

	rendered, err := renderYAMLNodeForValidation(root.Content[0], SchemaValidationPurposeRequestBody)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	require.NotSame(t, root.Content[0], rendered.RenderedNode)

	entry := resourceCacheEntryFromRenderedSchema(rendered, root.Content[0])
	require.NotNil(t, entry)
	assert.Same(t, root.Content[0], entry.SourceRootNode)
	assert.Same(t, rendered.RenderedNode, entry.RenderedNode)
}

func TestRenderYAMLNodeForValidation_DirectionalPurposeClonesAndPrunes(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object
required: [id, name]
properties:
  id:
    type: string
    readOnly: true
  name:
    type: string`), &root))

	rendered, err := renderYAMLNodeForValidation(root.Content[0], SchemaValidationPurposeRequestBody)

	require.NoError(t, err)
	require.NotNil(t, rendered)
	assert.NotSame(t, root.Content[0], rendered.RenderedNode)

	originalRequired := mappingValue(root.Content[0], "required")
	require.Len(t, originalRequired.Content, 2)

	renderedRequired := mappingValue(rendered.RenderedNode, "required")
	require.Len(t, renderedRequired.Content, 1)
	assert.Equal(t, "name", renderedRequired.Content[0].Value)
}

func TestCanonicalResourceName_EdgeCases(t *testing.T) {
	assert.Empty(t, canonicalResourceName(""))
	assert.Equal(t, "https://example.com/schema.yaml", canonicalResourceName("https://example.com/schema.yaml"))

	resourceName := canonicalResourceName(filepath.Join("fixtures", "models.yaml"))

	assert.True(t, strings.HasPrefix(resourceName, "file:///"))
	assert.True(t, strings.HasSuffix(resourceName, "/fixtures/models.yaml"))
}

func TestCollectSchemaRefValues_DocumentSequenceAndNilEdges(t *testing.T) {
	assert.Empty(t, collectSchemaRefValues(nil))
	assert.False(t, hasSchemaRefValueIn(nil, ""))

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`allOf:
  - $ref: '#/components/schemas/First'
  - properties:
      $ref:
        type: string
    items:
      $ref: '#/components/schemas/Second'`), &root))

	refs := collectSchemaRefValues(&root)

	assert.Equal(t, []string{
		"#/components/schemas/First",
		"#/components/schemas/Second",
	}, refs)
	assert.True(t, hasSchemaRefValue(mappingValue(root.Content[0], "allOf")))
}

func TestAddReachableSchemaResources_GuardsAndSkips(t *testing.T) {
	assert.NoError(t, addReachableSchemaResources(nil, nil, nil, nil, SchemaValidationPurposeGeneric, nil))

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`components:
  schemas:
    Root:
      type: object
      properties:
        missing:
          $ref: '#/components/schemas/Missing'
        child:
          $ref: '#/components/schemas/Child'
        other:
          $ref: '#/components/schemas/Child'
    Child:
      type: object
      properties:
        next:
          $ref: '#/components/schemas/Child'`), &root))

	specIndex := index.NewSpecIndex(&root)
	rootSchemaNode := mappingValue(mappingValue(mappingValue(root.Content[0], "components"), "schemas"), "Root")
	require.NotNil(t, rootSchemaNode)
	childSchemaNode := mappingValue(mappingValue(mappingValue(root.Content[0], "components"), "schemas"), "Child")
	require.NotNil(t, childSchemaNode)

	resourceSet := &schemaDocumentResourceSet{
		resources:     make(map[string][]byte),
		resourceNodes: make(map[string]*yaml.Node),
	}
	state := &schemaResourceBuildState{
		resourceNames: make(map[*index.SpecIndex]string),
		seenRefs:      make(map[string]struct{}),
		seenNodes: map[*yaml.Node]struct{}{
			childSchemaNode: {},
		},
	}

	require.NoError(t, addReachableSchemaResources(
		resourceSet,
		state,
		specIndex,
		rootSchemaNode,
		SchemaValidationPurposeGeneric,
		nil,
	))
	assert.NotEmpty(t, state.seenRefs)

	require.NoError(t, addReachableSchemaResources(
		resourceSet,
		state,
		specIndex,
		childSchemaNode,
		SchemaValidationPurposeGeneric,
		nil,
	))
}

func TestSchemaResourceIndex_Fallbacks(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object`), &root))
	preferredIndex := index.NewSpecIndex(&root)
	fallbackIndex := index.NewSpecIndex(&root)

	assert.Nil(t, schemaResourceIndex(nil, nil))
	assert.Same(t, preferredIndex, schemaResourceIndex(
		&index.Reference{Index: fallbackIndex},
		preferredIndex,
	))
	assert.Same(t, fallbackIndex, schemaResourceIndex(
		&index.Reference{Index: fallbackIndex},
		nil,
	))
}

func TestAppendJSONPointerTokensForNode_DocumentNodeDirectCall(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`items:
  - name: first`), &root))

	target := mappingValue(root.Content[0].Content[1].Content[0], "name")
	require.NotNil(t, target)

	var tokens []string
	ok := appendJSONPointerTokensForNode(&root, target, &tokens)

	require.True(t, ok)
	assert.Equal(t, []string{"items", "0", "name"}, tokens)
}

func TestCompileSchemaForValidation_ResourceCompileFailure(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Thing:
      $ref: '#/components/schemas/Bad'
    Bad:
      type: not-a-real-type`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)
	schema := model.Model.Components.Schemas.GetOrZero("Thing").Schema()

	compiled, err := CompileSchemaForValidation(
		schema,
		SchemaValidationPurposeGeneric,
		config.NewValidationOptions(),
		3.1,
	)

	assert.Nil(t, compiled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON schema compile failed")
}

func TestCompileSchemaForValidation_ExternalResourceCompileFailure(t *testing.T) {
	tempDir := t.TempDir()
	rootPath := filepath.Join(tempDir, "openapi.yaml")
	require.NoError(t, os.WriteFile(rootPath, []byte(`openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Thing:
      $ref: './models.yaml#/components/schemas/Bad'`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "models.yaml"), []byte(`components:
  schemas:
    Bad:
      type: not-a-real-type`), 0o600))

	docConfig := datamodel.NewDocumentConfiguration()
	docConfig.AllowFileReferences = true
	docConfig.BasePath = tempDir
	docConfig.SpecFilePath = rootPath
	docConfig.FileFilter = []string{"openapi.yaml", "models.yaml"}

	rootSpec, err := os.ReadFile(rootPath)
	require.NoError(t, err)
	doc, err := libopenapi.NewDocumentWithConfiguration(rootSpec, docConfig)
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	compiled, err := CompileSchemaForValidation(
		model.Model.Components.Schemas.GetOrZero("Thing").Schema(),
		SchemaValidationPurposeGeneric,
		config.NewValidationOptions(),
		3.1,
	)

	assert.Nil(t, compiled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON schema compile failed")
}

func TestCompileSchemaForValidation_RootResourceRenderFailure(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Root:
      $ref: '#/components/schemas/Child'
    Child:
      type: string`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)
	schema := model.Model.Components.Schemas.GetOrZero("Root").Schema()

	appendInvalidAliasValue(schema.GoLow().GetIndex().GetRootNode())
	resourceSet, err := buildSchemaDocumentResources(
		schema,
		SchemaValidationPurposeGeneric,
		nil,
	)

	assert.Nil(t, resourceSet)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema resource")
}

func TestCompileSchemaForValidation_NestedResourceRenderFailure(t *testing.T) {
	tempDir := t.TempDir()
	rootPath := filepath.Join(tempDir, "openapi.yaml")
	require.NoError(t, os.WriteFile(rootPath, []byte(`openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Root:
      $ref: './models.yaml#/components/schemas/Child'`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "models.yaml"), []byte(`components:
  schemas:
    Child:
      $ref: './grand.yaml#/components/schemas/Grand'`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "grand.yaml"), []byte(`components:
  schemas:
    Grand:
      type: string`), 0o600))

	docConfig := datamodel.NewDocumentConfiguration()
	docConfig.AllowFileReferences = true
	docConfig.BasePath = tempDir
	docConfig.SpecFilePath = rootPath
	docConfig.FileFilter = []string{"openapi.yaml", "models.yaml", "grand.yaml"}

	rootSpec, err := os.ReadFile(rootPath)
	require.NoError(t, err)
	doc, err := libopenapi.NewDocumentWithConfiguration(rootSpec, docConfig)
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)
	schema := model.Model.Components.Schemas.GetOrZero("Root").Schema()

	childRef, childIndex := schema.GoLow().GetIndex().SearchIndexForReference("./models.yaml#/components/schemas/Child")
	require.NotNil(t, childRef)
	require.NotNil(t, childIndex)
	grandRef, grandIndex := childIndex.SearchIndexForReference("./grand.yaml#/components/schemas/Grand")
	require.NotNil(t, grandRef)
	require.NotNil(t, grandIndex)

	appendInvalidAliasValue(grandIndex.GetRootNode())
	resourceSet, err := buildSchemaDocumentResources(
		schema,
		SchemaValidationPurposeGeneric,
		nil,
	)

	assert.Nil(t, resourceSet)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema resource")
}

func appendInvalidAliasValue(root *yaml.Node) {
	if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return
	}
	rootContent := root.Content[0]
	if rootContent.Kind != yaml.MappingNode {
		return
	}
	rootContent.Content = append(
		rootContent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "x-invalid-alias"},
		&yaml.Node{Kind: yaml.AliasNode},
	)
}

func TestCloneYAMLNodeAndPruneDirectionalRequiredEverywhere(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object
required: [id, name]
properties:
  id:
    type: string
    readOnly: true
  name:
    type: string`), &root))

	cloned := cloneYAMLNode(&root)
	pruneDirectionalRequiredEverywhere(cloned, SchemaValidationPurposeRequestBody)

	originalRequired := mappingValue(root.Content[0], "required")
	require.Len(t, originalRequired.Content, 2)

	clonedRequired := mappingValue(cloned.Content[0], "required")
	require.Len(t, clonedRequired.Content, 1)
	assert.Equal(t, "name", clonedRequired.Content[0].Value)
}

func TestCompileSchemaForValidation_CircularReference(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
components:
  schemas:
    Error:
      type: object
      required: [code]
      properties:
        code:
          type: string
        details:
          type: array
          items:
            $ref: '#/components/schemas/Error'`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	schema := model.Model.Components.Schemas.GetOrZero("Error").Schema()
	compiled, err := CompileSchemaForValidation(
		schema,
		SchemaValidationPurposeGeneric,
		config.NewValidationOptions(),
		3.1,
	)

	require.NoError(t, err)
	require.NotNil(t, compiled)
	require.NotNil(t, compiled.CompiledSchema)
	require.NotEmpty(t, compiled.ResourceNodes)
	assert.NoError(t, compiled.CompiledSchema.Validate(map[string]any{
		"code": "root",
		"details": []any{
			map[string]any{"code": "child"},
		},
	}))
	assert.Error(t, compiled.CompiledSchema.Validate(map[string]any{
		"code": "root",
		"details": []any{
			map[string]any{"code": 42},
		},
	}))
}

func TestSchemaValidator_LocalRecursiveReferenceReportsSourceLocation(t *testing.T) {
	spec := `openapi: "3.1.0"
info:
  title: Test
  version: "1"
paths:
  /:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Node'
components:
  schemas:
    Name:
      type: string
    Node:
      type: object
      properties:
        name:
          $ref: '#/components/schemas/Name'
        next:
          $ref: '#/components/schemas/Node'`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	validator := NewSchemaValidator()
	schema := model.Model.Components.Schemas.GetOrZero("Node").Schema()
	valid, validationErrors := validator.ValidateSchemaString(schema, `{"name": 42, "next": {"name": "ok"}}`)

	assert.False(t, valid)
	require.Len(t, validationErrors, 1)
	require.NotEmpty(t, validationErrors[0].SchemaValidationErrors)
	failureIndex := -1
	for i, candidate := range validationErrors[0].SchemaValidationErrors {
		if candidate != nil && strings.Contains(candidate.Reason, "got number") {
			failureIndex = i
			break
		}
	}
	require.NotEqual(t, -1, failureIndex)
	failure := validationErrors[0].SchemaValidationErrors[failureIndex]
	assert.Equal(t, 16, failure.Line)
	assert.Greater(t, failure.Column, 0)
	assert.Contains(t, failure.KeywordLocation, "/type")
}

func TestCompileSchemaForValidation_DirectionalRequiredAcrossReferences(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /products:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Product'
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Product'
components:
  schemas:
    Product:
      type: object
      required: [id, name, secret]
      properties:
        id:
          type: string
          readOnly: true
        name:
          type: string
        secret:
          type: string
          writeOnly: true`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	operation := model.Model.Paths.PathItems.GetOrZero("/products").Post
	requestSchema := operation.RequestBody.Content.GetOrZero("application/json").Schema.Schema()
	responseSchema := operation.Responses.Codes.GetOrZero("200").Content.GetOrZero("application/json").Schema.Schema()

	requestCompiled, err := CompileSchemaForValidation(
		requestSchema,
		SchemaValidationPurposeRequestBody,
		config.NewValidationOptions(),
		3.1,
	)
	require.NoError(t, err)
	assert.NoError(t, requestCompiled.CompiledSchema.Validate(map[string]any{
		"name":   "Desk",
		"secret": "internal",
	}))
	assert.Error(t, requestCompiled.CompiledSchema.Validate(map[string]any{
		"name": "Desk",
	}))

	responseCompiled, err := CompileSchemaForValidation(
		responseSchema,
		SchemaValidationPurposeResponseBody,
		config.NewValidationOptions(),
		3.1,
	)
	require.NoError(t, err)
	assert.NoError(t, responseCompiled.CompiledSchema.Validate(map[string]any{
		"id":   "p1",
		"name": "Desk",
	}))
	assert.Error(t, responseCompiled.CompiledSchema.Validate(map[string]any{
		"name": "Desk",
	}))
}
