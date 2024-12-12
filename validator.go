// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"net/http"
	"sync"

	"github.com/pb33f/libopenapi"
	"github.com/santhosh-tekuri/jsonschema/v6"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/parameters"
	"github.com/pb33f/libopenapi-validator/paths"
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
	// ValidateHttpRequestSync will validate an *http.Request object against an OpenAPI 3+ document syncronously and without spawning any goroutines.
	// The path, query, cookie and header parameters and request body are validated.
	ValidateHttpRequestSync(request *http.Request) (bool, []*errors.ValidationError)

	// ValidateHttpRequestWithPathItem will validate an *http.Request object against an OpenAPI 3+ document.
	// The path, query, cookie and header parameters and request body are validated.
	ValidateHttpRequestWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError)

	// ValidateHttpRequestSyncWithPathItem will validate an *http.Request object against an OpenAPI 3+ document syncronously and without spawning any goroutines.
	// The path, query, cookie and header parameters and request body are validated.
	ValidateHttpRequestSyncWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError)

	// ValidateHttpResponse will an *http.Response object against an OpenAPI 3+ document.
	// The response body is validated. The request is only used to extract the correct reponse from the spec.
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
}

// Option supports the 'Options Pattern' to define the behavior of a Validator
type Option func(*validator)

// WithRegexEngine allows for a custom regular expression engine to be used during validation.
func WithRegexEngine(engine jsonschema.RegexpEngine) Option {
	return func(v *validator) {
		v.regexEngine = engine
	}
}

// NewValidator will create a new Validator from an OpenAPI 3+ document
func NewValidator(document libopenapi.Document, opts ...Option) (Validator, []error) {
	m, errs := document.BuildV3Model()
	if errs != nil {
		return nil, errs
	}
	v := NewValidatorFromV3Model(&m.Model, opts...)
	v.(*validator).document = document
	return v, nil
}

// NewValidatorFromV3Model will create a new Validator from an OpenAPI Model
func NewValidatorFromV3Model(m *v3.Document, opts ...Option) Validator {

	v := &validator{v3Model: m}

	for _, opt := range opts {
		opt(v)
	}

	// create a new parameter validator
	v.paramValidator = parameters.NewParameterValidator(m, parameters.WithRegexEngine(v.regexEngine))

	// create aq new request body validator
	v.requestValidator = requests.NewRequestBodyValidator(m, requests.WithRegexEngine(v.regexEngine))

	// create a response body validator
	v.responseValidator = responses.NewResponseBodyValidator(m, responses.WithRegexEngine(v.regexEngine))

	return v
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
	return schema_validation.ValidateOpenAPIDocument(v.document)
}

func (v *validator) ValidateHttpResponse(
	request *http.Request,
	response *http.Response,
) (bool, []*errors.ValidationError) {
	var pathItem *v3.PathItem
	var pathValue string
	var errs []*errors.ValidationError

	pathItem, errs, pathValue = paths.FindPath(request, v.v3Model)
	if pathItem == nil || errs != nil {
		return false, errs
	}

	responseBodyValidator := v.responseValidator

	// validate response
	_, responseErrors := responseBodyValidator.ValidateResponseBodyWithPathItem(request, response, pathItem, pathValue)

	if len(responseErrors) > 0 {
		return false, responseErrors
	}
	return true, nil
}

func (v *validator) ValidateHttpRequestResponse(
	request *http.Request,
	response *http.Response,
) (bool, []*errors.ValidationError) {
	var pathItem *v3.PathItem
	var pathValue string
	var errs []*errors.ValidationError

	pathItem, errs, pathValue = paths.FindPath(request, v.v3Model)
	if pathItem == nil || errs != nil {
		return false, errs
	}

	responseBodyValidator := v.responseValidator

	// validate request and response
	_, requestErrors := v.ValidateHttpRequestWithPathItem(request, pathItem, pathValue)
	_, responseErrors := responseBodyValidator.ValidateResponseBodyWithPathItem(request, response, pathItem, pathValue)

	if len(requestErrors) > 0 || len(responseErrors) > 0 {
		return false, append(requestErrors, responseErrors...)
	}
	return true, nil
}

func (v *validator) ValidateHttpRequest(request *http.Request) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.v3Model)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateHttpRequestWithPathItem(request, pathItem, foundPath)
}

