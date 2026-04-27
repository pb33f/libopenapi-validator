// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package errors

import (
	"context"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/low"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"

	"github.com/pb33f/libopenapi-validator/helpers"
)

// Helper to create a mock v3.Parameter object with a schema
func createMockParameterWithSchema() *v3.Parameter {
	schemaProxy := &lowbase.SchemaProxy{}
	_ = schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}
	schemaProxy.Schema().Enum = low.NodeReference[[]low.ValueReference[*yaml.Node]]{
		KeyNode:   &yaml.Node{Line: 10, Column: 20},
		ValueNode: &yaml.Node{},
		Value: []low.ValueReference[*yaml.Node]{
			{Value: &yaml.Node{Value: "enum1"}},
			{Value: &yaml.Node{Value: "enum2"}},
		},
	}

	param := &lowv3.Parameter{
		Name:    low.NodeReference[string]{Value: "testParam"},
		Schema:  low.NodeReference[*lowbase.SchemaProxy]{Value: schemaProxy},
		Style:   low.NodeReference[string]{Value: "form", KeyNode: &yaml.Node{Line: 15, Column: 25}, ValueNode: &yaml.Node{}},
		Explode: low.NodeReference[bool]{Value: false, ValueNode: &yaml.Node{Line: 18, Column: 30}, KeyNode: &yaml.Node{}},
		Required: low.NodeReference[bool]{
			KeyNode:   &yaml.Node{Line: 22, Column: 32},
			ValueNode: &yaml.Node{},
		},
		KeyNode: &yaml.Node{},
	}
	return v3.NewParameter(param)
}

func TestIncorrectFormEncoding(t *testing.T) {
	param := createMockParameterWithSchema()
	qp := &helpers.QueryParam{
		Key:    "testParam",
		Values: []string{"incorrect,value"},
	}

	// Call the function
	err := IncorrectFormEncoding(param, qp, 0)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testParam' is not exploded correctly")
	require.Contains(t, err.Reason, "'testParam' has a default or 'form' encoding defined")
	require.Equal(t, 18, err.SpecLine)
	require.Equal(t, 30, err.SpecCol)
	require.Contains(t, err.HowToFix, "&testParam=incorrect&testParam=value'")
}

func TestIncorrectSpaceDelimiting(t *testing.T) {
	param := createMockParameterWithSchema()
	qp := &helpers.QueryParam{
		Key:    "testParam",
		Values: []string{"value1", "value2"},
	}

	// create a low level query parameter

	// Call the function
	err := IncorrectSpaceDelimiting(param, qp)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testParam' delimited incorrectly")
	require.Contains(t, err.Reason, "'spaceDelimited' style defined")
	require.Contains(t, err.HowToFix, "testParam=value1%20value2")
}

func TestIncorrectPipeDelimiting(t *testing.T) {
	param := createMockParameterWithSchema()
	qp := &helpers.QueryParam{
		Key:    "testParam",
		Values: []string{"value1", "value2"},
	}

	// Call the function
	err := IncorrectPipeDelimiting(param, qp)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testParam' delimited incorrectly")
	require.Contains(t, err.Reason, "'pipeDelimited' style defined")
	require.Contains(t, err.HowToFix, "testParam=value1|value2")
}

