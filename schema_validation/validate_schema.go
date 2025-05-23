// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"sync"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/utils"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"

	_ "embed"

	"github.com/pb33f/libopenapi-validator/config"
	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

// SchemaValidator is an interface that defines the methods for validating a *base.Schema (V3+ Only) object.
// There are 3 methods for validating a schema:
//
//	ValidateSchemaString accepts a schema object to validate against, and a JSON/YAML blob that is defined as a string.
//	ValidateSchemaObject accepts a schema object to validate against, and an object, created from unmarshalled JSON/YAML.
//	ValidateSchemaBytes accepts a schema object to validate against, and a JSON/YAML blob that is defined as a byte array.
type SchemaValidator interface {
	// ValidateSchemaString accepts a schema object to validate against, and a JSON/YAML blob that is defined as a string.
	ValidateSchemaString(schema *base.Schema, payload string) (bool, []*liberrors.ValidationError)

	// ValidateSchemaObject accepts a schema object to validate against, and an object, created from unmarshalled JSON/YAML.
	// This is a pre-decoded object that will skip the need to unmarshal a string of JSON/YAML.
	ValidateSchemaObject(schema *base.Schema, payload interface{}) (bool, []*liberrors.ValidationError)

	// ValidateSchemaBytes accepts a schema object to validate against, and a byte slice containing a schema to
	// validate against.
	ValidateSchemaBytes(schema *base.Schema, payload []byte) (bool, []*liberrors.ValidationError)
}

var instanceLocationRegex = regexp.MustCompile(`^/(\d+)`)

type schemaValidator struct {
	options *config.ValidationOptions
	logger  *slog.Logger
	lock    sync.Mutex
}

// NewSchemaValidatorWithLogger will create a new SchemaValidator instance, ready to accept schemas and payloads to validate.
func NewSchemaValidatorWithLogger(logger *slog.Logger, opts ...config.Option) SchemaValidator {
	options := config.NewValidationOptions(opts...)

	return &schemaValidator{options: options, logger: logger, lock: sync.Mutex{}}
}

// NewSchemaValidator will create a new SchemaValidator instance, ready to accept schemas and payloads to validate.
func NewSchemaValidator(opts ...config.Option) SchemaValidator {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	return NewSchemaValidatorWithLogger(logger, opts...)
}

func (s *schemaValidator) ValidateSchemaString(schema *base.Schema, payload string) (bool, []*liberrors.ValidationError) {
	return s.validateSchema(schema, []byte(payload), nil, s.logger)
}

func (s *schemaValidator) ValidateSchemaObject(schema *base.Schema, payload interface{}) (bool, []*liberrors.ValidationError) {
	return s.validateSchema(schema, nil, payload, s.logger)
}

func (s *schemaValidator) ValidateSchemaBytes(schema *base.Schema, payload []byte) (bool, []*liberrors.ValidationError) {
	return s.validateSchema(schema, payload, nil, s.logger)
}

