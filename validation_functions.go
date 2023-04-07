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
    "reflect"
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

func (s *SchemaValidationFailure) Error() string {
    return fmt.Sprintf("Reason: %s, Location: %s", s.Reason, s.Location)
}

type ValidationError struct {
    Message                string                     `json:"message" yaml:"message"`
    ValidationType         string                     `json:"validationType" yaml:"validationType"`
    ValidationSubType      string                     `json:"validationSubType" yaml:"validationSubType"`
    Reason                 string                     `json:"reason" yaml:"reason"`
    SpecLine               int                        `json:"specLine" yaml:"specLine"`
    SpecCol                int                        `json:"specColumn" yaml:"specColumn"`
    HowToFix               string                     `json:"howToFix" yaml:"howToFix"`
    SchemaValidationErrors []*SchemaValidationFailure `json:"validationErrors,omitempty" yaml:"validationErrors,omitempty"`
    Context                interface{}                `json:"-" yaml:"-"`
}

func (v *ValidationError) Error() string {
    if v.SchemaValidationErrors != nil {
        return fmt.Sprintf("Error: %s, Reason: %s, Validation Errors: %s, Line: %d, Column: %d",
            v.Message, v.Reason, v.SchemaValidationErrors, v.SpecLine, v.SpecCol)
    } else {
        return fmt.Sprintf("Error: %s, Reason: %s, Line: %d, Column: %d",
            v.Message, v.Reason, v.SpecLine, v.SpecCol)
    }
}

type Validator interface {
    ValidateHttpRequest(request *http.Request) (bool, []*ValidationError)
    ValidateQueryParams(request *http.Request) (bool, []*ValidationError)
    ValidateHeaderParams(request *http.Request) (bool, []*ValidationError)
    ValidateCookieParams(request *http.Request) (bool, []*ValidationError)
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
    rawObject any,
    rawBlob string,
    entity, reasonEntity, name, validationType, subValType string) []*ValidationError {

    var errors []*ValidationError

    // 1. build a JSON render of the schema.
    renderedSchema, _ := schema.Render()
    jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)

    // 2. decode the object into a json blob.
    var decodedObj interface{}
    rawIsMap := false
    if rawObject != nil {
        // check what type of object it is
        ot := reflect.TypeOf(rawObject)
        switch ot.Kind() {
        case reflect.Map:
            decodedObj = rawObject.(map[string]interface{})
            rawIsMap = true
        }
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
        var schemaValidationErrors []*SchemaValidationFailure
        for q := range schFlatErrs {
            er := schFlatErrs[q]
            if er.KeywordLocation == "" || strings.HasPrefix(er.Error, "doesn't validate with") {
                continue // ignore this error,
            }
            schemaValidationErrors = append(schemaValidationErrors, &SchemaValidationFailure{
                Reason:        er.Error,
                Location:      er.KeywordLocation,
                OriginalError: jk,
            })
        }
        // add the error to the list
        errors = append(errors, &ValidationError{
            ValidationType:    validationType,
            ValidationSubType: subValType,
            Message:           fmt.Sprintf("%s '%s' failed to validate", entity, name),
            Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
                "however it failed to pass a schema validation", reasonEntity, name),
            SpecLine:               schema.GoLow().Type.KeyNode.Line,
            SpecCol:                schema.GoLow().Type.KeyNode.Column,
            SchemaValidationErrors: schemaValidationErrors,
            HowToFix:               HowToFixParamInvalidSchema,
        })
    }

    // if there are no errors, check that the supplied value is even JSON
    if len(errors) == 0 {
        if rawIsMap {
            decodedMap := decodedObj.(map[string]interface{})
            if decodedMap == nil || len(decodedMap) == 0 {
                // add the error to the list
                errors = append(errors, &ValidationError{
                    ValidationType:    validationType,
                    ValidationSubType: subValType,
                    Message:           fmt.Sprintf("%s '%s' cannot be decoded", entity, name),
                    Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
                        "however it failed to be decoded as an object", reasonEntity, name),
                    SpecLine: schema.GoLow().Type.KeyNode.Line,
                    SpecCol:  schema.GoLow().Type.KeyNode.Column,
                    HowToFix: HowToFixDecodingError,
                })
            }
        }
    }
    return errors
}
