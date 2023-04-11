// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schemas

import (
    "encoding/json"
    "fmt"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/utils"
    "github.com/santhosh-tekuri/jsonschema/v5"
    "io"
    "net/http"
    "strings"
)

func ValidateRequestSchema(request *http.Request, schema *base.Schema) (bool, []*errors.ValidationError) {

    var validationErrors []*errors.ValidationError

    // render the schema, to be used for validation
    renderedSchema, _ := schema.RenderInline()
    jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)
    requestBody, _ := io.ReadAll(request.Body)

    var decodedObj interface{}
    _ = json.Unmarshal(requestBody, &decodedObj)

    compiler := jsonschema.NewCompiler()
    _ = compiler.AddResource("requestBody.json", strings.NewReader(string(jsonSchema)))
    jsch, _ := compiler.Compile("requestBody.json")

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
                continue // ignore this error, it's useless tbh, utter noise.
            }
            if er.Error != "" {
                schemaValidationErrors = append(schemaValidationErrors, &errors.SchemaValidationFailure{
                    Reason:        er.Error,
                    Location:      er.KeywordLocation,
                    OriginalError: jk,
                })
            }
        }
        // add the error to the list
        validationErrors = append(validationErrors, &errors.ValidationError{
            ValidationType:    helpers.RequestBodyValidation,
            ValidationSubType: helpers.RequestBodyValidationSchema,
            Message: fmt.Sprintf("%s request body for '%s' failed to validate schema",
                request.Method, request.URL.Path),
            Reason: "The request body is defined as an object, " +
                "however it does not meet the schema requirements of the specification.",
            SpecLine:               schema.GoLow().Type.KeyNode.Line,
            SpecCol:                schema.GoLow().Type.KeyNode.Column,
            SchemaValidationErrors: schemaValidationErrors,
            HowToFix:               errors.HowToFixInvalidSchema,
        })
    }
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