func (s *schemaValidator) validateSchema(schema *base.Schema, payload []byte, decodedObject interface{}, log *slog.Logger) (bool, []*liberrors.ValidationError) {
	var validationErrors []*liberrors.ValidationError

	if schema == nil {
		log.Info("schema is empty and cannot be validated. This generally means the schema is missing from the spec, or could not be read.")
		return false, validationErrors
	}

	// extract index of schema, and check the version
	// schemaIndex := schema.GoLow().Index
	var renderedSchema []byte

	// render the schema, to be used for validation, stop this from running concurrently, mutations are made to state
	// and, it will cause async issues.
	s.lock.Lock()
	renderedSchema, _ = schema.RenderInline()
	s.lock.Unlock()

	jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)

	if decodedObject == nil && len(payload) > 0 {
		err := json.Unmarshal(payload, &decodedObject)
		if err != nil {
			// cannot decode the request body, so it's not valid
			violation := &liberrors.SchemaValidationFailure{
				Reason:          err.Error(),
				Location:        "unavailable",
				ReferenceSchema: string(renderedSchema),
				ReferenceObject: string(payload),
			}
			validationErrors = append(validationErrors, &liberrors.ValidationError{
				ValidationType:         helpers.RequestBodyValidation,
				ValidationSubType:      helpers.Schema,
				Message:                "schema does not pass validation",
				Reason:                 fmt.Sprintf("The schema cannot be decoded: %s", err.Error()),
				SpecLine:               1,
				SpecCol:                0,
				SchemaValidationErrors: []*liberrors.SchemaValidationFailure{violation},
				HowToFix:               liberrors.HowToFixInvalidSchema,
				Context:                string(renderedSchema), // attach the rendered schema to the error
			})
			return false, validationErrors
		}

	}

	// Build the compiled JSON Schema
	jsch, err := helpers.NewCompiledSchema("schema", jsonSchema, s.options)

	var schemaValidationErrors []*liberrors.SchemaValidationFailure

	// is the schema even valid? did it compile?
	if err != nil {
		var ve *jsonschema.SchemaValidationError
		if errors.As(err, &ve) {
			if ve != nil {

				// no, this won't work, so we need to extract the errors and return them.
				// basicErrors := ve.BasicOutput().Errors
				// schemaValidationErrors = extractBasicErrors(basicErrors, renderedSchema, decodedObject, payload, ve, schemaValidationErrors)
				// cannot compile schema, so it's not valid
				violation := &liberrors.SchemaValidationFailure{
					Reason:          err.Error(),
					Location:        "unavailable",
					ReferenceSchema: string(renderedSchema),
					ReferenceObject: string(payload),
				}
				validationErrors = append(validationErrors, &liberrors.ValidationError{
					ValidationType:         helpers.RequestBodyValidation,
					ValidationSubType:      helpers.Schema,
					Message:                "schema does not pass validation",
					Reason:                 fmt.Sprintf("The schema cannot be decoded: %s", err.Error()),
					SpecLine:               1,
					SpecCol:                0,
					SchemaValidationErrors: []*liberrors.SchemaValidationFailure{violation},
					HowToFix:               liberrors.HowToFixInvalidSchema,
					Context:                string(renderedSchema), // attach the rendered schema to the error
				})
				return false, validationErrors
			}
		}
	}

	// 4. validate the object against the schema
	if jsch != nil && decodedObject != nil {
		scErrs := jsch.Validate(decodedObject)
		if scErrs != nil {

			var jk *jsonschema.ValidationError
			if errors.As(scErrs, &jk) {

				// flatten the validationErrors
				schFlatErr := jk.BasicOutput().Errors
				schemaValidationErrors = extractBasicErrors(schFlatErr, renderedSchema,
					decodedObject, payload, jk, schemaValidationErrors)
			}
			line := 1
			col := 0
			if schema.GoLow().Type.KeyNode != nil {
				line = schema.GoLow().Type.KeyNode.Line
				col = schema.GoLow().Type.KeyNode.Column
			}

			// add the error to the list
			validationErrors = append(validationErrors, &liberrors.ValidationError{
				ValidationType:         helpers.Schema,
				Message:                "schema does not pass validation",
				Reason:                 "Schema failed to validate against the contract requirements",
				SpecLine:               line,
				SpecCol:                col,
				SchemaValidationErrors: schemaValidationErrors,
				HowToFix:               liberrors.HowToFixInvalidSchema,
				Context:                string(renderedSchema), // attach the rendered schema to the error
			})
		}
	}
	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}

func extractBasicErrors(schFlatErrs []jsonschema.OutputUnit,
	renderedSchema []byte, decodedObject interface{},
	payload []byte, jk *jsonschema.ValidationError,
	schemaValidationErrors []*liberrors.SchemaValidationFailure,
) []*liberrors.SchemaValidationFailure {
	for q := range schFlatErrs {
		er := schFlatErrs[q]

		errMsg := er.Error.Kind.LocalizedString(message.NewPrinter(language.Tag{}))
		if helpers.IgnoreRegex.MatchString(errMsg) {
			continue // ignore this error, it's useless tbh, utter noise.
		}
		if er.Error != nil {

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

			violation := &liberrors.SchemaValidationFailure{
				Reason:           errMsg,
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
	return schemaValidationErrors
}
