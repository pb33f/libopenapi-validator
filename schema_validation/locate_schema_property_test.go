// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocateSchemaPropertyNodeByJSONPath_BadNode(t *testing.T) {
	assert.Nil(t, LocateSchemaPropertyNodeByJSONPath(nil, ""))
}

func TestLocateSchemaPropertyNode_EmptyPath(t *testing.T) {
	assert.Nil(t, locateSchemaPropertyNode(nil, ""))
}
