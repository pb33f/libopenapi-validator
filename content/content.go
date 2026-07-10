// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package content provides immutable per-validator HTTP body codec registries.
//
// Decoders convert request and response bodies into the JSON-compatible values
// consumed by schema validation. Encoders are used only when opt-in request
// default application rewrites a body. Registry lookup precedence is exact
// media type, structured suffix, type wildcard, then global wildcard.
package content

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"go.yaml.in/yaml/v4"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// Direction identifies whether a codec is processing request or response data.
type Direction uint8

const (
	// Request identifies request-body processing.
	Request Direction = iota
	// Response identifies response-body processing.
	Response
)

// DecodeInput contains the complete body-decoding context supplied by a validator.
type DecodeInput struct {
	Context context.Context // Context is the request context.
	Body    io.Reader       // Body is a fresh reader over the immutable body snapshot.
	Header  http.Header     // Header contains the request or response headers.
	// MediaType is the normalized media type without parameters.
	MediaType string
	// Parameters contains parsed media-type parameters such as charset or boundary.
	Parameters map[string]string
	Schema     *base.Schema // Schema is the OpenAPI media-type schema.
	// Encoding contains OpenAPI per-property encoding metadata when available.
	Encoding  interface{ GetOrZero(string) *v3.Encoding }
	Direction Direction // Direction identifies request or response processing.
}

// EncodeInput contains the complete body-encoding context.
type EncodeInput struct {
	Context   context.Context // Context is the request context.
	Value     any             // Value is the canonical decoded value to encode.
	Header    http.Header     // Header contains the staged request headers.
	MediaType string          // MediaType is the normalized media type.
	Schema    *base.Schema    // Schema is the OpenAPI media-type schema.
	// Encoding contains OpenAPI per-property encoding metadata when available.
	Encoding  interface{ GetOrZero(string) *v3.Encoding }
	Direction Direction // Direction identifies request or response processing.
}

// FailureContext retains HTTP and OpenAPI objects associated with a decoding or rewrite failure.
type FailureContext struct {
	Request   *http.Request  // Request is the request being validated.
	Response  *http.Response // Response is populated for response failures.
	Operation *v3.Operation  // Operation is the matched OpenAPI operation.
	MediaType *v3.MediaType  // MediaType is the selected OpenAPI media-type definition.
	Schema    *base.Schema   // Schema is the selected body schema.
}

// Decoder converts an HTTP entity into a JSON-compatible validation value.
type Decoder interface {
	// Decode returns a JSON-compatible validation value.
	Decode(*DecodeInput) (any, error)
}

// DecoderFunc adapts a function to Decoder.
type DecoderFunc func(*DecodeInput) (any, error)

// Decode calls f with input.
func (f DecoderFunc) Decode(input *DecodeInput) (any, error) { return f(input) }

// Encoder converts a validation value back into an HTTP entity.
type Encoder interface {
	// Encode serializes a canonical validation value into an HTTP body.
	Encode(*EncodeInput) ([]byte, error)
}

// EncoderFunc adapts a function to Encoder.
type EncoderFunc func(*EncodeInput) ([]byte, error)

// Encode calls f with input.
func (f EncoderFunc) Encode(input *EncodeInput) ([]byte, error) { return f(input) }

// Registration associates a normalized media-range with a decoder.
type Registration struct {
	MediaType string  // MediaType is an exact type or wildcard media range.
	Decoder   Decoder // Decoder handles values selected by MediaType.
}

// EncoderRegistration associates a normalized media-range with an encoder.
type EncoderRegistration struct {
	MediaType string  // MediaType is an exact type or wildcard media range.
	Encoder   Encoder // Encoder handles values selected by MediaType.
}

// Registry is an immutable, concurrency-safe, lock-free codec lookup table.
// Construct registries with NewRegistry and derive replacements with WithDecoder.
type Registry struct {
	decoders map[string]Decoder
	encoders map[string]Encoder
}

// CompatibilityDecoder marks a decoder placeholder that is replaced by a schema-aware validator adapter.
type CompatibilityDecoder interface {
	Decoder
	// CompatibilityKind identifies the legacy adapter that must replace this marker.
	CompatibilityKind() string
}

type compatibilityDecoder string