func (v *validator) ValidateHttpRequestWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
	// create a new parameter validator
	paramValidator := v.paramValidator

	// create a new request body validator
	reqBodyValidator := v.requestValidator

	// create some channels to handle async validation
	doneChan := make(chan struct{})
	errChan := make(chan []*errors.ValidationError)
	controlChan := make(chan struct{})

	// async param validation function.
	parameterValidationFunc := func(control chan struct{}, errorChan chan []*errors.ValidationError) {
		paramErrs := make(chan []*errors.ValidationError)
		paramControlChan := make(chan struct{})
		paramFunctionControlChan := make(chan struct{})
		var paramValidationErrors []*errors.ValidationError

		validations := []validationFunction{
			paramValidator.ValidatePathParamsWithPathItem,
			paramValidator.ValidateCookieParamsWithPathItem,
			paramValidator.ValidateHeaderParamsWithPathItem,
			paramValidator.ValidateQueryParamsWithPathItem,
			paramValidator.ValidateSecurityWithPathItem,
		}

		// listen for validation errors on parameters. everything will run async.
		paramListener := func(control chan struct{}, errorChan chan []*errors.ValidationError) {
			completedValidations := 0
			for {
				select {
				case vErrs := <-errorChan:
					paramValidationErrors = append(paramValidationErrors, vErrs...)
				case <-control:
					completedValidations++
					if completedValidations == len(validations) {
						paramFunctionControlChan <- struct{}{}
						return
					}
				}
			}
		}

		validateParamFunction := func(
			control chan struct{},
			errorChan chan []*errors.ValidationError,
			validatorFunc validationFunction,
		) {
			valid, pErrs := validatorFunc(request, pathItem, pathValue)
			if !valid {
				errorChan <- pErrs
			}
			control <- struct{}{}
		}
		go paramListener(paramControlChan, paramErrs)
		for i := range validations {
			go validateParamFunction(paramControlChan, paramErrs, validations[i])
		}

		// wait for all the validations to complete
		<-paramFunctionControlChan
		if len(paramValidationErrors) > 0 {
			errorChan <- paramValidationErrors
		}

		// let runValidation know we are done with this part.
		controlChan <- struct{}{}
	}

	requestBodyValidationFunc := func(control chan struct{}, errorChan chan []*errors.ValidationError) {
		valid, pErrs := reqBodyValidator.ValidateRequestBodyWithPathItem(request, pathItem, pathValue)
		if !valid {
			errorChan <- pErrs
		}
		control <- struct{}{}
	}

	// build async functions
	asyncFunctions := []validationFunctionAsync{
		parameterValidationFunc,
		requestBodyValidationFunc,
	}

	var validationErrors []*errors.ValidationError

	// sit and wait for everything to report back.
	go runValidation(controlChan, doneChan, errChan, &validationErrors, len(asyncFunctions))

	// run async functions
	for i := range asyncFunctions {
		go asyncFunctions[i](controlChan, errChan)
	}

	// wait for all the validations to complete
	<-doneChan
	return !(len(validationErrors) > 0), validationErrors
}

func (v *validator) ValidateHttpRequestSync(request *http.Request) (bool, []*errors.ValidationError) {
	pathItem, errs, foundPath := paths.FindPath(request, v.v3Model)
	if len(errs) > 0 {
		return false, errs
	}
	return v.ValidateHttpRequestSyncWithPathItem(request, pathItem, foundPath)
}

func (v *validator) ValidateHttpRequestSyncWithPathItem(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError) {
	// create a new parameter validator
	paramValidator := v.paramValidator

	// create a new request body validator
	reqBodyValidator := v.requestValidator

	validationErrors := make([]*errors.ValidationError, 0)

	paramValidationErrors := make([]*errors.ValidationError, 0)
	for _, validateFunc := range []validationFunction{
		paramValidator.ValidatePathParamsWithPathItem,
		paramValidator.ValidateCookieParamsWithPathItem,
		paramValidator.ValidateHeaderParamsWithPathItem,
		paramValidator.ValidateQueryParamsWithPathItem,
		paramValidator.ValidateSecurityWithPathItem,
	} {
		valid, pErrs := validateFunc(request, pathItem, pathValue)
		if !valid {
			paramValidationErrors = append(paramValidationErrors, pErrs...)
		}
	}

	valid, pErrs := reqBodyValidator.ValidateRequestBodyWithPathItem(request, pathItem, pathValue)
	if !valid {
		paramValidationErrors = append(paramValidationErrors, pErrs...)
	}

	validationErrors = append(validationErrors, paramValidationErrors...)
	return !(len(validationErrors) > 0), validationErrors
}

type validator struct {
	v3Model           *v3.Document
	document          libopenapi.Document
	paramValidator    parameters.ParameterValidator
	requestValidator  requests.RequestBodyValidator
	responseValidator responses.ResponseBodyValidator
	regexEngine       jsonschema.RegexpEngine
}

func runValidation(control, doneChan chan struct{},
	errorChan chan []*errors.ValidationError,
	validationErrors *[]*errors.ValidationError,
	total int,
) {
	var validationLock sync.Mutex
	completedValidations := 0
	for {
		select {
		case vErrs := <-errorChan:
			validationLock.Lock()
			*validationErrors = append(*validationErrors, vErrs...)
			validationLock.Unlock()
		case <-control:
			completedValidations++
			if completedValidations == total {
				doneChan <- struct{}{}
				return
			}
		}
	}
}

type (
	validationFunction      func(request *http.Request, pathItem *v3.PathItem, pathValue string) (bool, []*errors.ValidationError)
	validationFunctionAsync func(control chan struct{}, errorChan chan []*errors.ValidationError)
)
