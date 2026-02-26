package schema_validation

import (
	"testing"

	"github.com/pb33f/libopenapi"
	derrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
)

func TestIsURLEncodedContentType(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"application/x-www-form-urlencoded", true},
		{"APPLICATION/X-WWW-FORM-URLENCODED", true},
		{"application/x-www-form-urlencoded; charset=utf-8", true},
		{"application/json", false},
		{"", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, IsURLEncodedContentType(tt.input))
	}
}

func TestUnflattenValues(t *testing.T) {
	vals := map[string][]string{
		"simple":     {"val"},
		"arr[]":      {"1", "2"},
		"obj[prop]":  {"v1"},
		"deep[a][b]": {"v2"},
		"double":     {"1", "2"},
	}

	result := unflattenValues(vals)

	assert.Equal(t, "val", result["simple"])
	assert.Equal(t, []string{"1", "2"}, result["arr"])
	assert.Equal(t, []string{"1", "2"}, result["double"])

	obj, ok := result["obj"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "v1", obj["prop"])

	deep, ok := result["deep"].(map[string]any)
	assert.True(t, ok)
	inner, ok := deep["a"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "v2", inner["b"])
}

func TestBuildDeepMap_BranchCoverage(t *testing.T) {
	root := make(map[string]any)

	root["collision"] = "string_value"

	buildDeepMap(root, "collision[sub]", []string{"val"})

	assert.Equal(t, "string_value", root["collision"])

	root2 := make(map[string]any)
	buildDeepMap(root2, "arr[key]", []string{"a", "b"})

	inner := root2["arr"].(map[string]any)
	assert.Equal(t, []string{"a", "b"}, inner["key"])
}

func TestTransformURLEncodedToSchemaJSON(t *testing.T) {
	t.Run("Malformed URL Encoding", func(t *testing.T) {
		res, errs := TransformURLEncodedToSchemaJSON("bad_encoding=%zz", nil, nil)
		assert.Nil(t, res)
		assert.Len(t, errs, 1)
		assert.Equal(t, helpers.URLEncodedValidation, errs[0].ValidationType)
	})

	t.Run("Schema is Nil", func(t *testing.T) {
		res, errs := TransformURLEncodedToSchemaJSON("foo=bar", nil, nil)
		assert.Empty(t, errs)
		assert.Equal(t, "bar", res["foo"])
	})

	t.Run("Apply Encoding Rules & Reserved Characters", func(t *testing.T) {
		props := orderedmap.New[string, *base.SchemaProxy]()
		props.Set("jsonField", base.CreateSchemaProxy(&base.Schema{Type: []string{helpers.Object}}))
		props.Set("restricted", base.CreateSchemaProxy(&base.Schema{Type: []string{helpers.String}}))

		schema := &base.Schema{Properties: props}

		encodings := orderedmap.New[string, *v3.Encoding]()
		encodings.Set("jsonField", &v3.Encoding{ContentType: helpers.JSONContentType})
		encodings.Set("restricted", &v3.Encoding{AllowReserved: false})

		body := `jsonField={"id":1}&restricted=badvalue!`

		res, errs := TransformURLEncodedToSchemaJSON(body, schema, encodings)

		assert.IsType(t, map[string]any{}, res["jsonField"])

		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "contains reserved characters")
	})

	t.Run("Encoding Error (Invalid JSON content type)", func(t *testing.T) {
		props := orderedmap.New[string, *base.SchemaProxy]()
		props.Set("badJson", base.CreateSchemaProxy(&base.Schema{}))
		schema := &base.Schema{Properties: props}

		encodings := orderedmap.New[string, *v3.Encoding]()
		encodings.Set("badJson", &v3.Encoding{ContentType: helpers.JSONContentType})

		res, errs := TransformURLEncodedToSchemaJSON(`badJson={invalid`, schema, encodings)
		assert.Len(t, errs, 1)

		assert.Equal(t, helpers.URLEncodedValidation, errs[0].ValidationType)

		assert.Equal(t, "{invalid", res["badJson"])
	})

	t.Run("Coercion triggered", func(t *testing.T) {
		props := orderedmap.New[string, *base.SchemaProxy]()
		props.Set("num", base.CreateSchemaProxy(&base.Schema{Type: []string{helpers.Integer}}))
		schema := &base.Schema{Properties: props, Type: []string{helpers.Object}}

		res, errs := TransformURLEncodedToSchemaJSON("num=123", schema, nil)
		assert.Empty(t, errs)
		assert.Equal(t, int64(123), res["num"])
	})
}

