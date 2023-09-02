package errors

import (
	"fmt"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/high/v3"
	"net/url"
	"strings"
)

func IncorrectFormEncoding(param *v3.Parameter, qp *helpers.QueryParam, i int) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' is not exploded correctly", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' has a default or 'form' encoding defined, "+
			"however the value '%s' is encoded as an object or an array using commas. The contract defines "+
			"the explode value to set to 'true'", param.Name, qp.Values[i]),
		SpecLine: param.GoLow().Explode.ValueNode.Line,
		SpecCol:  param.GoLow().Explode.ValueNode.Column,
		Context:  param,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidFormEncode,
			helpers.CollapseCSVIntoFormStyle(param.Name, qp.Values[i])),
	}
}

func IncorrectSpaceDelimiting(param *v3.Parameter, qp *helpers.QueryParam) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' has 'spaceDelimited' style defined, "+
			"and explode is defined as false. There are multiple values (%d) supplied, instead of a single"+
			" space delimited value", param.Name, len(qp.Values)),
		SpecLine: param.GoLow().Style.ValueNode.Line,
		SpecCol:  param.GoLow().Style.ValueNode.Column,
		Context:  param,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidSpaceDelimitedObjectExplode,
			helpers.CollapseCSVIntoSpaceDelimitedStyle(param.Name, qp.Values)),
	}
}

func IncorrectPipeDelimiting(param *v3.Parameter, qp *helpers.QueryParam) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' has 'pipeDelimited' style defined, "+
			"and explode is defined as false. There are multiple values (%d) supplied, instead of a single"+
			" space delimited value", param.Name, len(qp.Values)),
		SpecLine: param.GoLow().Style.ValueNode.Line,
		SpecCol:  param.GoLow().Style.ValueNode.Column,
		Context:  param,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidPipeDelimitedObjectExplode,
			helpers.CollapseCSVIntoPipeDelimitedStyle(param.Name, qp.Values)),
	}
}

func InvalidDeepObject(param *v3.Parameter, qp *helpers.QueryParam) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' is not a valid deepObject", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' has the 'deepObject' style defined, "+
			"There are multiple values (%d) supplied, instead of a single "+
			"value", param.Name, len(qp.Values)),
		SpecLine: param.GoLow().Style.ValueNode.Line,
		SpecCol:  param.GoLow().Style.ValueNode.Column,
		Context:  param,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidDeepObjectMultipleValues,
			helpers.CollapseCSVIntoPipeDelimitedStyle(param.Name, qp.Values)),
	}
}

func QueryParameterMissing(param *v3.Parameter) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' is missing", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' is defined as being required, "+
			"however it's missing from the requests", param.Name),
		SpecLine: param.GoLow().Required.KeyNode.Line,
		SpecCol:  param.GoLow().Required.KeyNode.Column,
		HowToFix: HowToFixMissingValue,
	}
}

func HeaderParameterMissing(param *v3.Parameter) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationHeader,
		Message:           fmt.Sprintf("Header parameter '%s' is missing", param.Name),
		Reason: fmt.Sprintf("The header parameter '%s' is defined as being required, "+
			"however it's missing from the requests", param.Name),
		SpecLine: param.GoLow().Required.KeyNode.Line,
		SpecCol:  param.GoLow().Required.KeyNode.Column,
		HowToFix: HowToFixMissingValue,
	}
}

func HeaderParameterCannotBeDecoded(param *v3.Parameter, val string) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationHeader,
		Message:           fmt.Sprintf("Header parameter '%s' cannot be decoded", param.Name),
		Reason: fmt.Sprintf("The header parameter '%s' cannot be "+
			"extracted into an object, '%s' is malformed", param.Name, val),
		SpecLine: param.GoLow().Schema.Value.Schema().Type.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.Value.Schema().Type.KeyNode.Line,
		HowToFix: HowToFixInvalidEncoding,
	}
}

func IncorrectHeaderParamEnum(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	var enums []string
	for i := range sch.Enum {
		enums = append(enums, fmt.Sprint(sch.Enum[i]))
	}
	validEnums := strings.Join(enums, ", ")
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationHeader,
		Message:           fmt.Sprintf("Header parameter '%s' does not match allowed values", param.Name),
		Reason: fmt.Sprintf("The header parameter '%s' has pre-defined "+
			"values set via an enum. The value '%s' is not one of those values.", param.Name, ef),
		SpecLine: param.GoLow().Schema.Value.Schema().Enum.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.Value.Schema().Enum.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidEnum, ef, validEnums),
	}
}

func IncorrectQueryParamArrayBoolean(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query array parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid true/false value", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, item),
	}
}

func IncorrectCookieParamArrayBoolean(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationCookie,
		Message:           fmt.Sprintf("Cookie array parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The cookie parameter (which is an array) '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid true/false value", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, item),
	}
}

func IncorrectQueryParamArrayNumber(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query array parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, item),
	}
}

func IncorrectCookieParamArrayNumber(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationCookie,
		Message:           fmt.Sprintf("Cookie array parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The cookie parameter (which is an array) '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, item),
	}
}

func IncorrectParamEncodingJSON(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' is not valid JSON", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' is defined as being a JSON object, "+
			"however the value '%s' is not valid JSON", param.Name, ef),
		SpecLine: param.GoLow().FindContent(helpers.JSONContentType).ValueNode.Line,
		SpecCol:  param.GoLow().FindContent(helpers.JSONContentType).ValueNode.Column,
		Context:  sch,
		HowToFix: HowToFixInvalidJSON,
	}
}

func IncorrectQueryParamBool(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid boolean", param.Name, ef),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, ef),
	}
}

