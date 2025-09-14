// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package openapi_vocabulary

import (
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileNullable compiles the nullable keyword based on OpenAPI version
func compileNullable(ctx *jsonschema.CompilerContext, obj map[string]any, version VersionType) (jsonschema.SchemaExt, error) {
	v, exists := obj["nullable"]
	if !exists {
		return nil, nil
	}

	// Check if nullable is used in OpenAPI 3.1+ (not allowed)
	if version == Version31 {
		return nil, &OpenAPIKeywordError{
			Keyword: "nullable",
			Message: "nullable keyword is not allowed in OpenAPI 3.1+, use type: [\"baseType\", \"null\"] instead",
		}
	}

	// Validate that nullable is a boolean
	_, ok := v.(bool)
	if !ok {
		return nil, &OpenAPIKeywordError{
			Keyword: "nullable",
			Message: "nullable must be a boolean value",
		}
	}

	// For nullable: true, the actual transformation happens at the schema compilation level
	// This vocabulary just validates the keyword structure and enforces version rules
	return nil, nil
}
