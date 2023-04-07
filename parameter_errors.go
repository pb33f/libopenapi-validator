package main

import (
    "fmt"
    "github.com/pb33f/libopenapi/datamodel/high/base"
    "github.com/pb33f/libopenapi/datamodel/high/v3"
    "gopkg.in/yaml.v3"
    "net/url"
)

func (v *validator) incorrectFormEncoding(param *v3.Parameter, qp *queryParam, i int) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' is not exploded correctly", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' has a default or 'form' encoding defined, "+
            "however the value '%s' is encoded as an object or an array using commas. The contract defines "+
            "the explode value to set to 'true'", param.Name, qp.values[i]),
        SpecLine: param.GoLow().Explode.ValueNode.Line,
        SpecCol:  param.GoLow().Explode.ValueNode.Column,
        Context:  param,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidFormEncode,
            collapseCSVIntoFormStyle(param.Name, qp.values[i])),
    }
}

func (v *validator) incorrectSpaceDelimiting(param *v3.Parameter, qp *queryParam) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' has 'spaceDelimited' style defined, "+
            "and explode is defined as false. There are multiple values (%d) supplied, instead of a single"+
            " space delimited value", param.Name, len(qp.values)),
        SpecLine: param.GoLow().Style.ValueNode.Line,
        SpecCol:  param.GoLow().Style.ValueNode.Column,
        Context:  param,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidSpaceDelimitedObjectExplode,
            collapseCSVIntoSpaceDelimitedStyle(param.Name, qp.values)),
    }
}

func (v *validator) incorrectPipeDelimiting(param *v3.Parameter, qp *queryParam) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' delimited incorrectly", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' has 'pipeDelimited' style defined, "+
            "and explode is defined as false. There are multiple values (%d) supplied, instead of a single"+
            " space delimited value", param.Name, len(qp.values)),
        SpecLine: param.GoLow().Style.ValueNode.Line,
        SpecCol:  param.GoLow().Style.ValueNode.Column,
        Context:  param,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidPipeDelimitedObjectExplode,
            collapseCSVIntoPipeDelimitedStyle(param.Name, qp.values)),
    }
}

func (v *validator) invalidDeepObject(param *v3.Parameter, qp *queryParam) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' is not a valid deepObject", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' has the 'deepObject' style defined, "+
            "There are multiple values (%d) supplied, instead of a single "+
            "value", param.Name, len(qp.values)),
        SpecLine: param.GoLow().Style.ValueNode.Line,
        SpecCol:  param.GoLow().Style.ValueNode.Column,
        Context:  param,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidDeepObjectMultipleValues,
            collapseCSVIntoPipeDelimitedStyle(param.Name, qp.values)),
    }
}

func (v *validator) queryParameterMissing(param *v3.Parameter) *ValidationError {
    return &ValidationError{
        Message: fmt.Sprintf("Query parameter '%s' is missing", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' is defined as being required, "+
            "however it's missing from the request", param.Name),
        SpecLine: param.GoLow().Required.KeyNode.Line,
        SpecCol:  param.GoLow().Required.KeyNode.Column,
    }
}

func (v *validator) headerParameterMissing(param *v3.Parameter) *ValidationError {
    return &ValidationError{
        Message: fmt.Sprintf("Header parameter '%s' is missing", param.Name),
        Reason: fmt.Sprintf("The header parameter '%s' is defined as being required, "+
            "however it's missing from the request", param.Name),
        SpecLine: param.GoLow().Required.KeyNode.Line,
        SpecCol:  param.GoLow().Required.KeyNode.Column,
    }
}

func (v *validator) headerParameterNotDefined(paramName string, kn *yaml.Node) *ValidationError {
    return &ValidationError{
        Message:  fmt.Sprintf("Header parameter '%s' is not defined", paramName),
        Reason:   fmt.Sprintf("The header parameter '%s' is not defined as part of the specification", paramName),
        SpecLine: kn.Line,
        SpecCol:  kn.Column,
    }
}