func (d compatibilityDecoder) Decode(*DecodeInput) (any, error) {
	return nil, fmt.Errorf("%s compatibility decoder was not initialized", d)
}

func (d compatibilityDecoder) CompatibilityKind() string { return string(d) }

// XMLCompatibilityDecoder returns the marker used by the legacy XML validation option.
func XMLCompatibilityDecoder() Decoder { return compatibilityDecoder("xml") }

// FormCompatibilityDecoder returns the marker used by the legacy form validation option.
func FormCompatibilityDecoder() Decoder { return compatibilityDecoder("form") }

// Precompute returns a new immutable registry with resolved exact dispatch entries for declared media types.
func (r *Registry) Precompute(mediaTypes []string) *Registry {
	if r == nil {
		return nil
	}
	resolved := &Registry{decoders: make(map[string]Decoder, len(r.decoders)+len(mediaTypes)), encoders: make(map[string]Encoder, len(r.encoders)+len(mediaTypes))}
	for mediaType, decoder := range r.decoders {
		resolved.decoders[mediaType] = decoder
	}
	for mediaType, encoder := range r.encoders {
		resolved.encoders[mediaType] = encoder
	}
	for _, declared := range mediaTypes {
		normalized, _ := NormalizeMediaType(declared)
		if normalized == "" {
			continue
		}
		if _, exists := resolved.decoders[normalized]; !exists {
			if decoder, _, _ := r.Decoder(normalized); decoder != nil {
				resolved.decoders[normalized] = decoder
			}
		}
		if _, exists := resolved.encoders[normalized]; !exists {
			if encoder, _, _ := r.Encoder(normalized); encoder != nil {
				resolved.encoders[normalized] = encoder
			}
		}
	}
	return resolved
}

// ExactDecoder returns only an exact normalized registration without wildcard resolution.
func (r *Registry) ExactDecoder(mediaType string) Decoder {
	if r == nil {
		return nil
	}
	normalized, _ := NormalizeMediaType(mediaType)
	return r.decoders[normalized]
}

// WithDecoder returns a new immutable registry with one exact decoder replaced.
func (r *Registry) WithDecoder(mediaType string, decoder Decoder) *Registry {
	if r == nil {
		return NewRegistry([]Registration{{MediaType: mediaType, Decoder: decoder}}, nil)
	}
	resolved := r.Precompute(nil)
	key := normalizeRange(mediaType)
	if key != "" && decoder != nil {
		resolved.decoders[key] = decoder
	}
	return resolved
}

// NewRegistry freezes codec registrations. Later duplicate exact registrations win.
func NewRegistry(decoders []Registration, encoders []EncoderRegistration) *Registry {
	r := &Registry{decoders: make(map[string]Decoder, len(decoders)), encoders: make(map[string]Encoder, len(encoders))}
	for _, registration := range decoders {
		if key := normalizeRange(registration.MediaType); key != "" && registration.Decoder != nil {
			r.decoders[key] = registration.Decoder
		}
	}
	for _, registration := range encoders {
		if key := normalizeRange(registration.MediaType); key != "" && registration.Encoder != nil {
			r.encoders[key] = registration.Encoder
		}
	}
	return r
}

// Decoder resolves exact, structured-suffix, type-wildcard, then global-wildcard matches.
func (r *Registry) Decoder(mediaType string) (Decoder, string, map[string]string) {
	if r == nil {
		return nil, "", nil
	}
	if decoder := r.decoders[mediaType]; decoder != nil {
		return decoder, mediaType, nil
	}
	normalized, params := NormalizeMediaType(mediaType)
	if normalized == "" {
		return nil, "", params
	}
	if decoder := r.decoders[normalized]; decoder != nil {
		return decoder, normalized, params
	}
	parts := strings.SplitN(normalized, "/", 2)
	if len(parts) != 2 {
		return nil, normalized, params
	}
	if plus := strings.LastIndexByte(parts[1], '+'); plus >= 0 {
		if decoder := r.decoders[parts[0]+"/*"+parts[1][plus:]]; decoder != nil {
			return decoder, normalized, params
		}
	}
	if decoder := r.decoders[parts[0]+"/*"]; decoder != nil {
		return decoder, normalized, params
	}
	return r.decoders["*/*"], normalized, params
}

