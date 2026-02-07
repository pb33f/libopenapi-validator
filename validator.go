// Copyright 2023-2025 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/utils"
	"go.yaml.in/yaml/v4"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/cache"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi-validator/parameters"
	"github.com/pb33f/libopenapi-validator/radix"
	"github.com/pb33f/libopenapi-validator/requests"
	"github.com/pb33f/libopenapi-validator/responses"
	"github.com/pb33f/libopenapi-validator/schema_validation"
)

// Validator provides a coarse grained interface for validating an OpenAPI 3+ documents.
// There are three primary use-cases for validation
//
// Validating *http.Request objects against and OpenAPI 3+ document
// Validating *http.Response objects against an OpenAPI 3+ document
// Validating an OpenAPI 3+ document against the OpenAPI 3+ specification
type Validator interface {
	// ValidateHttpRequest will validate an *http.Request object against an OpenAPI 3+ document.
	// The path, query, cookie and header parameters and request body are validated.
	ValidateHttpRequest(request *http.Request) (bool, []*errors.ValidationError)
	// ValidateHttpRequestSync will validate an *http.Request object against an OpenAPI 3+ document synchronously and without spawning any goroutines.
	// The path, query, cookie and header parameters and request body are validated.
	ValidateHttpRequestSync(request *http.Request) (bool, []*errors.ValidationError)

	// ValidateHttpRequestWithPathItem will validate an *http.Request object against an OpenAPI 3+ document.
	// The path, query, cookie and header parameters and request body are validated.
	ValidateHttpRequestWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError)

	// ValidateHttpRequestSyncWithPathItem will validate an *http.Request object against an OpenAPI 3+ document synchronously and without spawning any goroutines.
	// The path, query, cookie and header parameters and request body are validated.
	ValidateHttpRequestSyncWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError)

	// ValidateHttpResponse will an *http.Response object against an OpenAPI 3+ document.
	// The response body is validated. The request is only used to extract the correct response from the spec.
	ValidateHttpResponse(request *http.Request, response *http.Response) (bool, []*errors.ValidationError)

	// ValidateHttpRequestResponse will validate both the *http.Request and *http.Response objects against an OpenAPI 3+ document.
	// The path, query, cookie and header parameters and request and response body are validated.
	ValidateHttpRequestResponse(request *http.Request, response *http.Response) (bool, []*errors.ValidationError)

	// ValidateDocument will validate an OpenAPI 3+ document against the 3.0 or 3.1 OpenAPI 3+ specification
	ValidateDocument() (bool, []*errors.ValidationError)

	// GetParameterValidator will return a parameters.ParameterValidator instance used to validate parameters
	GetParameterValidator() parameters.ParameterValidator

	// GetRequestBodyValidator will return a parameters.RequestBodyValidator instance used to validate request bodies
	GetRequestBodyValidator() requests.RequestBodyValidator

	// GetResponseBodyValidator will return a parameters.ResponseBodyValidator instance used to validate response bodies
	GetResponseBodyValidator() responses.ResponseBodyValidator

	// SetDocument will set the OpenAPI 3+ document to be validated
	SetDocument(document libopenapi.Document)
}

// NewValidator will create a new Validator from an OpenAPI 3+ document
func NewValidator(document libopenapi.Document, opts ...config.Option) (Validator, []error) {
	m, errs := document.BuildV3Model()
	if errs != nil {
		return nil, []error{errs}
	}
	v := NewValidatorFromV3Model(&m.Model, opts...)
	v.(*validator).document = document
	return v, nil
}