func TestQueryParameterMissing(t *testing.T) {
	param := createMockParameterWithSchema()

	// Call the function
	err := QueryParameterMissing(param, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testParam' is missing")
	require.Contains(t, err.Reason, "'testParam' is defined as being required")
	require.Equal(t, HowToFixMissingValue, err.HowToFix)
}

func TestHeaderParameterMissing(t *testing.T) {
	param := createMockParameterWithSchema()

	// Call the function
	err := HeaderParameterMissing(param, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Header parameter 'testParam' is missing")
	require.Contains(t, err.Reason, "'testParam' is defined as being required")
	require.Equal(t, HowToFixMissingValue, err.HowToFix)
}

func TestCookieParameterMissing(t *testing.T) {
	param := createMockParameterWithSchema()

	// Call the function
	err := CookieParameterMissing(param, "/test", "get", "")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Cookie parameter 'testParam' is missing")
	require.Contains(t, err.Reason, "'testParam' is defined as being required")
	require.Equal(t, HowToFixMissingValue, err.HowToFix)
}

func TestHeaderParameterCannotBeDecoded(t *testing.T) {
	param := createMockParameterWithSchema()
	val := "malformed_header_value"

	// Call the function
	err := HeaderParameterCannotBeDecoded(param, val, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Header parameter 'testParam' cannot be decoded")
	require.Contains(t, err.Reason, "'malformed_header_value' is malformed")
	require.Equal(t, HowToFixInvalidEncoding, err.HowToFix)
}

func TestIncorrectHeaderParamEnum(t *testing.T) {
	param := createMockParameterWithSchema()
	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil))
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}
	schemaProxy.Schema().Enum = low.NodeReference[[]low.ValueReference[*yaml.Node]]{
		KeyNode:   &yaml.Node{Line: 10, Column: 20},
		ValueNode: &yaml.Node{},
		Value: []low.ValueReference[*yaml.Node]{
			{Value: &yaml.Node{Value: "enum1"}},
			{Value: &yaml.Node{Value: "enum2"}},
		},
	}

	s := schemaProxy.Schema()

	// build a high level schema from the low level one
	schema := base.NewSchema(s)

	// Call the function with an invalid enum value
	err := IncorrectHeaderParamEnum(param, "invalidEnum", schema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Header parameter 'testParam' does not match allowed values")
	require.Contains(t, err.Reason, "'invalidEnum' is not one of those values")
	require.Equal(t, 10, err.SpecLine)
	require.Equal(t, 20, err.SpecCol)
	require.Contains(t, err.HowToFix, "enum1, enum2")
}

func TestIncorrectQueryParamArrayBoolean(t *testing.T) {
	param := createMockParameterWithSchema()
	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil))
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}
	schemaProxy.Schema().Items = low.NodeReference[*lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]]{
		KeyNode:   &yaml.Node{Line: 30, Column: 40},
		ValueNode: &yaml.Node{},
		Value:     &lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]{A: schemaProxy},
	}

	s := schemaProxy.Schema()

	// build a high level schema from the low level one
	schema := base.NewSchema(s)

	// Call the function with an invalid boolean value in the array
	err := IncorrectQueryParamArrayBoolean(param, "notBoolean", schema, schema.Items.A.Schema(), "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Query array parameter 'testParam' is not a valid boolean")
	require.Contains(t, err.Reason, "the value 'notBoolean' is not a valid true/false value")
	require.Contains(t, err.HowToFix, "true/false")
}

// Helper function to create a mock v3.Parameter with deepObject style
func createMockParameterWithDeepObjectStyle() *v3.Parameter {
	param := &lowv3.Parameter{
		Name:    low.NodeReference[string]{Value: "testParam"},
		Style:   low.NodeReference[string]{Value: "deepObject", KeyNode: &yaml.Node{Line: 12, Column: 22}, ValueNode: &yaml.Node{}}, // Correct ValueNode set
		Explode: low.NodeReference[bool]{Value: false},
	}
	return v3.NewParameter(param)
}

func TestInvalidDeepObject(t *testing.T) {
	param := createMockParameterWithDeepObjectStyle()

	// Create a mock query parameter with multiple values
	qp := &helpers.QueryParam{
		Key:    "testParam",
		Values: []string{"value1", "value2"},
	}

	// Call the function
	err := InvalidDeepObject(param, qp)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testParam' is not a valid deepObject")
	require.Contains(t, err.Reason, "'testParam' has the 'deepObject' style defined")
	require.Contains(t, err.HowToFix, "testParam=value1|value2")
}

func TestInvalidDeepObjectPathConflict(t *testing.T) {
	param := createMockParameterWithDeepObjectStyle()
	prefixParam := &helpers.QueryParam{
		Key:          "testParam",
		Values:       []string{"bad"},
		Property:     "nested",
		PropertyPath: []string{"nested"},
	}
	nestedParam := &helpers.QueryParam{
		Key:          "testParam",
		Values:       []string{"ok"},
		Property:     "nested",
		PropertyPath: []string{"nested", "child"},
	}

	err := InvalidDeepObjectPathConflict(param, prefixParam, nestedParam)

	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testParam' is not a valid deepObject")
	require.Contains(t, err.Reason, "property path 'nested'")
	require.Contains(t, err.Reason, "'nested.child'")
	require.Contains(t, err.HowToFix, "testParam[nested]")
	require.Contains(t, err.HowToFix, "testParam[nested][child]")
}

func TestInvalidDeepObjectPathConflict_NilPaths(t *testing.T) {
	param := createMockParameterWithDeepObjectStyle()

	err := InvalidDeepObjectPathConflict(param, nil, nil)

	require.NotNil(t, err)
	require.Contains(t, err.Reason, "property path ''")
	require.Contains(t, err.HowToFix, "testParam[]")
}

func TestInvalidDeepObjectPathConflict_PropertyFallback(t *testing.T) {
	param := createMockParameterWithDeepObjectStyle()
	prefixParam := &helpers.QueryParam{
		Key:      "testParam",
		Values:   []string{"bad"},
		Property: "nested",
	}
	nestedParam := &helpers.QueryParam{
		Key:      "testParam",
		Values:   []string{"ok"},
		Property: "nested.child",
	}

	err := InvalidDeepObjectPathConflict(param, prefixParam, nestedParam)

	require.NotNil(t, err)
	require.Contains(t, err.Reason, "property path 'nested'")
	require.Contains(t, err.Reason, "'nested.child'")
	require.Contains(t, err.HowToFix, "testParam[nested]")
	require.Contains(t, err.HowToFix, "testParam[nested.child]")
}

