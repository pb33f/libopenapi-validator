package config

import "github.com/santhosh-tekuri/jsonschema/v6"

// ValidationOptions A container for validation configuration.
//
// Generally fluent With... style functions are used to establish the desired behavior.
type ValidationOptions struct {
	RegexEngine        jsonschema.RegexpEngine
	FormatAssertions   bool
	ContentAssertions  bool
	SecurityValidation bool
	Formats            map[string]func(v any) error
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
			o.FormatAssertions = options.FormatAssertions
			o.ContentAssertions = options.ContentAssertions
			o.SecurityValidation = options.SecurityValidation
			o.Formats = options.Formats
		}
	}
}

// WithRegexEngine Assigns a custom regular-expression engine to be used during validation.
func WithRegexEngine(engine jsonschema.RegexpEngine) Option {
	return func(o *ValidationOptions) {
		o.RegexEngine = engine
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
