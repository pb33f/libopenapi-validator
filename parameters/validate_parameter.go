// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"encoding/json"
	"fmt"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/utils"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"net/url"
	"reflect"
	"strings"
)

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
	renderedSchema, _ := schema.Render()
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
	_ = compiler.AddResource(fmt.Sprintf("%s.json", name), strings.NewReader(string(jsonSchema)))
	jsch, _ := compiler.Compile(fmt.Sprintf("%s.json", name))

	// 4. validate the object against the schema
	var scErrs error
	if validEncoding {
		scErrs = jsch.Validate(decodedObj)
	}
	if scErrs != nil {
		jk := scErrs.(*jsonschema.ValidationError)

		// flatten the validationErrors
		schFlatErrs := jk.BasicOutput().Errors
		var schemaValidationErrors []*errors.SchemaValidationFailure
		for q := range schFlatErrs {
			er := schFlatErrs[q]
			if er.KeywordLocation == "" || strings.HasPrefix(er.Error, "doesn't validate with") {
				continue // ignore this error, it's not useful
			}
			schemaValidationErrors = append(schemaValidationErrors, &errors.SchemaValidationFailure{
				Reason:        er.Error,
				Location:      er.KeywordLocation,
				OriginalError: jk,
			})
		}
		// add the error to the list
		validationErrors = append(validationErrors, &errors.ValidationError{
			ValidationType:    validationType,
			ValidationSubType: subValType,
			Message:           fmt.Sprintf("%s '%s' failed to validate", entity, name),
			Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
				"however it failed to pass a schema validation", reasonEntity, name),
			SpecLine:               schema.GoLow().Type.KeyNode.Line,
			SpecCol:                schema.GoLow().Type.KeyNode.Column,
			SchemaValidationErrors: schemaValidationErrors,
			HowToFix:               errors.HowToFixInvalidSchema,
		})
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
