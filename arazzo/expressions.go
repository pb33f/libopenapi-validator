// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"encoding/json"
	"fmt"
	"mime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
	"github.com/pb33f/jsonpath/pkg/jsonpath"
	jsonpathconfig "github.com/pb33f/jsonpath/pkg/jsonpath/config"
	"github.com/pb33f/libopenapi/arazzo/expression"
)

type (
	exprScope     struct{ workflowID, stepID string }
	expressionUse struct {
		Expression               expression.Expression
		Path, WorkflowID, StepID string
	}
)

func checkExpressions(v *validation) {
	for i, raw := range array(v.root["workflows"]) {
		if !v.check() {
			return
		}
		workflow := object(raw)
		path := joinPtr("/workflows", strconv.Itoa(i))
		scope := exprScope{workflowID: text(workflow["workflowId"])}
		v.checkOutputs(workflow["outputs"], joinPtr(path, "outputs"), scope)
		for j, raw := range array(workflow["parameters"]) {
			v.checkParameterExpressions(object(raw), joinPtr(joinPtr(path, "parameters"), strconv.Itoa(j)), scope)
		}
		for _, field := range []string{"successActions", "failureActions"} {
			v.checkActions(workflow[field], joinPtr(path, field), scope)
		}
		for j, raw := range array(workflow["steps"]) {
			step := object(raw)
			sp := joinPtr(joinPtr(path, "steps"), strconv.Itoa(j))
			ss := exprScope{scope.workflowID, text(step["stepId"])}
			v.checkOutputs(step["outputs"], joinPtr(sp, "outputs"), ss)
			v.checkCriteria(step["successCriteria"], joinPtr(sp, "successCriteria"), ss)
			for k, raw := range array(step["parameters"]) {
				v.checkParameterExpressions(object(raw), joinPtr(joinPtr(sp, "parameters"), strconv.Itoa(k)), ss)
			}
			for _, field := range []string{"onSuccess", "onFailure"} {
				v.checkActions(step[field], joinPtr(sp, field), ss)
			}
			if s, ok := step["correlationId"].(string); ok {
				v.checkExpressionString(s, joinPtr(sp, "correlationId"), ss, true)
			}
			v.checkRequestBody(object(step["requestBody"]), joinPtr(sp, "requestBody"), ss)
		}
	}
	components := object(v.root["components"])
	for _, name := range semanticKeys(object(components["parameters"])) {
		raw := object(components["parameters"])[name]
		v.checkParameterExpressions(object(raw), joinPtr("/components/parameters", name), exprScope{})
	}
	for _, kind := range []string{"successActions", "failureActions"} {
		for _, name := range semanticKeys(object(components[kind])) {
			raw := object(components[kind])[name]
			v.checkActionExpressions(object(raw), joinPtr(joinPtr("/components", kind), name), exprScope{})
		}
	}
}

func (v *validation) checkParameterExpressions(item map[string]any, path string, scope exprScope) {
	v.checkValue(item["value"], joinPtr(path, "value"), scope)
}

func (v *validation) checkActionExpressions(item map[string]any, path string, scope exprScope) {
	v.checkCriteria(item["criteria"], joinPtr(path, "criteria"), scope)
	for i, raw := range array(item["parameters"]) {
		v.checkParameterExpressions(object(raw), joinPtr(joinPtr(path, "parameters"), strconv.Itoa(i)), scope)
	}
}

func (v *validation) checkActions(raw any, path string, scope exprScope) {
	for i, item := range array(raw) {
		v.checkActionExpressions(object(item), joinPtr(path, strconv.Itoa(i)), scope)
	}
}

func (v *validation) checkOutputs(raw any, path string, scope exprScope) {
	values := object(raw)
	for _, name := range semanticKeys(values) {
		value := values[name]
		p := joinPtr(path, name)
		if s, ok := value.(string); ok {
			v.checkExpressionString(s, p, scope, true)
		} else if v.version == "1.1" {
			v.checkSelector(object(value), p, scope)
		}
	}
}