// Encoder resolves an encoder with the same precedence as Decoder.
func (r *Registry) Encoder(mediaType string) (Encoder, string, map[string]string) {
	if r == nil {
		return nil, "", nil
	}
	normalized, params := NormalizeMediaType(mediaType)
	if encoder := r.encoders[normalized]; encoder != nil {
		return encoder, normalized, params
	}
	parts := strings.SplitN(normalized, "/", 2)
	if len(parts) == 2 {
		if plus := strings.LastIndexByte(parts[1], '+'); plus >= 0 {
			if encoder := r.encoders[parts[0]+"/*"+parts[1][plus:]]; encoder != nil {
				return encoder, normalized, params
			}
		}
		if encoder := r.encoders[parts[0]+"/*"]; encoder != nil {
			return encoder, normalized, params
		}
	}
	return r.encoders["*/*"], normalized, params
}

// NormalizeMediaType parses a media type and lowercases its lookup key.
func NormalizeMediaType(value string) (string, map[string]string) {
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil && mediaType == "" {
		return "", params
	}
	return strings.ToLower(mediaType), params
}

func normalizeRange(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.Contains(value, ";") {
		value, _ = NormalizeMediaType(value)
	}
	if strings.Count(value, "/") != 1 {
		return ""
	}
	return value
}

// DecodingError retains codec context while exposing the decoder failure.
type DecodingError struct {
	MediaType string    // MediaType is the normalized type selected for decoding.
	Direction Direction // Direction identifies request or response decoding.
	Err       error     // Err is the underlying decoder or canonicalization error.
}

// Error describes the media type, direction, and underlying decoder failure.
func (e *DecodingError) Error() string {
	if e == nil || e.Err == nil {
		return "body decoding failed"
	}
	return fmt.Sprintf("decode %s body as %s: %v", directionName(e.Direction), e.MediaType, e.Err)
}

// Unwrap exposes the underlying decoder or canonicalization failure.
func (e *DecodingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func directionName(direction Direction) string {
	if direction == Response {
		return "response"
	}
	return "request"
}

// Canonicalize recursively converts decoded values to the JSON validation data model.
// It rejects maps with non-string keys and values outside the documented primitive,
// []any, map[string]any, []byte, json.Number, and time.Time set.
func Canonicalize(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			canonical, err := Canonicalize(child)
			if err != nil {
				return nil, err
			}
			result[key] = canonical
		}
		return result, nil
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			stringKey, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("mapping key %v is not a string", key)
			}
			canonical, err := Canonicalize(child)
			if err != nil {
				return nil, err
			}
			result[stringKey] = canonical
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for i, child := range typed {
			canonical, err := Canonicalize(child)
			if err != nil {
				return nil, err
			}
			result[i] = canonical
		}
		return result, nil
	case int:
		return float64(typed), nil
	case int8:
		return float64(typed), nil
	case int16:
		return float64(typed), nil
	case int32:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case uint:
		return float64(typed), nil
	case uint8:
		return float64(typed), nil
	case uint16:
		return float64(typed), nil
	case uint32:
		return float64(typed), nil
	case uint64:
		return float64(typed), nil
	case float32:
		return float64(typed), nil
	case json.Number:
		return typed.Float64()
	case time.Time:
		return typed.Format(time.RFC3339Nano), nil
	case nil, string, bool, float64, []byte:
		return value, nil
	default:
		return nil, fmt.Errorf("decoded value of type %T is not JSON-compatible", value)
	}
}

// JSONDecoder returns the compatibility JSON decoder.
func JSONDecoder() Decoder {
	return DecoderFunc(func(input *DecodeInput) (any, error) {
		var value any
		if input == nil || input.Body == nil {
			return nil, nil
		}
		decoder := json.NewDecoder(input.Body)
		if err := decoder.Decode(&value); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, nil
			}
			return nil, err
		}
		if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
			if err == nil {
				return nil, errors.New("JSON body contains multiple values")
			}
			return nil, err
		}
		return value, nil
	})
}

// JSONEncoder returns the standard JSON encoder.
func JSONEncoder() Encoder {
	return EncoderFunc(func(input *EncodeInput) ([]byte, error) {
		if input == nil {
			return nil, nil
		}
		return json.Marshal(input.Value)
	})
}