// NewValidatorFromV3Model will create a new Validator from an OpenAPI Model
func NewValidatorFromV3Model(m *v3.Document, opts ...config.Option) Validator {
	options := config.NewValidationOptions(opts...)

	// Build radix tree for O(k) path lookup (where k = path depth)
	// Skip if explicitly set via WithPathTree (including nil to disable)
	if options.PathTree == nil && !options.IsPathTreeSet() {
		options.PathTree = radix.BuildPathTree(m)
	}

	// warm the schema caches by pre-compiling all schemas in the document
	// (warmSchemaCaches checks for nil cache and skips if disabled)
	warmSchemaCaches(m, options)

	// warm the regex cache by pre-compiling all path parameter regexes
	warmRegexCache(m, options)

	// Build the matcher chain: radix first (fast), regex fallback (handles complex patterns)
	var matchers matcherChain
	if options.PathTree != nil {
		matchers = append(matchers, &radixMatcher{pathLookup: options.PathTree})
	}
	matchers = append(matchers, &regexMatcher{regexCache: options.RegexCache})

	v := &validator{
		options:  options,
		v3Model:  m,
		matchers: matchers,
		version:  helpers.VersionToFloat(m.Version),
	}

	// create a new parameter validator
	v.paramValidator = parameters.NewParameterValidator(m, config.WithExistingOpts(options))

	// create aq new request body validator
	v.requestValidator = requests.NewRequestBodyValidator(m, config.WithExistingOpts(options))

	// create a response body validator
	v.responseValidator = responses.NewResponseBodyValidator(m, config.WithExistingOpts(options))

	return v
}

func (v *validator) SetDocument(document libopenapi.Document) {
	v.document = document
}

func (v *validator) GetParameterValidator() parameters.ParameterValidator {
	return v.paramValidator
}

func (v *validator) GetRequestBodyValidator() requests.RequestBodyValidator {
	return v.requestValidator
}

func (v *validator) GetResponseBodyValidator() responses.ResponseBodyValidator {
	return v.responseValidator
}

func (v *validator) ValidateDocument() (bool, []*errors.ValidationError) {
	if v.document == nil {
		return false, []*errors.ValidationError{{
			ValidationType:    helpers.DocumentValidation,
			ValidationSubType: helpers.ValidationMissing,
			Message:           "Document is not set",
			Reason:            "The document cannot be validated as it is not set",
			SpecLine:          1,
			SpecCol:           1,
			HowToFix:          "Set the document via `SetDocument` before validating",
		}}
	}
	var validationOpts []config.Option
	if v.options != nil {
		validationOpts = append(validationOpts, config.WithRegexEngine(v.options.RegexEngine))
	}
	return schema_validation.ValidateOpenAPIDocument(v.document, validationOpts...)
}

func (v *validator) ValidateHttpResponse(
	request *http.Request,
	response *http.Response,
) (bool, []*errors.ValidationError) {
	ctx, errs := v.buildRequestContext(request)
	if errs != nil {
		return false, errs
	}
	_, responseErrors := v.responseValidator.ValidateResponseBodyWithPathItem(
		request, response, ctx.route.pathItem, ctx.route.matchedPath)
	if len(responseErrors) > 0 {
		return false, responseErrors
	}
	return true, nil
}

func (v *validator) ValidateHttpRequestResponse(
	request *http.Request,
	response *http.Response,
) (bool, []*errors.ValidationError) {
	ctx, errs := v.buildRequestContext(request)
	if errs != nil {
		return false, errs
	}
	_, requestErrors := v.ValidateHttpRequestWithPathItem(request, ctx.route.pathItem, ctx.route.matchedPath)
	_, responseErrors := v.responseValidator.ValidateResponseBodyWithPathItem(
		request, response, ctx.route.pathItem, ctx.route.matchedPath)
	if len(requestErrors) > 0 || len(responseErrors) > 0 {
		return false, append(requestErrors, responseErrors...)
	}
	return true, nil
}

func (v *validator) ValidateHttpRequest(request *http.Request) (bool, []*errors.ValidationError) {
	// Fast path: use synchronous validation for requests without a body
	// to avoid unnecessary goroutine overhead.
	if request.Body == nil || request.ContentLength == 0 {
		return v.ValidateHttpRequestSync(request)
	}

	ctx, errs := v.buildRequestContext(request)
	if errs != nil {
		return false, errs
	}
	return v.validateWithContext(ctx)
}

