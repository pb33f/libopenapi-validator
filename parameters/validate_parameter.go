// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package parameters

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	stdError "errors"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/schema_validation"
)

const parameterSchemaVersion = 3.1

func ValidateSingleParameterSchema(
	schema *base.Schema,
	rawObject any,
	entity string,
	reasonEntity string,
	name string,
	validationType string,
	subValType string,
	o *config.ValidationOptions,
	pathTemplate string,
	operation string,
) (validationErrors []*errors.ValidationError) {
	var jsch *jsonschema.Schema
	var referenceSchema string

	// Try cache lookup first - avoids expensive schema compilation on each request
	if o != nil && o.SchemaCache != nil && schema != nil && schema.GoLow() != nil {
		hash := schema_validation.SchemaCacheKey(
			schema.GoLow().Hash(),
			parameterSchemaVersion,
			schema_validation.SchemaValidationPurposeGeneric,
		)
		if cached, ok := o.SchemaCache.Load(hash); ok && cached != nil && cached.CompiledSchema != nil {
			jsch = cached.CompiledSchema
			referenceSchema = cached.ReferenceSchema
		}
	}

	// Cache miss - compile the schema
	if jsch == nil {
		compiled, err := schema_validation.CompileSchemaForValidation(
			schema,
			schema_validation.SchemaValidationPurposeGeneric,
			o,
			parameterSchemaVersion,
		)
		if err != nil {
			return validationErrors
		}
		if compiled == nil || compiled.CompiledSchema == nil {
			return validationErrors
		}
		jsch = compiled.CompiledSchema
		referenceSchema = compiled.ReferenceSchema

		// Store in cache for future requests
		if o != nil && o.SchemaCache != nil && schema != nil && schema.GoLow() != nil {
			hash := schema_validation.SchemaCacheKey(
				schema.GoLow().Hash(),
				parameterSchemaVersion,
				schema_validation.SchemaValidationPurposeGeneric,
			)
			o.SchemaCache.Store(hash, compiled.ToCacheEntry(schema))
		}
	}

	// Validate the object and report any errors.
	scErrs := jsch.Validate(rawObject)
	var werras *jsonschema.ValidationError
	if stdError.As(scErrs, &werras) {
		validationErrors = formatJsonSchemaValidationError(
			schema, werras, entity, reasonEntity, name,
			validationType, subValType, pathTemplate, operation, referenceSchema,
		)
	}
	return validationErrors
}

// GetRenderedSchema returns a YAML string representation of the schema for error messages.
// It first checks the schema cache for a pre-rendered version, falling back to fresh rendering.
// This avoids expensive re-rendering on each validation when the cache is available.
func GetRenderedSchema(schema *base.Schema, opts *config.ValidationOptions) string {
	if schema == nil {
		return ""
	}

	// Try cache lookup first
	if opts != nil && opts.SchemaCache != nil && schema.GoLow() != nil {
		hash := schema.GoLow().Hash()
		if cached, ok := opts.SchemaCache.Load(hash); ok && cached != nil && len(cached.RenderedInline) > 0 {
			return string(cached.RenderedInline)
		}
	}

	// Cache miss - render fresh as YAML using validation mode
	renderCtx := base.NewInlineRenderContextForValidation()
	rendered, _ := schema.RenderInlineWithContext(renderCtx)
	return string(rendered)
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
	subValType string,
	validationOptions *config.ValidationOptions,
) []*errors.ValidationError {
	var validationErrors []*errors.ValidationError
	var jsch *jsonschema.Schema
	var referenceSchema string

	// Try cache lookup first - avoids expensive schema compilation on each request
	if validationOptions != nil && validationOptions.SchemaCache != nil && schema != nil && schema.GoLow() != nil {
		hash := schema_validation.SchemaCacheKey(
			schema.GoLow().Hash(),
			parameterSchemaVersion,
			schema_validation.SchemaValidationPurposeGeneric,
		)
		if cached, ok := validationOptions.SchemaCache.Load(hash); ok && cached != nil && cached.CompiledSchema != nil {
			jsch = cached.CompiledSchema
			referenceSchema = cached.ReferenceSchema
		}
	}

	// Cache miss - render and compile the schema
	if jsch == nil {
		compiled, err := schema_validation.CompileSchemaForValidation(
			schema,
			schema_validation.SchemaValidationPurposeGeneric,
			validationOptions,
			parameterSchemaVersion,
		)
		if err != nil {
			// schema compilation failed, return validation error instead of panicking
			validationErrors = append(validationErrors, &errors.ValidationError{
				ValidationType:    validationType,
				ValidationSubType: subValType,
				Message:           fmt.Sprintf("%s '%s' failed schema compilation", entity, name),
				Reason: fmt.Sprintf("%s '%s' schema compilation failed: %s",
					reasonEntity, name, err.Error()),
				SpecLine:      1,
				SpecCol:       0,
				ParameterName: name,
				HowToFix:      "check the parameter schema for invalid JSON Schema syntax, complex regex patterns, or unsupported schema constructs",
				Context:       schema,
			})
			return validationErrors
		}
		if compiled == nil || compiled.CompiledSchema == nil {
			return validationErrors
		}
		jsch = compiled.CompiledSchema
		referenceSchema = compiled.ReferenceSchema

		// Store in cache for future requests
		if validationOptions != nil && validationOptions.SchemaCache != nil && schema != nil && schema.GoLow() != nil {
			hash := schema_validation.SchemaCacheKey(
				schema.GoLow().Hash(),
				parameterSchemaVersion,
				schema_validation.SchemaValidationPurposeGeneric,
			)
			validationOptions.SchemaCache.Store(hash, compiled.ToCacheEntry(schema))
		}
	}

	// 3. decode the object into a json blob.
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
		err := json.Unmarshal([]byte(decodedString), &decodedObj)
		if err != nil {
			decodedObj = rawBlob
		}
		validEncoding = true
	}

	// 4. validate the object against the schema
	var scErrs error
	if validEncoding {
		p := decodedObj
		if rawIsMap {
			if g, ko := rawObject.(map[string]interface{}); ko {
				if len(g) == 0 || (g[""] != nil && g[""] == "") {
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
		validationErrors = formatJsonSchemaValidationError(
			schema, werras, entity, reasonEntity, name,
			validationType, subValType, "", "", referenceSchema,
		)
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
					SpecLine: schema.GoLow().RootNode.Line,
					SpecCol:  schema.GoLow().RootNode.Column,
					HowToFix: errors.HowToFixDecodingError,
				})
			}
		}
	}
	return validationErrors
}

