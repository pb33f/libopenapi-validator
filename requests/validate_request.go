// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/schema_validation"
)

var instanceLocationRegex = regexp.MustCompile(`^/(\d+)`)

// ValidateRequestSchema will validate a http.Request pointer against a schema.
// If validation fails, it will return a list of validation errors as the second return value.
// If compiledSchema is provided (non-nil), it will be used directly, skipping compilation.
func ValidateRequestSchema(
	request *http.Request,
	schema *base.Schema,
	renderedSchema,
	jsonSchema []byte,
	version float32,
	compiledSchema *jsonschema.Schema,
	opts ...config.Option,
) (bool, []*errors.ValidationError) {
	var validationErrors []*errors.ValidationError

	var requestBody []byte
	if request != nil && request.Body != nil {
		requestBody, _ = io.ReadAll(request.Body)

		// close the request body, so it can be re-read later by another player in the chain
		_ = request.Body.Close()
		request.Body = io.NopCloser(bytes.NewBuffer(requestBody))

	}

	var decodedObj interface{}

	if len(requestBody) > 0 {
		err := json.Unmarshal(requestBody, &decodedObj)
		if err != nil {
			// cannot decode the request body, so it's not valid
			violation := &errors.SchemaValidationFailure{
				Reason:          err.Error(),
				Location:        "unavailable",
				ReferenceSchema: string(renderedSchema),
				ReferenceObject: string(requestBody),
			}
			validationErrors = append(validationErrors, &errors.ValidationError{
				ValidationType:    helpers.RequestBodyValidation,
				ValidationSubType: helpers.Schema,
				Message: fmt.Sprintf("%s request body for '%s' failed to validate schema",
					request.Method, request.URL.Path),
				Reason:                 fmt.Sprintf("The request body cannot be decoded: %s", err.Error()),
				SpecLine:               1,
				SpecCol:                0,
				SchemaValidationErrors: []*errors.SchemaValidationFailure{violation},
				HowToFix:               errors.HowToFixInvalidSchema,
				Context:                string(renderedSchema), // attach the rendered schema to the error
			})
			return false, validationErrors
		}
	}

	// no request body? but we do have a schema?
	if len(requestBody) == 0 && len(jsonSchema) > 0 {

		line := schema.ParentProxy.GetSchemaKeyNode().Line
		col := schema.ParentProxy.GetSchemaKeyNode().Line
		if schema.Type != nil {
			line = schema.GoLow().Type.KeyNode.Line
			col = schema.GoLow().Type.KeyNode.Column
		}

		// cannot decode the request body, so it's not valid
		violation := &errors.SchemaValidationFailure{
			Reason:          "request body is empty, but there is a schema defined",
			ReferenceSchema: string(renderedSchema),
			ReferenceObject: string(requestBody),
		}
		validationErrors = append(validationErrors, &errors.ValidationError{
			ValidationType:    helpers.RequestBodyValidation,
			ValidationSubType: helpers.Schema,
			Message: fmt.Sprintf("%s request body is empty for '%s'",
				request.Method, request.URL.Path),
			Reason:                 "The request body is empty but there is a schema defined",
			SpecLine:               line,
			SpecCol:                col,
			SchemaValidationErrors: []*errors.SchemaValidationFailure{violation},
			HowToFix:               errors.HowToFixInvalidSchema,
			Context:                string(renderedSchema), // attach the rendered schema to the error
		})
		return false, validationErrors
	}

	// Use pre-compiled schema if available, otherwise compile now (for backward compatibility)
	var jsch *jsonschema.Schema
	if compiledSchema != nil {
		// Use the cached pre-compiled schema - this is the optimization!
		jsch = compiledSchema
	} else {
		// Compile the schema (for direct calls to this function without pre-compilation)
		validationOptions := config.NewValidationOptions(opts...)
		var err error
		jsch, err = helpers.NewCompiledSchemaWithVersion("requestBody", jsonSchema, validationOptions, version)
		if err != nil {
			validationErrors = append(validationErrors, &errors.ValidationError{
				ValidationType:    helpers.RequestBodyValidation,
				ValidationSubType: helpers.Schema,
				Message:           err.Error(),
				Reason:            "Failed to compile the request body schema.",
				Context:           string(jsonSchema),
			})
			return false, validationErrors
		}
	}

	// validate the object against the schema
	scErrs := jsch.Validate(decodedObj)
	if scErrs != nil {

		jk := scErrs.(*jsonschema.ValidationError)

		// flatten the validationErrors
		schFlatErrs := jk.BasicOutput().Errors
		var schemaValidationErrors []*errors.SchemaValidationFailure

		// re-encode the schema.
		var renderedNode yaml.Node
		_ = yaml.Unmarshal(renderedSchema, &renderedNode)
		for q := range schFlatErrs {
			er := schFlatErrs[q]

			errMsg := er.Error.Kind.LocalizedString(message.NewPrinter(language.Tag{}))

			if er.KeywordLocation == "" || helpers.IgnoreRegex.MatchString(errMsg) {
				continue // ignore this error, it's useless tbh, utter noise.
			}
			if er.Error != nil {

				// locate the violated property in the schema
				located := schema_validation.LocateSchemaPropertyNodeByJSONPath(renderedNode.Content[0], er.KeywordLocation)

				// extract the element specified by the instance
				val := instanceLocationRegex.FindStringSubmatch(er.InstanceLocation)
				var referenceObject string

				if len(val) > 0 {
					referenceIndex, _ := strconv.Atoi(val[1])
					if reflect.ValueOf(decodedObj).Type().Kind() == reflect.Slice {
						found := decodedObj.([]any)[referenceIndex]
						recoded, _ := json.MarshalIndent(found, "", "  ")
						referenceObject = string(recoded)
					}
				}
				if referenceObject == "" {
					referenceObject = string(requestBody)
				}

				errMsg := er.Error.Kind.LocalizedString(message.NewPrinter(language.Tag{}))

				violation := &errors.SchemaValidationFailure{
					Reason:          errMsg,
					Location:        er.KeywordLocation,
					FieldName:       helpers.ExtractFieldNameFromStringLocation(er.InstanceLocation),
					FieldPath:       helpers.ExtractJSONPathFromStringLocation(er.InstanceLocation),
					InstancePath:    helpers.ConvertStringLocationToPathSegments(er.InstanceLocation),
					ReferenceSchema: string(renderedSchema),
					ReferenceObject: referenceObject,
					OriginalError:   jk,
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

		line := 1
		col := 0
		if schema.GoLow().Type.KeyNode != nil {
			line = schema.GoLow().Type.KeyNode.Line
			col = schema.GoLow().Type.KeyNode.Column
		}

		// add the error to the list
		validationErrors = append(validationErrors, &errors.ValidationError{
			ValidationType:    helpers.RequestBodyValidation,
			ValidationSubType: helpers.Schema,
			Message: fmt.Sprintf("%s request body for '%s' failed to validate schema",
				request.Method, request.URL.Path),
			Reason: "The request body is defined as an object. " +
				"However, it does not meet the schema requirements of the specification",
			SpecLine:               line,
			SpecCol:                col,
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