func (v *validator) ValidateHttpRequestWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
	ctx := &requestContext{
		request: request,
		route: &resolvedRoute{
			pathItem:    pathItem,
			matchedPath: pathValue,
		},
		operation: helpers.OperationForMethod(request.Method, pathItem),
		version:   v.version,
	}
	return v.validateWithContext(ctx)
}

func (v *validator) validatePathParamsCtx(ctx *requestContext) (bool, []*errors.ValidationError) {
	return v.paramValidator.ValidatePathParamsWithPathItem(ctx.request, ctx.route.pathItem, ctx.route.matchedPath)
}

func (v *validator) validateQueryParamsCtx(ctx *requestContext) (bool, []*errors.ValidationError) {
	return v.paramValidator.ValidateQueryParamsWithPathItem(ctx.request, ctx.route.pathItem, ctx.route.matchedPath)
}

func (v *validator) validateHeaderParamsCtx(ctx *requestContext) (bool, []*errors.ValidationError) {
	return v.paramValidator.ValidateHeaderParamsWithPathItem(ctx.request, ctx.route.pathItem, ctx.route.matchedPath)
}

func (v *validator) validateCookieParamsCtx(ctx *requestContext) (bool, []*errors.ValidationError) {
	return v.paramValidator.ValidateCookieParamsWithPathItem(ctx.request, ctx.route.pathItem, ctx.route.matchedPath)
}

func (v *validator) validateSecurityCtx(ctx *requestContext) (bool, []*errors.ValidationError) {
	return v.paramValidator.ValidateSecurityWithPathItem(ctx.request, ctx.route.pathItem, ctx.route.matchedPath)
}

func (v *validator) validateRequestBodyCtx(ctx *requestContext) (bool, []*errors.ValidationError) {
	return v.requestValidator.ValidateRequestBodyWithPathItem(ctx.request, ctx.route.pathItem, ctx.route.matchedPath)
}

// validateRequestSync runs all validation functions sequentially using the request context.
func (v *validator) validateRequestSync(ctx *requestContext) (bool, []*errors.ValidationError) {
	var validationErrors []*errors.ValidationError
	for _, validateFunc := range []validationFunction{
		v.validatePathParamsCtx,
		v.validateCookieParamsCtx,
		v.validateHeaderParamsCtx,
		v.validateQueryParamsCtx,
		v.validateSecurityCtx,
		v.validateRequestBodyCtx,
	} {
		if valid, pErrs := validateFunc(ctx); !valid {
			validationErrors = append(validationErrors, pErrs...)
		}
	}
	return len(validationErrors) == 0, validationErrors
}

// validateWithContext runs all validation functions concurrently using a WaitGroup.
// This replaces the previous 9-goroutine/5-channel choreography with a simpler pattern.
func (v *validator) validateWithContext(ctx *requestContext) (bool, []*errors.ValidationError) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var allErrors []*errors.ValidationError

	validators := []validationFunction{
		v.validatePathParamsCtx,
		v.validateCookieParamsCtx,
		v.validateHeaderParamsCtx,
		v.validateQueryParamsCtx,
		v.validateSecurityCtx,
		v.validateRequestBodyCtx,
	}

	wg.Add(len(validators))
	for _, fn := range validators {
		go func(validate validationFunction) {
			defer wg.Done()
			if valid, errs := validate(ctx); !valid {
				mu.Lock()
				allErrors = append(allErrors, errs...)
				mu.Unlock()
			}
		}(fn)
	}
	wg.Wait()
	sortValidationErrors(allErrors)
	return len(allErrors) == 0, allErrors
}

func (v *validator) ValidateHttpRequestSync(request *http.Request) (bool, []*errors.ValidationError) {
	ctx, errs := v.buildRequestContext(request)
	if errs != nil {
		return false, errs
	}
	return v.validateRequestSync(ctx)
}

func (v *validator) ValidateHttpRequestSyncWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
	ctx := &requestContext{
		request: request,
		route: &resolvedRoute{
			pathItem:    pathItem,
			matchedPath: pathValue,
		},
		operation: helpers.OperationForMethod(request.Method, pathItem),
		version:   v.version,
	}
	return v.validateRequestSync(ctx)
}

