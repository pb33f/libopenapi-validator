// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package errors

import (
	"fmt"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"net/http"
	"strings"
)

func ResponseContentTypeNotFound(op *v3.Operation,
	request *http.Request,
	response *http.Response,
	code string,
	isDefault bool) *ValidationError {
	ct := response.Header.Get(helpers.ContentTypeHeader)
	mediaTypeString, _, _ := helpers.ExtractContentType(ct)
	var ctypes []string
	var specLine, specCol int
	var contentMap map[string]*v3.MediaType

	// check for a default type (applies to all codes without a match)
	if !isDefault {
		for k := range op.Responses.Codes[code].Content {
			ctypes = append(ctypes, k)
		}
		specLine = op.Responses.Codes[code].GoLow().Content.KeyNode.Line
		specCol = op.Responses.Codes[code].GoLow().Content.KeyNode.Column
		contentMap = op.Responses.Codes[code].Content
	} else {
		for k := range op.Responses.Default.Content {
			ctypes = append(ctypes, k)
		}
		specLine = op.Responses.Default.GoLow().Content.KeyNode.Line
		specCol = op.Responses.Default.GoLow().Content.KeyNode.Column
		contentMap = op.Responses.Default.Content
	}
	return &ValidationError{
		ValidationType:    helpers.ResponseBodyValidation,
		ValidationSubType: helpers.RequestBodyContentType,
		Message: fmt.Sprintf("%s / %s operation response content type '%s' does not exist",
			request.Method, code, mediaTypeString),
		Reason: fmt.Sprintf("The content type '%s' of the %s response received has not "+
			"been defined, it's an unknown type", mediaTypeString, request.Method),
		SpecLine: specLine,
		SpecCol:  specCol,
		Context:  op,
		HowToFix: fmt.Sprintf(HowToFixInvalidContentType,
			len(contentMap), strings.Join(ctypes, ", ")),
	}
}

func ResponseCodeNotFound(op *v3.Operation, request *http.Request, code int) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ResponseBodyValidation,
		ValidationSubType: helpers.ResponseBodyResponseCode,
		Message: fmt.Sprintf("%s operation request response code '%d' does not exist",
			request.Method, code),
		Reason: fmt.Sprintf("The reponse code '%d' of the %s request submitted has not "+
			"been defined, it's an unknown type", code, request.Method),
		SpecLine: op.GoLow().Responses.KeyNode.Line,
		SpecCol:  op.GoLow().Responses.KeyNode.Column,
		Context:  op,
		HowToFix: HowToFixInvalidResponseCode,
	}
}
