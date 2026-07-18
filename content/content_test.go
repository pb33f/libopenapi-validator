// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package content

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
)

func TestRegistryLookupPrecedenceAndIsolation(t *testing.T) {
	global := DecoderFunc(func(*DecodeInput) (any, error) { return "global", nil })
	typeWildcard := DecoderFunc(func(*DecodeInput) (any, error) { return "type", nil })
	suffix := DecoderFunc(func(*DecodeInput) (any, error) { return "suffix", nil })
	exactOld := DecoderFunc(func(*DecodeInput) (any, error) { return "old", nil })
	exactNew := DecoderFunc(func(*DecodeInput) (any, error) { return "new", nil })
	encoder := EncoderFunc(func(*EncodeInput) ([]byte, error) { return []byte("encoded"), nil })
	r := NewRegistry([]Registration{
		{"*/*", global},
		{"application/*", typeWildcard},
		{"application/*+json", suffix},
		{"application/problem+json", exactOld},
		{"application/problem+json", exactNew},
		{"bad", exactNew},
		{"text/plain", nil},
	}, []EncoderRegistration{{"application/*+json", encoder}, {"bad", encoder}, {"text/plain", nil}})

	decoder, mediaType, params := r.Decoder("application/problem+json")
	require.NotNil(t, decoder)
	value, err := decoder.Decode(nil)
	require.NoError(t, err)
	assert.Equal(t, "new", value)
	assert.Equal(t, "application/problem+json", mediaType)
	assert.Nil(t, params)

	decoder, mediaType, params = r.Decoder("application/vnd.test+json; charset=utf-8")
	value, err = decoder.Decode(nil)
	require.NoError(t, err)
	assert.Equal(t, "suffix", value)
	assert.Equal(t, "application/vnd.test+json", mediaType)
	assert.Equal(t, "utf-8", params["charset"])
	decoder, _, _ = r.Decoder("Application/Problem+JSON; charset=utf-8")
	value, _ = decoder.Decode(nil)
	assert.Equal(t, "new", value)

	decoder, _, _ = r.Decoder("application/yaml")
	value, _ = decoder.Decode(nil)
	assert.Equal(t, "type", value)
	decoder, _, _ = r.Decoder("image/png")
	value, _ = decoder.Decode(nil)
	assert.Equal(t, "global", value)

	resolvedEncoder, _, _ := r.Encoder("application/vnd.test+json")
	encoded, err := resolvedEncoder.Encode(nil)
	require.NoError(t, err)
	assert.Equal(t, "encoded", string(encoded))
	exactEncoder := EncoderFunc(func(*EncodeInput) ([]byte, error) { return []byte("exact"), nil })
	typeEncoder := EncoderFunc(func(*EncodeInput) ([]byte, error) { return []byte("type"), nil })
	globalEncoder := EncoderFunc(func(*EncodeInput) ([]byte, error) { return []byte("global"), nil })
	encoders := NewRegistry(nil, []EncoderRegistration{{"application/json", exactEncoder}, {"application/*", typeEncoder}, {"*/*", globalEncoder}})
	resolvedEncoder, _, _ = encoders.Encoder("application/json")
	encoded, _ = resolvedEncoder.Encode(nil)
	assert.Equal(t, "exact", string(encoded))
	resolvedEncoder, _, _ = encoders.Encoder("application/yaml")
	encoded, _ = resolvedEncoder.Encode(nil)
	assert.Equal(t, "type", string(encoded))
	resolvedEncoder, _, _ = encoders.Encoder("image/png")
	encoded, _ = resolvedEncoder.Encode(nil)
	assert.Equal(t, "global", string(encoded))
	assert.Nil(t, NewRegistry(nil, nil).decoders["bad"])
	assert.NotSame(t, r, NewRegistry(nil, nil))
}