// Values are Any slots: literals remain literals. A full, exact Selector shape
// is the only unambiguous selector in these unions; extra members denote data.
func selectorValue(m map[string]any) bool {
	context, ok := m["context"].(string)
	if !ok || !strings.HasPrefix(context, "$") || context == "$" {
		return false
	}
	if _, ok := m["selector"].(string); !ok {
		return false
	}
	kind, _ := selectorDialect(m["type"], "")
	if kind != "jsonpath" && kind != "xpath" && kind != "jsonpointer" {
		return false
	}
	for key := range m {
		if key != "context" && key != "selector" && key != "type" && !strings.HasPrefix(key, "x-") {
			return false
		}
	}
	return true
}

func (v *validation) checkValue(value any, path string, scope exprScope) {
	if !v.work(1, path) {
		return
	}
	switch val := value.(type) {
	case string:
		if !v.workBytes(len(val), path) {
			return
		}
		v.checkExpressionString(val, path, scope, false)
	case map[string]any:
		if v.version == "1.1" && selectorValue(val) {
			v.checkSelector(val, path, scope)
			return
		}
		_, hasContext := val["context"]
		_, hasSelector := val["selector"]
		_, hasType := val["type"]
		for _, key := range semanticKeys(val) {
			item := val[key]
			// In an Any union a selector-looking literal remains ordinary data.
			// Its selector string must not become an Arazzo expression merely
			// because a JSONPath begins with '$'. Other payload values still
			// contain expressions according to the Request Body field contract.
			if hasContext && hasSelector && hasType && (key == "context" || key == "selector" || key == "type") {
				continue
			}
			v.checkValue(item, joinPtr(path, key), scope)
		}
	case []any:
		for i, item := range val {
			v.checkValue(item, joinPtr(path, strconv.Itoa(i)), scope)
		}
	}
}

func (v *validation) expressionVersion() expression.SpecVersion {
	if v.version == "1.0" {
		return expression.Arazzo10
	}
	return expression.Arazzo11
}

func (v *validation) recordExpression(s, path string, scope exprScope, condition bool) error {
	if !v.work(1, path) || !v.workBytes(len(s), path) {
		return v.err
	}
	if len(s) > v.opts.limits.MaxPatternBytes {
		v.err = &Error{Kind: ErrorLimit, Location: v.location(path), Cause: fmt.Errorf("expression exceeds %d bytes", v.opts.limits.MaxPatternBytes)}
		return v.err
	}
	parsed, err := expression.ParseWithVersion(s, v.expressionVersion())
	// The condition/context examples explicitly permit property dereference on
	// structured sources. Parse the source grammar before its dereference suffix.
	if err != nil && (condition || v.version == "1.1") {
		for _, base := range []string{"$request.body", "$response.body", "$request.payload", "$response.payload", "$message.payload", "$message.body"} {
			if strings.HasPrefix(s, base+".") {
				parsed, err = expression.ParseWithVersion(base, v.expressionVersion())
				if !simplePropertyTail(s[len(base):]) {
					err = fmt.Errorf("invalid property dereference")
				}
				break
			}
		}
	}
	if err != nil {
		return err
	}
	if v.version == "1.0" && (parsed.Type == expression.Self || parsed.Type == expression.RequestPayload || parsed.Type == expression.ResponsePayload || strings.HasPrefix(s, "$message.")) {
		return fmt.Errorf("runtime expression is not defined in Arazzo 1.0")
	}
	v.expressionUses = append(v.expressionUses, expressionUse{parsed, path, scope.workflowID, scope.stepID})
	return nil
}

func simplePropertyTail(s string) bool {
	for len(s) > 0 {
		if s[0] != '.' {
			return false
		}
		s = s[1:]
		i := 0
		for i < len(s) && (s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || s[i] >= '0' && s[i] <= '9' || s[i] == '_' || s[i] == '-') {
			i++
		}
		if i == 0 {
			return false
		}
		s = s[i:]
	}
	return true
}

