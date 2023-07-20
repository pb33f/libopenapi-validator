// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package responses

import (
    "bytes"
    "encoding/json"
    "fmt"
    "github.com/pb33f/libopenapi-validator/errors"
    "github.com/pb33f/libopenapi-validator/helpers"
    "github.com/pb33f/libopenapi-validator/schema_validation"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/santhosh-tekuri/jsonschema/v5"
    "gopkg.in/yaml.v3"
    "io"
    "net/http"
    "reflect"
    "regexp"
    "strconv"
    "strings"
)

var instanceLocationRegex = regexp.MustCompile(`^/(\d+)`)

// ValidateResponseSchema will validate the response body for a http.Response pointer. The request is used to
// locate the operation in the specification, the response is used to ensure the response code, media type and the
// schema of the response body are valid.
//
// This function is used by the ValidateResponseBody function, but can be used independently.
func ValidateResponseSchema(
    request *http.Request,
    response *http.Response,
    schema *base.Schema,
    renderedSchema,
    jsonSchema []byte) (bool, []*errors.ValidationError) {

    var validationErrors []*errors.ValidationError

    responseBody, _ := io.ReadAll(response.Body)

    // close the request body, so it can be re-read later by another player in the chain
    _ = response.Body.Close()
    response.Body = io.NopCloser(bytes.NewBuffer(responseBody))

    var decodedObj interface{}
    _ = json.Unmarshal(responseBody, &decodedObj)

    // no response body? failed to decode anything? nothing to do here.
    if responseBody == nil || decodedObj == nil {
        return true, nil
    }

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

                // extract the element specified by the instance
                val := instanceLocationRegex.FindStringSubmatch(er.InstanceLocation)
                var referenceObject string

                if len(val) > 0 {
                    referenceIndex, _ := strconv.Atoi(val[1])
                    if reflect.ValueOf(decodedObj).Type().Kind() == reflect.Slice {
                        found := decodedObj.([]any)[referenceIndex]
                        recoded, _ := json.MarshalIndent(found, "", "  ")
                        referenceObject = string(recoded)
                    }
                }
                if referenceObject == "" {
                    referenceObject = string(responseBody)
                }

                violation := &errors.SchemaValidationFailure{
                    Reason:          er.Error,
                    Location:        er.KeywordLocation,
                    ReferenceSchema: string(renderedSchema),
                    ReferenceObject: referenceObject,
                    OriginalError:   jk,
                }
                // if we have a location within the schema, add it to the error
                if located != nil {

                    line := located.Line
                    // if the located node is a map or an array, then the actual human interpretable
                    // line on which the violation occurred is the line of the key, not the value.
                    if located.Kind == yaml.MappingNode || located.Kind == yaml.SequenceNode {
                        if line > 0 {
                            line--
                        }
                    }

                    // location of the violation within the rendered schema.
                    violation.Line = line
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
            Reason: fmt.Sprintf("The response body for status code '%d' is defined as an object. "+
                "However, it does not meet the schema requirements of the specification", response.StatusCode),
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
