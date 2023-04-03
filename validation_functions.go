// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/utils"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"net/http"
	"net/url"
	"strings"
)

const (
	ParameterValidation       = "parameter"
	ParameterValidationPath   = "path"
	ParameterValidationQuery  = "query"
	ParameterValidationHeader = "header"
	ParameterValidationCookie = "cookie"
)

type SchemaValidationFailure struct {
	Reason        string                      `json:"reason,omitempty" yaml:"reason,omitempty"`
	Location      string                      `json:"location,omitempty" yaml:"location,omitempty"`
	OriginalError *jsonschema.ValidationError `json:"-" yaml:"-"`
}

type ValidationError struct {
	Message           string                   `json:"message" yaml:"message"`
	ValidationType    string                   `json:"validationType" yaml:"validationType"`
	ValidationSubType string                   `json:"validationSubType" yaml:"validationSubType"`
	Reason            string                   `json:"reason" yaml:"reason"`
	SpecLine          int                      `json:"specLine" yaml:"specLine"`
	SpecCol           int                      `json:"specColumn" yaml:"specColumn"`
	HowToFix          string                   `json:"howToFix" yaml:"howToFix"`
	ValidationError   *SchemaValidationFailure `json:"validationError,omitempty" yaml:"validationError,omitempty"`
	Context           interface{}              `json:"-" yaml:"-"`
}

func (v *ValidationError) Error() string {
	if v.ValidationError != nil {
		return fmt.Sprintf("Error: %s, Reason: %s, Validation Error: %s, Line: %d, Column: %d",
			v.Message, v.Reason, v.ValidationError.Reason, v.SpecLine, v.SpecCol)
	} else {
		return fmt.Sprintf("Error: %s, Reason: %s, Line: %d, Column: %d",
			v.Message, v.Reason, v.SpecLine, v.SpecCol)
	}
}

type Validator interface {
	ValidateHttpRequest(request *http.Request) (bool, []*ValidationError)
	ValidateQueryParams(request *http.Request) (bool, []*ValidationError)
	FindPath(request *http.Request) (*v3.PathItem, []*ValidationError)
	AllValidationErrors() []*ValidationError
}

type validator struct {
	document *v3.Document
	errors   []*ValidationError
}

func NewValidator(document *v3.Document) Validator {
	return &validator{document: document}
}

func (v *validator) ValidateHttpRequest(request *http.Request) (bool, []*ValidationError) {

	// find path
	pathItem, errs := v.FindPath(request)
	if pathItem == nil || errs != nil {
		return false, errs
	}

	// validate query params
	//if !v.validateQueryParams(request) {
	//    return false, v.errors
	//}
	return false, nil
}

func (v *validator) AllValidationErrors() []*ValidationError {
	return v.errors
}

func (v *validator) validateSchema(
	schema *base.Schema,
	rawObject interface{},
	rawBlob string,
	entity, reasonEntity, name, validationType, subValType string) []*ValidationError {

	var errors []*ValidationError

	// 1. build a JSON render of the schema.
	renderedSchema, _ := schema.Render()
	jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)

	// 2. decode the object into a json blob.
	var decodedObj any
	if rawObject != nil {
		decodedObj = rawObject
	} else {
		decodedString, _ := url.QueryUnescape(rawBlob)
		_ = json.Unmarshal([]byte(decodedString), &decodedObj)
	}
	// 3. create a new json schema compiler and add the schema to it
	compiler := jsonschema.NewCompiler()
	_ = compiler.AddResource(fmt.Sprintf("%s.json", name), strings.NewReader(string(jsonSchema)))
	jsch, _ := compiler.Compile(fmt.Sprintf("%s.json", name))

	// 4. validate the object against the schema
	scErrs := jsch.Validate(decodedObj)
	if scErrs != nil {
		jk := scErrs.(*jsonschema.ValidationError)

		// flatten the errors
		schFlatErrs := jk.BasicOutput().Errors
		for q := range schFlatErrs {
			er := schFlatErrs[q]
			if er.KeywordLocation == "" || strings.HasPrefix(er.Error, "doesn't validate with") {
				continue // ignore this error,
			}
			// add the error to the list
			errors = append(errors, &ValidationError{
				ValidationType:    validationType,
				ValidationSubType: subValType,
				Message:           fmt.Sprintf("%s '%s' failed to validate", entity, name),
				Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
					"however it failed to pass a schema validation", reasonEntity, name),
				SpecLine: schema.GoLow().Type.KeyNode.Line,
				SpecCol:  schema.GoLow().Type.KeyNode.Column,
				ValidationError: &SchemaValidationFailure{
					Reason:        er.Error,
					Location:      er.KeywordLocation,
					OriginalError: jk,
				},
				HowToFix: HowToFixParamInvalidSchema,
			})
		}
	}
	return errors
}
