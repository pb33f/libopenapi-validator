// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var instanceLocationRegex = regexp.MustCompile(`^/(\d+)`)

// ValidateRequestSchema will validate an http.Request pointer against a schema.
// If validation fails, it will return a list of validation errors as the second return value.
func ValidateRequestSchema(
	request *http.Request,
	schema *base.Schema,
	renderedSchema,
	jsonSchema []byte) (bool, []*errors.ValidationError) {

	var validationErrors []*errors.ValidationError

	requestBody, _ := io.ReadAll(request.Body)

	// close the request body, so it can be re-read later by another player in the chain
	_ = request.Body.Close()
	request.Body = io.NopCloser(bytes.NewBuffer(requestBody))

	var decodedObj interface{}
	_ = json.Unmarshal(requestBody, &decodedObj)

	// no request body? failed to decode anything? nothing to do here.
	if requestBody == nil || decodedObj == nil {
		return true, nil
	}

	compiler := jsonschema.NewCompiler()
	_ = compiler.AddResource("requestBody.json", strings.NewReader(string(jsonSchema)))
	jsch, _ := compiler.Compile("requestBody.json")

	// 4. validate the object against the schema
	scErrs := jsch.Validate(decodedObj)
	if scErrs != nil {
		jk := scErrs.(*jsonschema.ValidationError)

		// flatten the validationErrors
		schFlatErrs := jk.BasicOutput().Errors
		var schemaValidationErrors []*errors.SchemaValidationFailure
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

				violation := &errors.SchemaValidationFailure{
					Reason:          er.Error,
					Location:        er.KeywordLocation,
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

		// add the error to the list
		validationErrors = append(validationErrors, &errors.ValidationError{
			ValidationType:    helpers.RequestBodyValidation,
			ValidationSubType: helpers.Schema,
			Message: fmt.Sprintf("%s request body for '%s' failed to validate schema",
				request.Method, request.URL.Path),
			Reason: "The request body is defined as an object. " +
				"However, it does not meet the schema requirements of the specification",
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