func (v *validation) checkExpressionString(s, path string, scope exprScope, required bool) {
	if strings.HasPrefix(s, "$") && s != "$" {
		if err := v.recordExpression(s, path, scope, false); err != nil && v.err == nil {
			v.add(CodeExpression, path, err.Error())
		}
		return
	}
	if strings.Contains(s, "{$") {
		_, err := v.checkEmbedded(s, path, scope)
		if err != nil && v.err == nil {
			v.add(CodeExpression, path, err.Error())
		}
		return
	}
	if required {
		v.add(CodeExpression, path, "value must be a runtime expression")
	}
}

// Only {$...} blocks are parsed. Other braces can belong to literal JSON or a
// regex quantifier and must not be interpreted as expression delimiters.
func (v *validation) checkEmbedded(s, path string, scope exprScope) (bool, error) {
	found := false
	for offset := 0; offset < len(s); {
		start := strings.Index(s[offset:], "{$")
		if start < 0 {
			return found, nil
		}
		start += offset
		end := strings.IndexByte(s[start+2:], '}')
		if end < 0 {
			return true, fmt.Errorf("unclosed embedded runtime expression")
		}
		end += start + 2
		if err := v.recordExpression(s[start+1:end], path, scope, false); err != nil {
			return true, err
		}
		found = true
		offset = end + 1
	}
	return found, nil
}

func (v *validation) checkCriteria(raw any, path string, scope exprScope) {
	for i, item := range array(raw) {
		if !v.check() {
			return
		}
		c := object(item)
		p := joinPtr(path, strconv.Itoa(i))
		condition, ok := c["condition"].(string)
		if !ok {
			continue
		}
		if !v.work(1, p) || !v.workBytes(len(condition), p+"/condition") {
			return
		}
		kind, version := selectorDialect(c["type"], "simple")
		if context, ok := c["context"].(string); ok {
			if err := v.recordExpression(context, joinPtr(p, "context"), scope, true); err != nil && v.err == nil {
				v.add(CodeExpression, joinPtr(p, "context"), err.Error())
			}
		} else if _, specified := c["type"]; specified {
			v.add(CodeExpression, p, "context is required when criterion type is specified")
		}
		cp := joinPtr(p, "condition")
		if kind == "simple" {
			parser := simpleParser{input: condition, v: v, path: cp, scope: scope}
			if err := parser.parse(); err != nil && v.err == nil {
				v.add(CodeExpression, cp, err.Error())
			}
		} else {
			embedded, err := v.checkEmbedded(condition, cp, scope)
			if err != nil && v.err == nil {
				v.add(CodeExpression, cp, err.Error())
				continue
			}
			if embedded {
				v.incomplete("criterion-syntax", cp, "", "final condition syntax depends on runtime expression substitution")
				continue
			}
			v.checkDialectSyntax(kind, version, condition, cp, CodeExpression)
		}
	}
}

func selectorDialect(raw any, fallback string) (string, string) {
	kind := fallback
	version := ""
	switch val := raw.(type) {
	case string:
		kind = val
	case map[string]any:
		kind = text(val["type"])
		version = text(val["version"])
	}
	if version == "" {
		switch kind {
		case "jsonpath":
			version = "rfc9535"
		case "xpath":
			version = "xpath-31"
		case "jsonpointer":
			version = "rfc6901"
		}
	}
	return kind, version
}

func (v *validation) checkSelector(m map[string]any, path string, scope exprScope) {
	if s, ok := m["context"].(string); ok {
		if err := v.recordExpression(s, joinPtr(path, "context"), scope, true); err != nil && v.err == nil {
			v.add(CodeExpression, joinPtr(path, "context"), err.Error())
		}
	}
	if s, ok := m["selector"].(string); ok {
		kind, version := selectorDialect(m["type"], "")
		v.checkDialectSyntax(kind, version, s, joinPtr(path, "selector"), CodeSelector)
	}
}

