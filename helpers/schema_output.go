// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import "github.com/santhosh-tekuri/jsonschema/v6"

// FlattenSchemaOutputErrors returns every output unit that carries an actual validation error.
func FlattenSchemaOutputErrors(output *jsonschema.OutputUnit) []jsonschema.OutputUnit {
	if output == nil {
		return nil
	}

	flattened := make([]jsonschema.OutputUnit, 0, countSchemaOutputErrors(*output))
	collectSchemaOutputErrors(*output, &flattened)
	return flattened
}

func countSchemaOutputErrors(output jsonschema.OutputUnit) int {
	count := 0
	if output.Error != nil {
		count++
	}
	for _, child := range output.Errors {
		count += countSchemaOutputErrors(child)
	}
	return count
}

func collectSchemaOutputErrors(output jsonschema.OutputUnit, flattened *[]jsonschema.OutputUnit) {
	if output.Error != nil {
		*flattened = append(*flattened, output)
	}
	for _, child := range output.Errors {
		collectSchemaOutputErrors(child, flattened)
	}
}
