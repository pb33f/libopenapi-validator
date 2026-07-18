// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/santhosh-tekuri/jsonschema/v6"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi-validator/content"
	"github.com/pb33f/libopenapi-validator/radix"
	"github.com/pb33f/libopenapi-validator/router"
)

// RegexCache can be set to enable compiled regex caching.
// It can be just a sync.Map, or a custom implementation with possible cleanup.
//
// Be aware that the cache should be thread safe
type RegexCache interface {
	Load(key any) (value any, ok bool) // Get a compiled regex from the cache
	Store(key, value any)              // Set a compiled regex to the cache
}

// AuthenticationFunc validates one security scheme for an HTTP request.
// Return nil when the scheme is satisfied; return an error to fail the current
// security requirement. Each invocation receives a fresh replayable body reader.
type AuthenticationFunc func(context.Context, *AuthenticationInput) error

// AuthenticationInput contains the request and OpenAPI security scheme details passed to an AuthenticationFunc.
type AuthenticationInput struct {
	Request            *http.Request      // Request is the request being authenticated.
	SecuritySchemeName string             // SecuritySchemeName is the component key from the requirement.
	SecurityScheme     *v3.SecurityScheme // SecurityScheme is the resolved OpenAPI scheme.
	Scopes             []string           // Scopes contains the scopes required by this requirement.
	Path               string             // Path is the matched OpenAPI path template.
	PathItem           *v3.PathItem       // PathItem is the matched OpenAPI path item.
	Operation          *v3.Operation      // Operation is the matched OpenAPI operation.
	PathParams         map[string]string  // PathParams contains decoded operation path parameters.
	Server             *v3.Server         // Server is the effective matched server, if explicit.
	ServerParams       map[string]string  // ServerParams contains decoded server variables.
}

// ContentParameterDecoder decodes an OpenAPI Parameter.content value and may select a schema.
type ContentParameterDecoder func(context.Context, *ContentParameterInput) (value any, schema *base.Schema, err error)

// ContentParameterInput contains raw parameter values and matched route context.
type ContentParameterInput struct {
	Parameter       *v3.Parameter     // Parameter is the OpenAPI parameter being decoded.
	RawValues       []string          // RawValues contains all query values or the single non-query value.
	MediaType       string            // MediaType is the declared Parameter.content media type.
	DefaultSchema   *base.Schema      // DefaultSchema is the schema declared by the selected media type.
	Request         *http.Request     // Request is the request being validated.
	PathParams      map[string]string // PathParams contains decoded operation path parameters.
	ServerVariables map[string]string // ServerVariables contains decoded server variables.
}