func (v *validation) checkDialectSyntax(kind, version, s, path string, code Code) {
	if !v.work(1, path) || !v.workBytes(len(s), path) {
		return
	}
	if len(s) > v.opts.limits.MaxPatternBytes {
		v.err = &Error{Kind: ErrorLimit, Location: v.location(path), Cause: fmt.Errorf("pattern exceeds %d bytes", v.opts.limits.MaxPatternBytes)}
		return
	}
	var err error
	switch kind {
	case "regex":
		_, err = regexp2.Compile(s, regexp2.ECMAScript)
	case "jsonpath":
		if version != "rfc9535" {
			v.incomplete("selector-syntax", path, "", "JSONPath dialect "+version+" has no static parser")
			return
		}
		_, err = jsonpath.NewPath(s, jsonpathconfig.WithStrictRFC9535(), jsonpathconfig.WithLazyContextTracking())
	case "jsonpointer":
		err = validatePointer(s)
	case "xpath":
		v.incomplete("selector-syntax", path, "", "XPath dialect "+version+" has no static parser")
		return
	default:
		return // structural validation reports unknown dialects
	}
	if err != nil {
		v.add(code, path, "invalid "+kind+" syntax: "+err.Error())
	}
}

func validatePointer(s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("invalid UTF-8")
	}
	if s != "" && s[0] != '/' {
		return fmt.Errorf("JSON Pointer must be empty or start with '/'")
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '~' {
			if i+1 == len(s) || s[i+1] != '0' && s[i+1] != '1' {
				return fmt.Errorf("invalid JSON Pointer escape")
			}
			i++
		}
	}
	return nil
}

func (v *validation) checkRequestBody(body map[string]any, path string, scope exprScope) {
	v.checkValue(body["payload"], joinPtr(path, "payload"), scope)
	media, _, _ := mime.ParseMediaType(text(body["contentType"]))
	for i, raw := range array(body["replacements"]) {
		r := object(raw)
		p := joinPtr(joinPtr(path, "replacements"), strconv.Itoa(i))
		v.checkValue(r["value"], joinPtr(p, "value"), scope)
		target, ok := r["target"].(string)
		if !ok {
			continue
		}
		kind, version := selectorDialect(r["targetSelectorType"], "")
		if kind == "" {
			switch {
			case media == "application/json" || strings.HasSuffix(media, "+json"):
				kind = "jsonpointer"
				version = "rfc6901"
			case media == "application/xml" || media == "text/xml" || strings.HasSuffix(media, "+xml"):
				kind = "xpath"
				version = "xpath-31"
			default:
				v.incomplete("replacement-target", joinPtr(p, "target"), "", "replacement dialect requires the target operation content type")
				continue
			}
		}
		v.checkDialectSyntax(kind, version, target, joinPtr(p, "target"), CodeSelector)
	}
}

// simpleParser checks the literal/operator grammar without evaluating any data.
// Recursive nesting shares the document depth cap; expression tokens use the
// upstream versioned runtime-expression parser and the same symbol-use hook.
type simpleParser struct {
	input      string
	pos, depth int
	v          *validation
	path       string
	scope      exprScope
}

func (p *simpleParser) parse() error {
	if len(p.input) > p.v.opts.limits.MaxPatternBytes {
		p.v.err = &Error{Kind: ErrorLimit, Location: p.v.location(p.path), Cause: fmt.Errorf("condition exceeds pattern limit")}
		return p.v.err
	}
	if err := p.binary(1); err != nil {
		return err
	}
	p.space()
	if p.pos != len(p.input) {
		return fmt.Errorf("unexpected token at byte %d", p.pos)
	}
	return nil
}