func formatJsonSchemaValidationError(
	schema *base.Schema,
	scErrs *jsonschema.ValidationError,
	entity string,
	reasonEntity string,
	name string,
	validationType string,
	subValType string,
	pathTemplate string,
	operation string,
	referenceSchema string,
) (validationErrors []*errors.ValidationError) {
	// flatten the validationErrors
	schFlatErrs := helpers.FlattenSchemaOutputErrors(scErrs.DetailedOutput())
	var schemaValidationErrors []*errors.SchemaValidationFailure
	for q := range schFlatErrs {
		er := schFlatErrs[q]

		errMsg := er.Error.Kind.LocalizedString(message.NewPrinter(language.Tag{}))
		if er.KeywordLocation == "" || helpers.IgnoreRegex.MatchString(errMsg) {
			continue // ignore this error, it's not useful
		}

		// Construct full OpenAPI path for KeywordLocation if pathTemplate and operation are provided
		keywordLocation := er.KeywordLocation
		if pathTemplate != "" && operation != "" && validationType == helpers.ParameterValidation {
			// er.KeywordLocation is relative to the schema (e.g., "/minLength" or "/enum")
			keyword := strings.TrimPrefix(er.KeywordLocation, "/")
			keywordLocation = helpers.ConstructParameterJSONPointer(pathTemplate, operation, name, keyword)
		}

		fail := &errors.SchemaValidationFailure{
			Reason:                  errMsg,
			FieldName:               helpers.ExtractFieldNameFromStringLocation(er.InstanceLocation),
			FieldPath:               helpers.ExtractJSONPathFromStringLocation(er.InstanceLocation),
			InstancePath:            helpers.ConvertStringLocationToPathSegments(er.InstanceLocation),
			KeywordLocation:         keywordLocation,
			OriginalJsonSchemaError: scErrs,
		}
		if referenceSchema != "" {
			fail.ReferenceSchema = referenceSchema
		} else if schema != nil {
			renderCtx := base.NewInlineRenderContextForValidation()
			rendered, err := schema.RenderInlineWithContext(renderCtx)
			if err == nil && rendered != nil {
				fail.ReferenceSchema = string(rendered)
			}
		}
		schemaValidationErrors = append(schemaValidationErrors, fail)
	}
	schemaType := "undefined"
	line := 0
	col := 0
	if len(schema.Type) > 0 {
		schemaType = schema.Type[0]
		line = schema.GoLow().Type.KeyNode.Line
		col = schema.GoLow().Type.KeyNode.Column
	} else {
		var sTypes []string
		seen := make(map[string]struct{})
		extractTypes := func(s *base.SchemaProxy) {
			pSch := s.Schema()
			if pSch != nil {
				for _, typ := range pSch.Type {
					if _, ok := seen[typ]; !ok {
						sTypes = append(sTypes, typ)
						seen[typ] = struct{}{}
					}
				}
			}
		}
		processPoly := func(schemas []*base.SchemaProxy) {
			for _, s := range schemas {
				extractTypes(s)
			}
		}

		// check if there is polymorphism going on here.
		if len(schema.AnyOf) > 0 || len(schema.AllOf) > 0 || len(schema.OneOf) > 0 {
			processPoly(schema.AnyOf)
			processPoly(schema.AllOf)
			processPoly(schema.OneOf)

			sep := "or"
			if len(schema.AllOf) > 0 {
				sep = "and a"
			}
			schemaType = strings.Join(sTypes, fmt.Sprintf(" %s ", sep))
		}

		line = schema.GoLow().RootNode.Line
		col = schema.GoLow().RootNode.Column
	}

	validationErrors = append(validationErrors, &errors.ValidationError{
		ValidationType:    validationType,
		ValidationSubType: subValType,
		Message:           fmt.Sprintf("%s '%s' failed to validate", entity, name),
		Reason: fmt.Sprintf("%s '%s' is defined as an %s, "+
			"however it failed to pass a schema validation", reasonEntity, name, schemaType),
		SpecLine:               line,
		SpecCol:                col,
		ParameterName:          name,
		SchemaValidationErrors: schemaValidationErrors,
		HowToFix:               errors.HowToFixInvalidSchema,
	})
	return validationErrors
}