func TestCompatibilityDecodersAndExactReplacement(t *testing.T) {
	xmlMarker := XMLCompatibilityDecoder()
	formMarker := FormCompatibilityDecoder()

	xmlCompatibility, ok := xmlMarker.(CompatibilityDecoder)
	require.True(t, ok)
	assert.Equal(t, "xml", xmlCompatibility.CompatibilityKind())
	_, err := xmlCompatibility.Decode(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xml compatibility decoder")

	formCompatibility, ok := formMarker.(CompatibilityDecoder)
	require.True(t, ok)
	assert.Equal(t, "form", formCompatibility.CompatibilityKind())

	registry := NewRegistry([]Registration{{MediaType: "application/xml", Decoder: xmlMarker}}, nil)
	exactCompatibility, ok := registry.ExactDecoder("Application/XML; charset=utf-8").(CompatibilityDecoder)
	require.True(t, ok)
	assert.Equal(t, "xml", exactCompatibility.CompatibilityKind())
	assert.Nil(t, registry.ExactDecoder("text/xml"))
	assert.Nil(t, (*Registry)(nil).ExactDecoder("application/xml"))

	replacement := DecoderFunc(func(*DecodeInput) (any, error) { return "replacement", nil })
	replaced := registry.WithDecoder("application/xml", replacement)
	value, err := replaced.ExactDecoder("application/xml").Decode(nil)
	require.NoError(t, err)
	assert.Equal(t, "replacement", value)
	_, originalIsMarker := registry.ExactDecoder("application/xml").(CompatibilityDecoder)
	assert.True(t, originalIsMarker, "the original registry must remain immutable")

	created := (*Registry)(nil).WithDecoder("text/xml", replacement)
	value, err = created.ExactDecoder("text/xml").Decode(nil)
	require.NoError(t, err)
	assert.Equal(t, "replacement", value)
	unchanged := registry.WithDecoder("bad media type", replacement)
	_, unchangedIsMarker := unchanged.ExactDecoder("application/xml").(CompatibilityDecoder)
	assert.True(t, unchangedIsMarker)
	unchanged = registry.WithDecoder("application/xml", nil)
	_, unchangedIsMarker = unchanged.ExactDecoder("application/xml").(CompatibilityDecoder)
	assert.True(t, unchangedIsMarker)
}

func TestRegistryNilMalformedAndWildcards(t *testing.T) {
	var r *Registry
	decoder, mediaType, params := r.Decoder("application/json")
	assert.Nil(t, decoder)
	assert.Empty(t, mediaType)
	assert.Nil(t, params)
	encoder, mediaType, params := r.Encoder("application/json")
	assert.Nil(t, encoder)
	assert.Empty(t, mediaType)
	assert.Nil(t, params)

	r = NewRegistry([]Registration{{"*/*", JSONDecoder()}}, nil)
	decoder, mediaType, _ = r.Decoder("not a media type")
	assert.Nil(t, decoder)
	assert.Empty(t, mediaType)
	decoder, mediaType, _ = r.Decoder("json")
	assert.Nil(t, decoder)
	assert.Equal(t, "json", mediaType)
	encoder, mediaType, _ = r.Encoder("not a media type")
	assert.Nil(t, encoder)
	assert.Empty(t, mediaType)
	encoder, mediaType, _ = NewRegistry(nil, []EncoderRegistration{{"*/*", JSONEncoder()}}).Encoder("json")
	assert.NotNil(t, encoder)
	assert.Equal(t, "json", mediaType)

	assert.Equal(t, "application/json", normalizeRange(" Application/JSON; charset=utf-8 "))
	assert.Empty(t, normalizeRange("invalid"))
}

func TestRegistryPrecompute(t *testing.T) {
	var nilRegistry *Registry
	assert.Nil(t, nilRegistry.Precompute([]string{"application/json"}))
	decoder := DecoderFunc(func(*DecodeInput) (any, error) { return "decoded", nil })
	encoder := EncoderFunc(func(*EncodeInput) ([]byte, error) { return []byte("encoded"), nil })
	original := NewRegistry(
		[]Registration{{"application/*+json", decoder}, {"text/plain", TextDecoder()}},
		[]EncoderRegistration{{"application/*+json", encoder}},
	)
	resolved := original.Precompute([]string{"application/vnd.test+json", "text/plain", "invalid", "not a media;=", "image/png"})
	assert.NotSame(t, original, resolved)
	assert.NotNil(t, resolved.decoders["application/vnd.test+json"])
	assert.NotNil(t, resolved.encoders["application/vnd.test+json"])
	assert.NotNil(t, resolved.decoders["text/plain"])
	assert.Nil(t, resolved.decoders["image/png"])
	assert.Nil(t, original.decoders["application/vnd.test+json"])
}

func TestCanonicalize(t *testing.T) {
	input := map[any]any{
		"numbers": []any{int(1), int8(2), int16(3), int32(4), int64(5), uint(6), uint8(7), uint16(8), uint32(9), uint64(10), float32(11.5)},
		"nested":  map[string]any{"ok": true},
	}
	value, err := Canonicalize(input)
	require.NoError(t, err)
	numbers := value.(map[string]any)["numbers"].([]any)
	for _, number := range numbers {
		assert.IsType(t, float64(0), number)
	}
	_, err = Canonicalize(map[any]any{1: "bad"})
	assert.ErrorContains(t, err, "not a string")
	for _, invalid := range []any{
		map[string]any{"nested": map[any]any{1: "bad"}},
		map[any]any{"nested": map[any]any{1: "bad"}},
		[]any{map[any]any{1: "bad"}},
	} {
		_, err = Canonicalize(invalid)
		assert.Error(t, err)
	}
	assert.Equal(t, "unchanged", mustCanonical(t, "unchanged"))
	number, err := Canonicalize(json.Number("12.5"))
	require.NoError(t, err)
	assert.Equal(t, 12.5, number)
	instant := time.Date(2026, 7, 10, 1, 2, 3, 4, time.UTC)
	canonicalInstant, err := Canonicalize(instant)
	require.NoError(t, err)
	assert.Equal(t, instant.Format(time.RFC3339Nano), canonicalInstant)
	bytesValue, err := Canonicalize([]byte("raw"))
	require.NoError(t, err)
	assert.Equal(t, []byte("raw"), bytesValue)
	_, err = Canonicalize(struct{ Value string }{"bad"})
	assert.ErrorContains(t, err, "not JSON-compatible")
}

func mustCanonical(t *testing.T, value any) any {
	t.Helper()
	canonical, err := Canonicalize(value)
	require.NoError(t, err)
	return canonical
}

func TestJSONYAMLAndStringDecoders(t *testing.T) {
	value, err := JSONDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString(`{"n":1.5}`)})
	require.NoError(t, err)
	assert.Equal(t, 1.5, value.(map[string]any)["n"])
	value, err = JSONDecoder().Decode(&DecodeInput{Body: bytes.NewReader(nil)})
	require.NoError(t, err)
	assert.Nil(t, value)
	_, err = JSONDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString(`{`)})
	assert.Error(t, err)
	_, err = JSONDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString(`{"first":true}{"second":true}`)})
	assert.ErrorContains(t, err, "multiple values")
	_, err = JSONDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString(`{"first":true} trailing`)})
	assert.Error(t, err)
	value, err = JSONDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("{\"first\":true}  \n\t")})
	require.NoError(t, err)
	assert.Equal(t, true, value.(map[string]any)["first"])
	value, err = JSONDecoder().Decode(nil)
	require.NoError(t, err)
	assert.Nil(t, value)

	value, err = YAMLDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("n: 2\nitems: [a, b]\n")})
	require.NoError(t, err)
	assert.Equal(t, float64(2), value.(map[string]any)["n"])
	_, err = YAMLDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("? [a, b]\n: value\n")})
	assert.Error(t, err)
	_, err = YAMLDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("[unterminated")})
	assert.Error(t, err)
	value, err = YAMLDecoder().Decode(nil)
	require.NoError(t, err)
	assert.Nil(t, value)

	value, err = TextDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("hello")})
	require.NoError(t, err)
	assert.Equal(t, "hello", value)
	value, err = BinaryDecoder().Decode(nil)
	require.NoError(t, err)
	assert.Equal(t, "", value)
	value, err = CSVDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("a,b\n1,2\n")})
	require.NoError(t, err)
	assert.Equal(t, "a,b\n1,2\n", value)
	_, err = CSVDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("a,b\n1\n")})
	assert.Error(t, err)
}