// YAMLDecoder returns a YAML decoder with JSON-compatible canonicalization.
func YAMLDecoder() Decoder {
	return DecoderFunc(func(input *DecodeInput) (any, error) {
		var value any
		if input == nil || input.Body == nil {
			return nil, nil
		}
		if err := yaml.NewDecoder(input.Body).Decode(&value); err != nil {
			return nil, err
		}
		return Canonicalize(value)
	})
}

// TextDecoder returns a decoder that preserves the body as a string.
func TextDecoder() Decoder { return stringDecoder(false) }

// CSVDecoder validates CSV syntax and preserves the original body as a string.
func CSVDecoder() Decoder { return stringDecoder(true) }

// BinaryDecoder returns a decoder for string/binary schemas.
func BinaryDecoder() Decoder { return stringDecoder(false) }

func stringDecoder(validateCSV bool) Decoder {
	return DecoderFunc(func(input *DecodeInput) (any, error) {
		if input == nil || input.Body == nil {
			return "", nil
		}
		body, err := io.ReadAll(input.Body)
		if err != nil {
			return nil, err
		}
		if validateCSV {
			if _, err = csv.NewReader(bytes.NewReader(body)).ReadAll(); err != nil {
				return nil, err
			}
		}
		return string(body), nil
	})
}

// FormDecoder returns a URL-encoded form decoder.
func FormDecoder() Decoder {
	return DecoderFunc(func(input *DecodeInput) (any, error) {
		if input == nil || input.Body == nil {
			return map[string]any{}, nil
		}
		body, err := io.ReadAll(input.Body)
		if err != nil {
			return nil, err
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		result := make(map[string]any, len(values))
		for key, value := range values {
			if len(value) == 1 {
				result[key] = coerceString(value[0], propertySchema(input.Schema, key))
			} else {
				items := make([]any, len(value))
				property := propertySchema(input.Schema, key)
				for i, item := range value {
					items[i] = coerceString(item, arrayItemSchema(property))
				}
				result[key] = items
			}
		}
		return result, nil
	})
}

// MultipartDecoder returns a form-data decoder using the boundary parameter.
func MultipartDecoder() Decoder {
	return DecoderFunc(func(input *DecodeInput) (any, error) {
		if input == nil || input.Body == nil {
			return map[string]any{}, nil
		}
		boundary := ""
		if input.Parameters != nil {
			boundary = input.Parameters["boundary"]
		}
		if boundary == "" {
			return nil, errors.New("multipart boundary is missing")
		}
		reader := multipart.NewReader(input.Body, boundary)
		values := make(map[string]any)
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			body, err := io.ReadAll(part)
			_ = part.Close()
			if err != nil {
				return nil, err
			}
			name := part.FormName()
			property := propertySchema(input.Schema, name)
			decodedBody := coerceString(string(body), arrayItemSchema(property))
			if previous, exists := values[name]; exists {
				switch prior := previous.(type) {
				case []any:
					values[name] = append(prior, coerceString(string(body), arrayItemSchema(property)))
				default:
					values[name] = []any{prior, coerceString(string(body), arrayItemSchema(property))}
				}
			} else {
				if isArraySchema(property) {
					values[name] = []any{decodedBody}
				} else {
					values[name] = decodedBody
				}
			}
		}
		return values, nil
	})
}

type xmlNode struct {
	XMLName xml.Name
	Text    string    `xml:",chardata"`
	Nodes   []xmlNode `xml:",any"`
}

// XMLDecoder returns a generic XML object decoder.
func XMLDecoder() Decoder {
	return DecoderFunc(func(input *DecodeInput) (any, error) {
		if input == nil || input.Body == nil {
			return nil, nil
		}
		var root xmlNode
		if err := xml.NewDecoder(input.Body).Decode(&root); err != nil {
			return nil, err
		}
		return xmlValue(root, input.Schema), nil
	})
}

func xmlValue(node xmlNode, schema *base.Schema) any {
	if len(node.Nodes) == 0 {
		return coerceString(strings.TrimSpace(node.Text), schema)
	}
	result := make(map[string]any)
	for _, child := range node.Nodes {
		property := propertySchema(schema, child.XMLName.Local)
		value := xmlValue(child, arrayItemSchema(property))
		if previous, exists := result[child.XMLName.Local]; exists {
			switch prior := previous.(type) {
			case []any:
				result[child.XMLName.Local] = append(prior, value)
			default:
				result[child.XMLName.Local] = []any{prior, value}
			}
		} else {
			if isArraySchema(property) {
				result[child.XMLName.Local] = []any{value}
			} else {
				result[child.XMLName.Local] = value
			}
		}
	}
	return result
}