func InvalidQueryParamNumber(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, ef),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, ef),
	}
}

func IncorrectQueryParamEnum(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	var enums []string
	for i := range sch.Enum {
		enums = append(enums, fmt.Sprint(sch.Enum[i]))
	}
	validEnums := strings.Join(enums, ", ")
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' does not match allowed values", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' has pre-defined "+
			"values set via an enum. The value '%s' is not one of those values.", param.Name, ef),
		SpecLine: param.GoLow().Schema.Value.Schema().Enum.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.Value.Schema().Enum.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidEnum, ef, validEnums),
	}
}

func IncorrectQueryParamEnumArray(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	var enums []string
	// look at that model fly!
	for i := range param.GoLow().Schema.Value.Schema().Items.Value.A.Schema().Enum.Value {
		enums = append(enums,
			fmt.Sprint(param.GoLow().Schema.Value.Schema().Items.Value.A.Schema().Enum.Value[i].Value))
	}
	validEnums := strings.Join(enums, ", ")
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query array parameter '%s' does not match allowed values", param.Name),
		Reason: fmt.Sprintf("The query array parameter '%s' has pre-defined "+
			"values set via an enum. The value '%s' is not one of those values.", param.Name, ef),
		SpecLine: param.GoLow().Schema.Value.Schema().Items.Value.A.Schema().Enum.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.Value.Schema().Items.Value.A.Schema().Enum.KeyNode.Line,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidEnum, ef, validEnums),
	}
}

func IncorrectReservedValues(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationQuery,
		Message:           fmt.Sprintf("Query parameter '%s' value contains reserved values", param.Name),
		Reason: fmt.Sprintf("The query parameter '%s' has 'allowReserved' set to false, "+
			"however the value '%s' contains one of the following characters: :/?#[]@!$&'()*+,;=", param.Name, ef),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixReservedValues, url.QueryEscape(ef)),
	}
}

func InvalidHeaderParamNumber(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationHeader,
		Message:           fmt.Sprintf("Header parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The header parameter '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, ef),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, ef),
	}
}

func InvalidCookieParamNumber(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationCookie,
		Message:           fmt.Sprintf("Cookie parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The cookie parameter '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, ef),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, ef),
	}
}

func IncorrectHeaderParamBool(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationHeader,
		Message:           fmt.Sprintf("Header parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The header parameter '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid boolean", param.Name, ef),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, ef),
	}
}

func IncorrectCookieParamBool(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationCookie,
		Message:           fmt.Sprintf("Cookie parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The cookie parameter '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid boolean", param.Name, ef),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, ef),
	}
}

func IncorrectCookieParamEnum(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	var enums []string
	for i := range sch.Enum {
		enums = append(enums, fmt.Sprint(sch.Enum[i]))
	}
	validEnums := strings.Join(enums, ", ")
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationCookie,
		Message:           fmt.Sprintf("Cookie parameter '%s' does not match allowed values", param.Name),
		Reason: fmt.Sprintf("The cookie parameter '%s' has pre-defined "+
			"values set via an enum. The value '%s' is not one of those values.", param.Name, ef),
		SpecLine: param.GoLow().Schema.Value.Schema().Enum.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.Value.Schema().Enum.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidEnum, ef, validEnums),
	}
}

func IncorrectHeaderParamArrayBoolean(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationHeader,
		Message:           fmt.Sprintf("Header array parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The header parameter (which is an array) '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid true/false value", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, item),
	}
}

func IncorrectHeaderParamArrayNumber(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationHeader,
		Message:           fmt.Sprintf("Header array parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The header parameter (which is an array) '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, item),
	}
}

func IncorrectPathParamBool(param *v3.Parameter, item string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationPath,
		Message:           fmt.Sprintf("Path parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The path parameter '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid boolean", param.Name, item),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, item),
	}
}

func IncorrectPathParamEnum(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
	var enums []string
	for i := range sch.Enum {
		enums = append(enums, fmt.Sprint(sch.Enum[i]))
	}
	validEnums := strings.Join(enums, ", ")
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationPath,
		Message:           fmt.Sprintf("Path parameter '%s' does not match allowed values", param.Name),
		Reason: fmt.Sprintf("The path parameter '%s' has pre-defined "+
			"values setvia an enum. The value '%s' is not one of those values.", param.Name, ef),
		SpecLine: param.GoLow().Schema.Value.Schema().Enum.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.Value.Schema().Enum.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidEnum, ef, validEnums),
	}
}

func IncorrectPathParamNumber(param *v3.Parameter, item string, sch *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationPath,
		Message:           fmt.Sprintf("Path parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The path parameter '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, item),
		SpecLine: param.GoLow().Schema.KeyNode.Line,
		SpecCol:  param.GoLow().Schema.KeyNode.Column,
		Context:  sch,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, item),
	}
}

func IncorrectPathParamArrayNumber(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationPath,
		Message:           fmt.Sprintf("Path array parameter '%s' is not a valid number", param.Name),
		Reason: fmt.Sprintf("The path parameter (which is an array) '%s' is defined as being a number, "+
			"however the value '%s' is not a valid number", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, item),
	}
}

func IncorrectPathParamArrayBoolean(
	param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
	return &ValidationError{
		ValidationType:    helpers.ParameterValidation,
		ValidationSubType: helpers.ParameterValidationPath,
		Message:           fmt.Sprintf("Path array parameter '%s' is not a valid boolean", param.Name),
		Reason: fmt.Sprintf("The path parameter (which is an array) '%s' is defined as being a boolean, "+
			"however the value '%s' is not a valid boolean", param.Name, item),
		SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
		SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
		Context:  itemsSchema,
		HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, item),
	}
}
