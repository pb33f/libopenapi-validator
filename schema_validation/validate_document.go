// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
    "fmt"
    "github.com/pb33f/libopenapi"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/schema_validation/openapi_schemas"
    "github.com/pb33f/libopenapi/utils"
    "github.com/santhosh-tekuri/jsonschema/v5"
    "strings"
)

func ValidateOpenAPIDocument(doc libopenapi.Document) (bool, []*errors.ValidationError) {

    // first determine if this is a swagger or an openapi document
    info := doc.GetSpecInfo()
    if info.SpecType == utils.OpenApi2 {
        return false, []*errors.ValidationError{{Message: "Swagger / OpenAPI 2.0 is not supported by the validator"}}
    }
    var loadedSchema string

    // check version of openapi and load schema
    switch info.Version {
    case "3.1.0", "3.1":
        loadedSchema = openapi_schemas.LoadSchema3_1(info.APISchema)
    default:
        loadedSchema = openapi_schemas.LoadSchema3_0(info.APISchema)
    }

    var validationErrors []*errors.ValidationError

    decodedDocument := *info.SpecJSON

    compiler := jsonschema.NewCompiler()
    _ = compiler.AddResource("schema.json", strings.NewReader(string(loadedSchema)))
    jsch, _ := compiler.Compile("schema.json")

    scErrs := jsch.Validate(decodedDocument)

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

                // locate the violated property in the schema
                located := LocateSchemaPropertyNodeByJSONPath(info.RootNode.Content[0], er.KeywordLocation)
                if located == nil {
                    // try again with the instance location
                    located = LocateSchemaPropertyNodeByJSONPath(info.RootNode.Content[0], er.InstanceLocation)
                }
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
            ValidationType: helpers.Schema,
            Message:        "Document does not pass validation",
            Reason: fmt.Sprintf("OpenAPI document is not valid according "+
                "to the %s specification", info.Version),
            SchemaValidationErrors: schemaValidationErrors,
            HowToFix:               errors.HowToFixInvalidSchema,
        })
    }
    if len(validationErrors) > 0 {
        return false, validationErrors
    }
    return true, nil
}