func propertySchema(schema *base.Schema, name string) *base.Schema {
	if schema == nil || schema.Properties == nil {
		return nil
	}
	proxy := schema.Properties.GetOrZero(name)
	if proxy == nil {
		return nil
	}
	return proxy.Schema()
}

func arrayItemSchema(schema *base.Schema) *base.Schema {
	if schema == nil || schema.Items == nil || !schema.Items.IsA() || schema.Items.A == nil {
		return schema
	}
	return schema.Items.A.Schema()
}

func isArraySchema(schema *base.Schema) bool {
	if schema == nil {
		return false
	}
	for _, kind := range schema.Type {
		if kind == "array" {
			return true
		}
	}
	return false
}

func coerceString(value string, schema *base.Schema) any {
	if schema == nil || len(schema.Type) == 0 {
		return value
	}
	return ParseScalar(value, schema.Type[0])
}

// ZipLimits bounds archive processing.
type ZipLimits struct {
	CompressedSize int64   // CompressedSize is the maximum archive size in bytes.
	ExpandedSize   int64   // ExpandedSize is the maximum total uncompressed size in bytes.
	Entries        int     // Entries is the maximum number of archive entries.
	ExpansionRatio float64 // ExpansionRatio is the maximum expanded-to-compressed size ratio.
}

// Validate requires finite positive archive limits.
func (l ZipLimits) Validate() error {
	if l.CompressedSize <= 0 || l.ExpandedSize <= 0 || l.Entries <= 0 || l.ExpansionRatio <= 0 {
		return errors.New("all ZIP limits must be positive")
	}
	return nil
}

// ZIPDecoder returns a decoder that validates archive metadata against limits and
// returns the original archive bytes as a string for schema validation. It does not
// extract archive entries. All limits must be positive.
func ZIPDecoder(limits ZipLimits) Decoder {
	return DecoderFunc(func(input *DecodeInput) (any, error) {
		if err := limits.Validate(); err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(input.Body, limits.CompressedSize+1))
		if err != nil {
			return nil, err
		}
		if int64(len(body)) > limits.CompressedSize {
			return nil, errors.New("ZIP compressed-size limit exceeded")
		}
		archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, err
		}
		if len(archive.File) > limits.Entries {
			return nil, errors.New("ZIP entry-count limit exceeded")
		}
		var expanded uint64
		for _, file := range archive.File {
			expanded += file.UncompressedSize64
			if expanded > uint64(limits.ExpandedSize) {
				return nil, errors.New("ZIP expanded-size limit exceeded")
			}
		}
		compressed := len(body)
		if float64(expanded)/float64(compressed) > limits.ExpansionRatio {
			return nil, errors.New("ZIP expansion-ratio limit exceeded")
		}
		return string(body), nil
	})
}

// StandardDecoderRegistrations returns non-archive built-in codecs.
func StandardDecoderRegistrations() []Registration {
	return []Registration{
		{"application/json", JSONDecoder()},
		{"application/*+json", JSONDecoder()},
		{"application/yaml", YAMLDecoder()},
		{"application/x-yaml", YAMLDecoder()},
		{"text/yaml", YAMLDecoder()},
		{"application/xml", XMLDecoder()},
		{"text/xml", XMLDecoder()},
		{"application/x-www-form-urlencoded", FormDecoder()},
		{"multipart/form-data", MultipartDecoder()},
		{"text/csv", CSVDecoder()},
		{"text/plain", TextDecoder()},
		{"text/*", TextDecoder()},
		{"application/octet-stream", BinaryDecoder()},
	}
}

// ParseScalar converts a string to a JSON-compatible primitive when requested by custom codecs.
func ParseScalar(value, kind string) any {
	switch kind {
	case "integer", "number":
		if number, err := strconv.ParseFloat(value, 64); err == nil {
			return number
		}
	case "boolean":
		if boolean, err := strconv.ParseBool(value); err == nil {
			return boolean
		}
	}
	return value
}
