// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"reflect"
	"regexp"
	"strconv"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	_ "embed"

	"github.com/pb33f/libopenapi-validator/config"
	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

// SchemaValidator is an interface that defines the methods for validating a *base.Schema (V3+ Only) object.
// There are 6 methods for validating a schema:
//
//	ValidateSchemaString accepts a schema object to validate against, and a JSON/YAML blob that is defined as a string.
//	ValidateSchemaObject accepts a schema object to validate against, and an object, created from unmarshalled JSON/YAML.
//	ValidateSchemaBytes accepts a schema object to validate against, and a JSON/YAML blob that is defined as a byte array.
//	ValidateSchemaStringWithVersion - version-aware validation that allows OpenAPI 3.0 keywords when version is specified.
//	ValidateSchemaObjectWithVersion - version-aware validation that allows OpenAPI 3.0 keywords when version is specified.
//	ValidateSchemaBytesWithVersion - version-aware validation that allows OpenAPI 3.0 keywords when version is specified.
type SchemaValidator interface {
	// ValidateSchemaString accepts a schema object to validate against, and a JSON/YAML blob that is defined as a string.
	// Uses OpenAPI 3.1+ validation by default (strict JSON Schema compliance).
	ValidateSchemaString(schema *base.Schema, payload string) (bool, []*liberrors.ValidationError)

	// ValidateSchemaObject accepts a schema object to validate against, and an object, created from unmarshalled JSON/YAML.
	// This is a pre-decoded object that will skip the need to unmarshal a string of JSON/YAML.
	// Uses OpenAPI 3.1+ validation by default (strict JSON Schema compliance).
	ValidateSchemaObject(schema *base.Schema, payload interface{}) (bool, []*liberrors.ValidationError)

	// ValidateSchemaBytes accepts a schema object to validate against, and a byte slice containing a schema to
	// validate against. Uses OpenAPI 3.1+ validation by default (strict JSON Schema compliance).
	ValidateSchemaBytes(schema *base.Schema, payload []byte) (bool, []*liberrors.ValidationError)

	// ValidateSchemaStringWithVersion accepts a schema object to validate against, a JSON/YAML blob, and an OpenAPI version.
	// When version is 3.0, OpenAPI 3.0-specific keywords like 'nullable' are allowed and processed.
	// When version is 3.1+, OpenAPI 3.0-specific keywords like 'nullable' will cause validation to fail.
	ValidateSchemaStringWithVersion(schema *base.Schema, payload string, version float32) (bool, []*liberrors.ValidationError)

	// ValidateSchemaObjectWithVersion accepts a schema object to validate against, an object, and an OpenAPI version.
	// When version is 3.0, OpenAPI 3.0-specific keywords like 'nullable' are allowed and processed.
	// When version is 3.1+, OpenAPI 3.0-specific keywords like 'nullable' will cause validation to fail.
	ValidateSchemaObjectWithVersion(schema *base.Schema, payload interface{}, version float32) (bool, []*liberrors.ValidationError)

	// ValidateSchemaBytesWithVersion accepts a schema object to validate against, a byte slice, and an OpenAPI version.
	// When version is 3.0, OpenAPI 3.0-specific keywords like 'nullable' are allowed and processed.
	// When version is 3.1+, OpenAPI 3.0-specific keywords like 'nullable' will cause validation to fail.
	ValidateSchemaBytesWithVersion(schema *base.Schema, payload []byte, version float32) (bool, []*liberrors.ValidationError)
}

var instanceLocationRegex = regexp.MustCompile(`^/(\d+)`)

type schemaValidator struct {
	options *config.ValidationOptions
	logger  *slog.Logger
}

// NewSchemaValidatorWithLogger will create a new SchemaValidator instance, ready to accept schemas and payloads to validate.
func NewSchemaValidatorWithLogger(logger *slog.Logger, opts ...config.Option) SchemaValidator {
	options := config.NewValidationOptions(opts...)

	return &schemaValidator{options: options, logger: logger}
}

// NewSchemaValidator will create a new SchemaValidator instance, ready to accept schemas and payloads to validate.
func NewSchemaValidator(opts ...config.Option) SchemaValidator {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	return NewSchemaValidatorWithLogger(logger, opts...)
}

