// Copyright 2023-2024 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package errors

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func getTestSchema() *base.Schema {
	spec := `openapi: 3.0.0
paths:
  /pet:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  age:
                    type: integer
                xml:
                  name: Cat`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	v3Doc, _ := doc.BuildV3Model()

	return v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()
}

func TestMissingPrefixError(t *testing.T) {
	schema := getTestSchema()
	err := MissingPrefix(schema, "prx")

	assert.NotNil(t, *err)
	assert.Equal(t, helpers.XmlValidationPrefix, (*err).ValidationSubType)
}

func TestMissingNamespaceError(t *testing.T) {
	schema := getTestSchema()
	err := MissingNamespace(schema, "http://ex.c")

	assert.NotNil(t, *err)
	assert.Equal(t, helpers.XmlValidationNamespace, (*err).ValidationSubType)
}

func TestInvalidPrefixError(t *testing.T) {
	schema := getTestSchema()
	err := InvalidPrefix(schema, "prx")

	assert.NotNil(t, *err)
	assert.Equal(t, helpers.XmlValidationPrefix, (*err).ValidationSubType)
}

func TestInvalidNamespaceError(t *testing.T) {
	schema := getTestSchema()
	err := InvalidNamespace(schema, "other", "http://ex.c", "prx")

	assert.NotNil(t, *err)
	assert.Equal(t, helpers.XmlValidationNamespace, (*err).ValidationSubType)
}

func TestInvalidParsing(t *testing.T) {
	err := InvalidXmlParsing("no data sent", "invalid-xml")

	assert.NotNil(t, (*err))
	assert.Equal(t, (*err).SchemaValidationErrors[0].Location, "xml parsing")
	assert.Equal(t, helpers.Schema, (*err).ValidationSubType)
}
