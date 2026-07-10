// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
)

func bodyModel(t testing.TB, mediaType, schema string, declared bool) *v3.Document {
	t.Helper()
	requestBody := ""
	if declared {
		requestBody = "requestBody:\n        required: true\n        content:\n          " + mediaType + ":\n            schema:\n" + indent(schema, 14)
	}
	spec := "openapi: 3.1.0\ninfo: {title: body, version: 1.0.0}\npaths:\n  /body:\n    post:\n      " + requestBody + "\n      responses: {\"204\": {description: ok}}\n"
	document, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	return &model.Model
}

func indent(value string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	return prefix + strings.ReplaceAll(value, "\n", "\n"+prefix)
}

func TestUndeclaredRequestBodyPolicy(t *testing.T) {
	model := bodyModel(t, "", "", false)
	validator := NewRequestBodyValidator(model, config.WithRejectUndeclaredRequestBody())
	t.Cleanup(validator.Release)

	request, _ := http.NewRequest(http.MethodPost, "http://example.com/body", strings.NewReader("payload"))
	valid, validationErrors := validator.ValidateRequestBody(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "not declared")
	body, _ := io.ReadAll(request.Body)
	assert.Equal(t, "payload", string(body))

	request, _ = http.NewRequest(http.MethodPost, "http://example.com/body", http.NoBody)
	valid, validationErrors = validator.ValidateRequestBody(request)
	require.True(t, valid, validationErrors)

	request = &http.Request{Method: http.MethodPost, URL: mustRequestURL(t, "http://example.com/body"), Header: make(http.Header), Body: errorReadCloser{}}
	valid, validationErrors = validator.ValidateRequestBody(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "inspected")
}

func TestStandardCustomAndUnsupportedRequestDecoders(t *testing.T) {
	schema := "type: object\nrequired: [count]\nproperties:\n  count: {type: integer}"
	model := bodyModel(t, "application/yaml", schema, true)
	validator := NewRequestBodyValidator(model)
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/body", strings.NewReader("count: 2"))
	request.Header.Set("Content-Type", "application/yaml")
	valid, validationErrors := validator.ValidateRequestBody(request)
	require.True(t, valid, validationErrors)
	validator.Release()

	unread := &countingReadCloser{}
	validator = NewRequestBodyValidator(model)
	request = &http.Request{
		Method: http.MethodPost, URL: mustRequestURL(t, "http://example.com/body"),
		Header: http.Header{"Content-Type": {"application/yaml"}}, Body: unread,
	}
	valid, validationErrors = validator.ValidateRequestBody(request)
	require.True(t, valid, validationErrors)
	assert.Zero(t, unread.reads)
	assert.Same(t, unread, request.Body)
	validator.Release()

	validator = NewRequestBodyValidator(model, config.WithRejectUnsupportedBodyContent())
	unread = &countingReadCloser{}
	request = &http.Request{
		Method: http.MethodPost, URL: mustRequestURL(t, "http://example.com/body"),
		Header: http.Header{"Content-Type": {"application/yaml"}}, Body: unread,
	}
	valid, validationErrors = validator.ValidateRequestBody(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "no registered decoder")
	assert.Zero(t, unread.reads)
	assert.Same(t, unread, request.Body)
	validator.Release()

	validator = NewRequestBodyValidator(model, config.WithStandardBodyDecoders())
	request, _ = http.NewRequest(http.MethodPost, "http://example.com/body", strings.NewReader("count: 2"))
	request.Header.Set("Content-Type", "application/yaml")
	valid, validationErrors = validator.ValidateRequestBody(request)
	require.True(t, valid, validationErrors)
	body, _ := io.ReadAll(request.Body)
	assert.Equal(t, "count: 2", string(body))
	validator.Release()

	canonicalFailure := content.DecoderFunc(func(*content.DecodeInput) (any, error) { return map[any]any{1: "bad"}, nil })
	validator = NewRequestBodyValidator(model, config.WithBodyDecoder("application/yaml", canonicalFailure))
	request, _ = http.NewRequest(http.MethodPost, "http://example.com/body", strings.NewReader("count: 2"))
	request.Header.Set("Content-Type", "application/yaml")
	valid, validationErrors = validator.ValidateRequestBody(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "could not be decoded")
	failureContext, ok := validationErrors[0].Context.(*content.FailureContext)
	require.True(t, ok)
	assert.Same(t, request, failureContext.Request)
	assert.NotNil(t, failureContext.Operation)
	assert.NotNil(t, failureContext.Schema)
	validator.Release()
}

