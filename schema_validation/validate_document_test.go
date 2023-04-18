// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "os"
    "testing"
)

func TestValidateDocument(t *testing.T) {

    petstore, _ := os.ReadFile("../test_specs/petstorev3.json")

    doc, _ := libopenapi.NewDocument(petstore)

    // validate!
    valid, errors := ValidateOpenAPIDocument(doc)

    assert.True(t, valid)
    assert.Len(t, errors, 0)

}

func TestValidateDocument_31(t *testing.T) {

    petstore, _ := os.ReadFile("../test_specs/valid_31.yaml")

    doc, _ := libopenapi.NewDocument(petstore)

    // validate!
    valid, errors := ValidateOpenAPIDocument(doc)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestValidateDocument_Invalid31(t *testing.T) {

    petstore, _ := os.ReadFile("../test_specs/invalid_31.yaml")

    doc, _ := libopenapi.NewDocument(petstore)

    // validate!
    valid, errors := ValidateOpenAPIDocument(doc)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Len(t, errors[0].SchemaValidationErrors, 4)

}

func TestValidateDocument_InvalidSwagger(t *testing.T) {

    petstore, _ := os.ReadFile("../test_specs/petstorev3.json")

    doc, _ := libopenapi.NewDocument(petstore)
    // fake the version so the validator things this is a swagger spec
    doc.GetSpecInfo().SpecType = "swagger"

    // validate!
    valid, errors := ValidateOpenAPIDocument(doc)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Swagger / OpenAPI 2.0 is not supported by the validator", errors[0].Message)

}