func (s *schemaValidator) ValidateSchemaString(schema *base.Schema, payload string) (bool, []*liberrors.ValidationError) {
	return s.validateSchemaWithVersion(schema, []byte(payload), nil, s.logger, 3.1)
}

func (s *schemaValidator) ValidateSchemaObject(schema *base.Schema, payload interface{}) (bool, []*liberrors.ValidationError) {
	return s.validateSchemaWithVersion(schema, nil, payload, s.logger, 3.1)
}

func (s *schemaValidator) ValidateSchemaBytes(schema *base.Schema, payload []byte) (bool, []*liberrors.ValidationError) {
	return s.validateSchemaWithVersion(schema, payload, nil, s.logger, 3.1)
}

func (s *schemaValidator) ValidateSchemaStringWithVersion(schema *base.Schema, payload string, version float32) (bool, []*liberrors.ValidationError) {
	return s.validateSchemaWithVersion(schema, []byte(payload), nil, s.logger, version)
}

func (s *schemaValidator) ValidateSchemaObjectWithVersion(schema *base.Schema, payload interface{}, version float32) (bool, []*liberrors.ValidationError) {
	return s.validateSchemaWithVersion(schema, nil, payload, s.logger, version)
}

func (s *schemaValidator) ValidateSchemaBytesWithVersion(schema *base.Schema, payload []byte, version float32) (bool, []*liberrors.ValidationError) {
	return s.validateSchemaWithVersion(schema, payload, nil, s.logger, version)
}

func (s *schemaValidator) validateSchemaWithVersion(schema *base.Schema, payload []byte, decodedObject interface{}, log *slog.Logger, version float32) (bool, []*liberrors.ValidationError) {
	var validationErrors []*liberrors.ValidationError

	if schema == nil {
		log.Info("schema is empty and cannot be validated. This generally means the schema is missing from the spec, or could not be read.")
		return false, validationErrors
	}

	var renderedSchema []byte
	var renderedNode *yaml.Node
	var resourceNodes map[string]*yaml.Node
	var compiledSchema *jsonschema.Schema

	// Check cache first — reuses existing SchemaCache (populated by NewValidationOptions).
	var cacheKey uint64
	canCache := s.options.SchemaCache != nil && schema.GoLow() != nil
	if canCache {
		// Include version in key so 3.0 (nullable) and 3.1 compile differently.
		cacheKey = schema.GoLow().Hash() ^ uint64(math.Float32bits(version))
		if cached, ok := s.options.SchemaCache.Load(cacheKey); ok &&
			cached != nil && cached.CompiledSchema != nil {
			renderedSchema = cached.RenderedInline
			renderedNode = cached.RenderedNode
			resourceNodes = cached.ResourceNodes
			compiledSchema = cached.CompiledSchema
		}
	}

	// Cache miss — render, convert to JSON, and compile.
	if compiledSchema == nil {
		compiled, compileErr := CompileSchemaForValidation(
			schema,
			SchemaValidationPurposeGeneric,
			s.options,
			version,
		)
		if compileErr != nil {
			line, col := schemaLineColumn(schema)
			validationErrors = append(validationErrors, &liberrors.ValidationError{
				ValidationType:    helpers.Schema,
				ValidationSubType: helpers.Schema,
				Message:           "schema compilation failed",
				Reason:            fmt.Sprintf("Schema compilation failed: %s", compileErr.Error()),
				SpecLine:          line,
				SpecCol:           col,
				HowToFix:          liberrors.HowToFixInvalidSchema,
				Context:           string(renderedSchema),
			})
			return false, validationErrors
		}
		renderedSchema = compiled.RenderedInline
		renderedNode = compiled.RenderedNode
		resourceNodes = compiled.ResourceNodes
		compiledSchema = compiled.CompiledSchema

		// Store in cache for subsequent validations of the same schema.
		if canCache && compiledSchema != nil {
			s.options.SchemaCache.Store(cacheKey, compiled.ToCacheEntry(schema))
		}
	}

	if decodedObject == nil && len(payload) > 0 {
		err := json.Unmarshal(payload, &decodedObject)
		if err != nil {
			// cannot decode the request body, so it's not valid
			line, col := schemaLineColumn(schema)
			validationErrors = append(validationErrors, &liberrors.ValidationError{
				ValidationType:    helpers.RequestBodyValidation,
				ValidationSubType: helpers.Schema,
				Message:           "schema does not pass validation",
				Reason:            fmt.Sprintf("The schema cannot be decoded: %s", err.Error()),
				SpecLine:          line,
				SpecCol:           col,
				HowToFix:          liberrors.HowToFixInvalidSchema,
				Context:           string(renderedSchema),
			})
			return false, validationErrors
		}
	}

	var schemaValidationErrors []*liberrors.SchemaValidationFailure

	if compiledSchema != nil && decodedObject != nil {
		scErrs := compiledSchema.Validate(decodedObject)
		if scErrs != nil {

			var jk *jsonschema.ValidationError
			if errors.As(scErrs, &jk) {

				// flatten the validationErrors
				schFlatErr := helpers.FlattenSchemaOutputErrors(jk.DetailedOutput())
				schemaValidationErrors = extractBasicErrors(schFlatErr, renderedSchema,
					renderedNode, resourceNodes, decodedObject, payload, jk, schemaValidationErrors)
			}
			line, col := schemaLineColumn(schema)

			validationErrors = append(validationErrors, &liberrors.ValidationError{
				ValidationType:         helpers.Schema,
				Message:                "schema does not pass validation",
				Reason:                 "Schema failed to validate against the contract requirements",
				SpecLine:               line,
				SpecCol:                col,
				SchemaValidationErrors: schemaValidationErrors,
				HowToFix:               liberrors.HowToFixInvalidSchema,
				Context:                string(renderedSchema),
			})
		}
	}
	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}

