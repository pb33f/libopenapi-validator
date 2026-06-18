// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
	"testing"

	"github.com/pb33f/testify/assert"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestFlattenSchemaOutputErrors(t *testing.T) {
	assert.Nil(t, FlattenSchemaOutputErrors(nil))

	output := &jsonschema.OutputUnit{
		Errors: []jsonschema.OutputUnit{
			{
				KeywordLocation: "/oneOf",
				Errors: []jsonschema.OutputUnit{
					{
						KeywordLocation:  "/oneOf/0/type",
						InstanceLocation: "/name",
						Error:            &jsonschema.OutputError{},
					},
				},
			},
			{
				KeywordLocation:  "/required",
				InstanceLocation: "",
				Error:            &jsonschema.OutputError{},
			},
		},
	}

	flattened := FlattenSchemaOutputErrors(output)

	requireLocations := []string{"/oneOf/0/type", "/required"}
	assert.Len(t, flattened, len(requireLocations))
	for i, location := range requireLocations {
		assert.Equal(t, location, flattened[i].KeywordLocation)
	}
}
