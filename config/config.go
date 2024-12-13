package config

import "github.com/santhosh-tekuri/jsonschema/v6"

// ValidationOptions A container for validation configuration.
//
// Generally fluent With... style functions are used to establish the desired behavior.
type ValidationOptions struct {
	RegexEngine jsonschema.RegexpEngine
}

// Option Enables an 'Options pattern' approach
type Option func(*ValidationOptions)

// NewOptions creates a new ValidationOptions instance with default values.
func NewOptions(opts ...Option) *ValidationOptions {

	// Create the set of default values
	o := &ValidationOptions{}

	// Apply any supplied overrides
	for _, opt := range opts {
		opt(o)
	}

	// Done
	return o
}

// WithRegexEngine Assigns a custom regular-expression engine to be used during validation.
func WithRegexEngine(engine jsonschema.RegexpEngine) Option {
	return func(o *ValidationOptions) {
		o.RegexEngine = engine
	}
}
