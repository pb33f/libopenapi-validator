package errors

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func getURLEncodingTestSchema() *base.Schema {
	spec := `openapi: 3.0.0
paths:
  /pet:
    get:
      responses:
        '200':
          content:
            application/x-www-form-urlencoded:
              encoding:
                animal:
                  contentType: application/json
              schema:
                type: object
                properties:
                  animal:
                    type: object`

	doc, _ := libopenapi.NewDocument([]byte(spec))
	v3Doc, _ := doc.BuildV3Model()

	return v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/x-www-form-urlencoded").Schema.Schema()
}

func TestInvalidURLEncodedParsing(t *testing.T) {
	err := InvalidURLEncodedParsing("no data sent", "invalid-formdata")

	assert.NotNil(t, (*err))
	assert.Equal(t, (*err).SchemaValidationErrors[0].Reason, "no data sent")
	assert.Equal(t, (*err).SchemaValidationErrors[0].ReferenceObject, "invalid-formdata")
	assert.Equal(t, helpers.Schema, (*err).ValidationSubType)
}

func TestInvalidTypeEncoding(t *testing.T) {
	err := InvalidTypeEncoding(getURLEncodingTestSchema(), "animal", helpers.JSONContentType)

	assert.NotNil(t, (*err))
	assert.Equal(t, helpers.InvalidTypeEncoding, (*err).ValidationSubType)
}

func TestReservedURLEncodedValue(t *testing.T) {
	err := ReservedURLEncodedValue(getURLEncodingTestSchema(), "animal", "!")

	assert.NotNil(t, (*err))
	assert.Equal(t, helpers.ReservedValues, (*err).ValidationSubType)
}
