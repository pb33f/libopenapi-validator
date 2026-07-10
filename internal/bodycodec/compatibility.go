// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package bodycodec adapts legacy schema-aware body transforms to content decoders.
package bodycodec

import (
	"encoding/json"
	"fmt"
	"io"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
	validatorerrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/schema_validation"
)

// ValidationErrors carries the original structured transform errors through the decoder contract.
type ValidationErrors struct {
	Errors []*validatorerrors.ValidationError // Errors contains the original structured transform failures.
}

// Error returns the first structured transform error.
func (e *ValidationErrors) Error() string {
	if e == nil || len(e.Errors) == 0 {
		return "body transform failed"
	}
	return e.Errors[0].Error()
}

// Apply replaces compatibility markers with schema-aware immutable decoder adapters.
func Apply(options *config.ValidationOptions) {
	if options == nil || options.BodyRegistry == nil {
		return
	}
	for _, registration := range []struct {
		mediaType string
		kind      string
		decoder   content.Decoder
	}{
		{"application/xml", "xml", xmlDecoder{}},
		{"text/xml", "xml", xmlDecoder{}},
		{"application/x-www-form-urlencoded", "form", formDecoder{}},
	} {
		marker, ok := options.BodyRegistry.ExactDecoder(registration.mediaType).(content.CompatibilityDecoder)
		if ok && marker.CompatibilityKind() == registration.kind {
			options.BodyRegistry = options.BodyRegistry.WithDecoder(registration.mediaType, registration.decoder)
		}
	}
}

type xmlDecoder struct{}

func (xmlDecoder) Decode(input *content.DecodeInput) (any, error) {
	body, err := readBody(input)
	if err != nil {
		return nil, err
	}
	value, validationErrors := schema_validation.TransformXMLToSchemaJSON(string(body), input.Schema)
	if len(validationErrors) > 0 {
		return nil, &ValidationErrors{Errors: validationErrors}
	}
	if _, err = json.Marshal(value); err != nil {
		return nil, &ValidationErrors{Errors: []*validatorerrors.ValidationError{
			validatorerrors.InvalidXMLParsing(err.Error(), string(body)),
		}}
	}
	return value, nil
}

type formDecoder struct{}

func (formDecoder) Decode(input *content.DecodeInput) (any, error) {
	body, err := readBody(input)
	if err != nil {
		return nil, err
	}
	encoding, _ := input.Encoding.(*orderedmap.Map[string, *v3.Encoding])
	value, validationErrors := schema_validation.TransformURLEncodedToSchemaJSON(string(body), input.Schema, encoding)
	if len(validationErrors) > 0 {
		return nil, &ValidationErrors{Errors: validationErrors}
	}
	if _, err = json.Marshal(value); err != nil {
		return nil, &ValidationErrors{Errors: []*validatorerrors.ValidationError{
			validatorerrors.InvalidURLEncodedParsing(err.Error(), string(body)),
		}}
	}
	return value, nil
}

func readBody(input *content.DecodeInput) ([]byte, error) {
	if input == nil || input.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}
