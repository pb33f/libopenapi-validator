package config

import (
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/pb33f/libopenapi-validator/cache"
)

// RegexCache can be set to enable compiled regex caching.
// It can be just a sync.Map, or a custom implementation with possible cleanup.
//
// Be aware that the cache should be thread safe
type RegexCache interface {
	Load(key any) (value any, ok bool) // Get a compiled regex from the cache
	Store(key, value any)              // Set a compiled regex to the cache
}

// ValidationOptions A container for validation configuration.
//
// Generally fluent With... style functions are used to establish the desired behavior.
type ValidationOptions struct {
	RegexEngine         jsonschema.RegexpEngine
	RegexCache          RegexCache // Enable compiled regex caching
	FormatAssertions    bool
	ContentAssertions   bool
	SecurityValidation  bool
	OpenAPIMode         bool // Enable OpenAPI-specific vocabulary validation
	AllowScalarCoercion bool // Enable string->boolean/number coercion
	Formats             map[string]func(v any) error
	SchemaCache         cache.SchemaCache // Optional cache for compiled schemas
}

// Option Enables an 'Options pattern' approach
type Option func(*ValidationOptions)

// NewValidationOptions creates a new ValidationOptions instance with default values.
func NewValidationOptions(opts ...Option) *ValidationOptions {
	// Create the set of default values
	o := &ValidationOptions{
		FormatAssertions:   false,
		ContentAssertions:  false,
		SecurityValidation: true,
		OpenAPIMode:        true,                    // Enable OpenAPI vocabulary by default
		SchemaCache:        cache.NewDefaultCache(), // Enable caching by default
	}

	// Apply any supplied overrides
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	// Done
	return o
}

// WithExistingOpts returns an Option that will copy the values from the supplied ValidationOptions instance
func WithExistingOpts(options *ValidationOptions) Option {
	return func(o *ValidationOptions) {
		if options != nil {
			o.RegexEngine = options.RegexEngine
			o.RegexCache = options.RegexCache
			o.FormatAssertions = options.FormatAssertions
			o.ContentAssertions = options.ContentAssertions
			o.SecurityValidation = options.SecurityValidation
			o.OpenAPIMode = options.OpenAPIMode
			o.AllowScalarCoercion = options.AllowScalarCoercion
			o.Formats = options.Formats
			o.SchemaCache = options.SchemaCache
		}
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

// WithSchemaCache sets a custom cache implementation or disables caching if nil.
// Pass nil to disable schema caching and skip cache warming during validator initialization.
// The default cache is a thread-safe sync.Map wrapper.
func WithSchemaCache(cache cache.SchemaCache) Option {
	return func(o *ValidationOptions) {
		o.SchemaCache = cache
	}
}
