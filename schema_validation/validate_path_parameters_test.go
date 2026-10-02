// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package schema_validation

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"

	"github.com/pb33f/libopenapi-validator/config"
)

func validateDocWithPathParams(t *testing.T, spec string) (bool, []string) {
	t.Helper()
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	valid, errs := ValidateOpenAPIDocument(doc, config.WithPathParameterDocumentValidation())
	reasons := make([]string, 0, len(errs))
	for _, e := range errs {
		reasons = append(reasons, e.Reason)
	}
	return valid, reasons
}

func TestValidatePathParameters_Valid(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.True(t, valid)
	assert.Empty(t, reasons)
}

func TestValidatePathParameters_PathLevelParamIsValid(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    parameters:
      - name: petId
        in: path
        required: true
        schema:
          type: string
    get:
      responses:
        "200":
          description: OK
    delete:
      responses:
        "204":
          description: No Content`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.True(t, valid)
	assert.Empty(t, reasons)
}

func TestValidatePathParameters_TemplateWithoutParameter(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      responses:
        "200":
          description: OK`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.False(t, valid)
	require.Len(t, reasons, 1)
	assert.Contains(t, reasons[0], `template variable "petId"`)
	assert.Contains(t, reasons[0], "get operation")
}

func TestValidatePathParameters_ParameterWithoutTemplate(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets:
    get:
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.False(t, valid)
	require.Len(t, reasons, 1)
	assert.Contains(t, reasons[0], `'path' parameter "petId"`)
	assert.Contains(t, reasons[0], "does not match any template variable")
}

func TestValidatePathParameters_RequiredHandledByBaseSchema(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      parameters:
        - name: petId
          in: path
          schema:
            type: string
      responses:
        "200":
          description: OK`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	valid, errs := ValidateOpenAPIDocument(doc)
	assert.False(t, valid)
	require.NotEmpty(t, errs)

	validWithOpt, errsWithOpt := ValidateOpenAPIDocument(doc, config.WithPathParameterDocumentValidation())
	assert.False(t, validWithOpt)
	assert.Len(t, errsWithOpt, len(errs))
}

func TestValidatePathParameters_EmptyPathItemIsExempt(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Test","version":"1.0.0"},"paths":{"/test/{param}":{}}}`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.True(t, valid)
	assert.Empty(t, reasons)
}

func TestValidatePathParameters_MultipleTemplateVars(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}/toys/{toyId}:
    get:
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.False(t, valid)
	require.Len(t, reasons, 1)
	assert.Contains(t, reasons[0], `template variable "toyId"`)
}

func TestValidatePathParameters_OnePerOperationMissing(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      parameters:
        - name: petId
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
    delete:
      responses:
        "204":
          description: No Content`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.False(t, valid)
	require.Len(t, reasons, 1)
	assert.Contains(t, reasons[0], "delete operation")
}

func TestValidatePathParameters_RefParameterResolved(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      parameters:
        - $ref: '#/components/parameters/PetId'
      responses:
        "200":
          description: OK
components:
  parameters:
    PetId:
      name: petId
      in: path
      required: true
      schema:
        type: string`

	valid, reasons := validateDocWithPathParams(t, spec)
	assert.True(t, valid)
	assert.Empty(t, reasons)
}

func TestValidatePathParameters_DisabledByDefault(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      responses:
        "200":
          description: OK`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	valid, errs := ValidateOpenAPIDocument(doc)
	assert.True(t, valid)
	assert.Empty(t, errs)
}

func TestValidatePathParameters_ReportsLineAndContext(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Test
  version: 1.0.0
paths:
  /pets/{petId}:
    get:
      responses:
        "200":
          description: OK`

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)

	valid, errs := ValidateOpenAPIDocument(doc, config.WithPathParameterDocumentValidation())
	assert.False(t, valid)
	require.Len(t, errs, 1)
	assert.Equal(t, 6, errs[0].SpecLine)
	assert.Equal(t, "/paths/~1pets~1{petId}", errs[0].Context)
	assert.Contains(t, errs[0].HowToFix, "in: path")
}