type validator struct {
	options           *config.ValidationOptions
	v3Model           *v3.Document
	document          libopenapi.Document
	paramValidator    parameters.ParameterValidator
	requestValidator  requests.RequestBodyValidator
	responseValidator responses.ResponseBodyValidator
	matchers          matcherChain
	version           float32 // cached OAS version (3.0 or 3.1)
}

type validationFunction func(ctx *requestContext) (bool, []*errors.ValidationError)

// sortValidationErrors sorts validation errors for deterministic ordering.
// Errors are sorted by validation type first, then by message.
func sortValidationErrors(errs []*errors.ValidationError) {
	sort.Slice(errs, func(i, j int) bool {
		if errs[i].ValidationType != errs[j].ValidationType {
			return errs[i].ValidationType < errs[j].ValidationType
		}
		return errs[i].Message < errs[j].Message
	})
}

// warmSchemaCaches pre-compiles all schemas in the OpenAPI document and stores them in the validator caches.
// This frontloads the compilation cost so that runtime validation doesn't need to compile schemas.
func warmSchemaCaches(
	doc *v3.Document,
	options *config.ValidationOptions,
) {
	// Skip warming if cache is nil (explicitly disabled via WithSchemaCache(nil))
	if doc == nil || doc.Paths == nil || doc.Paths.PathItems == nil || options.SchemaCache == nil {
		return
	}

	schemaCache := options.SchemaCache

	// Walk through all paths and operations
	for pathPair := doc.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
		pathItem := pathPair.Value()

		// Get all operations for this path (handles all HTTP methods including OpenAPI 3.2+ extensions)
		operations := pathItem.GetOperations()
		if operations == nil {
			continue
		}

		for opPair := operations.First(); opPair != nil; opPair = opPair.Next() {
			operation := opPair.Value()
			if operation == nil {
				continue
			}

			// Warm request body schemas
			if operation.RequestBody != nil && operation.RequestBody.Content != nil {
				for contentPair := operation.RequestBody.Content.First(); contentPair != nil; contentPair = contentPair.Next() {
					mediaType := contentPair.Value()
					if mediaType.Schema != nil {
						warmMediaTypeSchema(mediaType, schemaCache, options)
					}
				}
			}

			// Warm response body schemas
			if operation.Responses != nil {
				// Warm status code responses
				if operation.Responses.Codes != nil {
					for codePair := operation.Responses.Codes.First(); codePair != nil; codePair = codePair.Next() {
						response := codePair.Value()
						if response != nil && response.Content != nil {
							for contentPair := response.Content.First(); contentPair != nil; contentPair = contentPair.Next() {
								mediaType := contentPair.Value()
								if mediaType.Schema != nil {
									warmMediaTypeSchema(mediaType, schemaCache, options)
								}
							}
						}
					}
				}

				// Warm default response schemas
				if operation.Responses.Default != nil && operation.Responses.Default.Content != nil {
					for contentPair := operation.Responses.Default.Content.First(); contentPair != nil; contentPair = contentPair.Next() {
						mediaType := contentPair.Value()
						if mediaType.Schema != nil {
							warmMediaTypeSchema(mediaType, schemaCache, options)
						}
					}
				}
			}

			// Warm parameter schemas
			if operation.Parameters != nil {
				for _, param := range operation.Parameters {
					if param != nil {
						warmParameterSchema(param, schemaCache, options)
					}
				}
			}
		}

		// Warm path-level parameters
		if pathItem.Parameters != nil {
			for _, param := range pathItem.Parameters {
				if param != nil {
					warmParameterSchema(param, schemaCache, options)
				}
			}
		}
	}
}

