package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/dlclark/regexp2"
	"github.com/goccy/go-yaml"
	"github.com/pb33f/libopenapi"
	"github.com/santhosh-tekuri/jsonschema/v6"

	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
)

type customRegexp regexp2.Regexp

func (re *customRegexp) MatchString(s string) bool {
	matched, err := (*regexp2.Regexp)(re).MatchString(s)
	return err == nil && matched
}

func (re *customRegexp) String() string {
	return (*regexp2.Regexp)(re).String()
}

type regexEngine struct {
	runtimeOption regexp2.RegexOptions
}

func (e *regexEngine) run(s string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(s, e.runtimeOption)
	if err != nil {
		return nil, err
	}
	return (*customRegexp)(re), nil
}

var regexParsingOptionsMap = map[string]regexp2.RegexOptions{
	"none":                    regexp2.None,
	"ignorecase":              regexp2.IgnoreCase,
	"multiline":               regexp2.Multiline,
	"explicitcapture":         regexp2.ExplicitCapture,
	"compiled":                regexp2.Compiled,
	"singleline":              regexp2.Singleline,
	"ignorepatternwhitespace": regexp2.IgnorePatternWhitespace,
	"righttoleft":             regexp2.RightToLeft,
	"debug":                   regexp2.Debug,
	"ecmascript":              regexp2.ECMAScript,
	"re2":                     regexp2.RE2,
	"unicode":                 regexp2.Unicode,
}

var (
	defaultRegexEngine  = ""
	regexParsingOptions = flag.String("regexengine", defaultRegexEngine, `Specify the regex parsing option to use.
                         Supported values are: 
                           Engines: re2 (default), ecmascript
                           Flags:  ignorecase, multiline, explicitcapture, compiled, 
                                   singleline, ignorepatternwhitespace, righttoleft, 
                                   debug, unicode
                         If not specified, the default libopenapi option is "re2".

If not specified, the default libopenapi regex engine is "re2"".`)
	convertYAMLToJSON = flag.Bool("yaml2json", false, `Convert YAML files to JSON before validation.
						libopenapi passes map[interface{}]interface{} structures for deeply nested objects
						or complex mappings, which are not allowed in JSON and cannot be validated by jsonschema.
						This flag allows pre-converting from YAML to JSON to bypass this limitation of the libopenapi.
						Default is false.`)
)

// main is the entry point for validating an OpenAPI Specification (OAS) document.
// It uses the libopenapi-validator library to check if the provided OAS document
// conforms to the OpenAPI specification.
//
// This tool accepts a single input file (YAML or JSON) and provides optional flags:
//
// `--regexengine` flag to customize the regex engine used during validation.
// This is useful for cases where the spec uses regex patterns that require engines
// like ECMAScript or RE2.
//
// Supported regex options include:
//   - Engines: re2 (default), ecmascript
//   - Flags:  ignorecase, multiline, explicitcapture, compiled, singleline,
//     ignorepatternwhitespace, righttoleft, debug, unicode
//
// `--yaml2json` flag to convert YAML files to JSON before validation.
// libopenapi passes map[interface{}]interface{} structures for deeply nested
// objects or complex mappings, which are not allowed in JSON and cannot be
// validated by jsonschema. This flag allows pre-converting from YAML to JSON
// to bypass this limitation of the libopenapi. Default is false.
//
// Example usage:
//
//	go run main.go --regexengine=ecmascript ./my-api-spec.yaml
//	go run main.go --yaml2json ./my-api-spec.yaml
//
// If validation passes, the tool logs a success message.
// If the document is invalid or there is a processing error, it logs details and exits non-zero.
func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: validate [OPTIONS] <file>

Validates an OpenAPI document using libopenapi-validator.

Options:
  --regexengine string   Specify the regex parsing option to use.
                         Supported values are:
                           Engines: re2 (default), ecmascript
                           Flags:  ignorecase, multiline, explicitcapture, compiled,
                                   singleline, ignorepatternwhitespace, righttoleft,
                                   debug, unicode
                         If not specified, the default libopenapi option is "re2".

  --yaml2json            Convert YAML files to JSON before validation.
						 libopenapi passes map[interface{}]interface{}
                         structures for deeply nested objects or complex mappings, which
                         are not allowed in JSON and cannot be validated by jsonschema.
                         This flag allows pre-converting from YAML to JSON to bypass this
                         limitation of the libopenapi.
                         (default: false)

  -h, --help             Show this help message and exit.
`)
	}

	for _, arg := range os.Args[1:] {
		if arg == "--help" || arg == "-h" {
			flag.Usage()
			os.Exit(0)
		}
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	flag.Parse()
	filename := flag.Arg(0)
	if len(flag.Args()) != 1 || filename == "" {
		logger.Error("missing file argument", slog.Any("args", os.Args))
		flag.Usage()
		os.Exit(1)
	}
	validationOpts := []config.Option{}
	if *regexParsingOptions != "" {
		regexEngineOption, ok := regexParsingOptionsMap[*regexParsingOptions]
		if !ok {
			logger.Error("unsupported regex option provided",
				slog.String("provided", *regexParsingOptions),
				slog.Any("supported", []string{
					"none",
					"ignorecase",
					"multiline",
					"explicitcapture",
					"compiled",
					"singleline",
					"ignorepatternwhitespace",
					"righttoleft",
					"debug",
					"ecmascript",
					"re2",
					"unicode",
				}),
			)
			os.Exit(1)
		}
		reEngine := &regexEngine{
			runtimeOption: regexEngineOption,
		}

		validationOpts = append(validationOpts, config.WithRegexEngine(reEngine.run))
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		logger.Error("error reading file", slog.String("provided", filename), slog.Any("error", err))
		os.Exit(1)
	}

	if *convertYAMLToJSON {
		var v interface{}
		if err := yaml.Unmarshal(data, &v); err == nil {
			data, err = yaml.YAMLToJSON(data)
			if err != nil {
				logger.Error("invalid api spec: error converting yaml to json", slog.Any("error", err))
				os.Exit(1)
			}
		}
	}

	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		logger.Error("error creating new libopenapi document", slog.Any("error", err))
		os.Exit(1)
	}

	docValidator, validatorErrs := validator.NewValidator(doc, validationOpts...)
	if len(validatorErrs) > 0 {
		logger.Error("error creating a new validator", slog.Any("errors", errors.Join(validatorErrs...)))
		os.Exit(1)
	}

	valid, validationErrs := docValidator.ValidateDocument()
	if !valid {
		logger.Error("validation errors", slog.Any("errors", validationErrs))
		os.Exit(1)
	}
	logger.Info("document passes all validations", slog.String("filename", filename))
}
