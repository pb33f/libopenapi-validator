// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/pb33f/libopenapi-validator/config"
	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

type nonStringMappingKey struct {
	Value    string
	Tag      string
	Path     []string
	Line     int
	Column   int
	Sequence bool
}

func normalizeJSON(data any) (any, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var normalized any
	_ = json.Unmarshal(d, &normalized)
	return normalized, nil
}

func findNonStringMappingKey(rootNode *yaml.Node) *nonStringMappingKey {
	if rootNode == nil {
		return nil
	}
	return findNonStringMappingKeyInNode(rootNode, nil)
}

func findNonStringMappingKeyInNode(node *yaml.Node, path []string) *nonStringMappingKey {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if found := findNonStringMappingKeyInNode(child, path); found != nil {
				return found
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			if isMergeMappingKey(keyNode) {
				if found := findNonStringMappingKeyInMergeValue(valueNode, path); found != nil {
					return found
				}
				continue
			}
			nextPath := appendPathSegment(path, keyNode.Value)
			if !isStringMappingKey(keyNode) {
				return &nonStringMappingKey{
					Value:    keyNode.Value,
					Tag:      keyNode.ShortTag(),
					Path:     nextPath,
					Line:     keyNode.Line,
					Column:   keyNode.Column,
					Sequence: keyNode.Kind == yaml.SequenceNode,
				}
			}
			if found := findNonStringMappingKeyInNode(valueNode, nextPath); found != nil {
				return found
			}
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if found := findNonStringMappingKeyInNode(child, appendPathSegment(path, strconv.Itoa(i))); found != nil {
				return found
			}
		}
	}

	return nil
}

func findNonStringMappingKeyInMergeValue(node *yaml.Node, path []string) *nonStringMappingKey {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.AliasNode:
		return findNonStringMappingKeyInMergeValue(node.Alias, path)
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if found := findNonStringMappingKeyInMergeValue(child, path); found != nil {
				return found
			}
		}
		return nil
	default:
		return findNonStringMappingKeyInNode(node, path)
	}
}

func isStringMappingKey(keyNode *yaml.Node) bool {
	if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
		return false
	}
	return keyNode.ShortTag() == "!!str"
}

func isMergeMappingKey(keyNode *yaml.Node) bool {
	if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
		return false
	}
	return keyNode.ShortTag() == "!!merge" && keyNode.Value == "<<"
}

func appendPathSegment(path []string, segment string) []string {
	next := make([]string, 0, len(path)+1)
	next = append(next, path...)
	return append(next, segment)
}

func buildJSONPointer(path []string) string {
	if len(path) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, segment := range path {
		builder.WriteByte('/')
		builder.WriteString(helpers.EscapeJSONPointerSegment(segment))
	}
	return builder.String()
}

func buildNonStringMappingKeyError(key *nonStringMappingKey) *liberrors.ValidationError {
	pointer := buildJSONPointer(key.Path)
	reason := fmt.Sprintf("OpenAPI documents require string mapping keys, but found %s key %q at %s",
		yamlKeyType(key), key.Value, pointer)
	howToFix := "Quote YAML mapping keys that should be strings, because OpenAPI documents must be representable as JSON objects"

	if isOperationResponseStatusCodeKey(key.Path) {
		reason = fmt.Sprintf("Response status code keys must be strings, quote %s as %q at %s",
			key.Value, key.Value, pointer)
		howToFix = fmt.Sprintf("Quote the response status code key, for example use %q instead of %s",
			key.Value, key.Value)
	}

	return &liberrors.ValidationError{
		ValidationType:    helpers.Schema,
		ValidationSubType: "document",
		Message:           "OpenAPI document validation failed",
		Reason:            reason,
		SpecLine:          key.Line,
		SpecCol:           key.Column,
		HowToFix:          howToFix,
		Context:           pointer,
	}
}

func yamlKeyType(key *nonStringMappingKey) string {
	if key == nil {
		return "non-string"
	}
	if key.Sequence {
		return "sequence"
	}
	return strings.TrimPrefix(key.Tag, "!!")
}

func isOperationResponseStatusCodeKey(path []string) bool {
	if len(path) < 5 || path[0] != "paths" || path[len(path)-2] != "responses" {
		return false
	}
	for _, segment := range path[2 : len(path)-2] {
		if isHTTPMethod(segment) {
			return true
		}
	}
	return false
}

func isHTTPMethod(segment string) bool {
	switch strings.ToLower(segment) {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	default:
		return false
	}
}

func buildDocumentDecodeError(reason, context string) *liberrors.ValidationError {
	return &liberrors.ValidationError{
		ValidationType:    helpers.Schema,
		ValidationSubType: "document",
		Message:           "OpenAPI document validation failed",
		Reason:            reason,
		SpecLine:          1,
		SpecCol:           0,
		HowToFix:          "ensure the OpenAPI document is valid YAML/JSON and can be represented as JSON",
		Context:           context,
	}
}

// ValidateOpenAPIDocument will validate an OpenAPI document against the OpenAPI 2, 3.0 and 3.1 schemas (depending on version)
// It will return true if the document is valid, false if it is not and a slice of ValidationError pointers.
func ValidateOpenAPIDocument(doc libopenapi.Document, opts ...config.Option) (bool, []*liberrors.ValidationError) {
	return ValidateOpenAPIDocumentWithPrecompiled(doc, nil, opts...)
}