func TestFormMultipartAndXMLDecoders(t *testing.T) {
	value, err := FormDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("one=1&many=a&many=b")})
	require.NoError(t, err)
	assert.Equal(t, "1", value.(map[string]any)["one"])
	assert.Equal(t, []any{"a", "b"}, value.(map[string]any)["many"])
	value, err = FormDecoder().Decode(nil)
	require.NoError(t, err)
	assert.Empty(t, value)
	_, err = FormDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("bad=%zz")})
	assert.Error(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormField("name")
	require.NoError(t, err)
	_, _ = part.Write([]byte("one"))
	part, _ = writer.CreateFormField("name")
	_, _ = part.Write([]byte("two"))
	part, _ = writer.CreateFormField("name")
	_, _ = part.Write([]byte("three"))
	require.NoError(t, writer.Close())
	value, err = MultipartDecoder().Decode(&DecodeInput{Body: &body, Parameters: map[string]string{"boundary": writer.Boundary()}})
	require.NoError(t, err)
	assert.Equal(t, []any{"one", "two", "three"}, value.(map[string]any)["name"])
	_, err = MultipartDecoder().Decode(&DecodeInput{Body: bytes.NewReader(nil)})
	assert.ErrorContains(t, err, "boundary")
	value, err = MultipartDecoder().Decode(nil)
	require.NoError(t, err)
	assert.Empty(t, value)

	value, err = XMLDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("<root><name>one</name><name>two</name><name>three</name><nested><ok>true</ok></nested></root>")})
	require.NoError(t, err)
	object := value.(map[string]any)
	assert.Equal(t, []any{"one", "two", "three"}, object["name"])
	assert.Equal(t, "true", object["nested"].(map[string]any)["ok"])
	_, err = XMLDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("<bad>")})
	assert.Error(t, err)
	value, err = XMLDecoder().Decode(nil)
	require.NoError(t, err)
	assert.Nil(t, value)
}

