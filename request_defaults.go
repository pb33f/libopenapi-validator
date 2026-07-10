// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"go.yaml.in/yaml/v4"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/content"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/internal/requeststate"
)

type requestValidation func(*http.Request, *v3.PathItem, string) (bool, []*errors.ValidationError)

func (v *validator) validateWithRequestDefaults(request *http.Request, pathItem *v3.PathItem, pathValue string, validate requestValidation) (bool, []*errors.ValidationError) {
	staged, preparationErrors := v.stageRequestDefaults(request, pathItem)
	if len(preparationErrors) > 0 {
		return false, preparationErrors
	}
	valid, validationErrors := validate(staged, pathItem, pathValue)
	if !valid {
		return false, validationErrors
	}
	request.URL = staged.URL
	request.Header = staged.Header
	request.Body = staged.Body
	request.GetBody = staged.GetBody
	request.ContentLength = staged.ContentLength
	return true, nil
}

func (v *validator) stageRequestDefaults(request *http.Request, pathItem *v3.PathItem) (*http.Request, []*errors.ValidationError) {
	if request == nil {
		return request, []*errors.ValidationError{{
			ValidationType: helpers.RequestBodyValidation, ValidationSubType: helpers.Schema,
			Message: "request is nil", Reason: "request defaults require a request",
		}}
	}
	staged := new(http.Request)
	*staged = *request
	if request.URL != nil {
		clonedURL := new(url.URL)
		*clonedURL = *request.URL
		staged.URL = clonedURL
	}
	staged.Header = request.Header.Clone()
	if staged.Header == nil {
		staged.Header = make(http.Header)
	}
	body, err := requeststate.Snapshot(request)
	if err != nil {
		return nil, []*errors.ValidationError{requestDefaultError("request body could not be staged", err)}
	}
	requeststate.Install(staged, body)

	operation := helpers.ExtractOperation(staged, pathItem)
	if operation == nil {
		return staged, nil
	}
	for _, parameter := range helpers.ExtractParamsForOperation(staged, pathItem) {
		if parameter == nil {
			continue
		}
		var schema *base.Schema
		contentWrapped := false
		if parameter.Schema != nil {
			schema = parameter.Schema.Schema()
		} else if parameter.Content != nil && parameter.Content.First() != nil {
			mediaType := parameter.Content.First().Value()
			if mediaType != nil && mediaType.Schema != nil {
				schema = mediaType.Schema.Schema()
				contentWrapped = true
			}
		}
		if schema == nil || schema.Default == nil {
			continue
		}
		if !parameterNeedsDefault(staged, parameter, v.options.ValidateRequestQuery) {
			continue
		}
		value, decodeErr := defaultValue(schema.Default)
		if decodeErr != nil {
			return nil, []*errors.ValidationError{requestDefaultError("parameter default could not be decoded", decodeErr)}
		}
		if !contentWrapped && containsObjectDefault(value) {
			return nil, []*errors.ValidationError{requestDefaultError(
				"parameter default uses unsupported object serialization",
				fmt.Errorf("parameter %q must use Parameter.content or a scalar/array default", parameter.Name),
			)}
		}
		serialized := serializeDefault(value, ",")
		if contentWrapped {
			serialized = serializeContentDefault(value)
		}
		switch parameter.In {
		case helpers.Query:
			query := staged.URL.Query()
			if contentWrapped {
				query.Set(parameter.Name, serialized)
			} else {
				writeQueryDefault(query, parameter, value)
			}
			staged.URL.RawQuery = query.Encode()
		case helpers.Header:
			staged.Header.Set(parameter.Name, serialized)
		case helpers.Cookie:
			staged.AddCookie(&http.Cookie{Name: parameter.Name, Value: serialized})
		}
	}

	if !v.options.ValidateRequestBody || operation.RequestBody == nil || len(body) == 0 {
		return staged, nil
	}
	contentType, _ := content.NormalizeMediaType(staged.Header.Get(helpers.ContentTypeHeader))
	mediaType := requestMediaType(operation, contentType)
	if mediaType == nil || mediaType.Schema == nil {
		return staged, nil
	}
	decoder, normalized, parameters := v.options.BodyRegistry.Decoder(contentType)
	if decoder == nil {
		return nil, []*errors.ValidationError{requestDefaultError("request body defaults require a decoder", fmt.Errorf("no decoder for %s", contentType))}
	}
	schema := mediaType.Schema.Schema()
	decoded, decodeErr := decoder.Decode(&content.DecodeInput{
		Context: staged.Context(), Body: bytes.NewReader(body), Header: staged.Header,
		MediaType: normalized, Parameters: parameters, Schema: schema, Encoding: mediaType.Encoding, Direction: content.Request,
	})
	if decodeErr != nil {
		return nil, []*errors.ValidationError{requestDefaultError("request body could not be decoded for defaults", decodeErr)}
	}
	decoded, decodeErr = content.Canonicalize(decoded)
	if decodeErr != nil {
		return nil, []*errors.ValidationError{requestDefaultError("request body could not be canonicalized for defaults", decodeErr)}
	}
	changed, applyErr := applySchemaDefaults(decoded, schema)
	if applyErr != nil {
		return nil, []*errors.ValidationError{requestDefaultError("request body defaults could not be applied", applyErr)}
	}
	if !changed {
		return staged, nil
	}
	encoder, _, _ := v.options.BodyRegistry.Encoder(contentType)
	if encoder == nil {
		return nil, []*errors.ValidationError{requestDefaultError("request body defaults require an encoder", fmt.Errorf("no encoder for %s", contentType))}
	}
	rewritten, encodeErr := encoder.Encode(&content.EncodeInput{
		Context: staged.Context(), Value: decoded, Header: staged.Header, MediaType: contentType,
		Schema: schema, Encoding: mediaType.Encoding, Direction: content.Request,
	})
	if encodeErr != nil {
		return nil, []*errors.ValidationError{requestDefaultError("request body defaults could not be encoded", encodeErr)}
	}
	requeststate.Install(staged, rewritten)
	return staged, nil
}