// ValidationOptions A container for validation configuration.
//
// Generally fluent With... style functions are used to establish the desired behavior.
type ValidationOptions struct {
	RegexEngine        jsonschema.RegexpEngine
	RegexCache         RegexCache // Enable compiled regex caching
	FormatAssertions   bool
	ContentAssertions  bool
	SecurityValidation bool
	AuthenticationFunc AuthenticationFunc
	// ContentParameterDecoder replaces built-in Parameter.content decoding when non-nil.
	ContentParameterDecoder ContentParameterDecoder
	// ValidateContentParameters enables built-in JSON decoding for path, header, and cookie Parameter.content values.
	// Existing query Parameter.content behavior remains enabled independently.
	ValidateContentParameters     bool
	OpenAPIMode                   bool // Enable OpenAPI-specific vocabulary validation
	AllowScalarCoercion           bool // Enable string->boolean/number coercion
	Formats                       map[string]func(v any) error
	SchemaCache                   cache.SchemaCache         // Optional cache for compiled schemas
	SchemaResourceCache           cache.SchemaResourceCache // Optional cache for rendered document-level schema resources
	PathTree                      radix.PathLookup          // O(k) path lookup via radix tree (built automatically)
	Router                        router.Router             // Shared immutable request router, when constructed by the high-level validator.
	pathTreeDisabled              bool                      // Internal: true if radix tree auto-build was disabled via DisablePathTree
	Logger                        *slog.Logger              // Logger for debug/error output (nil = silent)
	AllowXMLBodyValidation        bool                      // Allows to convert XML to JSON for validating a request/response body.
	AllowURLEncodedBodyValidation bool                      // Allows to convert URL Encoded to JSON for validating a request/response body.
	BodyRegistry                  *content.Registry         // BodyRegistry is the frozen per-validator body codec registry.
	RejectUnsupportedBodyContent  bool                      // RejectUnsupportedBodyContent rejects declared media types without a decoder.
	RejectUndeclaredRequestBody   bool                      // RejectUndeclaredRequestBody rejects bodies on operations without requestBody.
	ValidateRequestQuery          bool                      // ValidateRequestQuery controls high-level query validation.
	ValidateRequestBody           bool                      // ValidateRequestBody controls high-level request-body validation.
	ValidateResponseBody          bool                      // ValidateResponseBody controls high-level response-body validation.
	ValidateResponseStatus        bool                      // ValidateResponseStatus rejects undocumented response status codes.
	RequestDefaults               bool                      // RequestDefaults stages and atomically applies request defaults.
	StrictServerMatching          bool                      // StrictServerMatching matches scheme, host, port, base path, and server variables.
	bodyDecoders                  []content.Registration
	bodyEncoders                  []content.EncoderRegistration
	borrowedState                 bool

	// strict mode options - detect undeclared properties even when additionalProperties: true
	StrictMode                bool     // Enable strict property validation
	StrictIgnorePaths         []string // Instance JSONPath patterns to exclude from strict checks
	StrictIgnoredHeaders      []string // Headers to always ignore in strict mode (nil = use defaults)
	strictIgnoredHeadersMerge bool     // Internal: true if merging with defaults
	StrictRejectReadOnly      bool     // Reject readOnly properties in requests
	StrictRejectWriteOnly     bool     // Reject writeOnly properties in responses
}

// Option Enables an 'Options pattern' approach
type Option func(*ValidationOptions)