func createMockParameterForBooleanArray() *v3.Parameter {
	param := &lowv3.Parameter{
		Name: low.NodeReference[string]{Value: "testCookieParam"},
	}
	return v3.NewParameter(param)
}

// Helper function to create a mock base.Schema with boolean items schema
func createMockLowBaseSchemaForBooleanArray() *lowbase.Schema {
	itemsSchema := &lowbase.Schema{
		Type: low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
			Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
				A: "boolean",
			},
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}

	schemaProxy := &lowbase.SchemaProxy{}

	itemsSchema.Items = low.NodeReference[*lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]]{
		Value: &lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]{
			A: schemaProxy,
		},
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
	}

	_ = schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}

	return itemsSchema
}

func TestIncorrectCookieParamArrayBoolean(t *testing.T) {
	// Create mock parameter and schemas
	param := createMockParameterForBooleanArray()
	baseSchema := createMockLowBaseSchemaForBooleanArray()
	s := base.NewSchema(baseSchema)
	itemsSchema := base.NewSchema(baseSchema.Items.Value.A.Schema())

	// Call the function with an invalid boolean value in the array
	err := IncorrectCookieParamArrayBoolean(param, "notBoolean", s, itemsSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Equal(t, "testCookieParam", err.ParameterName)
	require.Contains(t, err.Message, "Cookie array parameter 'testCookieParam' is not a valid boolean")
	require.Contains(t, err.Reason, "the value 'notBoolean' is not a valid true/false value")
	require.Contains(t, err.HowToFix, "true/false")
}

// Helper function to create a mock v3.Parameter for number array validation
func createMockParameterForNumberArray() *v3.Parameter {
	param := &lowv3.Parameter{
		Name: low.NodeReference[string]{Value: "testQueryParam"},
	}
	return v3.NewParameter(param)
}

// Helper function to create a mock base.Schema with number items schema
func createMockLowBaseSchemaForNumberArray() *lowbase.Schema {
	itemsSchema := &lowbase.Schema{
		Type: low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
			Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
				A: "number",
			},
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}

	schemaProxy := &lowbase.SchemaProxy{}

	itemsSchema.Items = low.NodeReference[*lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]]{
		Value: &lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]{
			A: schemaProxy,
		},
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
	}

	_ = schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}

	return itemsSchema
}

func TestIncorrectQueryParamArrayInteger(t *testing.T) {
	// Create mock parameter and schemas
	param := createMockParameterForNumberArray()
	baseSchema := createMockLowBaseSchemaForNumberArray()
	s := base.NewSchema(baseSchema)
	itemsSchema := base.NewSchema(baseSchema.Items.Value.A.Schema())

	// Call the function with an invalid number value in the array
	err := IncorrectQueryParamArrayInteger(param, "notNumber", s, itemsSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query array parameter 'testQueryParam' is not a valid integer")
	require.Contains(t, err.Reason, "the value 'notNumber' is not a valid integer")
	require.Contains(t, err.HowToFix, "notNumber")
}

func TestIncorrectQueryParamArrayNumber(t *testing.T) {
	// Create mock parameter and schemas
	param := createMockParameterForNumberArray()
	baseSchema := createMockLowBaseSchemaForNumberArray()
	s := base.NewSchema(baseSchema)
	itemsSchema := base.NewSchema(baseSchema.Items.Value.A.Schema())

	// Call the function with an invalid number value in the array
	err := IncorrectQueryParamArrayNumber(param, "notNumber", s, itemsSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query array parameter 'testQueryParam' is not a valid number")
	require.Contains(t, err.Reason, "the value 'notNumber' is not a valid number")
	require.Contains(t, err.HowToFix, "notNumber")
}

// Helper function to create a mock v3.Parameter for cookie number array validation
func createMockParameterForCookieNumberArray() *v3.Parameter {
	param := &lowv3.Parameter{
		Name: low.NodeReference[string]{Value: "testCookieParam"},
	}
	return v3.NewParameter(param)
}

// Helper function to create a mock base.Schema with number items schema
func createMockLowBaseSchemaForCookieNumberArray() *lowbase.Schema {
	itemsSchema := &lowbase.Schema{
		Type: low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
			Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
				A: "number",
			},
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}

	schemaProxy := &lowbase.SchemaProxy{}

	itemsSchema.Items = low.NodeReference[*lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]]{
		Value: &lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]{
			A: schemaProxy,
		},
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
	}

	_ = schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}

	return itemsSchema
}