func (v *validator) incorrectQueryParamArrayBoolean(
    param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query array parameter '%s' is not a valid boolean", param.Name),
        Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being a boolean, "+
            "however the value '%s' is not a valid true/false value", param.Name, item),
        SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
        SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
        Context:  itemsSchema,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, item),
    }
}

func (v *validator) incorrectQueryParamArrayNumber(
    param *v3.Parameter, item string, sch *base.Schema, itemsSchema *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query array parameter '%s' is not a valid number", param.Name),
        Reason: fmt.Sprintf("The query parameter (which is an array) '%s' is defined as being a number, "+
            "however the value '%s' is not a valid number", param.Name, item),
        SpecLine: sch.Items.A.GoLow().Schema().Type.KeyNode.Line,
        SpecCol:  sch.Items.A.GoLow().Schema().Type.KeyNode.Column,
        Context:  itemsSchema,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, item),
    }
}

func (v *validator) incorrectParamEncodingJSON(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' is not valid JSON", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' is defined as being a JSON object, "+
            "however the value '%s' is not valid JSON", param.Name, ef),
        SpecLine: param.GoLow().FindContent(JSONContentType).ValueNode.Line,
        SpecCol:  param.GoLow().FindContent(JSONContentType).ValueNode.Column,
        Context:  sch,
        HowToFix: HowToFixInvalidJSON,
    }
}

func (v *validator) incorrectQueryParamBool(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' is not a valid boolean", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' is defined as being a boolean, "+
            "however the value '%s' is not a valid boolean", param.Name, ef),
        SpecLine: param.GoLow().Schema.KeyNode.Line,
        SpecCol:  param.GoLow().Schema.KeyNode.Column,
        Context:  sch,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, ef),
    }
}

func (v *validator) invalidQueryParamNumber(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' is not a valid number", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' is defined as being a number, "+
            "however the value '%s' is not a valid number", param.Name, ef),
        SpecLine: param.GoLow().Schema.KeyNode.Line,
        SpecCol:  param.GoLow().Schema.KeyNode.Column,
        Context:  sch,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, ef),
    }
}

func (v *validator) incorrectReservedValues(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationQuery,
        Message:           fmt.Sprintf("Query parameter '%s' value contains reserved values", param.Name),
        Reason: fmt.Sprintf("The query parameter '%s' has 'allowReserved' set to false, "+
            "however the value '%s' contains one of the following characters: :/?#[]@!$&'()*+,;=", param.Name, ef),
        SpecLine: param.GoLow().Schema.KeyNode.Line,
        SpecCol:  param.GoLow().Schema.KeyNode.Column,
        Context:  sch,
        HowToFix: fmt.Sprintf(HowToFixReservedValues, url.QueryEscape(ef)),
    }
}

func (v *validator) invalidHeaderParamNumber(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationHeader,
        Message:           fmt.Sprintf("Header parameter '%s' is not a valid number", param.Name),
        Reason: fmt.Sprintf("The header parameter '%s' is defined as being a number, "+
            "however the value '%s' is not a valid number", param.Name, ef),
        SpecLine: param.GoLow().Schema.KeyNode.Line,
        SpecCol:  param.GoLow().Schema.KeyNode.Column,
        Context:  sch,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidNumber, ef),
    }
}

func (v *validator) incorrectHeaderParamBool(param *v3.Parameter, ef string, sch *base.Schema) *ValidationError {
    return &ValidationError{
        ValidationType:    ParameterValidation,
        ValidationSubType: ParameterValidationHeader,
        Message:           fmt.Sprintf("Header parameter '%s' is not a valid boolean", param.Name),
        Reason: fmt.Sprintf("The header parameter '%s' is defined as being a boolean, "+
            "however the value '%s' is not a valid boolean", param.Name, ef),
        SpecLine: param.GoLow().Schema.KeyNode.Line,
        SpecCol:  param.GoLow().Schema.KeyNode.Column,
        Context:  sch,
        HowToFix: fmt.Sprintf(HowToFixParamInvalidBoolean, ef),
    }
}