// NewValidationOptions creates a new ValidationOptions instance with default values.
func NewValidationOptions(opts ...Option) *ValidationOptions {
	// create the set of default values
	o := &ValidationOptions{
		FormatAssertions:       false,
		ContentAssertions:      false,
		SecurityValidation:     true,
		OpenAPIMode:            true, // Enable OpenAPI vocabulary by default
		ValidateRequestQuery:   true,
		ValidateRequestBody:    true,
		ValidateResponseBody:   true,
		ValidateResponseStatus: true,
		SchemaCache:            cache.NewDefaultCache(),               // Enable compiled schema caching by default
		SchemaResourceCache:    cache.NewDefaultSchemaResourceCache(), // Enable rendered resource caching by default
		bodyDecoders: []content.Registration{
			{MediaType: "application/json", Decoder: content.JSONDecoder()},
			{MediaType: "application/*+json", Decoder: content.JSONDecoder()},
		},
		bodyEncoders: []content.EncoderRegistration{
			{MediaType: "application/json", Encoder: content.JSONEncoder()},
			{MediaType: "application/*+json", Encoder: content.JSONEncoder()},
		},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	if o.BodyRegistry == nil {
		o.BodyRegistry = content.NewRegistry(o.bodyDecoders, o.bodyEncoders)
	}
	o.bodyDecoders = nil
	o.bodyEncoders = nil
	return o
}

// Release clears cached validation state and drops references that can keep
// parsed documents, rendered schemas, path trees, or user-provided callbacks alive.
func (o *ValidationOptions) Release() {
	if o == nil {
		return
	}
	if !o.borrowedState {
		releaseIfSupported(o.SchemaCache)
		releaseIfSupported(o.SchemaResourceCache)
		releaseIfSupported(o.PathTree)
	}

	o.RegexEngine = nil
	o.RegexCache = nil
	o.AuthenticationFunc = nil
	o.ContentParameterDecoder = nil
	o.ValidateContentParameters = false
	o.Formats = nil
	o.SchemaCache = nil
	o.SchemaResourceCache = nil
	o.PathTree = nil
	o.Router = nil
	o.Logger = nil
	o.StrictIgnorePaths = nil
	o.StrictIgnoredHeaders = nil
	o.BodyRegistry = nil
	o.bodyDecoders = nil
	o.bodyEncoders = nil
	o.borrowedState = false
}

type releaser interface {
	Release()
}

func releaseIfSupported(value any) {
	if r, ok := value.(releaser); ok {
		r.Release()
	}
}

// WithExistingOpts returns an Option that will copy the values from the supplied ValidationOptions instance
func WithExistingOpts(options *ValidationOptions) Option {
	return func(o *ValidationOptions) {
		if options != nil {
			o.borrowedState = true
			o.RegexEngine = options.RegexEngine
			o.RegexCache = options.RegexCache
			o.FormatAssertions = options.FormatAssertions
			o.ContentAssertions = options.ContentAssertions
			o.SecurityValidation = options.SecurityValidation
			o.AuthenticationFunc = options.AuthenticationFunc
			o.ContentParameterDecoder = options.ContentParameterDecoder
			o.ValidateContentParameters = options.ValidateContentParameters
			o.OpenAPIMode = options.OpenAPIMode
			o.AllowScalarCoercion = options.AllowScalarCoercion
			o.Formats = options.Formats
			o.SchemaCache = options.SchemaCache
			o.SchemaResourceCache = options.SchemaResourceCache
			o.PathTree = options.PathTree
			o.Router = options.Router
			o.pathTreeDisabled = options.pathTreeDisabled
			o.Logger = options.Logger
			o.AllowXMLBodyValidation = options.AllowXMLBodyValidation
			o.AllowURLEncodedBodyValidation = options.AllowURLEncodedBodyValidation
			o.BodyRegistry = options.BodyRegistry
			o.RejectUnsupportedBodyContent = options.RejectUnsupportedBodyContent
			o.RejectUndeclaredRequestBody = options.RejectUndeclaredRequestBody
			o.ValidateRequestQuery = options.ValidateRequestQuery
			o.ValidateRequestBody = options.ValidateRequestBody
			o.ValidateResponseBody = options.ValidateResponseBody
			o.ValidateResponseStatus = options.ValidateResponseStatus
			o.RequestDefaults = options.RequestDefaults
			o.StrictServerMatching = options.StrictServerMatching
			o.StrictMode = options.StrictMode
			o.StrictIgnorePaths = options.StrictIgnorePaths
			o.StrictIgnoredHeaders = options.StrictIgnoredHeaders
			o.strictIgnoredHeadersMerge = options.strictIgnoredHeadersMerge
			o.StrictRejectReadOnly = options.StrictRejectReadOnly
			o.StrictRejectWriteOnly = options.StrictRejectWriteOnly
		}
	}
}

// WithLogger sets the logger for validation debug/error output.
// If not set, logging is silent (nil logger is handled gracefully).
func WithLogger(logger *slog.Logger) Option {
	return func(o *ValidationOptions) {
		o.Logger = logger
	}
}

// WithRegexEngine Assigns a custom regular-expression engine to be used during validation.
func WithRegexEngine(engine jsonschema.RegexpEngine) Option {
	return func(o *ValidationOptions) {
		o.RegexEngine = engine
	}
}

// WithRegexCache assigns a cache for compiled regular expressions.
// A sync.Map should be sufficient for most use cases. It does not implement any cleanup
func WithRegexCache(regexCache RegexCache) Option {
	return func(o *ValidationOptions) {
		o.RegexCache = regexCache
	}
}

// WithFormatAssertions enables checks for 'format' assertions (such as date, date-time, uuid, etc)
func WithFormatAssertions() Option {
	return func(o *ValidationOptions) {
		o.FormatAssertions = true
	}
}

// WithContentAssertions enables checks for contentType, contentEncoding, etc
func WithContentAssertions() Option {
	return func(o *ValidationOptions) {
		o.ContentAssertions = true
	}
}

// WithoutSecurityValidation disables security validation for request validation
func WithoutSecurityValidation() Option {
	return func(o *ValidationOptions) {
		o.SecurityValidation = false
	}
}

// WithAuthenticationFunc sets a custom function for validating security requirements.
// When set, the function is authoritative for all security scheme types, including oauth2 and openIdConnect.
func WithAuthenticationFunc(fn AuthenticationFunc) Option {
	return func(o *ValidationOptions) {
		o.AuthenticationFunc = fn
	}
}

// WithContentParameterDecoder enables Parameter.content validation with a custom per-validator decoder.
func WithContentParameterDecoder(decoder ContentParameterDecoder) Option {
	return func(o *ValidationOptions) {
		o.ContentParameterDecoder = decoder
		o.ValidateContentParameters = decoder != nil
	}
}

// WithContentParameterValidation enables built-in JSON Parameter.content validation for path, header, and cookie parameters.
// Existing query Parameter.content behavior remains enabled independently for compatibility.
func WithContentParameterValidation() Option {
	return func(o *ValidationOptions) { o.ValidateContentParameters = true }
}

// WithCustomFormat adds custom formats and their validators that checks for custom 'format' assertions
// When you add different validators with the same name, they will be overridden,
// and only the last registration will take effect.
func WithCustomFormat(name string, validator func(v any) error) Option {
	return func(o *ValidationOptions) {
		if o.Formats == nil {
			o.Formats = make(map[string]func(v any) error)
		}

		o.Formats[name] = validator
	}
}

// WithOpenAPIMode enables OpenAPI-specific keyword validation (default: true)
func WithOpenAPIMode() Option {
	return func(o *ValidationOptions) {
		o.OpenAPIMode = true
	}
}

// WithoutOpenAPIMode disables OpenAPI-specific keyword validation
func WithoutOpenAPIMode() Option {
	return func(o *ValidationOptions) {
		o.OpenAPIMode = false
	}
}

// WithScalarCoercion enables string to boolean/number coercion (Jackson-style)
func WithScalarCoercion() Option {
	return func(o *ValidationOptions) {
		o.AllowScalarCoercion = true
	}
}

// WithXmlBodyValidation enables converting an XML body to a JSON when validating the schema from a request and response body
// The default option is set to false
func WithXmlBodyValidation() Option {
	return func(o *ValidationOptions) {
		o.AllowXMLBodyValidation = true
		o.bodyDecoders = append(o.bodyDecoders,
			content.Registration{MediaType: "application/xml", Decoder: content.XMLCompatibilityDecoder()},
			content.Registration{MediaType: "text/xml", Decoder: content.XMLCompatibilityDecoder()},
		)
	}
}

// WithURLEncodedBodyValidation enables converting an URL Encoded body to a JSON when validating the schema from a request and response body
// The default option is set to false
func WithURLEncodedBodyValidation() Option {
	return func(o *ValidationOptions) {
		o.AllowURLEncodedBodyValidation = true
		o.bodyDecoders = append(o.bodyDecoders, content.Registration{
			MediaType: "application/x-www-form-urlencoded", Decoder: content.FormCompatibilityDecoder(),
		})
	}
}

// WithBodyDecoder registers a per-validator body decoder. Later exact registrations win.
func WithBodyDecoder(mediaType string, decoder content.Decoder) Option {
	return func(o *ValidationOptions) {
		o.BodyRegistry = nil
		o.bodyDecoders = append(o.bodyDecoders, content.Registration{MediaType: mediaType, Decoder: decoder})
	}
}

// WithBodyEncoder registers a per-validator body encoder. Later exact registrations win.
func WithBodyEncoder(mediaType string, encoder content.Encoder) Option {
	return func(o *ValidationOptions) {
		o.BodyRegistry = nil
		o.bodyEncoders = append(o.bodyEncoders, content.EncoderRegistration{MediaType: mediaType, Encoder: encoder})
	}
}

// WithStandardBodyDecoders enables YAML, generic XML, URL-encoded forms,
// multipart forms, plain text, CSV, and binary codecs. ZIP remains separately
// opt-in through WithZipBodyDecoder.
func WithStandardBodyDecoders() Option {
	return func(o *ValidationOptions) {
		o.BodyRegistry = nil
		o.bodyDecoders = append(o.bodyDecoders, content.StandardDecoderRegistrations()...)
	}
}

// WithZipBodyDecoder enables ZIP validation bounded by limits.
func WithZipBodyDecoder(limits content.ZipLimits) Option {
	return func(o *ValidationOptions) {
		o.BodyRegistry = nil
		o.bodyDecoders = append(o.bodyDecoders, content.Registration{
			MediaType: "application/zip", Decoder: content.ZIPDecoder(limits),
		})
	}
}

// WithRejectUnsupportedBodyContent rejects declared body media types without a decoder.
func WithRejectUnsupportedBodyContent() Option {
	return func(o *ValidationOptions) { o.RejectUnsupportedBodyContent = true }
}

// WithRejectUndeclaredRequestBody rejects non-empty bodies on operations without requestBody.
func WithRejectUndeclaredRequestBody() Option {
	return func(o *ValidationOptions) { o.RejectUndeclaredRequestBody = true }
}

// WithoutRequestQueryParameterValidation excludes query validation from high-level request validation.
func WithoutRequestQueryParameterValidation() Option {
	return func(o *ValidationOptions) { o.ValidateRequestQuery = false }
}

// WithoutRequestBodyValidation excludes all request-body policy, defaults, decoding, and validation.
func WithoutRequestBodyValidation() Option {
	return func(o *ValidationOptions) { o.ValidateRequestBody = false }
}

// WithoutResponseBodyValidation excludes response content and schema checks while retaining status and headers.
func WithoutResponseBodyValidation() Option {
	return func(o *ValidationOptions) { o.ValidateResponseBody = false }
}

// WithoutResponseStatusValidation allows undocumented response status codes.
func WithoutResponseStatusValidation() Option {
	return func(o *ValidationOptions) { o.ValidateResponseStatus = false }
}

// WithRequestDefaults stages query, header, cookie, and request-body defaults
// and commits them atomically after successful decoding, encoding, and validation.
func WithRequestDefaults() Option {
	return func(o *ValidationOptions) { o.RequestDefaults = true }
}

// WithStrictServerMatching enables standalone-router server semantics in high-level validation.
func WithStrictServerMatching() Option {
	return func(o *ValidationOptions) { o.StrictServerMatching = true }
}

// WithSchemaCache sets a custom cache implementation or disables caching if nil.
// Pass nil to disable schema caching and skip cache warming during validator initialization.
// The default cache is a thread-safe sync.Map wrapper.
func WithSchemaCache(schemaCache cache.SchemaCache) Option {
	return func(o *ValidationOptions) {
		o.SchemaCache = schemaCache
	}
}

// WithSchemaResourceCache sets a cache for rendered document-level schema resources.
// Pass nil to disable resource reuse when compiling referenced schemas.
// Cached entries retain source YAML nodes, so long-lived shared caches should be bounded or scoped deliberately.
func WithSchemaResourceCache(schemaResourceCache cache.SchemaResourceCache) Option {
	return func(o *ValidationOptions) {
		o.SchemaResourceCache = schemaResourceCache
	}
}

// WithPathTree sets a custom radix tree for path matching.
// The default is built automatically from the OpenAPI specification.
func WithPathTree(pathTree radix.PathLookup) Option {
	return func(o *ValidationOptions) {
		o.PathTree = pathTree
	}
}

// DisablePathTree prevents automatic radix tree construction.
// Use this to fall back to regex-based path matching only.
func DisablePathTree() Option {
	return func(o *ValidationOptions) {
		o.pathTreeDisabled = true
	}
}

// WithStrictMode enables strict property validation.
// In strict mode, undeclared properties are reported as errors even when
// additionalProperties: true would normally allow them.
//
// This is useful for API governance scenarios where you want to ensure
// clients only send properties that are explicitly documented in the
// OpenAPI specification.
func WithStrictMode() Option {
	return func(o *ValidationOptions) {
		o.StrictMode = true
	}
}

// WithStrictIgnorePaths sets JSONPath patterns for paths to exclude from strict validation.
// Patterns use glob syntax:
//   - * matches a single path segment
//   - ** matches any depth (zero or more segments)
//   - [*] matches any array index
//   - \* escapes a literal asterisk
//
// Examples:
//   - "$.body.metadata.*" - any property under metadata
//   - "$.body.**.x-*" - any x-* property at any depth
//   - "$.headers.X-*" - any header starting with X-
func WithStrictIgnorePaths(paths ...string) Option {
	return func(o *ValidationOptions) {
		o.StrictIgnorePaths = paths
	}
}

// WithStrictRejectReadOnly enables rejection of readOnly properties in requests.
// When enabled, readOnly properties present in request bodies are reported as
// validation errors instead of being silently skipped.
func WithStrictRejectReadOnly() Option {
	return func(o *ValidationOptions) {
		o.StrictRejectReadOnly = true
	}
}

// WithStrictRejectWriteOnly enables rejection of writeOnly properties in responses.
// When enabled, writeOnly properties present in response bodies are reported as
// validation errors instead of being silently skipped.
func WithStrictRejectWriteOnly() Option {
	return func(o *ValidationOptions) {
		o.StrictRejectWriteOnly = true
	}
}

// WithStrictIgnoredHeaders replaces the default ignored headers list entirely.
// Use this to fully control which headers are ignored in strict mode.
// For the default list, see the strict package's DefaultIgnoredHeaders.
func WithStrictIgnoredHeaders(headers ...string) Option {
	return func(o *ValidationOptions) {
		o.StrictIgnoredHeaders = headers
		o.strictIgnoredHeadersMerge = false
	}
}

// WithStrictIgnoredHeadersExtra adds headers to the default ignored list.
// Unlike WithStrictIgnoredHeaders, this merges with the defaults rather
// than replacing them.
func WithStrictIgnoredHeadersExtra(headers ...string) Option {
	return func(o *ValidationOptions) {
		o.StrictIgnoredHeaders = headers
		o.strictIgnoredHeadersMerge = true
	}
}

// defaultIgnoredHeaders contains standard HTTP headers ignored by default.
// This is the fallback list used when no custom headers are configured.
var defaultIgnoredHeaders = []string{
	"content-type", "content-length", "accept", "authorization",
	"user-agent", "host", "connection", "accept-encoding",
	"accept-language", "cache-control", "pragma", "origin",
	"referer", "cookie", "date", "etag", "expires",
	"if-match", "if-none-match", "if-modified-since",
	"last-modified", "transfer-encoding", "vary", "x-forwarded-for",
	"x-forwarded-proto", "x-real-ip", "x-request-id",
	"request-start-time", // Added by some API clients for timing
}

// IsPathTreeDisabled returns true if radix tree auto-build was disabled via DisablePathTree.
func (o *ValidationOptions) IsPathTreeDisabled() bool {
	return o.pathTreeDisabled
}

// GetEffectiveStrictIgnoredHeaders returns the list of headers to ignore
// based on configuration. Returns defaults if not configured, merged list
// if extra headers were added, or replaced list if headers were fully replaced.
func (o *ValidationOptions) GetEffectiveStrictIgnoredHeaders() []string {
	if o.StrictIgnoredHeaders == nil {
		return defaultIgnoredHeaders
	}
	if o.strictIgnoredHeadersMerge {
		return append(defaultIgnoredHeaders, o.StrictIgnoredHeaders...)
	}
	return o.StrictIgnoredHeaders
}
