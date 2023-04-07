// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
    "github.com/pb33f/libopenapi"
    "github.com/stretchr/testify/assert"
    "net/http"
    "testing"
)

func TestNewValidator_CookieParamNumberValid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
    request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "1"})

    valid, errors := v.ValidateCookieParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamNumberValidFloat(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
    request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "123.455"})

    valid, errors := v.ValidateCookieParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamNumberInvalid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: number`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
    request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "false"})

    valid, errors := v.ValidateCookieParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Convert the value 'false' into a number", errors[0].HowToFix)
}

func TestNewValidator_CookieParamBooleanValid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: boolean`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
    request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "true"})

    valid, errors := v.ValidateCookieParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}

func TestNewValidator_CookieParamBooleanInvalid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          schema:
            type: boolean`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
    request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "12345"})

    valid, errors := v.ValidateCookieParams(request)

    assert.False(t, valid)
    assert.Len(t, errors, 1)
    assert.Equal(t, "Convert the value '12345' into a true/false value", errors[0].HowToFix)
}

func TestNewValidator_CookieParamObjectValid(t *testing.T) {

    spec := `openapi: 3.1.0
paths:
  /burgers/beef:
    get:
      parameters:
        - name: PattyPreference
          in: cookie
          required: true
          explode: false
          schema:
            type: object
            properties:
              pink:
                type: boolean
              number:
                type: number
            required: [pink, number]`

    doc, _ := libopenapi.NewDocument([]byte(spec))
    m, _ := doc.BuildV3Model()
    v := NewValidator(&m.Model)

    request, _ := http.NewRequest(http.MethodGet, "https://things.com/burgers/beef", nil)
    request.AddCookie(&http.Cookie{Name: "PattyPreference", Value: "pink,true,number,2"})

    valid, errors := v.ValidateCookieParams(request)

    assert.True(t, valid)
    assert.Len(t, errors, 0)
}