func TestRequestDecoderReadAndTypedErrors(t *testing.T) {
	schema := "type: string"
	model := bodyModel(t, "application/custom", schema, true)
	validator := NewRequestBodyValidator(model, config.WithBodyDecoder("application/custom", content.TextDecoder()))
	request := &http.Request{Method: http.MethodPost, URL: mustRequestURL(t, "http://example.com/body"), Header: http.Header{"Content-Type": {"application/custom"}}, Body: errorReadCloser{}}
	valid, validationErrors := validator.ValidateRequestBody(request)
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Message, "could not be read")
	validator.Release()

	failing := content.DecoderFunc(func(*content.DecodeInput) (any, error) { return nil, errors.New("decode failed") })
	for _, mediaType := range []string{"application/xml", "application/x-www-form-urlencoded"} {
		model = bodyModel(t, mediaType, schema, true)
		validator = NewRequestBodyValidator(model, config.WithBodyDecoder(mediaType, failing))
		request, _ = http.NewRequest(http.MethodPost, "http://example.com/body", strings.NewReader("bad"))
		request.Header.Set("Content-Type", mediaType)
		valid, validationErrors = validator.ValidateRequestBody(request)
		assert.False(t, valid)
		require.NotEmpty(t, validationErrors)
		if strings.Contains(mediaType, "xml") {
			assert.Contains(t, validationErrors[0].Message, "xml")
		} else {
			assert.Contains(t, validationErrors[0].Message, "form-urlencoded")
		}
		validator.Release()
	}
}

func TestVendorJSONDecoderCompatibility(t *testing.T) {
	model := bodyModel(t, "application/vnd.test+json", "type: object\nrequired: [ok]\nproperties: {ok: {type: boolean}}", true)
	validator := NewRequestBodyValidator(model)
	t.Cleanup(validator.Release)
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/body", bytes.NewBufferString(`{"ok":true}`))
	request.Header.Set("Content-Type", "application/vnd.test+json")
	valid, validationErrors := validator.ValidateRequestBody(request)
	require.True(t, valid, validationErrors)
}

func TestEveryStandardRequestBodyCodec(t *testing.T) {
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
		{"text", "text/plain", "text/plain", "type: string\nminLength: 1", []byte("value"), []config.Option{config.WithStandardBodyDecoders()}},
		{"csv", "text/csv", "text/csv", "type: string\nminLength: 1", []byte("name,value\none,two\n"), []config.Option{config.WithStandardBodyDecoders()}},
		{"binary", "application/octet-stream", "application/octet-stream", "type: string\nformat: binary", []byte{0, 1, 2}, []config.Option{config.WithStandardBodyDecoders()}},
		{"zip", "application/zip", "application/zip", "type: string", zipBody.Bytes(), []config.Option{config.WithZipBodyDecoder(zipLimits)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := bodyModel(t, test.mediaType, test.schema, true)
			validator := NewRequestBodyValidator(model, test.opts...)
			defer validator.Release()
			request, err := http.NewRequest(http.MethodPost, "http://example.com/body", bytes.NewReader(test.body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", test.contentType)
			valid, validationErrors := validator.ValidateRequestBody(request)
			require.True(t, valid, validationErrors)
			require.Empty(t, validationErrors)
			restored, readErr := io.ReadAll(request.Body)
			require.NoError(t, readErr)
			assert.Equal(t, test.body, restored)
		})
	}
}

func TestLegacyValidateRequestSchemaMalformedJSON(t *testing.T) {
	model := bodyModel(t, "application/json", "type: object", true)
	schema := model.Paths.PathItems.GetOrZero("/body").Post.RequestBody.Content.GetOrZero("application/json").Schema.Schema()
	request, _ := http.NewRequest(http.MethodPost, "http://example.com/body", strings.NewReader(`{`))
	valid, validationErrors := ValidateRequestSchema(&ValidateRequestSchemaInput{Request: request, Schema: schema, Version: 3.1})
	assert.False(t, valid)
	require.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "cannot be decoded")
}

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReadCloser) Close() error             { return nil }

type countingReadCloser struct {
	reads int
}

func (r *countingReadCloser) Read([]byte) (int, error) {
	r.reads++
	return 0, errors.New("body must not be read")
}

func (*countingReadCloser) Close() error { return nil }

func mustRequestURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	require.NoError(t, err)
	return parsed
}

func BenchmarkJSONRequestBodyValidation(b *testing.B) {
	model := bodyModel(b, "application/json", "type: object\nrequired: [ok]\nproperties: {ok: {type: boolean}}", true)
	validator := NewRequestBodyValidator(model)
	b.Cleanup(validator.Release)
	body := []byte(`{"ok":true}`)
	b.ReportAllocs()
	for b.Loop() {
		request, _ := http.NewRequest(http.MethodPost, "http://example.com/body", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		_, _ = validator.ValidateRequestBody(request)
	}
}

func BenchmarkVendorJSONRequestBodyValidation(b *testing.B) {
	model := bodyModel(b, "application/vnd.example+json", "type: object\nrequired: [ok]\nproperties: {ok: {type: boolean}}", true)
	validator := NewRequestBodyValidator(model)
	b.Cleanup(validator.Release)
	body := []byte(`{"ok":true}`)
	b.ReportAllocs()
	for b.Loop() {
		request, _ := http.NewRequest(http.MethodPost, "http://example.com/body", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/vnd.example+json")
		_, _ = validator.ValidateRequestBody(request)
	}
}