func TestIncorrectCookieParamArrayNumber(t *testing.T) {
	// Create mock parameter and schemas
	param := createMockParameterForCookieNumberArray()
	baseSchema := createMockLowBaseSchemaForCookieNumberArray()
	s := base.NewSchema(baseSchema)
	itemsSchema := base.NewSchema(baseSchema.Items.Value.A.Schema())

	// Call the function with an invalid number value in the cookie array
	err := IncorrectCookieParamArrayNumber(param, "notNumber", s, itemsSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Equal(t, "testCookieParam", err.ParameterName)
	require.Contains(t, err.Message, "Cookie array parameter 'testCookieParam' is not a valid number")
	require.Contains(t, err.Reason, "the value 'notNumber' is not a valid number")
	require.Contains(t, err.HowToFix, "notNumber")
}

// Helper function to create a mock v3.Parameter
func createMockParameter() *v3.Parameter {
	schemaProxy := &lowbase.SchemaProxy{}
	_ = schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)

	m := orderedmap.New[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]()
	m.Set(low.KeyReference[string]{Value: "application/json"}, low.ValueReference[*lowv3.MediaType]{ValueNode: &yaml.Node{}, Value: &lowv3.MediaType{}})
	param := &lowv3.Parameter{
		Name: low.NodeReference[string]{Value: "testQueryParam"},
		Content: low.NodeReference[*orderedmap.Map[low.KeyReference[string], low.ValueReference[*lowv3.MediaType]]]{
			Value:     m,
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
		Schema: low.NodeReference[*lowbase.SchemaProxy]{
			Value:     schemaProxy,
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
		Required: low.NodeReference[bool]{
			KeyNode: &yaml.Node{},
		},
	}
	return v3.NewParameter(param)
}

// Helper function to create a mock base.Schema
func createMockLowBaseSchema() *lowbase.Schema {
	itemsSchema := &lowbase.Schema{
		Type: low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
			Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
				A: "boolean",
			},
			KeyNode:   &yaml.Node{},
			ValueNode: &yaml.Node{},
		},
	}

	schemaProxy := &lowbase.SchemaProxy{}

	itemsSchema.Items = low.NodeReference[*lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]]{
		Value: &lowbase.SchemaDynamicValue[*lowbase.SchemaProxy, bool]{
			A: schemaProxy,
		},
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
	}

	_ = schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}

	return itemsSchema
}

func TestIncorrectParamEncodingJSON(t *testing.T) {
	param := createMockParameter()
	baseSchema := createMockLowBaseSchema()

	// Call the function with an invalid JSON value
	err := IncorrectParamEncodingJSON(param, "invalidJSON", base.NewSchema(baseSchema), "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testQueryParam' is not valid JSON")
	require.Contains(t, err.Reason, "the value 'invalidJSON' is not valid JSON")
	require.Equal(t, HowToFixInvalidJSON, err.HowToFix)
}

func TestIncorrectQueryParamBool(t *testing.T) {
	param := createMockParameter()
	baseSchema := createMockLowBaseSchema()

	lschemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, lschemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil))
	lschemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
	}
	param.GoLow().Schema.KeyNode = &yaml.Node{}
	param.Schema = base.NewSchemaProxy(&low.NodeReference[*lowbase.SchemaProxy]{
		Value:     lschemaProxy,
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
	})

	// Call the function with an invalid boolean value
	err := IncorrectQueryParamBool(param, "notBoolean", base.NewSchema(baseSchema), "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testQueryParam' is not a valid boolean")
	require.Contains(t, err.Reason, "the value 'notBoolean' is not a valid boolean")
	require.Contains(t, err.HowToFix, "true/false")
}

func TestInvalidQueryParamNumber(t *testing.T) {
	param := createMockParameter()
	baseSchema := createMockLowBaseSchema()

	// Call the function with an invalid number value
	err := InvalidQueryParamNumber(param, "notNumber", base.NewSchema(baseSchema), "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testQueryParam' is not a valid number")
	require.Contains(t, err.Reason, "the value 'notNumber' is not a valid number")
	require.Contains(t, err.HowToFix, "notNumber")
}

func TestInvalidQueryParamInteger(t *testing.T) {
	param := createMockParameter()
	baseSchema := createMockLowBaseSchema()

	// Call the function with an invalid number value
	err := InvalidQueryParamInteger(param, "notNumber", base.NewSchema(baseSchema), "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testQueryParam' is not a valid integer")
	require.Contains(t, err.Reason, "the value 'notNumber' is not a valid integer")
	require.Contains(t, err.HowToFix, "notNumber")
}

