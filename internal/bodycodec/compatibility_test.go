// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package bodycodec

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
	validatorerrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
)

func TestApplyReplacesOnlyCompatibilityMarkers(t *testing.T) {
	Apply(nil)
	Apply(&config.ValidationOptions{})

	options := config.NewValidationOptions(config.WithXmlBodyValidation(), config.WithURLEncodedBodyValidation())
	Apply(options)
	for _, mediaType := range []string{"application/xml", "text/xml", "application/x-www-form-urlencoded"} {
		decoder := options.BodyRegistry.ExactDecoder(mediaType)
		require.NotNil(t, decoder)
		_, isMarker := decoder.(content.CompatibilityDecoder)
		assert.False(t, isMarker, mediaType)
	}

	custom := content.DecoderFunc(func(*content.DecodeInput) (any, error) { return "custom", nil })
	customOptions := config.NewValidationOptions(
		config.WithXmlBodyValidation(),
		config.WithBodyDecoder("application/xml", custom),
		config.WithBodyDecoder("text/xml", content.FormCompatibilityDecoder()),
	)
	Apply(customOptions)
	value, err := customOptions.BodyRegistry.ExactDecoder("application/xml").Decode(nil)
	require.NoError(t, err)
	assert.Equal(t, "custom", value)
	marker, ok := customOptions.BodyRegistry.ExactDecoder("text/xml").(content.CompatibilityDecoder)
	require.True(t, ok)
	assert.Equal(t, "form", marker.CompatibilityKind())

	Apply(options)
	_, isMarker := options.BodyRegistry.ExactDecoder("application/xml").(content.CompatibilityDecoder)
	assert.False(t, isMarker, "applying twice must be idempotent")
}

func TestCompatibilityDecodersPreserveSchemaAwareValuesAndErrors(t *testing.T) {
	options := config.NewValidationOptions(config.WithXmlBodyValidation(), config.WithURLEncodedBodyValidation())
	Apply(options)

	xmlDecoder := options.BodyRegistry.ExactDecoder("application/xml")
	value, err := xmlDecoder.Decode(&content.DecodeInput{Body: strings.NewReader("<name>Dave</name>"), Schema: &base.Schema{}})
	require.NoError(t, err)
	assert.NotNil(t, value)

	formDecoder := options.BodyRegistry.ExactDecoder("application/x-www-form-urlencoded")
	value, err = formDecoder.Decode(&content.DecodeInput{Body: strings.NewReader("name=Dave")})
	require.NoError(t, err)
	assert.Equal(t, "Dave", value.(map[string]any)["name"])

	_, err = xmlDecoder.Decode(&content.DecodeInput{Body: strings.NewReader("<broken><")})
	var validationErrors *ValidationErrors
	require.ErrorAs(t, err, &validationErrors)
	require.Len(t, validationErrors.Errors, 1)
	assert.Contains(t, validationErrors.Errors[0].Reason, "malformed xml")

	_, err = formDecoder.Decode(&content.DecodeInput{Body: strings.NewReader("bad=%zz")})
	require.ErrorAs(t, err, &validationErrors)
	require.Len(t, validationErrors.Errors, 1)
	assert.Equal(t, helpers.URLEncodedValidation, validationErrors.Errors[0].ValidationType)
}

func TestCompatibilityDecodersReportNonJSONNumbers(t *testing.T) {
	properties := orderedmap.New[string, *base.SchemaProxy]()
	properties.Set("bad_number", base.CreateSchemaProxy(&base.Schema{Type: []string{helpers.Number}}))
	schema := &base.Schema{Type: []string{helpers.Object}, Properties: properties}
	options := config.NewValidationOptions(config.WithXmlBodyValidation(), config.WithURLEncodedBodyValidation())
	Apply(options)

	_, err := options.BodyRegistry.ExactDecoder("application/xml").Decode(&content.DecodeInput{
		Body: strings.NewReader("<bad_number>NaN</bad_number>"), Schema: schema,
	})
	var validationErrors *ValidationErrors
	require.ErrorAs(t, err, &validationErrors)
	require.Len(t, validationErrors.Errors, 1)
	assert.Equal(t, "xml example is malformed", validationErrors.Errors[0].Message)

	_, err = options.BodyRegistry.ExactDecoder("application/x-www-form-urlencoded").Decode(&content.DecodeInput{
		Body: strings.NewReader("bad_number=NaN"), Schema: schema,
	})
	require.ErrorAs(t, err, &validationErrors)
	require.Len(t, validationErrors.Errors, 1)
	assert.Equal(t, "Unable to parse form-urlencoded body", validationErrors.Errors[0].Message)
}

func TestValidationErrorsAndBodyReadFailures(t *testing.T) {
	var nilErrors *ValidationErrors
	assert.Equal(t, "body transform failed", nilErrors.Error())
	assert.Equal(t, "body transform failed", (&ValidationErrors{}).Error())
	structured := validatorerrors.InvalidXMLParsing("bad", "body")
	assert.Equal(t, structured.Error(), (&ValidationErrors{Errors: []*validatorerrors.ValidationError{structured}}).Error())

	body, err := readBody(nil)
	require.NoError(t, err)
	assert.Nil(t, body)
	body, err = readBody(&content.DecodeInput{})
	require.NoError(t, err)
	assert.Nil(t, body)

	_, err = readBody(&content.DecodeInput{Body: failingReader{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read body")
	assert.True(t, errors.Is(err, errRead))

	for _, decoder := range []content.Decoder{xmlDecoder{}, formDecoder{}} {
		_, err = decoder.Decode(&content.DecodeInput{Body: failingReader{}})
		require.Error(t, err)
		assert.True(t, errors.Is(err, errRead))
	}
}

var errRead = errors.New("read failure")

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errRead }

var _ io.Reader = failingReader{}