// warmMediaTypeSchema warms the cache for a media type schema
func warmMediaTypeSchema(mediaType *v3.MediaType, schemaCache cache.SchemaCache, options *config.ValidationOptions) {
	if mediaType != nil && mediaType.Schema != nil {
		hash := mediaType.GoLow().Schema.Value.Hash()

		if _, exists := schemaCache.Load(hash); !exists {
			schema := mediaType.Schema.Schema()
			if schema != nil {
				renderCtx := base.NewInlineRenderContext()
				renderedInline, _ := schema.RenderInlineWithContext(renderCtx)
				referenceSchema := string(renderedInline)
				renderedJSON, _ := utils.ConvertYAMLtoJSON(renderedInline)
				if len(renderedInline) > 0 {
					compiledSchema, _ := helpers.NewCompiledSchema(fmt.Sprintf("%x", hash), renderedJSON, options)

					// Pre-parse YAML node for error reporting (avoids re-parsing on each error)
					var renderedNode yaml.Node
					_ = yaml.Unmarshal(renderedInline, &renderedNode)

					schemaCache.Store(hash, &cache.SchemaCacheEntry{
						Schema:          schema,
						RenderedInline:  renderedInline,
						ReferenceSchema: referenceSchema,
						RenderedJSON:    renderedJSON,
						CompiledSchema:  compiledSchema,
						RenderedNode:    &renderedNode,
					})
				}
			}
		}
	}
}

// warmParameterSchema warms the cache for a parameter schema
func warmParameterSchema(param *v3.Parameter, schemaCache cache.SchemaCache, options *config.ValidationOptions) {
	if param != nil {
		var schema *base.Schema
		var hash uint64

		// Parameters can have schemas in two places: schema property or content property
		if param.Schema != nil {
			schema = param.Schema.Schema()
			if schema != nil {
				hash = param.GoLow().Schema.Value.Hash()
			}
		} else if param.Content != nil {
			// Check content for schema
			for contentPair := param.Content.First(); contentPair != nil; contentPair = contentPair.Next() {
				mediaType := contentPair.Value()
				if mediaType.Schema != nil {
					schema = mediaType.Schema.Schema()
					if schema != nil {
						hash = mediaType.GoLow().Schema.Value.Hash()
					}
					break // Only process first content type
				}
			}
		}

		if schema != nil {
			if _, exists := schemaCache.Load(hash); !exists {
				renderCtx := base.NewInlineRenderContext()
				renderedInline, _ := schema.RenderInlineWithContext(renderCtx)
				referenceSchema := string(renderedInline)
				renderedJSON, _ := utils.ConvertYAMLtoJSON(renderedInline)
				if len(renderedInline) > 0 {
					compiledSchema, _ := helpers.NewCompiledSchema(fmt.Sprintf("%x", hash), renderedJSON, options)

					// Pre-parse YAML node for error reporting (avoids re-parsing on each error)
					var renderedNode yaml.Node
					_ = yaml.Unmarshal(renderedInline, &renderedNode)

					// Store in cache using the shared SchemaCache type
					schemaCache.Store(hash, &cache.SchemaCacheEntry{
						Schema:          schema,
						RenderedInline:  renderedInline,
						ReferenceSchema: referenceSchema,
						RenderedJSON:    renderedJSON,
						CompiledSchema:  compiledSchema,
						RenderedNode:    &renderedNode,
					})
				}
			}
		}
	}
}

// warmRegexCache pre-compiles all path parameter regexes in the OpenAPI document and stores them in the regex cache.
// This frontloads the compilation cost so that runtime validation doesn't need to compile regexes for path segments.
func warmRegexCache(doc *v3.Document, options *config.ValidationOptions) {
	if doc == nil || doc.Paths == nil || doc.Paths.PathItems == nil || options.RegexCache == nil {
		return
	}

	for pathPair := doc.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
		pathKey := pathPair.Key()
		segments := strings.Split(pathKey, "/")
		for _, segment := range segments {
			if segment == "" {
				continue
			}
			// Only compile segments that contain path parameters (have braces)
			if !strings.Contains(segment, "{") {
				continue
			}
			if _, found := options.RegexCache.Load(segment); !found {
				r, err := helpers.GetRegexForPath(segment)
				if err == nil {
					options.RegexCache.Store(segment, r)
				}
			}
		}
	}
}