func TestSchemaAwareFormMultipartAndXMLDecoding(t *testing.T) {
	document, err := libopenapi.NewDocument([]byte(`openapi: 3.1.0
info: {title: codecs, version: 1.0.0}
components:
  schemas:
    Input:
      type: object
      properties:
        count: {type: integer}
        enabled: {type: boolean}
        values: {type: array, items: {type: number}}`))
	require.NoError(t, err)
	model, buildErr := document.BuildV3Model()
	require.NoError(t, buildErr)
	schema := model.Model.Components.Schemas.GetOrZero("Input").Schema()

	value, err := FormDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("count=2&enabled=true&values=1.5&values=2.5"), Schema: schema})
	require.NoError(t, err)
	form := value.(map[string]any)
	assert.Equal(t, float64(2), form["count"])
	assert.Equal(t, true, form["enabled"])
	assert.Equal(t, []any{1.5, 2.5}, form["values"])

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, pair := range [][2]string{{"count", "2"}, {"values", "1.5"}, {"values", "2.5"}} {
		part, partErr := writer.CreateFormField(pair[0])
		require.NoError(t, partErr)
		_, partErr = part.Write([]byte(pair[1]))
		require.NoError(t, partErr)
	}
	require.NoError(t, writer.Close())
	value, err = MultipartDecoder().Decode(&DecodeInput{Body: &body, Parameters: map[string]string{"boundary": writer.Boundary()}, Schema: schema})
	require.NoError(t, err)
	form = value.(map[string]any)
	assert.Equal(t, float64(2), form["count"])
	assert.Equal(t, []any{1.5, 2.5}, form["values"])

	value, err = XMLDecoder().Decode(&DecodeInput{Body: bytes.NewBufferString("<root><count>2</count><enabled>true</enabled><values>1.5</values><values>2.5</values></root>"), Schema: schema})
	require.NoError(t, err)
	xmlObject := value.(map[string]any)
	assert.Equal(t, float64(2), xmlObject["count"])
	assert.Equal(t, true, xmlObject["enabled"])
	assert.Equal(t, []any{1.5, 2.5}, xmlObject["values"])
	assert.Nil(t, propertySchema(schema, "missing"))
}

