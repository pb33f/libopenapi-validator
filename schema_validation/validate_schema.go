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
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"reflect"
	"regexp"
	"strconv"
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

var instanceLocationRegex = regexp.MustCompile(`^/(\d+)`)

type schemaValidator struct {
	logger *zap.SugaredLogger
}

// NewSchemaValidator will create a new SchemaValidator instance, ready to accept schemas and payloads to validate.
func NewSchemaValidator() SchemaValidator {
	logger, _ := zap.NewProduction()
	return &schemaValidator{logger: logger.Sugar()}
}

func (s *schemaValidator) ValidateSchemaString(schema *base.Schema, payload string) (bool, []*errors.ValidationError) {
	return validateSchema(schema, []byte(payload), nil, s.logger)
}

func (s *schemaValidator) ValidateSchemaObject(schema *base.Schema, payload interface{}) (bool, []*errors.ValidationError) {
	return validateSchema(schema, nil, payload, s.logger)
}

func (s *schemaValidator) ValidateSchemaBytes(schema *base.Schema, payload []byte) (bool, []*errors.ValidationError) {
	return validateSchema(schema, payload, nil, s.logger)
}

func validateSchema(schema *base.Schema, payload []byte, decodedObject interface{}, log *zap.SugaredLogger) (bool, []*errors.ValidationError) {

	var validationErrors []*errors.ValidationError

	if schema == nil {
		log.Infoln("schema is empty and cannot be validated. This generally means the schema is missing from the spec, or could not be read.")
		return false, validationErrors
	}

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

					// extract the element specified by the instance
					val := instanceLocationRegex.FindStringSubmatch(er.InstanceLocation)
					var referenceObject string

					if len(val) > 0 {
						referenceIndex, _ := strconv.Atoi(val[1])
						if reflect.ValueOf(decodedObject).Type().Kind() == reflect.Slice {
							found := decodedObject.([]any)[referenceIndex]
							recoded, _ := json.MarshalIndent(found, "", "  ")
							referenceObject = string(recoded)
						}
					}
					if referenceObject == "" {
						referenceObject = string(payload)
					}

					violation := &errors.SchemaValidationFailure{
						Reason:           er.Error,
						Location:         er.InstanceLocation,
						DeepLocation:     er.KeywordLocation,
						AbsoluteLocation: er.AbsoluteKeywordLocation,
						ReferenceSchema:  string(renderedSchema),
						ReferenceObject:  referenceObject,
						OriginalError:    jk,
					}
					// if we have a location within the schema, add it to the error
					if located != nil {
						line := located.Line
						// if the located node is a map or an array, then the actual human interpretable
						// line on which the violation occurred is the line of the key, not the value.
						if located.Kind == yaml.MappingNode || located.Kind == yaml.SequenceNode {
							if line > 0 {
								line--
							}
						}

						// location of the violation within the rendered schema.
						violation.Line = line
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