func schemaLineColumn(schema *base.Schema) (int, int) {
	if schema == nil || schema.GoLow() == nil || schema.GoLow().Type.KeyNode == nil {
		return 1, 0
	}
	return schema.GoLow().Type.KeyNode.Line, schema.GoLow().Type.KeyNode.Column
}

func extractBasicErrors(schFlatErrs []jsonschema.OutputUnit,
	renderedSchema []byte, renderedNode *yaml.Node,
	resourceNodes map[string]*yaml.Node,
	decodedObject interface{},
	payload []byte, jk *jsonschema.ValidationError,
	schemaValidationErrors []*liberrors.SchemaValidationFailure,
) []*liberrors.SchemaValidationFailure {
	// Extract property name info once before processing errors (performance optimization)
	propertyInfo := extractPropertyNameFromError(jk)

	// Determine root content node ONCE (not per-error).
	// NodeBuilder.Render() returns MappingNode directly, no DocumentNode unwrapping needed.
	var rootNode *yaml.Node
	if renderedNode != nil {
		rootNode = renderedNode
		if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
			rootNode = rootNode.Content[0]
		}
	} else if len(renderedSchema) > 0 {
		// Fallback: parse bytes ONCE
		var docNode yaml.Node
		_ = yaml.Unmarshal(renderedSchema, &docNode)
		if len(docNode.Content) > 0 {
			rootNode = docNode.Content[0]
		}
	}

	for q := range schFlatErrs {
		er := schFlatErrs[q]

		errMsg := er.Error.Kind.LocalizedString(message.NewPrinter(language.Tag{}))
		if helpers.IgnoreRegex.MatchString(errMsg) {
			continue // ignore this error, it's useless tbh, utter noise.
		}
		if er.Error != nil {

			// locate the violated property in the schema
			var located *yaml.Node
			if rootNode != nil {
				located = LocateSchemaPropertyNodeByJSONPathWithResources(
					rootNode,
					resourceNodes,
					er.KeywordLocation,
					er.AbsoluteKeywordLocation,
				)
			}

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
				Reason:                  errMsg,
				FieldName:               helpers.ExtractFieldNameFromStringLocation(er.InstanceLocation),
				FieldPath:               helpers.ExtractJSONPathFromStringLocation(er.InstanceLocation),
				InstancePath:            helpers.ConvertStringLocationToPathSegments(er.InstanceLocation),
				KeywordLocation:         er.KeywordLocation,
				ReferenceSchema:         string(renderedSchema),
				ReferenceObject:         referenceObject,
				OriginalJsonSchemaError: jk,
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
			} else if rootNode != nil {
				// handles property name validation errors that don't provide useful InstanceLocation
				applyPropertyNameFallback(propertyInfo, rootNode, violation)
			}
			schemaValidationErrors = append(schemaValidationErrors, violation)
		}
	}
	return schemaValidationErrors
}