func TestApplyEncodingRules(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	t.Run("DeepObject Style", func(t *testing.T) {
		enc := &v3.Encoding{Style: "deepObject"}

		res, _ := applyEncodingRules("not-map", enc, nil)
		assert.Equal(t, "not-map", res)

		m := map[string]any{"k": "v"}
		res2, _ := applyEncodingRules(m, enc, nil)
		assert.Equal(t, m, res2)
	})

	t.Run("Array Delimiters", func(t *testing.T) {
		schema := &base.Schema{Type: []string{helpers.Array}}

		encSpace := &v3.Encoding{Style: "spaceDelimited"}
		res, _ := applyEncodingRules("a b c", encSpace, schema)
		assert.Equal(t, []string{"a", "b", "c"}, res)

		encPipe := &v3.Encoding{Style: "pipeDelimited"}
		res, _ = applyEncodingRules("a|b|c", encPipe, schema)
		assert.Equal(t, []string{"a", "b", "c"}, res)

		encForm := &v3.Encoding{Style: "form", Explode: boolPtr(false)}
		res, _ = applyEncodingRules("a,b,c", encForm, schema)
		assert.Equal(t, []string{"a", "b", "c"}, res)
	})
}

func TestValidateEncodingRecursive(t *testing.T) {
	var errs []*derrors.ValidationError

	validateEncodingRecursive("p", "val!", true, &errs, nil)
	assert.Empty(t, errs)

	validateEncodingRecursive("p", "val!", false, &errs, nil)
	assert.Len(t, errs, 1)

	errs = nil
	validateEncodingRecursive("arr", []any{"ok", "bad!"}, false, &errs, nil)
	assert.Len(t, errs, 1)

	errs = nil
	validateEncodingRecursive("map", map[string]any{"k": "bad!"}, false, &errs, nil)
	assert.Len(t, errs, 1)

	errs = nil
	validateEncodingRecursive("s_arr", []string{"ok", "bad!"}, false, &errs, nil)
	assert.Len(t, errs, 1)
}

func TestCoerceValue(t *testing.T) {
	schemaInt := &base.Schema{Type: []string{helpers.Integer}}
	schemaNum := &base.Schema{Type: []string{helpers.Number}}
	schemaBool := &base.Schema{Type: []string{helpers.Boolean}}
	schemaStr := &base.Schema{Type: []string{helpers.String}}

	t.Run("Complex Schema Aggregation (AllOf)", func(t *testing.T) {
		s := &base.Schema{
			AllOf: []*base.SchemaProxy{
				base.CreateSchemaProxy(schemaInt),
			},
		}
		res := coerceValue("123", s)
		assert.Equal(t, int64(123), res)
	})

	t.Run("No Target Types", func(t *testing.T) {
		res := coerceValue("val", &base.Schema{})
		assert.Equal(t, "val", res)
		res = coerceValue("newVal", nil)
		assert.Equal(t, "newVal", res)
	})

	t.Run("String Slice input (take first)", func(t *testing.T) {
		res := coerceValue([]string{"123"}, schemaInt)
		assert.Equal(t, int64(123), res)
	})

	t.Run("Integer Conversions", func(t *testing.T) {
		assert.Equal(t, "abc", coerceValue("abc", schemaInt))
		assert.Equal(t, "", coerceValue("", schemaInt))
		assert.Equal(t, 123, coerceValue(123, schemaInt))
		assert.Equal(t, int64(123), coerceValue("123", schemaInt))
	})

	t.Run("Number Conversions", func(t *testing.T) {
		assert.Equal(t, 12.34, coerceValue("12.34", schemaNum))
		assert.Equal(t, "abc", coerceValue("abc", schemaNum))
		assert.Equal(t, 13.2, coerceValue(13.2, schemaNum))
		assert.Equal(t, 5, coerceValue(5, nil))
	})

	t.Run("Boolean Conversions", func(t *testing.T) {
		assert.Equal(t, true, coerceValue("true", schemaBool))
		assert.Equal(t, 123, coerceValue(123, schemaBool))
	})

	t.Run("String Conversions", func(t *testing.T) {
		assert.Equal(t, "val", coerceValue("val", schemaStr))
		assert.Equal(t, "123", coerceValue(123, schemaStr))
	})

	t.Run("Array Conversions", func(t *testing.T) {
		arrSchema := &base.Schema{
			Type: []string{helpers.Array},
			Items: &base.DynamicValue[*base.SchemaProxy, bool]{A: base.CreateSchemaProxy(&base.Schema{
				Type: []string{helpers.Integer},
			})},
		}

		noItem := coerceValue("a", &base.Schema{
			Type: []string{helpers.Array},
		})
		assert.Equal(t, []any{"a"}, noItem)

		res1 := coerceValue([]any{"1", "2"}, arrSchema)
		assert.Equal(t, []any{int64(1), int64(2)}, res1)

		res2 := coerceValue([]string{"1", "2"}, arrSchema)
		assert.Equal(t, []any{int64(1), int64(2)}, res2)

		mapInput := map[string]any{"1": "20", "0": "10"}
		res3 := coerceValue(mapInput, arrSchema)
		assert.IsType(t, []any{}, res3)
		sliceRes := res3.([]any)
		assert.Equal(t, int64(10), sliceRes[0])
		assert.Equal(t, int64(20), sliceRes[1])

		mapBad := map[string]any{"foo": "bar"}
		res4 := coerceValue(mapBad, arrSchema)
		assert.Equal(t, mapBad, res4)

		res5 := coerceValue("10", arrSchema)
		assert.Equal(t, []any{int64(10)}, res5)
	})

	t.Run("Object Conversions", func(t *testing.T) {
		objSchema := &base.Schema{
			Type:       []string{helpers.Object},
			Properties: orderedmap.New[string, *base.SchemaProxy](),
		}
		objSchema.Properties.Set("num", base.CreateSchemaProxy(schemaInt))

		input := map[string]any{"num": "55", "other": "val"}

		res := coerceValue(input, objSchema)
		resMap, ok := res.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, int64(55), resMap["num"])
		assert.Equal(t, "val", resMap["other"])
	})
}

