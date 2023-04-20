// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"bytes"
	"encoding/json"
	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestNewValidator_ValidateHttpRequest_ValidPostSimpleSchema(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                patties:
                  type: integer
                vegetarian:
                  type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	v, _ := NewValidator(doc)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2,
		"vegetarian": true,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateHttpRequest(request)

	assert.True(t, valid)
	assert.Len(t, errors, 0)

}

func TestNewValidator_ValidateHttpRequest_InvalidPostSchema(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                patties:
                  type: integer
                vegetarian:
                  type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	v, _ := NewValidator(doc)

	// mix up the primitives to fire two schema violations.
	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    false, // wrong.
		"vegetarian": false,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateHttpRequest(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "expected integer, but got boolean", errors[0].SchemaValidationErrors[0].Reason)

}

func TestNewValidator_ValidateHttpRequest_InvalidQuery(t *testing.T) {

	spec := `openapi: 3.1.0
paths:
  /burgers/createBurger:
    parameters:
       - in: query
         name: cheese
         required: true
         schema:
           type: string
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                patties:
                  type: integer
                vegetarian:
                  type: boolean`

	doc, _ := libopenapi.NewDocument([]byte(spec))

	v, _ := NewValidator(doc)

	body := map[string]interface{}{
		"name":       "Big Mac",
		"patties":    2, // wrong.
		"vegetarian": false,
	}

	bodyBytes, _ := json.Marshal(body)

	request, _ := http.NewRequest(http.MethodPost, "https://things.com/burgers/createBurger",
		bytes.NewBuffer(bodyBytes))
	request.Header.Set("Content-Type", "application/json")

	valid, errors := v.ValidateHttpRequest(request)

	assert.False(t, valid)
	assert.Len(t, errors, 1)
	assert.Equal(t, "Query parameter 'cheese' is missing", errors[0].Message)

}