// ValidateOpenAPIDocumentWithPrecompiled validates an OpenAPI document against the OAS JSON Schema.
// When compiledSchema is non-nil it is used directly, skipping schema compilation.
// When SpecJSONBytes is available on the document's SpecInfo, the normalizeJSON round-trip is
// bypassed in favour of a single jsonschema.UnmarshalJSON call.
func ValidateOpenAPIDocumentWithPrecompiled(doc libopenapi.Document, compiledSchema *jsonschema.Schema, opts ...config.Option) (bool, []*liberrors.ValidationError) {
	options := config.NewValidationOptions(opts...)

	info := doc.GetSpecInfo()
	loadedSchema := info.APISchema
	var validationErrors []*liberrors.ValidationError

	// Check if both JSON representations are nil before proceeding
	if info.SpecJSON == nil && info.SpecJSONBytes == nil {
		validationErrors = append(validationErrors, &liberrors.ValidationError{
			ValidationType:    helpers.Schema,
			ValidationSubType: "document",
			Message:           "OpenAPI document validation failed",
			Reason:            "The document's SpecJSON is nil, indicating the document was not properly parsed or is empty",
			SpecLine:          1,
			SpecCol:           0,
			HowToFix:          "ensure the OpenAPI document is valid YAML/JSON and can be properly parsed by libopenapi",
			Context:           "document root",
		})
		return false, validationErrors
	}

	if info.RootNode != nil {
		if invalidKey := findNonStringMappingKey(info.RootNode); invalidKey != nil {
			return false, []*liberrors.ValidationError{buildNonStringMappingKeyError(invalidKey)}
		}
	}

	// Use the precompiled schema if provided, otherwise compile it
	jsch := compiledSchema
	if jsch == nil {
		var err error
		jsch, err = helpers.NewCompiledSchema("schema", []byte(loadedSchema), options)
		if err != nil {
			validationErrors = append(validationErrors, &liberrors.ValidationError{
				ValidationType:    helpers.Schema,
				ValidationSubType: "compilation",
				Message:           "OpenAPI document schema compilation failed",
				Reason:            fmt.Sprintf("The OpenAPI schema failed to compile: %s", err.Error()),
				SpecLine:          1,
				SpecCol:           0,
				HowToFix:          "check the OpenAPI schema for invalid JSON Schema syntax, complex regex patterns, or unsupported schema constructs",
				Context:           loadedSchema,
			})
			return false, validationErrors
		}
	}

	// Build the normalized document value for validation.
	// Prefer SpecJSONBytes (single unmarshal) over SpecJSON (marshal+unmarshal round-trip).
	var normalized any
	if info.SpecJSONBytes != nil && len(*info.SpecJSONBytes) > 0 {
		var err error
		normalized, err = jsonschema.UnmarshalJSON(bytes.NewReader(*info.SpecJSONBytes))
		if err != nil {
			// Fall back to normalizeJSON if UnmarshalJSON fails
			if info.SpecJSON != nil {
				normalized, err = normalizeJSON(*info.SpecJSON)
				if err != nil {
					return false, []*liberrors.ValidationError{buildDocumentDecodeError(
						fmt.Sprintf("The OpenAPI document cannot be converted to JSON: %s", err.Error()),
						"SpecJSON",
					)}
				}
			} else {
				return false, []*liberrors.ValidationError{buildDocumentDecodeError(
					fmt.Sprintf("The document's SpecJSONBytes cannot be decoded as JSON: %s", err.Error()),
					"SpecJSONBytes",
				)}
			}
		}
	} else if info.SpecJSON != nil {
		var err error
		normalized, err = normalizeJSON(*info.SpecJSON)
		if err != nil {
			return false, []*liberrors.ValidationError{buildDocumentDecodeError(
				fmt.Sprintf("The OpenAPI document cannot be converted to JSON: %s", err.Error()),
				"SpecJSON",
			)}
		}
	}

	// Validate the document
	scErrs := jsch.Validate(normalized)

	var schemaValidationErrors []*liberrors.SchemaValidationFailure

	if scErrs != nil {

		var jk *jsonschema.ValidationError
		if errors.As(scErrs, &jk) {

			// flatten the validationErrors
			schFlatErrs := jk.BasicOutput().Errors

			// Extract property name info once before processing errors (performance optimization)
			propertyInfo := extractPropertyNameFromError(jk)

			for q := range schFlatErrs {
				er := schFlatErrs[q]

				errMsg := er.Error.Kind.LocalizedString(message.NewPrinter(language.Tag{}))
				if er.KeywordLocation == "" || helpers.IgnorePolyRegex.MatchString(errMsg) {
					continue // ignore this error, it's useless tbh, utter noise.
				}
				if errMsg != "" {

					// locate the violated property in the schema
					located := LocateSchemaPropertyNodeByJSONPath(info.RootNode.Content[0], er.InstanceLocation)
					violation := &liberrors.SchemaValidationFailure{
						Reason:                  errMsg,
						FieldName:               helpers.ExtractFieldNameFromStringLocation(er.InstanceLocation),
						FieldPath:               helpers.ExtractJSONPathFromStringLocation(er.InstanceLocation),
						InstancePath:            helpers.ConvertStringLocationToPathSegments(er.InstanceLocation),
						KeywordLocation:         er.KeywordLocation,
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
					} else {
						// handles property name validation errors that don't provide useful InstanceLocation
						applyPropertyNameFallback(propertyInfo, info.RootNode.Content[0], violation)
					}
					schemaValidationErrors = append(schemaValidationErrors, violation)
				}
			}
		}

		// add the error to the list
		validationErrors = append(validationErrors, &liberrors.ValidationError{
			ValidationType: helpers.Schema,
			Message:        "Document does not pass validation",
			Reason: fmt.Sprintf("OpenAPI document is not valid according "+
				"to the %s specification", info.Version),
			SchemaValidationErrors: schemaValidationErrors,
			HowToFix:               liberrors.HowToFixInvalidSchema,
		})
	}
	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}
