// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"encoding/json"
	stdError "errors"
	"fmt"
	"github.com/pb33f/libopenapi-validator/helpers"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"net/url"
	"reflect"
	"strings"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/utils"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func ValidateSingleParameterSchema(
	schema *base.Schema,
	rawObject any,
	entity string,
	reasonEntity string,
	name string,
	validationType string,
	subValType string,
) (validationErrors []*errors.ValidationError) {
	jsch := compileSchema(name, buildJsonRender(schema))

	scErrs := jsch.Validate(rawObject)
	var werras *jsonschema.ValidationError
	if stdError.As(scErrs, &werras) {
		validationErrors = formatJsonSchemaValidationError(schema, werras, entity, reasonEntity, name, validationType, subValType)
	}
	return validationErrors
}

// compileSchema create a new json schema compiler and add the schema to it.
func compileSchema(name string, jsonSchema []byte) *jsonschema.Schema {
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(helpers.NewCompilerLoader())
	decodedSchema, _ := jsonschema.UnmarshalJSON(strings.NewReader(string(jsonSchema))) // decode the schema into a json blob
	_ = compiler.AddResource(fmt.Sprintf("%s.json", name), decodedSchema)
	jsch, _ := compiler.Compile(fmt.Sprintf("%s.json", name))
	return jsch
}

// buildJsonRender build a JSON render of the schema.
func buildJsonRender(schema *base.Schema) []byte {
	renderedSchema, _ := schema.Render()
	jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)
	return jsonSchema
}

// ValidateParameterSchema will validate a parameter against a raw object, or a blob of json/yaml.
// It will return a list of validation errors, if any.
//
//	schema: the schema to validate against
//	rawObject: the object to validate (leave empty if using a blob)
//	rawBlob: the blob to validate (leave empty if using an object)
//	entity: the entity being validated
//	reasonEntity: the entity that caused the validation to be called
//	name: the name of the parameter
//	validationType: the type of validation being performed
//	subValType: the type of sub-validation being performed
func ValidateParameterSchema(
	schema *base.Schema,
	rawObject any,
	rawBlob,
	entity,
	reasonEntity,
	name,
	validationType,
	subValType string) []*errors.ValidationError {

	var validationErrors []*errors.ValidationError

	// 1. build a JSON render of the schema.
	renderedSchema, _ := schema.RenderInline()
	jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)

	// 2. decode the object into a json blob.
	var decodedObj interface{}
	rawIsMap := false
	validEncoding := false
	if rawObject != nil {
		// check what type of object it is
		ot := reflect.TypeOf(rawObject)
		var ok bool
		switch ot.Kind() {
		case reflect.Map:
			if decodedObj, ok = rawObject.(map[string]interface{}); ok {
				rawIsMap = true
				validEncoding = true
			} else {
				rawIsMap = true
			}
		}
	} else {
		decodedString, _ := url.QueryUnescape(rawBlob)
		_ = json.Unmarshal([]byte(decodedString), &decodedObj)
		validEncoding = true
	}
	// 3. create a new json schema compiler and add the schema to it
	compiler := jsonschema.NewCompiler()

	decodedSchema, _ := jsonschema.UnmarshalJSON(strings.NewReader(string(jsonSchema)))
	_ = compiler.AddResource(fmt.Sprintf("%s.json", name), decodedSchema)
	jsch, _ := compiler.Compile(fmt.Sprintf("%s.json", name))

	// 4. validate the object against the schema
	var scErrs error
	if validEncoding {
		p := decodedObj
		if rawIsMap {
			if g, ko := rawObject.(map[string]interface{}); ko {
				if len(g) == 0 || g[""] != nil {
					p = nil
				}
			}
		}
		if p != nil {

			// check if any of the items have an empty key
			skip := false
			if rawIsMap {
				for k := range p.(map[string]interface{}) {
					if k == "" {
						validationErrors = append(validationErrors, &errors.ValidationError{
							ValidationType:    validationType,
							ValidationSubType: subValType,
							Message:           fmt.Sprintf("%s '%s' failed to validate", entity, name),
							Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
								"however it failed to pass a schema validation", reasonEntity, name),
							SpecLine:               schema.GoLow().Type.KeyNode.Line,
							SpecCol:                schema.GoLow().Type.KeyNode.Column,
							SchemaValidationErrors: nil,
							HowToFix:               errors.HowToFixInvalidSchema,
						})
						skip = true
						break
					}
				}
			}
			if !skip {
				scErrs = jsch.Validate(p)
			}
		}
	}
	var werras *jsonschema.ValidationError
	if stdError.As(scErrs, &werras) {
		validationErrors = formatJsonSchemaValidationError(schema, werras, entity, reasonEntity, name, validationType, subValType)
	}

	// if there are no validationErrors, check that the supplied value is even JSON
	if len(validationErrors) == 0 {
		if rawIsMap {
			if !validEncoding {
				// add the error to the list
				validationErrors = append(validationErrors, &errors.ValidationError{
					ValidationType:    validationType,
					ValidationSubType: subValType,
					Message:           fmt.Sprintf("%s '%s' cannot be decoded", entity, name),
					Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
						"however it failed to be decoded as an object", reasonEntity, name),
					SpecLine: schema.GoLow().Type.KeyNode.Line,
					SpecCol:  schema.GoLow().Type.KeyNode.Column,
					HowToFix: errors.HowToFixDecodingError,
				})
			}
		}
	}
	return validationErrors
}

func formatJsonSchemaValidationError(schema *base.Schema, scErrs *jsonschema.ValidationError, entity string, reasonEntity string, name string, validationType string, subValType string) (validationErrors []*errors.ValidationError) {
	// flatten the validationErrors
	schFlatErrs := scErrs.BasicOutput().Errors
	var schemaValidationErrors []*errors.SchemaValidationFailure
	for q := range schFlatErrs {
		er := schFlatErrs[q]

		errMsg := er.Error.Kind.LocalizedString(message.NewPrinter(language.Tag{}))
		if er.KeywordLocation == "" || helpers.IgnoreRegex.MatchString(errMsg) {
			continue // ignore this error, it's not useful
		}

		fail := &errors.SchemaValidationFailure{
			Reason:   errMsg,
			Location: er.KeywordLocation,
			//OriginalError: scErrs,
		}
		if schema != nil {
			rendered, err := schema.RenderInline()
			if err == nil && rendered != nil {
				fail.ReferenceSchema = fmt.Sprintf("%s", rendered)
			}
		}
		schemaValidationErrors = append(schemaValidationErrors, fail)
	}
	schemaType := "undefined"
	if len(schema.Type) > 0 {
		schemaType = schema.Type[0]
	}
	validationErrors = append(validationErrors, &errors.ValidationError{
		ValidationType:    validationType,
		ValidationSubType: subValType,
		Message:           fmt.Sprintf("%s '%s' failed to validate", entity, name),
		Reason: fmt.Sprintf("%s '%s' is defined as an %s, "+
			"however it failed to pass a schema validation", reasonEntity, name, schemaType),
		SpecLine:               schema.GoLow().Type.KeyNode.Line,
		SpecCol:                schema.GoLow().Type.KeyNode.Column,
		SchemaValidationErrors: schemaValidationErrors,
		HowToFix:               errors.HowToFixInvalidSchema,
	})
	return validationErrors
}
