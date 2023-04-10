// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schemas

import (
    "encoding/json"
    "fmt"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/utils"
    "github.com/santhosh-tekuri/jsonschema/v5"
    "net/url"
    "reflect"
    "strings"
)

func ValidateParameterSchema(
    schema *base.Schema,
    rawObject any,
    rawBlob,
    entity,
    reasonEntity,
    name,
    validationType,
    subValType string) []*errors.ValidationError {

    var validationErrors []*errors.ValidationError

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

        // flatten the validationErrors
        schFlatErrs := jk.BasicOutput().Errors
        var schemaValidationErrors []*errors.SchemaValidationFailure
        for q := range schFlatErrs {
            er := schFlatErrs[q]
            if er.KeywordLocation == "" || strings.HasPrefix(er.Error, "doesn't validate with") {
                continue // ignore this error,
            }
            schemaValidationErrors = append(schemaValidationErrors, &errors.SchemaValidationFailure{
                Reason:        er.Error,
                Location:      er.KeywordLocation,
                OriginalError: jk,
            })
        }
        // add the error to the list
        validationErrors = append(validationErrors, &errors.ValidationError{
            ValidationType:    validationType,
            ValidationSubType: subValType,
            Message:           fmt.Sprintf("%s '%s' failed to validate", entity, name),
            Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
                "however it failed to pass a schema validation", reasonEntity, name),
            SpecLine:               schema.GoLow().Type.KeyNode.Line,
            SpecCol:                schema.GoLow().Type.KeyNode.Column,
            SchemaValidationErrors: schemaValidationErrors,
            HowToFix:               errors.HowToFixParamInvalidSchema,
        })
    }

    // if there are no validationErrors, check that the supplied value is even JSON
    if len(validationErrors) == 0 {
        if rawIsMap {
            decodedMap := decodedObj.(map[string]interface{})
            if decodedMap == nil || len(decodedMap) == 0 {
                // add the error to the list
                validationErrors = append(validationErrors, &errors.ValidationError{
                    ValidationType:    validationType,
                    ValidationSubType: subValType,
                    Message:           fmt.Sprintf("%s '%s' cannot be decoded", entity, name),
                    Reason: fmt.Sprintf("%s '%s' is defined as an object, "+
                        "however it failed to be decoded as an object", reasonEntity, name),
                    SpecLine: schema.GoLow().Type.KeyNode.Line,
                    SpecCol:  schema.GoLow().Type.KeyNode.Column,
                    HowToFix: errors.HowToFixDecodingError,
                })
            }
        }
    }
    return validationErrors
}
