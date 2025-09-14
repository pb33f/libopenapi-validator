// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package openapi_vocabulary

import (
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// OpenAPIVocabularyURL is the vocabulary URL for OpenAPI-specific keywords
const OpenAPIVocabularyURL = "https://pb33f.io/openapi-validator/vocabulary"

// VersionType represents OpenAPI specification versions
type VersionType int

const (
	// Version30 represents OpenAPI 3.0.x
	Version30 VersionType = iota
	// Version31 represents OpenAPI 3.1.x (and later)
	Version31
)

// NewOpenAPIVocabulary creates a vocabulary for OpenAPI-specific keywords
// version determines which keywords are allowed/forbidden
func NewOpenAPIVocabulary(version VersionType) *jsonschema.Vocabulary {
	return &jsonschema.Vocabulary{
		URL:    OpenAPIVocabularyURL,
		Schema: nil, // We don't validate the vocabulary schema itself
		Compile: func(ctx *jsonschema.CompilerContext, obj map[string]any) (jsonschema.SchemaExt, error) {
			return compileOpenAPIKeywords(ctx, obj, version)
		},
	}
}

// compileOpenAPIKeywords compiles all OpenAPI-specific keywords found in the schema object
func compileOpenAPIKeywords(ctx *jsonschema.CompilerContext, obj map[string]any, version VersionType) (jsonschema.SchemaExt, error) {
	var extensions []jsonschema.SchemaExt

	// Handle nullable keyword
	if ext, err := compileNullable(ctx, obj, version); err != nil {
		return nil, err
	} else if ext != nil {
		extensions = append(extensions, ext)
	}

	// Handle discriminator keyword
	if ext, err := compileDiscriminator(ctx, obj, version); err != nil {
		return nil, err
	} else if ext != nil {
		extensions = append(extensions, ext)
	}

	// Handle example keyword
	if ext, err := compileExample(ctx, obj, version); err != nil {
		return nil, err
	} else if ext != nil {
		extensions = append(extensions, ext)
	}

	// Handle deprecated keyword
	if ext, err := compileDeprecated(ctx, obj, version); err != nil {
		return nil, err
	} else if ext != nil {
		extensions = append(extensions, ext)
	}

	// Return combined extension if any keywords were found
	if len(extensions) == 0 {
		return nil, nil
	}

	return &combinedExtension{extensions: extensions}, nil
}

// combinedExtension combines multiple OpenAPI extensions into one
type combinedExtension struct {
	extensions []jsonschema.SchemaExt
}

func (c *combinedExtension) Validate(ctx *jsonschema.ValidatorContext, v any) {
	for _, ext := range c.extensions {
		ext.Validate(ctx, v)
	}
}