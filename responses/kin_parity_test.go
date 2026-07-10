// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
)

func responseModel(t *testing.T, mediaType, schema string) *v3.Document {
	t.Helper()
	spec := "openapi: 3.1.0\ninfo: {title: response, version: 1.0.0}\npaths:\n  /body:\n    get:\n      responses:\n        \"200\":\n          description: ok\n          headers:\n            X-Rate: {required: true, schema: {type: integer}}\n          content:\n            " + mediaType + ":\n              schema:\n" + responseIndent(schema, 16) + "\n"
	document, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	return &model.Model
}

func responseIndent(value string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	return prefix + strings.ReplaceAll(value, "\n", "\n"+prefix)
}

func responseRequest(t *testing.T) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "http://example.com/body", nil)
	require.NoError(t, err)
	return request
}

func TestStandardAndUnsupportedResponseDecoders(t *testing.T) {
	model := responseModel(t, "application/yaml", "type: object\nrequired: [count]\nproperties: {count: {type: integer}}")
	request := responseRequest(t)
	newResponse := func() *http.Response {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/yaml"}, "X-Rate": {"2"}}, Body: io.NopCloser(strings.NewReader("count: 2"))}
	}

	validator := NewResponseBodyValidator(model)
	valid, validationErrors := validator.ValidateResponseBody(request, newResponse())
	require.True(t, valid, validationErrors)
	validator.Release()
	var response *http.Response

	for _, body := range []io.ReadCloser{http.NoBody, nil} {
		validator = NewResponseBodyValidator(model)
		response = &http.Response{
			StatusCode: 200, Header: http.Header{"Content-Type": {"application/yaml"}, "X-Rate": {"2"}}, Body: body,
		}
		valid, validationErrors = validator.ValidateResponseBody(request, response)
		require.True(t, valid, validationErrors)
		validator.Release()
	}

	validator = NewResponseBodyValidator(model, config.WithRejectUnsupportedBodyContent())
	response = &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": {"application/yaml"}, "X-Rate": {"2"}}, Body: http.NoBody,
	}
	valid, validationErrors = validator.ValidateResponseBody(request, response)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "no registered decoder")
	validator.Release()

	unread := &countingResponseBody{}
	validator = NewResponseBodyValidator(model)
	response = &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": {"application/yaml"}, "X-Rate": {"2"}}, Body: unread,
	}
	valid, validationErrors = validator.ValidateResponseBody(request, response)
	require.True(t, valid, validationErrors)
	assert.Zero(t, unread.reads)
	assert.Same(t, unread, response.Body)
	validator.Release()

	validator = NewResponseBodyValidator(model, config.WithRejectUnsupportedBodyContent())
	unread = &countingResponseBody{}
	response = &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": {"application/yaml"}, "X-Rate": {"2"}}, Body: unread,
	}
	valid, validationErrors = validator.ValidateResponseBody(request, response)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "no registered decoder")
	assert.Zero(t, unread.reads)
	assert.Same(t, unread, response.Body)
	validator.Release()

	validator = NewResponseBodyValidator(model, config.WithStandardBodyDecoders())
	response = newResponse()
	valid, validationErrors = validator.ValidateResponseBody(request, response)
	require.True(t, valid, validationErrors)
	body, _ := io.ReadAll(response.Body)
	assert.Equal(t, "count: 2", string(body))
	validator.Release()

	canonicalFailure := content.DecoderFunc(func(*content.DecodeInput) (any, error) { return map[any]any{1: "bad"}, nil })
	validator = NewResponseBodyValidator(model, config.WithBodyDecoder("application/yaml", canonicalFailure))
	valid, validationErrors = validator.ValidateResponseBody(request, newResponse())
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "could not be decoded")
	failureContext, ok := validationErrors[0].Context.(*content.FailureContext)
	require.True(t, ok)
	assert.Same(t, request, failureContext.Request)
	assert.NotNil(t, failureContext.Response)
	assert.NotNil(t, failureContext.Operation)
	validator.Release()
}

type countingResponseBody struct {
	reads int
}

func (r *countingResponseBody) Read([]byte) (int, error) {
	r.reads++
	return 0, errors.New("body must not be read")
}

func (*countingResponseBody) Close() error { return nil }

func TestResponseDecoderTypedErrors(t *testing.T) {
	failing := content.DecoderFunc(func(*content.DecodeInput) (any, error) { return nil, errors.New("decode failed") })
	for _, mediaType := range []string{"application/xml", "application/x-www-form-urlencoded"} {
		model := responseModel(t, mediaType, "type: string")
		validator := NewResponseBodyValidator(model, config.WithBodyDecoder(mediaType, failing))
		response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {mediaType}, "X-Rate": {"2"}}, Body: io.NopCloser(strings.NewReader("bad"))}
		valid, validationErrors := validator.ValidateResponseBody(responseRequest(t), response)
		assert.False(t, valid)
		require.NotEmpty(t, validationErrors)
		assert.Contains(t, validationErrors[0].Message, "could not be decoded")
		validator.Release()
	}
}

func TestResponseBodyAndStatusPolicies(t *testing.T) {
	model := responseModel(t, "application/json", "type: object\nrequired: [ok]\nproperties: {ok: {type: boolean}}")
	request := responseRequest(t)
	validator := NewResponseBodyValidator(model, config.WithoutResponseBodyValidation())
	response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(`{"ok":"bad"}`))}
	valid, validationErrors := validator.ValidateResponseBody(request, response)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "Missing required header")
	response = &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}, "X-Rate": {"2"}}, Body: io.NopCloser(bytes.NewBufferString(`{"ok":"bad"}`))}
	valid, validationErrors = validator.ValidateResponseBody(request, response)
	require.True(t, valid, validationErrors)
	validator.Release()

	strict := NewResponseBodyValidator(model)
	response = &http.Response{StatusCode: 299, Header: make(http.Header), Body: http.NoBody}
	valid, validationErrors = strict.ValidateResponseBody(request, response)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
	strict.Release()

	lenient := NewResponseBodyValidator(model, config.WithoutResponseStatusValidation())
	valid, validationErrors = lenient.ValidateResponseBody(request, response)
	require.True(t, valid, validationErrors)
	lenient.Release()
}

