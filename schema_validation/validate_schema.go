// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"encoding/json"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/utils"
	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
	"gopkg.in/yaml.v3"
	"strings"
)

// SchemaValidator is an interface that defines the methods for validating a *base.Schema (V3+ Only) object.
// There are 3 methods for validating a schema:
//
//	ValidateSchemaString accepts a schema object to validate against, and a JSON/YAML blob that is defined as a string.
//	ValidateSchemaObject accepts a schema object to validate against, and an object, created from unmarshalled JSON/YAML.
//	ValidateSchemaBytes accepts a schema object to validate against, and a JSON/YAML blob that is defined as a byte array.
type SchemaValidator interface {

	// ValidateSchemaString accepts a schema object to validate against, and a JSON/YAML blob that is defined as a string.
	ValidateSchemaString(schema *base.Schema, payload string) (bool, []*errors.ValidationError)

	// ValidateSchemaObject accepts a schema object to validate against, and an object, created from unmarshalled JSON/YAML.
	// This is a pre-decoded object that will skip the need to unmarshal a string of JSON/YAML.
	ValidateSchemaObject(schema *base.Schema, payload interface{}) (bool, []*errors.ValidationError)

	// ValidateSchemaBytes accepts a schema object to validate against, and a byte slice containing a schema to
	// validate against.
	ValidateSchemaBytes(schema *base.Schema, payload []byte) (bool, []*errors.ValidationError)
}

type schemaValidator struct{}

// NewSchemaValidator will create a new SchemaValidator instance, ready to accept schemas and payloads to validate.
func NewSchemaValidator() SchemaValidator {
	return &schemaValidator{}
}

func (s *schemaValidator) ValidateSchemaString(schema *base.Schema, payload string) (bool, []*errors.ValidationError) {
	return validateSchema(schema, []byte(payload), nil)
}

func (s *schemaValidator) ValidateSchemaObject(schema *base.Schema, payload interface{}) (bool, []*errors.ValidationError) {
	return validateSchema(schema, nil, payload)
}

func (s *schemaValidator) ValidateSchemaBytes(schema *base.Schema, payload []byte) (bool, []*errors.ValidationError) {
	return validateSchema(schema, payload, nil)
}

func validateSchema(schema *base.Schema, payload []byte, decodedObject interface{}) (bool, []*errors.ValidationError) {

	var validationErrors []*errors.ValidationError

	// render the schema, to be used for validation
	renderedSchema, _ := schema.RenderInline()
	jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)

	if decodedObject == nil {
		_ = json.Unmarshal(payload, &decodedObject)
	}
	compiler := jsonschema.NewCompiler()
	_ = compiler.AddResource("schema.json", strings.NewReader(string(jsonSchema)))
	jsch, _ := compiler.Compile("schema.json")

	// 4. validate the object against the schema
	scErrs := jsch.Validate(decodedObject)
	if scErrs != nil {
		var schemaValidationErrors []*errors.SchemaValidationFailure

		// check for invalid JSON type errors.
		if _, ok := scErrs.(jsonschema.InvalidJSONTypeError); ok {
			violation := &errors.SchemaValidationFailure{
				Reason:   scErrs.Error(),
				Location: "unavailable", // we don't have a location for this error, so we'll just say it's unavailable.
			}
			schemaValidationErrors = append(schemaValidationErrors, violation)
		}

		if jk, ok := scErrs.(*jsonschema.ValidationError); ok {

			// flatten the validationErrors
			schFlatErrs := jk.BasicOutput().Errors

			for q := range schFlatErrs {
				er := schFlatErrs[q]
				if er.KeywordLocation == "" || strings.HasPrefix(er.Error, "doesn't validate with") {
					continue // ignore this error, it's useless tbh, utter noise.
				}
				if er.Error != "" {

					// re-encode the schema.
					var renderedNode yaml.Node
					_ = yaml.Unmarshal(renderedSchema, &renderedNode)

					// locate the violated property in the schema
					located := LocateSchemaPropertyNodeByJSONPath(renderedNode.Content[0], er.KeywordLocation)
					violation := &errors.SchemaValidationFailure{
						Reason:        er.Error,
						Location:      er.KeywordLocation,
						OriginalError: jk,
					}
					// if we have a location within the schema, add it to the error
					if located != nil {
						// location of the violation within the rendered schema.
						violation.Line = located.Line
						violation.Column = located.Column
					}
					schemaValidationErrors = append(schemaValidationErrors, violation)
				}
			}
		}

		// add the error to the list
		validationErrors = append(validationErrors, &errors.ValidationError{
			ValidationType:         helpers.Schema,
			Message:                "schema does not pass validation",
			Reason:                 "Schema failed to validate against the contract requirements",
			SpecLine:               schema.GoLow().Type.KeyNode.Line,
			SpecCol:                schema.GoLow().Type.KeyNode.Column,
			SchemaValidationErrors: schemaValidationErrors,
			HowToFix:               errors.HowToFixInvalidSchema,
			Context:                string(renderedSchema), // attach the rendered schema to the error
		})
	}
	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}
