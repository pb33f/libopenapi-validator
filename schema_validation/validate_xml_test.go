// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT
package schema_validation

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func TestValidateXML_Issue346_BasicXMLWithName(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: Test
  version: 1.0.0
paths:
  /pet:
    get:
      responses:
        '200':
          description: success
          content:
            application/xml:
              schema:
                type: object
                properties:
                  nice:
                    type: string
                xml:
                  name: Cat
              example: "<Cat><nice>true</nice></Cat>"`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()
	valid, validationErrors := validator.ValidateXMLString(schema, "<Cat><nice>true</nice></Cat>")

	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_MalformedXML(t *testing.T) {
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
                xml:
                  name: Cat`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// empty xml should fail
	valid, validationErrors := validator.ValidateXMLString(schema, "")

	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "empty xml")
}

func TestValidateXML_WithAttributes(t *testing.T) {
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
                  id:
                    type: integer
                    xml:
                      attribute: true
                  name:
                    type: string
                xml:
                  name: Cat`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()
	valid, validationErrors := validator.ValidateXMLString(schema, `<Cat id="123"><name>Fluffy</name></Cat>`)

	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_TypeValidation(t *testing.T) {
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

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid integer
	valid, validationErrors := validator.ValidateXMLString(schema, "<Cat><age>5</age></Cat>")
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)

	// invalid - string instead of integer
	valid, validationErrors = validator.ValidateXMLString(schema, "<Cat><age>not-a-number</age></Cat>")
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
}

func TestValidateXML_WrappedArray(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /pets:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  pets:
                    type: array
                    xml:
                      wrapped: true
                    items:
                      type: object
                      properties:
                        name:
                          type: string
                        age:
                          type: integer
                      xml:
                        name: pet
                xml:
                  name: Pets`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/pets").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid wrapped array
	validXML := `<Pets><pets><pet><name>Fluffy</name><age>3</age></pet><pet><name>Spot</name><age>5</age></pet></pets></Pets>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)

	// invalid - wrong type in array item
	invalidXML := `<Pets><pets><pet><name>Fluffy</name><age>not-a-number</age></pet></pets></Pets>`
	valid, validationErrors = validator.ValidateXMLString(schema, invalidXML)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
}

func TestValidateXML_MultiplePropertiesWithCustomNames(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /user:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  userId:
                    type: integer
                    xml:
                      name: id
                  userName:
                    type: string
                    xml:
                      name: username
                  userEmail:
                    type: string
                    xml:
                      name: email
                xml:
                  name: User`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/user").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid xml with custom element names
	validXML := `<User><id>42</id><username>johndoe</username><email>john@example.com</email></User>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_MixedAttributesAndElements(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /book:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  id:
                    type: integer
                    xml:
                      attribute: true
                  isbn:
                    type: string
                    xml:
                      attribute: true
                  title:
                    type: string
                  author:
                    type: string
                  price:
                    type: number
                xml:
                  name: Book`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/book").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid xml with both attributes and elements
	validXML := `<Book id="1" isbn="978-3-16-148410-0"><title>Go Programming</title><author>John Doe</author><price>29.99</price></Book>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_NestedObjects(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /order:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  orderId:
                    type: integer
                  customer:
                    type: object
                    properties:
                      name:
                        type: string
                      address:
                        type: object
                        properties:
                          street:
                            type: string
                          city:
                            type: string
                xml:
                  name: Order`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/order").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid nested xml
	validXML := `<Order><orderId>123</orderId><customer><name>Jane Doe</name><address><street>123 Main St</street><city>Springfield</city></address></customer></Order>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_TypeCoercion(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /data:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  intValue:
                    type: integer
                  floatValue:
                    type: number
                  stringValue:
                    type: string
                  boolValue:
                    type: string
                xml:
                  name: Data`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/data").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// goxml2json should coerce numeric strings to numbers
	validXML := `<Data><intValue>42</intValue><floatValue>3.14</floatValue><stringValue>hello</stringValue><boolValue>true</boolValue></Data>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_SchemaViolations(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /product:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                required:
                  - productId
                  - name
                properties:
                  productId:
                    type: integer
                  name:
                    type: string
                  description:
                    type: string
                xml:
                  name: Product`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/product").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// missing required property 'name'
	invalidXML := `<Product><productId>123</productId></Product>`
	valid, validationErrors := validator.ValidateXMLString(schema, invalidXML)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)

	// valid - all required properties present
	validXML := `<Product><productId>123</productId><name>Widget</name></Product>`
	valid, validationErrors = validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)

	// valid with optional property
	validXML = `<Product><productId>123</productId><name>Widget</name><description>A useful widget</description></Product>`
	valid, validationErrors = validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_ComplexRealWorld_SOAP(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /api:
    post:
      responses:
        '200':
          content:
            application/soap+xml:
              schema:
                type: object
                properties:
                  status:
                    type: string
                  requestId:
                    type: string
                    xml:
                      attribute: true
                  timestamp:
                    type: integer
                  data:
                    type: object
                    properties:
                      value:
                        type: string
                xml:
                  name: Response`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/api").Post.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/soap+xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid soap-like xml
	validXML := `<Response requestId="req-12345"><status>success</status><timestamp>1699372800</timestamp><data><value>result</value></data></Response>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_EmptyAndWhitespace(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /test:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  value:
                    type: string
                xml:
                  name: Test`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/test").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid xml with whitespace
	validXML := `<Test>
		<value>hello</value>
	</Test>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)

	// valid xml with empty element
	validXML = `<Test><value></value></Test>`
	valid, validationErrors = validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_WithNamespace(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /message:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  subject:
                    type: string
                  body:
                    type: string
                xml:
                  name: Message`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/message").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid xml with namespace (goxml2json strips namespace prefixes)
	validXML := `<msg:Message xmlns:msg="http://example.com/message"><msg:subject>Hello</msg:subject><msg:body>World</msg:body></msg:Message>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_PropertyMismatch(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /config:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                required:
                  - enabled
                  - maxRetries
                properties:
                  enabled:
                    type: boolean
                  maxRetries:
                    type: integer
                xml:
                  name: Config`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/config").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// xml has wrong element names (should be 'enabled' and 'maxRetries')
	// this should fail because required properties are missing
	invalidXML := `<Config><isEnabled>true</isEnabled><retries>5</retries></Config>`
	valid, validationErrors := validator.ValidateXMLString(schema, invalidXML)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
}

func TestValidateXML_AttributeTypeMismatch(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /item:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  id:
                    type: integer
                    xml:
                      attribute: true
                  quantity:
                    type: integer
                    xml:
                      attribute: true
                  name:
                    type: string
                xml:
                  name: Item`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/item").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid - attributes are integers
	validXML := `<Item id="123" quantity="5"><name>Widget</name></Item>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)

	// invalid - attribute is not an integer
	invalidXML := `<Item id="abc" quantity="5"><name>Widget</name></Item>`
	valid, validationErrors = validator.ValidateXMLString(schema, invalidXML)
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
}

func TestValidateXML_FloatPrecision(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /measurement:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  temperature:
                    type: number
                  humidity:
                    type: number
                  pressure:
                    type: number
                xml:
                  name: Measurement`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/measurement").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// valid xml with float values
	validXML := `<Measurement><temperature>23.456</temperature><humidity>65.2</humidity><pressure>1013.25</pressure></Measurement>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)

	// valid - integers are acceptable for number type
	validXML = `<Measurement><temperature>23</temperature><humidity>65</humidity><pressure>1013</pressure></Measurement>`
	valid, validationErrors = validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_Version30_WithNullable(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /item:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  value:
                    type: string
                    nullable: true
                xml:
                  name: Item`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/item").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// test with version 3.0 - should allow nullable keyword
	valid, validationErrors := validator.ValidateXMLStringWithVersion(schema, "<Item><value>test</value></Item>", 3.0)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_NilSchema(t *testing.T) {
	validator := NewXMLValidator()

	// test with nil schema - should return false with empty errors
	valid, validationErrors := validator.ValidateXMLString(nil, "<Test>value</Test>")
	assert.False(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_NilSchemaInTransformation(t *testing.T) {
	// directly test applyXMLTransformations with nil schema (line 94)
	xmlNsMap := make(map[string]string, 2)
	result, err := applyXMLTransformations(map[string]interface{}{"test": "value"}, nil, &xmlNsMap)
	assert.NotNil(t, result)
	assert.Len(t, err, 0)
	assert.Equal(t, map[string]interface{}{"test": "value"}, result)
}

func TestValidateXML_TransformWithNilPropertySchemaProxy(t *testing.T) {
	// directly test applyXMLTransformations when a property schema proxy returns nil (line 119)
	// this can happen with circular refs or unresolved refs in edge cases
	// create a schema with properties but we'll simulate a nil schema scenario
	// by testing the transformation directly
	data := map[string]interface{}{
		"test": "value",
	}

	// schema with properties but no XML config - tests property iteration
	schema := &base.Schema{
		Properties: nil, // will trigger line 109 early return
	}
	xmlNsMap := make(map[string]string, 2)
	result, err := applyXMLTransformations(data, schema, &xmlNsMap)
	assert.Len(t, err, 0)
	assert.Equal(t, data, result)
}

func TestValidateXML_NoProperties(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /empty:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                xml:
                  name: Empty`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/empty").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// schema with no properties should still validate
	valid, validationErrors := validator.ValidateXMLString(schema, "<Empty><anything>value</anything></Empty>")
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_PrimitiveValue(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /simple:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: string
                xml:
                  name: Value`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/simple").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// primitive value (non-object) should work
	valid, validationErrors := validator.ValidateXMLString(schema, "<Value>hello world</Value>")
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_ArrayNotWrapped(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /items:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      type: string
                xml:
                  name: Items`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/items").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// array without wrapped - items are direct siblings
	validXML := `<Items><items>one</items><items>two</items><items>three</items></Items>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_WrappedArrayWithWrongItemName(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /collection:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    xml:
                      wrapped: true
                    items:
                      additionalProperties: false
                      type: object
                      properties:
                        value:
                          type: string
                      xml:
                        name: record
                xml:
                  name: Collection`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/collection").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()

	// wrapper contains items with wrong name (item instead of record)
	// this tests the fallback path where unwrapped element is not found
	xmlWithWrongItemName := `<Collection><data><item><value>test</value></item></data></Collection>`
	valid, _ := validator.ValidateXMLString(schema, xmlWithWrongItemName)
	assert.False(t, valid)

	xmlWithWrightItemName := `<Collection><data><record><value>test</value></record></data></Collection>`
	valid, _ = validator.ValidateXMLString(schema, xmlWithWrightItemName)
	assert.True(t, valid)
}

func TestValidateXML_DirectArrayValue(t *testing.T) {
	// test unwrapArrayElement with non-map value (line 160)
	schema := &base.Schema{
		Type: []string{"array"},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: &base.SchemaProxy{},
		},
		XML: &base.XML{
			Wrapped: true,
		},
	}

	// when val is already an array (not a map), it should return as-is
	arrayVal := []interface{}{"one", "two", "three"}
	result := unwrapArrayElement(arrayVal, "", schema)
	assert.Equal(t, arrayVal, result)
}

func TestValidateXML_UnwrapArrayElementMissingItem(t *testing.T) {
	// test unwrapArrayElement when wrapper map doesn't contain expected item (line 177)
	schema := &base.Schema{
		Type: []string{"array"},
		Items: &base.DynamicValue[*base.SchemaProxy, bool]{
			A: &base.SchemaProxy{},
		},
		XML: &base.XML{
			Wrapped: true,
		},
	}

	// wrapper map contains wrong key - should return map as-is (line 177)
	wrapperMap := map[string]interface{}{"wrongKey": []interface{}{"one", "two"}}
	result := unwrapArrayElement(wrapperMap, "", schema)
	assert.Equal(t, wrapperMap, result)
}

func TestTransformXMLToSchemaJSON_EmptyString(t *testing.T) {
	// test empty string error path (line 68)
	schema := &base.Schema{}
	_, err := TransformXMLToSchemaJSON("", schema)
	assert.Len(t, err, 1)
	assert.Contains(t, err[0].Reason, "empty xml content")
}

func TestApplyXMLTransformations_NoXMLName(t *testing.T) {
	// test schema without xml.name - data stays wrapped
	schema := &base.Schema{
		Properties: nil,
	}
	xmlNsMap := make(map[string]string, 2)
	data := map[string]interface{}{"Cat": map[string]interface{}{"nice": "true"}}
	result, err := applyXMLTransformations(data, schema, &xmlNsMap)
	assert.Len(t, err, 0)
	assert.Equal(t, data, result)
}

func TestIsXMLContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{"application/xml", "application/xml", true},
		{"text/xml", "text/xml", true},
		{"application/soap+xml", "application/soap+xml", true},
		{"application/json", "application/json", false},
		{"text/plain", "text/plain", false},
		{"with whitespace", "  application/xml  ", true},
		{"mixed case", "APPLICATION/XML", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsXMLContentType(tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransformXMLToSchemaJSON_InvalixXml(t *testing.T) {
	schema := &base.Schema{}
	_, err := TransformXMLToSchemaJSON("<xmlaaaaaaaaaaaaaaaaaa><", schema)
	assert.Len(t, err, 1)
	assert.Contains(t, err[0].Reason, "malformed xml")
}

func TestValidateXmlNs_NoData(t *testing.T) {
	errors := validateXmlNs(nil, nil, "", nil)
	assert.Len(t, errors, 0)
}

func getXmlTestSchema(t *testing.T) *base.Schema {
	spec := `openapi: 3.1
paths:
 /collection:
  get:
    responses:
      '200':
        content:
          application/xml:
            schema:
              type: object
              additionalProperties: false
              properties:
                body:
                  type: object
                  required:
                    - id
                    - success
                    - payload
                  xml:
                    prefix: t
                    namespace: http://assert.t
                    name: reqBody
                  properties:
                    id: 
                      type: integer
                      xml:
                        attribute: true
                    success:
                      xml:
                        name: ok
                        prefix: j
                        namespace: http://j.j
                      type: boolean
                    payload:
                      oneOf:
                        - type: integer
                        - type: object
                data:
                  type: array
                  xml:
                    wrapped: true
                    name: list
                  items:
                    additionalProperties: false
                    type: object
                    required:
                      - value
                    properties:
                      value:
                        type: string
                        xml:
                          namespace: http://prop.arr
                          prefix: arr
                    xml:
                      name: record
                      prefix: unt
                      namespace: http://expect.t
              xml:
                name: Collection`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/collection").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	return schema
}

func TestValidateXmlNs_InvalidPrefix(t *testing.T) {
	schema := getXmlTestSchema(t)
	validator := NewXMLValidator()
	xmlPayload := `<Collection><reqBody></reqBody></Collection>`
	valid, err := validator.ValidateXMLString(schema, xmlPayload)

	assert.False(t, valid)
	assert.Equal(t, helpers.XmlValidationPrefix, err[0].ValidationSubType)
}

func TestValidateXmlNs_InvalidNamespace(t *testing.T) {
	schema := getXmlTestSchema(t)
	validator := NewXMLValidator()
	xmlPayload := `<Collection><t:reqBody xmlns:t="incorrectUrl"></t:reqBody></Collection>`
	valid, err := validator.ValidateXMLString(schema, xmlPayload)

	assert.False(t, valid)
	assert.Equal(t, helpers.XmlValidationNamespace, err[0].ValidationSubType)
}

func TestValidateXmlNs_InvalidNamespaceInRoot(t *testing.T) {
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
                xml:
                  name: Cat
                  prefix: c
                  namespace: http://cat.ca`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()
	xmlPayload := `<c:Cat xmlns:c="invalid"></c:Cat>`

	valid, validationErrors := validator.ValidateXMLString(schema, xmlPayload)

	assert.False(t, valid)
	assert.Equal(t, "The namespace from prefix 'c' differs from the xml", validationErrors[0].Message)
	assert.Equal(t, helpers.XmlValidationNamespace, validationErrors[0].ValidationSubType)
}

func TestValidateXmlNs_CorrectNamespaceInRoot(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /pet:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: string
                xml:
                  name: Cat
                  prefix: c
                  namespace: http://cat.ca`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/pet").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewXMLValidator()
	xmlPayload := `<c:Cat xmlns:c="http://cat.ca">meow</c:Cat>`

	valid, validationErrors := validator.ValidateXMLString(schema, xmlPayload)

	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestConvertBasedOnSchema_XmlSuccessfullyConverted(t *testing.T) {
	schema := getXmlTestSchema(t)
	validator := NewXMLValidator()

	xmlPayload := `<Collection><t:reqBody xmlns:t="http://assert.t" id="2"><j:ok xmlns:j="http://j.j">true</j:ok><payload><any>2</any></payload></t:reqBody>
<list xmlns:unt="http://expect.t"><unt:record><arr:value xmlns:arr="http://prop.arr">Text</arr:value></unt:record></list></Collection>`

	valid, err := validator.ValidateXMLString(schema, xmlPayload)

	assert.True(t, valid)
	assert.Len(t, err, 0)
}

func TestConvertBasedOnSchema_MissingPrefixInObjectProperties(t *testing.T) {
	schema := getXmlTestSchema(t)
	validator := NewXMLValidator()

	xmlPayload := `<Collection><t:reqBody xmlns:t="http://assert.t" id="2"><ok>true</ok><payload><any>2</any></payload></t:reqBody>
<list xmlns:unt="http://expect.t"><unt:record><arr:value xmlns:arr="http://prop.arr">Text</arr:value></unt:record></list></Collection>`

	valid, err := validator.ValidateXMLString(schema, xmlPayload)

	assert.False(t, valid)
	assert.Equal(t, helpers.XmlValidationPrefix, err[0].ValidationSubType)
	assert.Equal(t, "The prefix 'j' is defined in the schema, however it's missing from the xml", err[0].Message)
}

func TestConvertBasedOnSchema_MissingPrefixInArrayItemProperties(t *testing.T) {
	schema := getXmlTestSchema(t)
	validator := NewXMLValidator()

	xmlPayload := `<Collection><t:reqBody xmlns:t="http://assert.t" id="2"><j:ok xmlns:j="http://j.j">true</j:ok><payload><any>2</any></payload></t:reqBody>
<list xmlns:unt="http://expect.t"><unt:record><value>Text</value></unt:record></list></Collection>`

	valid, err := validator.ValidateXMLString(schema, xmlPayload)

	assert.False(t, valid)
	assert.Equal(t, helpers.XmlValidationPrefix, err[0].ValidationSubType)
	assert.Equal(t, "The prefix 'arr' is defined in the schema, however it's missing from the xml", err[0].Message)
}

func TestApplyXMLTransformations_IncorrectSchema(t *testing.T) {
	schema := getXmlTestSchema(t)
	validator := NewXMLValidator()

	xmlPayload := `<Collection><t:reqBody xmlns:t="http://assert.t" id="2"><j:ok xmlns:j="http://j.j">NotBoolean</j:ok><payload><any>NotInteger</any></payload></t:reqBody>
<list xmlns:unt="http://expect.t"><unt:record><arr:value xmlns:arr="http://prop.arr">Text</arr:value></unt:record></list></Collection>`

	valid, err := validator.ValidateXMLString(schema, xmlPayload)

	assert.False(t, valid)
	assert.Equal(t, "got string, want boolean", err[0].SchemaValidationErrors[0].Reason)
	assert.Equal(t, "schema does not pass validation", err[0].Message)
}