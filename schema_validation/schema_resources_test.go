// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/pb33f/libopenapi-validator/config"
)

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

		compiled, err := CompileSchemaForValidation(
			schema,
			SchemaValidationPurposeGeneric,
			config.NewValidationOptions(),
			3.1,
		)

		assert.Nil(t, compiled)
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

func TestSingleSchemaCompilePreferred_LocalReferenceSchemaUsesResourceCompiler(t *testing.T) {
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
	assert.True(t, singleSchemaCompilePreferred(inlineSchema, inlineRendered))

	referencedSchema := model.Model.Components.Schemas.GetOrZero("Referenced").Schema()
	referencedRendered, err := renderRootSchemaForValidation(referencedSchema, SchemaValidationPurposeGeneric)
	require.NoError(t, err)
	assert.False(t, singleSchemaCompilePreferred(referencedSchema, referencedRendered))
	assert.True(t, schemaHasReachableRefs(referencedSchema))
}

func TestSingleSchemaCompilePreferred_ExternalResourceSchemaUsesResourceCompiler(t *testing.T) {
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
	assert.False(t, singleSchemaCompilePreferred(referencedSchema, referencedRendered))
	require.True(t, schemaHasReachableRefs(referencedSchema))

	resourceSet, err := buildSchemaDocumentResources(referencedSchema, SchemaValidationPurposeGeneric)
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

	assert.True(t, singleSchemaCompilePreferred(inlineSchema, inlineRendered))
	assert.False(t, schemaHasReachableRefs(inlineSchema))
	resourceSet, err := buildSchemaDocumentResources(inlineSchema, SchemaValidationPurposeGeneric)
	require.NoError(t, err)
	assert.Nil(t, resourceSet)
}

func TestBuildSchemaDocumentResources_NoLowSchema(t *testing.T) {
	resourceSet, err := buildSchemaDocumentResources(
		&base.Schema{},
		SchemaValidationPurposeGeneric,
	)

	assert.Nil(t, resourceSet)
	require.NoError(t, err)
	assert.False(t, schemaHasReachableRefs(nil))
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
	rendered, err = addSchemaDocumentResource(
		resources,
		resourceNodes,
		"",
		&yaml.Node{Kind: yaml.MappingNode},
		SchemaValidationPurposeGeneric,
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
	)
	require.NoError(t, err)
	assert.Nil(t, rendered)
	assert.Empty(t, resources)
	assert.Empty(t, resourceNodes)

	var resourceRoot yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`type: object`), &resourceRoot))

	rendered, err = addSchemaDocumentResource(
		resources,
		resourceNodes,
		"https://example.com/root.json",
		&resourceRoot,
		SchemaValidationPurposeGeneric,
	)
	require.NoError(t, err)
	require.NotNil(t, rendered)
	assert.NotEmpty(t, resources["https://example.com/root.json"])
	assert.Same(t, rendered.RenderedNode, resourceNodes["https://example.com/root.json"])

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
}

func TestAddReachableSchemaResources_GuardsAndSkips(t *testing.T) {
	assert.NoError(t, addReachableSchemaResources(nil, nil, nil, nil, SchemaValidationPurposeGeneric))

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
	))
	assert.NotEmpty(t, state.seenRefs)

	require.NoError(t, addReachableSchemaResources(
		resourceSet,
		state,
		specIndex,
		childSchemaNode,
		SchemaValidationPurposeGeneric,
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
	compiled, err := CompileSchemaForValidation(
		schema,
		SchemaValidationPurposeGeneric,
		config.NewValidationOptions(),
		3.1,
	)

	assert.Nil(t, compiled)
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
	compiled, err := CompileSchemaForValidation(
		schema,
		SchemaValidationPurposeGeneric,
		config.NewValidationOptions(),
		3.1,
	)

	assert.Nil(t, compiled)
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