func TestIncorrectQueryParamEnum(t *testing.T) {
	enum := `enum: [fish, crab, lobster]`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.Value.Schema().Enum.KeyNode = &yaml.Node{}

	// Call the function with an invalid enum value
	err := IncorrectQueryParamEnum(param, "invalidEnum", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'testQueryParam' does not match allowed values")
	require.Contains(t, err.Reason, "'invalidEnum' is not one of those values")
	require.Contains(t, err.HowToFix, "fish, crab, lobster")
}

func TestIncorrectQueryParamEnumArray(t *testing.T) {
	enum := `items:
  enum: [fish, crab, lobster]`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.Value = schemaProxy
	param.GoLow().Schema.Value.Schema().Items.Value.A.Schema().Enum.Value = []low.ValueReference[*yaml.Node]{
		{Value: &yaml.Node{Value: "fish, crab, lobster"}},
	}

	// Call the function with an invalid enum value
	err := IncorrectQueryParamEnumArray(param, "invalidEnum", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query array parameter 'testQueryParam' does not match allowed values")
	require.Contains(t, err.Reason, "'invalidEnum' is not one of those values")
	require.Contains(t, err.HowToFix, "fish, crab, lobster")
}

func TestIncorrectReservedValues(t *testing.T) {
	enum := `name: bork`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Name = "borked::?^&*"

	err := IncorrectReservedValues(param, "borked::?^&*", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "borked::?^&*", err.ParameterName)
	require.Contains(t, err.Message, "Query parameter 'borked::?^&*' value contains reserved values")
	require.Contains(t, err.Reason, "The query parameter 'borked::?^&*' has 'allowReserved' set to false")
	require.Contains(t, err.HowToFix, "borked%3A%3A%3F%5E%26%2A")
}

func TestInvalidHeaderParamInteger(t *testing.T) {
	enum := `name: blip`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Name = "bunny"

	err := InvalidHeaderParamInteger(param, "bunmy", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "bunny", err.ParameterName)
	require.Contains(t, err.Message, "Header parameter 'bunny' is not a valid integer")
	require.Contains(t, err.Reason, "The header parameter 'bunny' is defined as being an integer")
	require.Contains(t, err.HowToFix, "bunmy")
}

func TestInvalidHeaderParamNumber(t *testing.T) {
	enum := `name: blip`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Name = "bunny"

	err := InvalidHeaderParamNumber(param, "bunmy", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "bunny", err.ParameterName)
	require.Contains(t, err.Message, "Header parameter 'bunny' is not a valid number")
	require.Contains(t, err.Reason, "The header parameter 'bunny' is defined as being a number")
	require.Contains(t, err.HowToFix, "bunmy")
}

func TestInvalidCookieParamNumber(t *testing.T) {
	enum := `name: blip`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Name = "cookies"

	err := InvalidCookieParamNumber(param, "milky", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Equal(t, "cookies", err.ParameterName)
	require.Contains(t, err.Message, "Cookie parameter 'cookies' is not a valid number")
	require.Contains(t, err.Reason, "The cookie parameter 'cookies' is defined as being a number")
	require.Contains(t, err.HowToFix, "milky")
}

func TestInvalidCookieParamInteger(t *testing.T) {
	enum := `name: blip`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Name = "cookies"

	err := InvalidCookieParamInteger(param, "milky", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Equal(t, "cookies", err.ParameterName)
	require.Contains(t, err.Message, "Cookie parameter 'cookies' is not a valid integer")
	require.Contains(t, err.Reason, "The cookie parameter 'cookies' is defined as being an integer")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectHeaderParamBool(t *testing.T) {
	enum := `name: blip`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Name = "cookies"

	err := IncorrectHeaderParamBool(param, "milky", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "cookies", err.ParameterName)
	require.Contains(t, err.Message, "Header parameter 'cookies' is not a valid boolean")
	require.Contains(t, err.Reason, "The header parameter 'cookies' is defined as being a boolean")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectCookieParamBool(t *testing.T) {
	enum := `name: blip`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Name = "cookies"

	err := IncorrectCookieParamBool(param, "milky", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Equal(t, "cookies", err.ParameterName)
	require.Contains(t, err.Message, "Cookie parameter 'cookies' is not a valid boolean")
	require.Contains(t, err.Reason, "The cookie parameter 'cookies' is defined as being a boolean")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectCookieParamEnum(t *testing.T) {
	enum := `enum: [fish, crab, lobster]
items:
  enum: [fish, crab, lobster]`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(enum), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.Value = schemaProxy
	param.GoLow().Schema.Value.Schema().Enum.Value = []low.ValueReference[*yaml.Node]{
		{Value: &yaml.Node{Value: "fish, crab, lobster"}},
	}
	param.GoLow().Schema.Value.Schema().Enum.KeyNode = &yaml.Node{}

	err := IncorrectCookieParamEnum(param, "milky", highSchema, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Cookie parameter 'testQueryParam' does not match allowed values")
	require.Contains(t, err.Reason, "The cookie parameter 'testQueryParam' has pre-defined values set via an enum")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectHeaderParamArrayBoolean(t *testing.T) {
	items := `items:
  type: boolean`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	highSchema.GoLow().Items.Value.A.Schema()

	param := createMockParameter()
	param.Name = "bubbles"

	err := IncorrectHeaderParamArrayBoolean(param, "milky", highSchema, nil, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "bubbles", err.ParameterName)
	require.Contains(t, err.Message, "Header array parameter 'bubbles' is not a valid boolean")
	require.Contains(t, err.Reason, "The header parameter (which is an array) 'bubbles' is defined as being a boolean")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectHeaderParamArrayNumber(t *testing.T) {
	items := `items:
  type: number`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	highSchema.GoLow().Items.Value.A.Schema()

	param := createMockParameter()
	param.Name = "bubbles"

	err := IncorrectHeaderParamArrayNumber(param, "milky", highSchema, nil, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Equal(t, "bubbles", err.ParameterName)
	require.Contains(t, err.Message, "Header array parameter 'bubbles' is not a valid number")
	require.Contains(t, err.Reason, "The header parameter (which is an array) 'bubbles' is defined as being a number")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectPathParamBool(t *testing.T) {
	items := `items:
  type: number`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.KeyNode = &yaml.Node{}

	err := IncorrectPathParamBool(param, "milky", highSchema, "/test-path", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Path parameter 'testQueryParam' is not a valid boolean")
	require.Contains(t, err.Reason, "The path parameter 'testQueryParam' is defined as being a boolean")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectPathParamEnum(t *testing.T) {
	items := `enum: [fish, crab, lobster]
items:
  enum: [fish, crab, lobster]`

	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.Value = schemaProxy
	param.GoLow().Schema.Value.Schema().Enum.Value = []low.ValueReference[*yaml.Node]{
		{Value: &yaml.Node{Value: "fish, crab, lobster"}},
	}
	param.GoLow().Schema.Value.Schema().Enum.KeyNode = &yaml.Node{}

	err := IncorrectPathParamEnum(param, "milky", highSchema, "/test-path", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Path parameter 'testQueryParam' does not match allowed values")
	require.Contains(t, err.Reason, "The path parameter 'testQueryParam' has pre-defined values set via an enum")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectPathParamNumber(t *testing.T) {
	items := `items:
  type: number`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.KeyNode = &yaml.Node{}

	err := IncorrectPathParamNumber(param, "milky", highSchema, "/test-path", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Path parameter 'testQueryParam' is not a valid number")
	require.Contains(t, err.Reason, "The path parameter 'testQueryParam' is defined as being a number")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectPathParamInteger(t *testing.T) {
	items := `items:
  type: integer`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.KeyNode = &yaml.Node{}

	err := IncorrectPathParamInteger(param, "milky", highSchema, "/test-path", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Path parameter 'testQueryParam' is not a valid integer")
	require.Contains(t, err.Reason, "The path parameter 'testQueryParam' is defined as being an integer")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectPathParamArrayNumber(t *testing.T) {
	items := `items:
  type: number`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	highSchema.GoLow().Items.Value.A.Schema()

	param := createMockParameter()
	param.Name = "bubbles"

	err := IncorrectPathParamArrayNumber(param, "milky", highSchema, nil, "/test-path", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "bubbles", err.ParameterName)
	require.Contains(t, err.Message, "Path array parameter 'bubbles' is not a valid number")
	require.Contains(t, err.Reason, "The path parameter (which is an array) 'bubbles' is defined as being a number")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectPathParamArrayInteger(t *testing.T) {
	items := `items:
  type: integer`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	highSchema.GoLow().Items.Value.A.Schema()

	param := createMockParameter()
	param.Name = "bubbles"

	err := IncorrectPathParamArrayInteger(param, "milky", highSchema, nil, "/test-path", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "bubbles", err.ParameterName)
	require.Contains(t, err.Message, "Path array parameter 'bubbles' is not a valid integer")
	require.Contains(t, err.Reason, "The path parameter (which is an array) 'bubbles' is defined as being an integer")
	require.Contains(t, err.HowToFix, "milky")
}

func TestIncorrectPathParamArrayBoolean(t *testing.T) {
	items := `items:
  type: number`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	highSchema.GoLow().Items.Value.A.Schema()

	param := createMockParameter()
	param.Name = "bubbles"

	err := IncorrectPathParamArrayBoolean(param, "milky", highSchema, nil, "/test-path", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "bubbles", err.ParameterName)
	require.Contains(t, err.Message, "Path array parameter 'bubbles' is not a valid boolean")
	require.Contains(t, err.Reason, "The path parameter (which is an array) 'bubbles' is defined as being a boolean")
	require.Contains(t, err.HowToFix, "milky")
}

func TestPathParameterMissing(t *testing.T) {
	items := `required: 
  - testQueryParam`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.KeyNode = &yaml.Node{}

	err := PathParameterMissing(param, "/test/{testQueryParam}", "/test/")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationPath, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Path parameter 'testQueryParam' is missing")
	require.Contains(t, err.Reason, "The path parameter 'testQueryParam' is defined as being required")
	require.Contains(t, err.HowToFix, "Ensure the value has been set")
}

func TestPathParameterMaxItems(t *testing.T) {
	items := `maxItems: 5
items:
  type: string`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.KeyNode = &yaml.Node{}

	err := IncorrectParamArrayMaxNumItems(param, param.Schema.Schema(), 10, 25, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query array parameter 'testQueryParam' has too many items")
	require.Contains(t, err.Reason, "The query parameter (which is an array) 'testQueryParam' has a maximum item length of 10, however the request provided 25 items")
	require.Contains(t, err.HowToFix, "Reduce the number of items in the array to 10 or less")
}

func TestPathParameterMinItems(t *testing.T) {
	items := `minItems: 5
items:
  type: string`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.KeyNode = &yaml.Node{}

	err := IncorrectParamArrayMinNumItems(param, param.Schema.Schema(), 10, 5, "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query array parameter 'testQueryParam' does not have enough items")
	require.Contains(t, err.Reason, "The query parameter (which is an array) 'testQueryParam' has a minimum items length of 10, however the request provided 5 items")
	require.Contains(t, err.HowToFix, "Increase the number of items in the array to 10 or more")
}

func TestPathParameterUniqueItems(t *testing.T) {
	items := `uniqueItems: true
items:
  type: string`
	var n yaml.Node
	_ = yaml.Unmarshal([]byte(items), &n)

	schemaProxy := &lowbase.SchemaProxy{}
	require.NoError(t, schemaProxy.Build(context.Background(), n.Content[0], n.Content[0], nil))

	highSchema := base.NewSchema(schemaProxy.Schema())
	param := createMockParameter()
	param.Schema = base.CreateSchemaProxy(highSchema)
	param.GoLow().Schema.KeyNode = &yaml.Node{}

	err := IncorrectParamArrayUniqueItems(param, param.Schema.Schema(), "fish, cake", "/test-path", "get", "{}")

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Equal(t, "testQueryParam", err.ParameterName)
	require.Contains(t, err.Message, "Query array parameter 'testQueryParam' contains non-unique items")
	require.Contains(t, err.Reason, "The query parameter (which is an array) 'testQueryParam' contains the following duplicates: 'fish, cake'")
	require.Contains(t, err.HowToFix, "Ensure the array values are all unique")
}

// createMinimalParameter creates a parameter with nil GoLow nodes to test nil safety.
func createMinimalParameter() *v3.Parameter {
	param := &lowv3.Parameter{
		Name: low.NodeReference[string]{Value: "minParam"},
		// All node references intentionally left with nil KeyNode/ValueNode
	}
	return v3.NewParameter(param)
}

func TestParameterErrors_NilGoLowNodes(t *testing.T) {
	// Tests that all parameter error constructors handle nil GoLow nodes
	// without panicking. This covers the crash scenario from wiretap #134.
	param := createMinimalParameter()
	qp := &helpers.QueryParam{
		Key:    "minParam",
		Values: []string{"value"},
	}
	sch := &base.Schema{}

	t.Run("IncorrectFormEncoding", func(t *testing.T) {
		err := IncorrectFormEncoding(param, qp, 0)
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectSpaceDelimiting", func(t *testing.T) {
		err := IncorrectSpaceDelimiting(param, qp)
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPipeDelimiting", func(t *testing.T) {
		err := IncorrectPipeDelimiting(param, qp)
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidDeepObject", func(t *testing.T) {
		err := InvalidDeepObject(param, qp)
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidDeepObjectPathConflict", func(t *testing.T) {
		err := InvalidDeepObjectPathConflict(param, qp, &helpers.QueryParam{
			Key:          "test",
			Values:       []string{"ok"},
			Property:     "foo",
			PropertyPath: []string{"foo", "bar"},
		})
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("QueryParameterMissing", func(t *testing.T) {
		err := QueryParameterMissing(param, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("HeaderParameterMissing", func(t *testing.T) {
		err := HeaderParameterMissing(param, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("CookieParameterMissing", func(t *testing.T) {
		err := CookieParameterMissing(param, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("HeaderParameterCannotBeDecoded", func(t *testing.T) {
		err := HeaderParameterCannotBeDecoded(param, "bad", "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectHeaderParamEnum", func(t *testing.T) {
		err := IncorrectHeaderParamEnum(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectQueryParamEnum", func(t *testing.T) {
		err := IncorrectQueryParamEnum(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectCookieParamEnum", func(t *testing.T) {
		err := IncorrectCookieParamEnum(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPathParamEnum", func(t *testing.T) {
		err := IncorrectPathParamEnum(param, "bad", sch, "/test", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectQueryParamBool", func(t *testing.T) {
		err := IncorrectQueryParamBool(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidQueryParamInteger", func(t *testing.T) {
		err := InvalidQueryParamInteger(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidQueryParamNumber", func(t *testing.T) {
		err := InvalidQueryParamNumber(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectReservedValues", func(t *testing.T) {
		err := IncorrectReservedValues(param, "a:b", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidHeaderParamInteger", func(t *testing.T) {
		err := InvalidHeaderParamInteger(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidHeaderParamNumber", func(t *testing.T) {
		err := InvalidHeaderParamNumber(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidCookieParamInteger", func(t *testing.T) {
		err := InvalidCookieParamInteger(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("InvalidCookieParamNumber", func(t *testing.T) {
		err := InvalidCookieParamNumber(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectHeaderParamBool", func(t *testing.T) {
		err := IncorrectHeaderParamBool(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectCookieParamBool", func(t *testing.T) {
		err := IncorrectCookieParamBool(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPathParamBool", func(t *testing.T) {
		err := IncorrectPathParamBool(param, "bad", sch, "/test", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPathParamInteger", func(t *testing.T) {
		err := IncorrectPathParamInteger(param, "bad", sch, "/test", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPathParamNumber", func(t *testing.T) {
		err := IncorrectPathParamNumber(param, "bad", sch, "/test", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectParamEncodingJSON", func(t *testing.T) {
		err := IncorrectParamEncodingJSON(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectQueryParamEnumArray", func(t *testing.T) {
		err := IncorrectQueryParamEnumArray(param, "bad", sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("PathParameterMissing", func(t *testing.T) {
		err := PathParameterMissing(param, "/test/{id}", "/test/123")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})
}

func TestParameterErrors_NilSchemaItems(t *testing.T) {
	// Tests array parameter error constructors with nil Items in schema.
	param := createMinimalParameter()
	sch := &base.Schema{} // no Items set

	t.Run("IncorrectQueryParamArrayBoolean", func(t *testing.T) {
		err := IncorrectQueryParamArrayBoolean(param, "bad", sch, sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectParamArrayMaxNumItems", func(t *testing.T) {
		err := IncorrectParamArrayMaxNumItems(param, sch, 5, 10, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectParamArrayMinNumItems", func(t *testing.T) {
		err := IncorrectParamArrayMinNumItems(param, sch, 5, 2, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectParamArrayUniqueItems", func(t *testing.T) {
		err := IncorrectParamArrayUniqueItems(param, sch, "dup", "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectCookieParamArrayBoolean", func(t *testing.T) {
		err := IncorrectCookieParamArrayBoolean(param, "bad", sch, sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectQueryParamArrayInteger", func(t *testing.T) {
		err := IncorrectQueryParamArrayInteger(param, "bad", sch, sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectQueryParamArrayNumber", func(t *testing.T) {
		err := IncorrectQueryParamArrayNumber(param, "bad", sch, sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectCookieParamArrayNumber", func(t *testing.T) {
		err := IncorrectCookieParamArrayNumber(param, "bad", sch, sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectHeaderParamArrayBoolean", func(t *testing.T) {
		err := IncorrectHeaderParamArrayBoolean(param, "bad", sch, sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectHeaderParamArrayNumber", func(t *testing.T) {
		err := IncorrectHeaderParamArrayNumber(param, "bad", sch, sch, "/test", "get", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPathParamArrayNumber", func(t *testing.T) {
		err := IncorrectPathParamArrayNumber(param, "bad", sch, sch, "/test", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPathParamArrayInteger", func(t *testing.T) {
		err := IncorrectPathParamArrayInteger(param, "bad", sch, sch, "/test", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})

	t.Run("IncorrectPathParamArrayBoolean", func(t *testing.T) {
		err := IncorrectPathParamArrayBoolean(param, "bad", sch, sch, "/test", "{}")
		require.NotNil(t, err)
		require.Equal(t, 1, err.SpecLine)
		require.Equal(t, 0, err.SpecCol)
	})
}
