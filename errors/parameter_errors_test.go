// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package errors

import (
	"context"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"testing"

	"github.com/pb33f/libopenapi-validator/helpers"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/datamodel/low"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	lowv3 "github.com/pb33f/libopenapi/datamodel/low/v3"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Helper to create a mock v3.Parameter object with a schema
func createMockParameterWithSchema() *v3.Parameter {
	schemaProxy := &lowbase.SchemaProxy{}
	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
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
	require.Contains(t, err.Message, "Query parameter 'testParam' delimited incorrectly")
	require.Contains(t, err.Reason, "'pipeDelimited' style defined")
	require.Contains(t, err.HowToFix, "testParam=value1|value2")
}

func TestQueryParameterMissing(t *testing.T) {
	param := createMockParameterWithSchema()

	// Call the function
	err := QueryParameterMissing(param)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Contains(t, err.Message, "Query parameter 'testParam' is missing")
	require.Contains(t, err.Reason, "'testParam' is defined as being required")
	require.Equal(t, HowToFixMissingValue, err.HowToFix)
}

func TestHeaderParameterMissing(t *testing.T) {
	param := createMockParameterWithSchema()

	// Call the function
	err := HeaderParameterMissing(param)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Contains(t, err.Message, "Header parameter 'testParam' is missing")
	require.Contains(t, err.Reason, "'testParam' is defined as being required")
	require.Equal(t, HowToFixMissingValue, err.HowToFix)
}

func TestHeaderParameterCannotBeDecoded(t *testing.T) {
	param := createMockParameterWithSchema()
	val := "malformed_header_value"

	// Call the function
	err := HeaderParameterCannotBeDecoded(param, val)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Contains(t, err.Message, "Header parameter 'testParam' cannot be decoded")
	require.Contains(t, err.Reason, "'malformed_header_value' is malformed")
	require.Equal(t, HowToFixInvalidEncoding, err.HowToFix)
}

func TestIncorrectHeaderParamEnum(t *testing.T) {
	param := createMockParameterWithSchema()
	schemaProxy := &lowbase.SchemaProxy{}
	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
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
	err := IncorrectHeaderParamEnum(param, "invalidEnum", schema)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationHeader, err.ValidationSubType)
	require.Contains(t, err.Message, "Header parameter 'testParam' does not match allowed values")
	require.Contains(t, err.Reason, "'invalidEnum' is not one of those values")
	require.Equal(t, 10, err.SpecLine)
	require.Equal(t, 20, err.SpecCol)
	require.Contains(t, err.HowToFix, "enum1, enum2")
}

func TestIncorrectQueryParamArrayBoolean(t *testing.T) {

	param := createMockParameterWithSchema()
	schemaProxy := &lowbase.SchemaProxy{}
	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
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
	err := IncorrectQueryParamArrayBoolean(param, "notBoolean", schema, schema.Items.A.Schema())

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
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
	require.Contains(t, err.Message, "Query parameter 'testParam' is not a valid deepObject")
	require.Contains(t, err.Reason, "'testParam' has the 'deepObject' style defined")
	require.Contains(t, err.HowToFix, "testParam=value1|value2")
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

	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
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
	err := IncorrectCookieParamArrayBoolean(param, "notBoolean", s, itemsSchema)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
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

	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
	schemaProxy.Schema().Type = low.NodeReference[lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]]{
		KeyNode:   &yaml.Node{},
		ValueNode: &yaml.Node{},
		Value: lowbase.SchemaDynamicValue[string, []low.ValueReference[string]]{
			A: "string",
		},
	}

	return itemsSchema
}

func TestIncorrectQueryParamArrayNumber(t *testing.T) {
	// Create mock parameter and schemas
	param := createMockParameterForNumberArray()
	baseSchema := createMockLowBaseSchemaForNumberArray()
	s := base.NewSchema(baseSchema)
	itemsSchema := base.NewSchema(baseSchema.Items.Value.A.Schema())

	// Call the function with an invalid number value in the array
	err := IncorrectQueryParamArrayNumber(param, "notNumber", s, itemsSchema)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
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

	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
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
	err := IncorrectCookieParamArrayNumber(param, "notNumber", s, itemsSchema)

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationCookie, err.ValidationSubType)
	require.Contains(t, err.Message, "Cookie array parameter 'testCookieParam' is not a valid number")
	require.Contains(t, err.Reason, "the value 'notNumber' is not a valid number")
	require.Contains(t, err.HowToFix, "notNumber")
}

// Helper function to create a mock v3.Parameter
func createMockParameter() *v3.Parameter {

	schemaProxy := &lowbase.SchemaProxy{}
	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)

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
			Value: schemaProxy,
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

	schemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
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
	err := IncorrectParamEncodingJSON(param, "invalidJSON", base.NewSchema(baseSchema))

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Contains(t, err.Message, "Query parameter 'testQueryParam' is not valid JSON")
	require.Contains(t, err.Reason, "the value 'invalidJSON' is not valid JSON")
	require.Equal(t, HowToFixInvalidJSON, err.HowToFix)
}

func TestIncorrectQueryParamBool(t *testing.T) {
	param := createMockParameter()
	baseSchema := createMockLowBaseSchema()

	lschemaProxy := &lowbase.SchemaProxy{}
	lschemaProxy.Build(context.Background(), &yaml.Node{}, &yaml.Node{}, nil)
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
	err := IncorrectQueryParamBool(param, "notBoolean", base.NewSchema(baseSchema))

	// Validate the error
	require.NotNil(t, err)
	require.Equal(t, helpers.ParameterValidation, err.ValidationType)
	require.Equal(t, helpers.ParameterValidationQuery, err.ValidationSubType)
	require.Contains(t, err.Message, "Query parameter 'testQueryParam' is not a valid boolean")
	require.Contains(t, err.Reason, "the value 'notBoolean' is not a valid boolean")
	require.Contains(t, err.HowToFix, "true/false")
}
