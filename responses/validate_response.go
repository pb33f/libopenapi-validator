// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
    "encoding/json"
    "fmt"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/schema_validation"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/utils"
    "github.com/santhosh-tekuri/jsonschema/v5"
    "gopkg.in/yaml.v3"
    "io"
    "net/http"
    "strings"
)

// ValidateResponseSchema will validate the response body for a http.Response pointer. The request is used to
// locate the operation in the specification, the response is used to ensure the response code, media type and the
// schema of the response body are valid.
//
// This function is used by the ValidateResponseBody function, but can be used independently.
func ValidateResponseSchema(
    request *http.Request,
    response *http.Response,
    schema *base.Schema) (bool, []*errors.ValidationError) {

    var validationErrors []*errors.ValidationError

    // render the schema, to be used for validation
    renderedSchema, _ := schema.RenderInline()
    jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)
    responseBody, _ := io.ReadAll(response.Body)

    var decodedObj interface{}
    _ = json.Unmarshal(responseBody, &decodedObj)

    // create a new jsonschema compiler and add in the rendered JSON schema.
    compiler := jsonschema.NewCompiler()
    fName := fmt.Sprintf("%s.json", helpers.ResponseBodyValidation)
    _ = compiler.AddResource(fName,
        strings.NewReader(string(jsonSchema)))
    jsch, _ := compiler.Compile(fName)

    // validate the object against the schema
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

                // re-encode the schema.
                var renderedNode yaml.Node
                _ = yaml.Unmarshal(renderedSchema, &renderedNode)

                // locate the violated property in the schema
                located := schema_validation.LocateSchemaPropertyNodeByJSONPath(renderedNode.Content[0], er.KeywordLocation)
                violation := &errors.SchemaValidationFailure{
                    Reason:        er.Error,
                    Location:      er.KeywordLocation,
                    OriginalError: jk,
                }
                // if we have a location within the schema, add it to the error
                if located != nil {
                    // location of the violation within the rendered schema.
                    violation.Line = located.Line
                    violation.Column = located.Column
                }
                schemaValidationErrors = append(schemaValidationErrors, violation)
            }
        }

        // add the error to the list
        validationErrors = append(validationErrors, &errors.ValidationError{
            ValidationType:    helpers.ResponseBodyValidation,
            ValidationSubType: helpers.Schema,
            Message: fmt.Sprintf("%d response body for '%s' failed to validate schema",
                response.StatusCode, request.URL.Path),
            Reason: "The response body for status code '%d' is defined as an object. " +
                "However, it does not meet the schema requirements of the specification",
            SpecLine:               schema.GoLow().Type.KeyNode.Line,
            SpecCol:                schema.GoLow().Type.KeyNode.Column,
            SchemaValidationErrors: schemaValidationErrors,
            HowToFix:               errors.HowToFixInvalidSchema,
            Context:                string(renderedSchema), // attach the rendered schema to the error
        })
    }
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