func TestIsArraySchema(t *testing.T) {
	assert.False(t, isArraySchema(nil))
	assert.False(t, isArraySchema(&base.Schema{Type: []string{"string"}}))
	assert.True(t, isArraySchema(&base.Schema{Type: []string{"array"}}))
}

func TestComplexBodies(t *testing.T) {
	spec := `{
  "openapi": "3.1.0",
  "paths": {
    "/posts": {
      "put": {
        "requestBody": {
          "content": {
            "application/x-www-form-urlencoded": {
              "encoding": {
                "payload": {
                  "contentType": "application/json"
                },
                "title": {
                  "allowReserved": true
                },
                "pipeArr": {
                  "style": "pipeDelimited"
                },
                "spaceArr": {
                  "style": "spaceDelimited"
                },
                "unexplodedArr": {
                  "explode": false
                }
              },
              "schema": {
                "additionalProperties": false,
                "properties": {
                  "content": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "additionalProperties": false,
                      "properties": {
                        "name": {"oneOf": [
                        {
                          "type": ["boolean"]
                        },
                         {
                          "type": ["integer"]
                        }]}
                      }
                    }
                  },
                  "bool": {
                    "type": ["boolean"],
                    "enum": [false]
                  },
                  "reserved": {
                    "type": ["string"]
                  },
                  "title": {
                    "type": ["string"]
                  },
                  "pipeArr": {
                    "type": "array",
                    "items": {"type": "integer"}
                  },
                  "spaceArr": {
                    "type": "array",
                    "items": {"type": "integer"}
                  },
                  "unexplodedArr": {
                    "type": "array",
                    "items": {"type": "integer"}
                  },
                  "payload": {
                    "type": "object",
                      "additionalProperties": false,
                      "properties": {
                        "hey": {
                          "type": "array",
                          "items": {
                          "type": "boolean"
                        }
                      }
                    }
                  }
                },
                "required": ["title", "bool"],
                "type": "object"
              }
            }
          },
          "required": true
        }
      }
    }
  }
}
`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	contentSchema := v3Doc.Model.Paths.PathItems.GetOrZero("/posts").Put.RequestBody.Content.GetOrZero("application/x-www-form-urlencoded")
	schema := contentSchema.Schema.Schema()
	encoding := contentSchema.Encoding

	v := NewURLEncodedValidator()

	valid, errs := v.ValidateURLEncodedString(schema, encoding, "bool=false&title=test&content[0][name]=true")
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, "bool=false&title=test&content[0][name]=4")
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, "bool=false&title=test&content[0][name]=4.4")
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, "bool=true&title=test&content[0][name]=true")
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, "bool=false&content[0][name]=true")
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, "bool=false&title")
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title&payload={"hey": [true, false]}`)
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title&payload={"hey": [2], "adittional": false}`)
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title=do not use #`)
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title&reserved=do not use #`)
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title&pipeArr=1|2|3`)
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title&spaceArr=1 2 3`)
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title&spaceArr=1%202%203`)
	assert.True(t, valid)
	assert.Len(t, errs, 0)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, `bool=false&title&unexplodedArr=1,2,3`)
	assert.True(t, valid)
	assert.Len(t, errs, 0)
}

func TestValidateURLEncoded(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /collection:
    get:
      responses:
        '200':
          content:
            application/x-www-form-urlencoded:
              schema:
                type: object`

	doc, err := libopenapi.NewDocument([]byte(spec))
	assert.NoError(t, err)

	v3Doc, err := doc.BuildV3Model()
	assert.NoError(t, err)

	contentSchema := v3Doc.Model.Paths.PathItems.GetOrZero("/collection").Get.Responses.Codes.GetOrZero("200").Content.GetOrZero("application/x-www-form-urlencoded")
	schema := contentSchema.Schema.Schema()
	encoding := contentSchema.Encoding

	v := NewURLEncodedValidator()

	valid, errs := v.ValidateURLEncodedStringWithVersion(schema, encoding, "a=1", 3.1)
	assert.True(t, valid)
	assert.Empty(t, errs)

	valid, _ = v.ValidateURLEncodedStringWithVersion(nil, nil, "a=1", 3.1)
	assert.False(t, valid)

	valid, errs = v.ValidateURLEncodedString(schema, encoding, "a=1")
	assert.True(t, valid)
	assert.Empty(t, errs)

	valid, _ = v.ValidateURLEncodedString(nil, nil, "a=1")
	assert.False(t, valid)
}
