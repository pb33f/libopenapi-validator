// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package openapi_vocabulary

import (
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// discriminatorExtension handles the OpenAPI discriminator keyword
type discriminatorExtension struct {
	propertyName string
	mapping      map[string]string // value -> schema reference
}

func (d *discriminatorExtension) Validate(ctx *jsonschema.ValidatorContext, v any) {
	// Validate that discriminator structure is correct
	// For now, we only validate the structure, not the discriminator semantics
	// Full discriminator validation would require schema resolution which is complex

	obj, ok := v.(map[string]any)
	if !ok {
		return // discriminator only applies to objects
	}

	// Check if discriminator property exists in the object
	if d.propertyName != "" {
		if _, exists := obj[d.propertyName]; !exists {
			ctx.AddError(&DiscriminatorPropertyMissingError{
				PropertyName: d.propertyName,
			})
		}
	}
}

// compileDiscriminator compiles the discriminator keyword
func CompileDiscriminator(ctx *jsonschema.CompilerContext, obj map[string]any, version VersionType) (jsonschema.SchemaExt, error) {
	v, exists := obj["discriminator"]
	if !exists {
		return nil, nil
	}

	// Validate discriminator structure
	discriminator, ok := v.(map[string]any)
	if !ok {
		return nil, &OpenAPIKeywordError{
			Keyword: "discriminator",
			Message: "discriminator must be an object",
		}
	}

	// Extract propertyName (required)
	propertyNameValue, exists := discriminator["propertyName"]
	if !exists {
		return nil, &OpenAPIKeywordError{
			Keyword: "discriminator",
			Message: "discriminator must have a propertyName field",
		}
	}

	propertyName, ok := propertyNameValue.(string)
	if !ok {
		return nil, &OpenAPIKeywordError{
			Keyword: "discriminator",
			Message: "discriminator propertyName must be a string",
		}
	}

	// Extract mapping (optional)
	var mapping map[string]string
	if mappingValue, exists := discriminator["mapping"]; exists {
		mappingObj, ok := mappingValue.(map[string]any)
		if !ok {
			return nil, &OpenAPIKeywordError{
				Keyword: "discriminator",
				Message: "discriminator mapping must be an object",
			}
		}

		mapping = make(map[string]string)
		for key, value := range mappingObj {
			if strValue, ok := value.(string); ok {
				mapping[key] = strValue
			} else {
				return nil, &OpenAPIKeywordError{
					Keyword: "discriminator",
					Message: "discriminator mapping values must be strings",
				}
			}
		}
	}

	return &discriminatorExtension{
		propertyName: propertyName,
		mapping:      mapping,
	}, nil
}