func TestNilResponseBodyUsesLegacyMissingBodyError(t *testing.T) {
	model := responseModel(t, "application/json", "type: object")
	validator := NewResponseBodyValidator(model)
	t.Cleanup(validator.Release)
	response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}, "X-Rate": {"2"}}}
	valid, validationErrors := validator.ValidateResponseBody(responseRequest(t), response)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "missing")
}

func TestResponseDefaultExclusionAndLegacyJSONMediaName(t *testing.T) {
	defaultSpec := `openapi: 3.1.0
info: {title: default, version: 1.0.0}
paths:
  /body:
    get:
      responses:
        default:
          description: fallback
          content: {application/json: {schema: {type: object}}}`
	document, err := libopenapi.NewDocument([]byte(defaultSpec))
	require.NoError(t, err)
	built, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	validator := NewResponseBodyValidator(&built.Model, config.WithoutResponseBodyValidation())
	response := &http.Response{StatusCode: 299, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`bad`))}
	valid, validationErrors := validator.ValidateResponseBody(responseRequest(t), response)
	require.True(t, valid, validationErrors)
	validator.Release()

	model := responseModel(t, "foo/json", "type: object\nrequired: [ok]\nproperties: {ok: {type: boolean}}")
	validator = NewResponseBodyValidator(model)
	response = &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"foo/json"}, "X-Rate": {"2"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}
	valid, validationErrors = validator.ValidateResponseBody(responseRequest(t), response)
	require.True(t, valid, validationErrors)
	validator.Release()
}

func TestResponseOperationWithoutResponsesIsSkipped(t *testing.T) {
	document, err := libopenapi.NewDocument([]byte(`openapi: 3.1.0
info: {title: no responses, version: 1.0.0}
paths:
  /body:
    get: {summary: no responses}`))
	require.NoError(t, err)
	built, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	validator := NewResponseBodyValidator(&built.Model)
	t.Cleanup(validator.Release)
	response := &http.Response{StatusCode: 200, Header: make(http.Header), Body: http.NoBody}
	valid, validationErrors := validator.ValidateResponseBody(responseRequest(t), response)
	require.True(t, valid, validationErrors)
}

func TestEveryStandardResponseBodyCodec(t *testing.T) {
	var multipartBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBody)
	part, err := multipartWriter.CreateFormField("name")
	require.NoError(t, err)
	_, err = part.Write([]byte("value"))
	require.NoError(t, err)
	require.NoError(t, multipartWriter.Close())

	var zipBody bytes.Buffer
	zipWriter := zip.NewWriter(&zipBody)
	file, err := zipWriter.Create("value.txt")
	require.NoError(t, err)
	_, err = file.Write([]byte("value"))
	require.NoError(t, err)
	require.NoError(t, zipWriter.Close())
	zipLimits := content.ZipLimits{CompressedSize: int64(zipBody.Len()) + 1, ExpandedSize: 1024, Entries: 2, ExpansionRatio: 10}

	tests := []struct {
		name        string
		mediaType   string
		contentType string
		schema      string
		body        []byte
		opts        []config.Option
	}{
		{"yaml", "application/yaml", "application/yaml", "type: object\nrequired: [name]\nproperties: {name: {type: string}}", []byte("name: value"), []config.Option{config.WithStandardBodyDecoders()}},
		{"xml", "application/xml", "application/xml", "type: object\nrequired: [name]\nproperties: {name: {type: string}}", []byte("<root><name>value</name></root>"), []config.Option{config.WithStandardBodyDecoders()}},
		{"form", "application/x-www-form-urlencoded", "application/x-www-form-urlencoded", "type: object\nrequired: [name]\nproperties: {name: {type: string}}", []byte("name=value"), []config.Option{config.WithStandardBodyDecoders()}},
		{"multipart", "multipart/form-data", multipartWriter.FormDataContentType(), "type: object\nrequired: [name]\nproperties: {name: {type: string}}", multipartBody.Bytes(), []config.Option{config.WithStandardBodyDecoders()}},
		{"text", "text/plain", "text/plain", "type: string", []byte("value"), []config.Option{config.WithStandardBodyDecoders()}},
		{"csv", "text/csv", "text/csv", "type: string", []byte("name,value\none,two\n"), []config.Option{config.WithStandardBodyDecoders()}},
		{"binary", "application/octet-stream", "application/octet-stream", "type: string\nformat: binary", []byte{0, 1, 2}, []config.Option{config.WithStandardBodyDecoders()}},
		{"zip", "application/zip", "application/zip", "type: string", zipBody.Bytes(), []config.Option{config.WithZipBodyDecoder(zipLimits)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := responseModel(t, test.mediaType, test.schema)
			validator := NewResponseBodyValidator(model, test.opts...)
			defer validator.Release()
			response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {test.contentType}, "X-Rate": {"2"}}, Body: io.NopCloser(bytes.NewReader(test.body))}
			valid, validationErrors := validator.ValidateResponseBody(responseRequest(t), response)
			require.True(t, valid, validationErrors)
			restored, readErr := io.ReadAll(response.Body)
			require.NoError(t, readErr)
			assert.Equal(t, test.body, restored)
		})
	}
}
