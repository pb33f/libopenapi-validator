// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"net/http"
	"strings"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/utils"
)

func (v *requestBodyValidator) ValidateRequestBody(request *http.Request) (bool, []*errors.ValidationError) {
	// find path
	var pathItem *v3.PathItem = v.pathItem
	if v.pathItem == nil {
		var validationErrors []*errors.ValidationError
		pathItem, validationErrors, _ = paths.FindPath(request, v.document)
		if pathItem == nil || validationErrors != nil {
			v.errors = validationErrors
			return false, validationErrors
		}
	}

	operation := helpers.ExtractOperation(request, pathItem)
	if operation.RequestBody == nil {
		return true, nil
	}

	// extract the content type from the request
	contentType := request.Header.Get(helpers.ContentTypeHeader)
	if contentType == "" {
		return false, []*errors.ValidationError{errors.RequestContentTypeNotFound(operation, request)}
	}

	// extract the media type from the content type header.
	ct, _, _ := helpers.ExtractContentType(contentType)
	mediaType, ok := operation.RequestBody.Content.Get(ct)
	if !ok {
		return false, []*errors.ValidationError{errors.RequestContentTypeNotFound(operation, request)}
	}

	// we currently only support JSON validation for request bodies
	// this will capture *everything* that contains some form of 'json' in the content type
	if !strings.Contains(strings.ToLower(contentType), helpers.JSONType) {
		return true, nil
	}

	// Nothing to validate
	if mediaType.Schema == nil {
		return true, nil
	}

	// extract schema from media type
	var schema *base.Schema
	var renderedInline, renderedJSON []byte

	// have we seen this schema before? let's hash it and check the cache.
	hash := mediaType.GoLow().Schema.Value.Hash()

	// perform work only once and cache the result in the validator.
	if cacheHit, ch := v.schemaCache[hash]; ch {
		// got a hit, use cached values
		schema = cacheHit.schema
		renderedInline = cacheHit.renderedInline
		renderedJSON = cacheHit.renderedJSON

	} else {

		// render the schema inline and perform the intensive work of rendering and converting
		// this is only performed once per schema and cached in the validator.
		schema = mediaType.Schema.Schema()
		renderedInline, _ = schema.RenderInline()
		renderedJSON, _ = utils.ConvertYAMLtoJSON(renderedInline)
		v.schemaCache[hash] = &schemaCache{
			schema:         schema,
			renderedInline: renderedInline,
			renderedJSON:   renderedJSON,
		}
	}

	// render the schema, to be used for validation
	return ValidateRequestSchema(request, schema, renderedInline, renderedJSON)
}
