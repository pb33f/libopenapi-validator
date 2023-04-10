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
    "io"
    "net/http"
    "strings"
)

func ValidateRequestSchema(request *http.Request, schema *base.Schema) (bool, []*errors.ValidationError) {

    // render the schema, to be used for validation
    renderedSchema, _ := schema.Render()
    jsonSchema, _ := utils.ConvertYAMLtoJSON(renderedSchema)
    requestBody, _ := io.ReadAll(request.Body)

    var decodedObj interface{}
    _ = json.Unmarshal(requestBody, &decodedObj)

    compiler := jsonschema.NewCompiler()
    _ = compiler.AddResource("requestBody.json", strings.NewReader(string(jsonSchema)))
    jsch, _ := compiler.Compile("requestBody.json")

    // 4. validate the object against the schema
    scErrs := jsch.Validate(decodedObj)
    fmt.Print(scErrs)

    return true, nil
}
