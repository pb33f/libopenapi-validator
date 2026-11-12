// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"

	"github.com/pb33f/libopenapi-validator/config"
)

// The scenario specs below cover the gateway shapes the main benchmark file
// does not: format assertions, strict server matching, urlencoded bodies,
// JSON response bodies, and a large routing table.

const formatAssertionsSpec = `openapi: 3.1.0
info:
  title: bench
  version: 1.0.0
paths:
  /users/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: ok
`

const strictServerSpec = `openapi: 3.1.0
info:
  title: bench
  version: 1.0.0
servers:
  - url: https://things.com/api
paths:
  /users/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: ok
`

const urlEncodedSpec = `openapi: 3.1.0
info:
  title: bench
  version: 1.0.0
paths:
  /burgers:
    post:
      requestBody:
        required: true
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                size:
                  type: string
      responses:
        '200':
          description: ok
`

const responseJSONSpec = `openapi: 3.1.0
info:
  title: bench
  version: 1.0.0
paths:
  /reports/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [ok]
                properties:
                  ok:
                    type: boolean
`

// largeRouteSpec builds a document with count templated path items.
func largeRouteSpec(count int) string {
	var spec strings.Builder
	spec.WriteString(`openapi: 3.1.0
info:
  title: bench
  version: 1.0.0
paths:
`)
	for i := 0; i < count; i++ {
		fmt.Fprintf(&spec, "  /resource-%d/{id}:\n    get:\n      parameters:\n        - name: id\n          in: path\n          required: true\n          schema:\n            type: string\n            format: uuid\n      responses:\n        '200':\n          description: ok\n", i)
	}
	return spec.String()
}

func nopBody(body []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(body))
}

// prepareScenarioValidator builds a validator for the given spec and options
// and fails the benchmark on build errors.
func prepareScenarioValidator(b *testing.B, spec string, opts ...config.Option) Validator {
	b.Helper()

	doc, err := libopenapi.NewDocument([]byte(spec))
	if err != nil {
		b.Fatalf("error creating document: %v", err)
	}

	v, errs := NewValidator(doc, opts...)
	if len(errs) > 0 {
		b.Fatalf("errors building validator: %v", errs)
	}
	b.Cleanup(v.Release)
	return v
}

// BenchmarkValidateHttpRequestSyncFormatAssertions measures one valid GET with
// format assertions enabled, so the uuid format machinery runs on every call.
func BenchmarkValidateHttpRequestSyncFormatAssertions(b *testing.B) {
	v := prepareScenarioValidator(b, formatAssertionsSpec, config.WithFormatAssertions())

	req := benchRequest(b, http.MethodGet, "/users/3f2504e0-4f89-11d3-9a0c-0305e82c3301", nil)

	b.ReportAllocs()

	for b.Loop() {
		valid, validationErrors := v.ValidateHttpRequestSync(req)
		if !valid {
			b.Fatalf("request should validate, got %d errors", len(validationErrors))
		}
	}
}

// BenchmarkValidateHttpRequestSyncStrictServer measures one valid GET with
// strict server matching enabled: scheme, host and base path take part in
// routing.
func BenchmarkValidateHttpRequestSyncStrictServer(b *testing.B) {
	v := prepareScenarioValidator(b, strictServerSpec, config.WithStrictServerMatching())

	req := benchRequest(b, http.MethodGet, "https://things.com/api/users/3f2504e0-4f89-11d3-9a0c-0305e82c3301", nil)

	b.ReportAllocs()

	for b.Loop() {
		valid, validationErrors := v.ValidateHttpRequestSync(req)
		if !valid {
			b.Fatalf("request should validate, got %d errors", len(validationErrors))
		}
	}
}

// BenchmarkValidateHttpRequestSyncURLEncoded measures one valid POST with a
// form-urlencoded body. The request is rebuilt per iteration, because
// validation consumes the body reader.
func BenchmarkValidateHttpRequestSyncURLEncoded(b *testing.B) {
	v := prepareScenarioValidator(b, urlEncodedSpec, config.WithURLEncodedBodyValidation())

	body := []byte("name=cheeseburger&size=large")

	b.ReportAllocs()

	for b.Loop() {
		req := benchRequest(b, http.MethodPost, "/burgers", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		valid, validationErrors := v.ValidateHttpRequestSync(req)
		if !valid {
			b.Fatalf("request should validate, got %d errors", len(validationErrors))
		}
	}
}

// BenchmarkValidateHttpResponseJSONBody measures one valid 200 response with a
// JSON body. The response is rebuilt per iteration, because validation
// consumes the body reader.
func BenchmarkValidateHttpResponseJSONBody(b *testing.B) {
	v := prepareScenarioValidator(b, responseJSONSpec)

	req := benchRequest(b, http.MethodGet, "/reports/3f2504e0-4f89-11d3-9a0c-0305e82c3301", nil)
	body := []byte(`{"ok":true}`)

	b.ReportAllocs()

	for b.Loop() {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       nopBody(body),
		}

		valid, validationErrors := v.ValidateHttpResponse(req, resp)
		if !valid {
			b.Fatalf("response should validate, got %d errors", len(validationErrors))
		}
	}
}

// BenchmarkValidateHttpRequestSyncLargeRoutes measures one valid request
// against a document with 200 path items, where the routing table and the
// regex fallback show their cost.
func BenchmarkValidateHttpRequestSyncLargeRoutes(b *testing.B) {
	v := prepareScenarioValidator(b, largeRouteSpec(200))

	req := benchRequest(b, http.MethodGet, "/resource-150/3f2504e0-4f89-11d3-9a0c-0305e82c3301", nil)

	b.ReportAllocs()

	for b.Loop() {
		valid, validationErrors := v.ValidateHttpRequestSync(req)
		if !valid {
			b.Fatalf("request should validate, got %d errors", len(validationErrors))
		}
	}
}
