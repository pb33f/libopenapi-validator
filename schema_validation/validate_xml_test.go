// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"testing"

	"github.com/pb33f/libopenapi"
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

	validator := NewSchemaValidator()
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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()
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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

	// test with version 3.0 - should allow nullable keyword
	valid, validationErrors := validator.ValidateXMLStringWithVersion(schema, "<Item><value>test</value></Item>", 3.0)
	assert.True(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_NilSchema(t *testing.T) {
	validator := NewSchemaValidator()

	// test with nil schema - should return false with empty errors
	valid, validationErrors := validator.ValidateXMLString(nil, "<Test>value</Test>")
	assert.False(t, valid)
	assert.Len(t, validationErrors, 0)
}

func TestValidateXML_TrulyMalformedXML(t *testing.T) {
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
                xml:
                  name: Test`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/test").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewSchemaValidator()

	// test with completely malformed xml - mismatched tags
	valid, validationErrors := validator.ValidateXMLString(schema, "<Test><bad>value</wrong></Test>")
	assert.False(t, valid)
	assert.NotEmpty(t, validationErrors)
	assert.Contains(t, validationErrors[0].Reason, "xml")
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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

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

	validator := NewSchemaValidator()

	// wrapper contains items with wrong name (item instead of record)
	// this tests the fallback path where unwrapped element is not found
	xmlWithWrongItemName := `<Collection><data><item><value>test</value></item></data></Collection>`
	valid, validationErrors := validator.ValidateXMLString(schema, xmlWithWrongItemName)

	// it should still process (might fail schema validation but won't crash)
	assert.NotNil(t, validationErrors)
}

func TestValidateXML_WrappedArrayAsNonMap(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /list:
    get:
      responses:
        '200':
          content:
            application/xml:
              schema:
                type: object
                properties:
                  values:
                    type: array
                    xml:
                      wrapped: true
                    items:
                      type: string
                      xml:
                        name: value
                xml:
                  name: List`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	schema := v3Doc.Model.Paths.PathItems.GetOrZero("/list").Get.Responses.Codes.GetOrZero("200").
		Content.GetOrZero("application/xml").Schema.Schema()

	validator := NewSchemaValidator()

	// unwrapped array (direct values) - tests non-map value path
	validXML := `<List><values>one</values><values>two</values></List>`
	valid, validationErrors := validator.ValidateXMLString(schema, validXML)

	// this tests the path where val is already an array, not a wrapper map
	assert.NotNil(t, validationErrors)
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
