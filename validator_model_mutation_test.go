// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
)

func mutationSpec(aSchema, tSchema string) string {
	return `openapi: 3.1.0
info:
  title: validator-mutates-model repro
  version: "1.0"
paths:
  /things:
    get:
      parameters:
        - name: a
          in: query
          schema:
` + aSchema + `
        - name: b
          in: query
          schema:
            type: integer
      responses:
        '200':
          description: ok
components:
  schemas:
    T:
` + tSchema + `
`
}

const mutationExternalSchemas = `T:
  type: object
  properties:
    self:
      $ref: '#/T'
`

func mutationParamType(t *testing.T, model *v3.Document, name string) string {
	t.Helper()
	for pair := model.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		for _, p := range pair.Value().Get.Parameters {
			if p.Name != name {
				continue
			}
			if p.Schema != nil {
				if ts := p.Schema.Schema().Type; len(ts) > 0 {
					return strings.Join(ts, ",")
				}
			}
			return "<empty>"
		}
	}
	require.Failf(t, "parameter not found", "parameter %q not found", name)
	return ""
}

func loadMutationModel(t *testing.T, spec string) (libopenapi.Document, *v3.Document) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte(spec), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ext.yaml"), []byte(mutationExternalSchemas), 0o644))

	doc, err := libopenapi.NewDocumentWithConfiguration([]byte(spec), &datamodel.DocumentConfiguration{
		BasePath:            dir,
		AllowFileReferences: true,
	})
	require.NoError(t, err)

	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	return doc, &model.Model
}

func TestNewValidator_DoesNotMutateDocumentModel(t *testing.T) {
	const (
		nestedRef  = "            allOf:\n              - $ref: '#/components/schemas/T'"
		directRef  = "            $ref: '#/components/schemas/T'"
		nestedExt  = "            allOf:\n              - $ref: './ext.yaml#/T'"
		directExt  = "            $ref: './ext.yaml#/T'"
		plainT     = "      type: object"
		recursiveT = "      type: object\n      properties:\n        self:\n          $ref: '#/components/schemas/T'"
	)

	cases := []struct {
		name    string
		aSchema string
		tSchema string
	}{
		{"nested ref, non-recursive target", nestedRef, plainT},
		{"nested ref, recursive target", nestedRef, recursiveT},
		{"direct internal ref, recursive target", directRef, recursiveT},
		{"direct external ref, recursive target", directExt, plainT},
		{"nested ref, external recursive target", nestedExt, plainT},
		{"direct internal ref, non-recursive target", directRef, plainT},
	}

	for _, tc := range cases {
		spec := mutationSpec(tc.aSchema, tc.tSchema)

		t.Run(tc.name, func(t *testing.T) {
			doc, model := loadMutationModel(t, spec)

			_, errs := NewValidator(doc)
			require.Empty(t, errs)

			got := mutationParamType(t, model, "b")
			assert.Equal(t, "integer", got, "building the validator corrupted parameter b's type")
		})
	}
}