func (p *simpleParser) space() {
	for p.pos < len(p.input) && strings.ContainsRune(" \t\r\n", rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *simpleParser) binary(min int) error {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > p.v.opts.limits.MaxDepth {
		p.v.err = &Error{Kind: ErrorLimit, Location: p.v.location(p.path), Cause: fmt.Errorf("condition nesting exceeds depth limit")}
		return p.v.err
	}
	if err := p.atom(); err != nil {
		return err
	}
	for {
		p.space()
		op, priority := p.operator()
		if priority < min {
			return nil
		}
		p.pos += len(op)
		if err := p.binary(priority + 1); err != nil {
			return err
		}
	}
}

func (p *simpleParser) operator() (string, int) {
	for _, o := range []struct {
		s string
		n int
	}{{"||", 1}, {"&&", 2}, {"==", 3}, {"!=", 3}, {"<=", 3}, {">=", 3}, {"<", 3}, {">", 3}} {
		if strings.HasPrefix(p.input[p.pos:], o.s) {
			return o.s, o.n
		}
	}
	return "", 0
}

func (p *simpleParser) atom() error {
	p.space()
	if p.pos == len(p.input) {
		return fmt.Errorf("missing condition operand")
	}
	start := p.pos
	switch p.input[p.pos] {
	case '!':
		p.pos++
		return p.binary(4)
	case '(':
		p.pos++
		if err := p.binary(1); err != nil {
			return err
		}
		p.space()
		if p.pos == len(p.input) || p.input[p.pos] != ')' {
			return fmt.Errorf("unclosed condition grouping")
		}
		p.pos++
	case '\'':
		p.pos++
		closed := false
		for p.pos < len(p.input) {
			if p.input[p.pos] == '\'' {
				p.pos++
				if p.pos < len(p.input) && p.input[p.pos] == '\'' {
					p.pos++
					continue
				}
				closed = true
				break
			}
			p.pos++
		}
		if !closed {
			return fmt.Errorf("unclosed string literal")
		}
	case '$':
		if prefix := simpleHeaderPrefix(p.input[start:]); prefix != "" {
			p.pos += len(prefix)
			// Header names consume the full RFC 7230 tchar token. Whitespace
			// separates logical operators from otherwise ambiguous header names.
			for p.pos < len(p.input) && simpleHeaderTchar(p.input[p.pos]) {
				if strings.HasPrefix(p.input[p.pos:], "!=") && !strings.HasPrefix(p.input[p.pos:], "!==") {
					break
				}
				p.pos++
			}
		} else {
			p.pos++
			pointer := false
			for p.pos < len(p.input) {
				c := p.input[p.pos]
				if strings.ContainsRune(" \t\r\n()", rune(c)) {
					break
				}
				if c == '#' {
					pointer = true
				}
				if !pointer && strings.ContainsRune("[]!<>=&|", rune(c)) {
					break
				}
				p.pos++
			}
		}
		if err := p.v.recordExpression(p.input[start:p.pos], p.path, p.scope, true); err != nil {
			return err
		}
	default:
		for p.pos < len(p.input) && !strings.ContainsRune(" \t\r\n()[]!<>=&|", rune(p.input[p.pos])) {
			p.pos++
		}
		token := p.input[start:p.pos]
		if token != "true" && token != "false" && token != "null" {
			var val any
			decoder := json.NewDecoder(strings.NewReader(token))
			decoder.UseNumber()
			if !json.Valid([]byte(token)) || decoder.Decode(&val) != nil {
				return fmt.Errorf("invalid condition literal at byte %d", start)
			}
			if _, ok := val.(json.Number); !ok {
				return fmt.Errorf("condition strings must use single quotes")
			}
		}
	}
	for {
		p.space()
		if p.pos == len(p.input) {
			return nil
		}
		switch p.input[p.pos] {
		case '[':
			p.pos++
			p.space()
			start = p.pos
			for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
				p.pos++
			}
			if p.pos == start {
				return fmt.Errorf("index must be a nonnegative integer")
			}
			p.space()
			if p.pos == len(p.input) || p.input[p.pos] != ']' {
				return fmt.Errorf("unclosed index")
			}
			p.pos++
		case '.':
			p.pos++
			start = p.pos
			for p.pos < len(p.input) && (p.input[p.pos] >= 'a' && p.input[p.pos] <= 'z' || p.input[p.pos] >= 'A' && p.input[p.pos] <= 'Z' || p.input[p.pos] >= '0' && p.input[p.pos] <= '9' || p.input[p.pos] == '_' || p.input[p.pos] == '-') {
				p.pos++
			}
			if p.pos == start {
				return fmt.Errorf("missing property name")
			}
		default:
			return nil
		}
	}
}

// simpleHeaderPrefix isolates header-name token rules from other runtime sources.
func simpleHeaderPrefix(s string) string {
	for _, prefix := range []string{"$request.header.", "$response.header.", "$message.header."} {
		if strings.HasPrefix(s, prefix) {
			return prefix
		}
	}
	return ""
}

func simpleHeaderTchar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c))
}