func TestZipDecoderLimits(t *testing.T) {
	archive := zipBytes(t, map[string]string{"one.txt": "hello"})
	limits := ZipLimits{CompressedSize: int64(len(archive)) + 1, ExpandedSize: 100, Entries: 2, ExpansionRatio: 10}
	value, err := ZIPDecoder(limits).Decode(&DecodeInput{Body: bytes.NewReader(archive)})
	require.NoError(t, err)
	assert.Equal(t, string(archive), value)

	_, err = ZIPDecoder(ZipLimits{}).Decode(&DecodeInput{Body: bytes.NewReader(archive)})
	assert.ErrorContains(t, err, "positive")
	_, err = ZIPDecoder(ZipLimits{1, 100, 2, 10}).Decode(&DecodeInput{Body: bytes.NewReader(archive)})
	assert.ErrorContains(t, err, "compressed-size")
	_, err = ZIPDecoder(ZipLimits{1000, 100, 1, 10}).Decode(&DecodeInput{Body: bytes.NewReader(zipBytes(t, map[string]string{"a": "a", "b": "b"}))})
	assert.ErrorContains(t, err, "entry-count")
	_, err = ZIPDecoder(ZipLimits{1000, 1, 2, 10}).Decode(&DecodeInput{Body: bytes.NewReader(archive)})
	assert.ErrorContains(t, err, "expanded-size")
	_, err = ZIPDecoder(ZipLimits{1000, 100, 2, 0.001}).Decode(&DecodeInput{Body: bytes.NewReader(archive)})
	assert.ErrorContains(t, err, "expansion-ratio")
	_, err = ZIPDecoder(limits).Decode(&DecodeInput{Body: bytes.NewBufferString("not zip")})
	assert.Error(t, err)
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range files {
		file, err := writer.Create(name)
		require.NoError(t, err)
		_, err = file.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func TestEncodersErrorsAndHelpers(t *testing.T) {
	encoded, err := JSONEncoder().Encode(&EncodeInput{Context: context.Background(), Value: map[string]any{"ok": true}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(encoded))
	encoded, err = JSONEncoder().Encode(nil)
	require.NoError(t, err)
	assert.Nil(t, encoded)
	_, err = JSONEncoder().Encode(&EncodeInput{Value: make(chan int)})
	assert.Error(t, err)

	sentinel := errors.New("boom")
	decodingErr := &DecodingError{MediaType: "text/plain", Direction: Response, Err: sentinel}
	assert.ErrorIs(t, decodingErr, sentinel)
	assert.Contains(t, decodingErr.Error(), "response")
	assert.Equal(t, "body decoding failed", (*DecodingError)(nil).Error())
	assert.Nil(t, (*DecodingError)(nil).Unwrap())
	assert.Equal(t, "request", directionName(Request))

	assert.Equal(t, float64(42), ParseScalar("42", "integer"))
	assert.Equal(t, float64(1.5), ParseScalar("1.5", "number"))
	assert.Equal(t, true, ParseScalar("true", "boolean"))
	assert.Equal(t, "nope", ParseScalar("nope", "integer"))
	assert.Equal(t, "value", ParseScalar("value", "string"))
	assert.NotEmpty(t, StandardDecoderRegistrations())
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestDecoderReadFailures(t *testing.T) {
	inputs := []Decoder{TextDecoder(), FormDecoder()}
	for _, decoder := range inputs {
		_, err := decoder.Decode(&DecodeInput{Body: failingReader{}})
		assert.ErrorContains(t, err, "read failed")
	}
	_, err := MultipartDecoder().Decode(&DecodeInput{Body: failingReader{}, Parameters: map[string]string{"boundary": "boundary"}})
	assert.Error(t, err)
	multipartBody := []byte("--boundary\r\nContent-Disposition: form-data; name=\"field\"\r\n\r\nvalue")
	_, err = MultipartDecoder().Decode(&DecodeInput{Body: &dataThenError{data: multipartBody}, Parameters: map[string]string{"boundary": "boundary"}})
	assert.ErrorContains(t, err, "read failed")
	_, err = ZIPDecoder(ZipLimits{100, 100, 1, 10}).Decode(&DecodeInput{Body: failingReader{}})
	assert.ErrorContains(t, err, "read failed")
}

type dataThenError struct {
	data []byte
}

func (r *dataThenError) Read(target []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, errors.New("read failed")
	}
	n := copy(target, r.data)
	r.data = r.data[n:]
	return n, nil
}

var _ io.Reader = failingReader{}

func BenchmarkRegistryExactLookup(b *testing.B) {
	registry := NewRegistry(StandardDecoderRegistrations(), nil)
	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = registry.Decoder("application/json")
	}
}

func BenchmarkRegistryPrecomputedVendorJSONLookup(b *testing.B) {
	registry := NewRegistry(StandardDecoderRegistrations(), nil).Precompute([]string{"application/vnd.example+json"})
	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = registry.Decoder("application/vnd.example+json")
	}
}

func BenchmarkCustomDecoderLookup(b *testing.B) {
	custom := DecoderFunc(func(*DecodeInput) (any, error) { return nil, nil })
	registry := NewRegistry([]Registration{{"application/custom", custom}}, nil)
	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = registry.Decoder("application/custom")
	}
}

func BenchmarkMultipartDecode(b *testing.B) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormField("name")
	_, _ = part.Write([]byte("value"))
	_ = writer.Close()
	data := body.Bytes()
	boundary := writer.Boundary()
	decoder := MultipartDecoder()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = decoder.Decode(&DecodeInput{Body: bytes.NewReader(data), Parameters: map[string]string{"boundary": boundary}})
	}
}
