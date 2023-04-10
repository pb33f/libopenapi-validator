// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package helpers

import (
    "fmt"
    v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
    "net/http"
    "strconv"
    "strings"
)

type QueryParam struct {
    Key      string
    Values   []string
    Property string
}

func ExtractParamsForOperation(request *http.Request, item *v3.PathItem) []*v3.Parameter {
    params := item.Parameters
    switch request.Method {
    case http.MethodGet:
        if item.Get != nil {
            params = append(params, item.Get.Parameters...)
        }
    case http.MethodPost:
        if item.Post != nil {
            params = append(params, item.Post.Parameters...)
        }
    case http.MethodPut:
        if item.Put != nil {
            params = append(params, item.Put.Parameters...)
        }
    case http.MethodDelete:
        if item.Delete != nil {
            params = append(params, item.Delete.Parameters...)
        }
    case http.MethodOptions:
        if item.Options != nil {
            params = append(params, item.Options.Parameters...)
        }
    case http.MethodHead:
        if item.Head != nil {
            params = append(params, item.Head.Parameters...)
        }
    case http.MethodPatch:
        if item.Patch != nil {
            params = append(params, item.Patch.Parameters...)
        }
    case http.MethodTrace:
        if item.Trace != nil {
            params = append(params, item.Trace.Parameters...)
        }
    }
    return params
}

func cast(v string) any {

    if v == "true" || v == "false" {
        b, _ := strconv.ParseBool(v)
        return b
    }
    if i, err := strconv.ParseFloat(v, 64); err == nil {
        // check if this is an int or not
        if !strings.Contains(v, Period) {
            iv, _ := strconv.ParseInt(v, 10, 64)
            return iv
        }
        return i
    }
    return v
}

// deepObject encoding is a technique used to encode objects into query parameters. Kinda nuts.
func ConstructParamMapFromDeepObjectEncoding(values []*QueryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        if decoded[v.Key] == nil {
            props := make(map[string]interface{})
            props[v.Property] = cast(v.Values[0])
            decoded[v.Key] = props
        } else {
            decoded[v.Key].(map[string]interface{})[v.Property] = cast(v.Values[0])
        }
    }
    return decoded
}

func ConstructParamMapFromQueryParamInput(values map[string][]*QueryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, q := range values {
        for _, v := range q {
            decoded[v.Key] = cast(v.Values[0])
        }
    }
    return decoded
}

// Pipes are always a good alternative to commas, personally I think they're better, if I were encoding, I would
// use pipes instead of commas, so much can go wrong with a comma, but a pipe? hardly ever.
func ConstructParamMapFromPipeEncoding(values []*QueryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        props := make(map[string]interface{})
        // explode PSV into array
        exploded := strings.Split(v.Values[0], Pipe)
        for i := range exploded {
            if i%2 == 0 {
                props[exploded[i]] = cast(exploded[i+1])
            }
        }
        decoded[v.Key] = props
    }
    return decoded
}

// Don't use spaces to delimit anything unless you really know what the hell you're doing. Perhaps the
// easiest way to blow something up, unless you're tokenizing strings... don't do this.
func ConstructParamMapFromSpaceEncoding(values []*QueryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        props := make(map[string]interface{})
        // explode SSV into array
        exploded := strings.Split(v.Values[0], Space)
        for i := range exploded {
            if i%2 == 0 {
                props[exploded[i]] = cast(exploded[i+1])
            }
        }
        decoded[v.Key] = props
    }
    return decoded
}

func ConstructMapFromCSV(csv string) map[string]interface{} {
    decoded := make(map[string]interface{})
    // explode SSV into array
    exploded := strings.Split(csv, Comma)
    for i := range exploded {
        if i%2 == 0 {
            if len(exploded) == i+1 {
                break
            }
            decoded[exploded[i]] = cast(exploded[i+1])
        }
    }
    return decoded
}

func ConstructKVFromCSV(values string) map[string]interface{} {
    props := make(map[string]interface{})
    exploded := strings.Split(values, Comma)
    for i := range exploded {
        obK := strings.Split(exploded[i], Equals)
        if len(obK) == 2 {
            props[obK[0]] = cast(obK[1])
        }
    }
    return props
}

func ConstructKVFromLabelEncoding(values string) map[string]interface{} {
    props := make(map[string]interface{})
    exploded := strings.Split(values, Period)
    for i := range exploded {
        obK := strings.Split(exploded[i], Equals)
        if len(obK) == 2 {
            props[obK[0]] = cast(obK[1])
        }
    }
    return props
}

func ConstructKVFromMatrixCSV(values string) map[string]interface{} {
    props := make(map[string]interface{})
    exploded := strings.Split(values, SemiColon)
    for i := range exploded {
        obK := strings.Split(exploded[i], Equals)
        if len(obK) == 2 {
            props[obK[0]] = cast(obK[1])
        }
    }
    return props
}

func ConstructParamMapFromFormEncodingArray(values []*QueryParam) map[string]interface{} {
    decoded := make(map[string]interface{})
    for _, v := range values {
        props := make(map[string]interface{})
        // explode SSV into array
        exploded := strings.Split(v.Values[0], Comma)
        for i := range exploded {
            if i%2 == 0 {
                props[exploded[i]] = cast(exploded[i+1])
            }
        }
        decoded[v.Key] = props
    }
    return decoded
}

func DoesFormParamContainDelimiter(value, style string) bool {
    if strings.Contains(value, Comma) && (style == "" || style == Form) {
        return true
    }
    return false
}

func ExplodeQueryValue(value, style string) []string {
    switch style {
    case SpaceDelimited:
        return strings.Split(value, Space)
    case PipeDelimited:
        return strings.Split(value, Pipe)
    default:
        return strings.Split(value, Comma)
    }

}

func CollapseCSVIntoFormStyle(key string, value string) string {
    return fmt.Sprintf("&%s=%s", key,
        strings.Join(strings.Split(value, ","), fmt.Sprintf("&%s=", key)))
}

func CollapseCSVIntoSpaceDelimitedStyle(key string, values []string) string {
    return fmt.Sprintf("%s=%s", key, strings.Join(values, "%20"))
}

func CollapseCSVIntoPipeDelimitedStyle(key string, values []string) string {
    return fmt.Sprintf("%s=%s", key, strings.Join(values, Pipe))
}