func parameterNeedsDefault(request *http.Request, parameter *v3.Parameter, validateQuery bool) bool {
	if request == nil || parameter == nil {
		return false
	}
	switch parameter.In {
	case helpers.Query:
		return validateQuery && request.URL != nil && !request.URL.Query().Has(parameter.Name)
	case helpers.Header:
		return !headerExists(request.Header, parameter.Name)
	case helpers.Cookie:
		_, err := request.Cookie(parameter.Name)
		return err != nil
	}
	return false
}

func headerExists(header http.Header, name string) bool {
	for key := range header {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func containsObjectDefault(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return true
	case []any:
		for _, item := range typed {
			if containsObjectDefault(item) {
				return true
			}
		}
	}
	return false
}

func serializeContentDefault(value any) string {
	encoded, _ := json.Marshal(value) // defaultValue already proves JSON encodability.
	return string(encoded)
}

func requestDefaultError(message string, err error) *errors.ValidationError {
	return &errors.ValidationError{
		ValidationType: helpers.RequestBodyValidation, ValidationSubType: helpers.Schema,
		Message: message, Reason: err.Error(), HowToFix: "fix the default, decoder, encoder, or request value before enabling request defaults",
	}
}

func defaultValue(node *yaml.Node) (any, error) {
	var value any
	if err := node.Decode(&value); err != nil {
		return nil, err
	}
	canonical, err := content.Canonicalize(value)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func writeQueryDefault(query url.Values, parameter *v3.Parameter, value any) {
	if values, ok := value.([]any); ok {
		if parameter.IsExploded() {
			for _, item := range values {
				query.Add(parameter.Name, fmt.Sprint(item))
			}
			return
		}
		query.Set(parameter.Name, serializeDefault(value, ","))
		return
	}
	query.Set(parameter.Name, fmt.Sprint(value))
}

func serializeDefault(value any, separator string) string {
	if values, ok := value.([]any); ok {
		parts := make([]string, len(values))
		for i, item := range values {
			parts[i] = fmt.Sprint(item)
		}
		return strings.Join(parts, separator)
	}
	return fmt.Sprint(value)
}

func requestMediaType(operation *v3.Operation, mediaType string) *v3.MediaType {
	if operation == nil || operation.RequestBody == nil || operation.RequestBody.Content == nil {
		return nil
	}
	if exact := operation.RequestBody.Content.GetOrZero(mediaType); exact != nil {
		return exact
	}
	parts := strings.SplitN(mediaType, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	for pair := operation.RequestBody.Content.First(); pair != nil; pair = pair.Next() {
		declared := strings.SplitN(strings.ToLower(pair.Key()), "/", 2)
		if len(declared) == 2 && (declared[0] == "*" || declared[0] == parts[0]) && (declared[1] == "*" || declared[1] == parts[1]) {
			return pair.Value()
		}
	}
	return nil
}

func applySchemaDefaults(value any, schema *base.Schema) (bool, error) {
	if schema == nil {
		return false, nil
	}
	changed := false
	for _, allOf := range schema.AllOf {
		if allOf == nil {
			continue
		}
		applied, err := applySchemaDefaults(value, allOf.Schema())
		if err != nil {
			return false, err
		}
		changed = changed || applied
	}
	if object, ok := value.(map[string]any); ok && schema.Properties != nil {
		for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
			if pair.Value() == nil {
				continue
			}
			property := pair.Value().Schema()
			if property == nil || (property.ReadOnly != nil && *property.ReadOnly) {
				continue
			}
			current, exists := object[pair.Key()]
			if !exists && property.Default != nil {
				defaulted, err := defaultValue(property.Default)
				if err != nil {
					return false, err
				}
				object[pair.Key()] = defaulted
				changed = true
				continue
			}
			if exists && current != nil {
				applied, err := applySchemaDefaults(current, property)
				if err != nil {
					return false, err
				}
				changed = changed || applied
			}
		}
	}
	if array, ok := value.([]any); ok && schema.Items != nil && schema.Items.IsA() {
		itemSchema := schema.Items.A.Schema()
		for _, item := range array {
			applied, err := applySchemaDefaults(item, itemSchema)
			if err != nil {
				return false, err
			}
			changed = changed || applied
		}
	}
	return changed, nil
}
