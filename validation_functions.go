// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package validator

import (
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/parameters"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi-validator/requests"
	"github.com/pb33f/libopenapi-validator/responses"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"net/http"
	"sync"
)

type Validator interface {
	ValidateHttpRequest(request *http.Request) (bool, []*errors.ValidationError)
	ValidateHttpResponse(request *http.Request, response *http.Response) (bool, []*errors.ValidationError)
	ValidateDocument() (bool, []*errors.ValidationError)
	GetParameterValidator() parameters.ParameterValidator
	GetRequestBodyValidator() requests.RequestBodyValidator
	GetResponseBodyValidator() responses.ResponseBodyValidator
}

// NewValidator will create a new Validator from an OpenAPI 3+ document
func NewValidator(document libopenapi.Document) (Validator, []error) {
	m, errs := document.BuildV3Model()
	if errs != nil {
		return nil, errs
	}

	// create a new parameter validator
	paramValidator := parameters.NewParameterValidator(&m.Model)

	// create a new request body validator
	reqBodyValidator := requests.NewRequestBodyValidator(&m.Model)

	// create a response body validator
	respBodyValidator := responses.NewResponseBodyValidator(&m.Model)

	return &validator{
		v3Model:           &m.Model,
		document:          document,
		requestValidator:  reqBodyValidator,
		responseValidator: respBodyValidator,
		paramValidator:    paramValidator,
	}, nil
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

func (v *validator) ValidateHttpResponse(request *http.Request, response *http.Response) (bool, []*errors.ValidationError) {
	// find path
	pathItem, errs, pathValue := paths.FindPath(request, v.v3Model)
	if pathItem == nil || errs != nil {
		v.errors = errs
		return false, errs
	}

	// create a new parameter validator
	responseBodyValidator := v.responseValidator
	responseBodyValidator.SetPathItem(pathItem, pathValue)
	return responseBodyValidator.ValidateResponseBody(request, response)
}

func (v *validator) ValidateHttpRequest(request *http.Request) (bool, []*errors.ValidationError) {

	// find path
	pathItem, errs, pathValue := paths.FindPath(request, v.v3Model)
	if pathItem == nil || errs != nil {
		v.errors = errs
		return false, errs
	}

	// create a new parameter validator
	paramValidator := v.paramValidator
	paramValidator.SetPathItem(pathItem, pathValue)

	// create a new request body validator
	reqBodyValidator := v.requestValidator
	reqBodyValidator.SetPathItem(pathItem, pathValue)

	// create some channels to handle async validation
	doneChan := make(chan bool)
	errChan := make(chan []*errors.ValidationError)
	controlChan := make(chan bool)

	parameterValidationFunc := func(control chan bool, errorChan chan []*errors.ValidationError) {
		paramErrs := make(chan []*errors.ValidationError)
		paramControlChan := make(chan bool)
		paramFunctionControlChan := make(chan bool)
		var paramValidationErrors []*errors.ValidationError

		validations := []validationFunction{
			paramValidator.ValidatePathParams,
			paramValidator.ValidateCookieParams,
			paramValidator.ValidateHeaderParams,
			paramValidator.ValidateQueryParams,
		}

		paramListener := func(control chan bool, errorChan chan []*errors.ValidationError) {
			completedValidations := 0
			for {
				select {
				case vErrs := <-errorChan:
					paramValidationErrors = append(paramValidationErrors, vErrs...)
				case <-control:
					completedValidations++
					if completedValidations == len(validations) {
						paramFunctionControlChan <- true
						return
					}
				}
			}
		}

		validateParamFunction := func(
			control chan bool,
			errorChan chan []*errors.ValidationError,
			validatorFunc validationFunction) {
			valid, pErrs := validatorFunc(request)
			if !valid {
				errorChan <- pErrs
			}
			control <- true
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
		controlChan <- true
	}

	requestBodyValidationFunc := func(control chan bool, errorChan chan []*errors.ValidationError) {
		valid, pErrs := reqBodyValidator.ValidateRequestBody(request)
		if !valid {
			errorChan <- pErrs
		}
		control <- true
	}

	// build async functions
	asyncFunctions := []validationFunctionAsync{
		parameterValidationFunc,
		requestBodyValidationFunc,
	}

	var validationErrors []*errors.ValidationError
	var validationLock sync.Mutex

	runValidation := func(control chan bool, errorChan chan []*errors.ValidationError) {
		completedValidations := 0
		for {
			select {
			case vErrs := <-errorChan:
				validationLock.Lock()
				validationErrors = append(validationErrors, vErrs...)
				validationLock.Unlock()
			case <-control:
				completedValidations++
				if completedValidations == len(asyncFunctions) {
					doneChan <- true
					return
				}
			}
		}
	}

	// sit and wait for everything to report back.
	go runValidation(controlChan, errChan)

	// run async functions
	for i := range asyncFunctions {
		go asyncFunctions[i](controlChan, errChan)
	}

	// wait for all the validations to complete
	<-doneChan

	if len(validationErrors) > 0 {
		return false, validationErrors
	}
	return true, nil
}

type validator struct {
	v3Model           *v3.Document
	document          libopenapi.Document
	paramValidator    parameters.ParameterValidator
	requestValidator  requests.RequestBodyValidator
	responseValidator responses.ResponseBodyValidator
	errors            []*errors.ValidationError
}

type validationFunction func(request *http.Request) (bool, []*errors.ValidationError)
type validationFunctionAsync func(control chan bool, errorChan chan []*errors.ValidationError)
