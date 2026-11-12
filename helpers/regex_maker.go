package helpers

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	baseDefaultPattern        = "[^/]*"
	DefaultPatternRegex       = regexp.MustCompile("^([^/]*)$")
	DefaultPatternRegexString = DefaultPatternRegex.String()
)

// regexCache stores compiled regular expressions by path template.
//
// Request validation calls GetRegexForPath for each path template on every
// request. The key set is the route table of the document, which is small and
// stable for the life of the process, so the cache holds one entry per unique
// template. Each entry holds a few KB (key string plus compiled regex), so a
// normal route table costs a few MB at most. Callers that pass an unbounded
// number of unique templates grow the cache without limit.
//
// ClearRegexCache empties the cache. Call it between document lifecycles in
// long-running processes to bound memory.
var regexCache sync.Map // map[string]*regexp.Regexp

// ClearRegexCache removes every cached compiled regex. The next call for a
// template compiles it again. Safe to call while other goroutines call
// GetRegexForPath.
func ClearRegexCache() {
	regexCache.Clear()
}

// GetRegexForPath returns a compiled regular expression for the given path template.
//
// This function takes a path template string `tpl` and generates a regular expression
// that matches the structure of the template. The template can include placeholders
// enclosed in braces `{}` with optional custom patterns.
//
// Placeholders in the template can be defined as:
//   - `{name}`: Matches any sequence of characters except '/'
//   - `{name:pattern}`: Matches the specified custom pattern
//
// The function ensures that the template is well-formed, with balanced and properly
// nested braces. If the template is invalid, an error is returned.
//
// Compiled regexes are cached by template string. The first call for a template
// compiles it; later calls return the cached regex. The cache is safe for
// concurrent use. Invalid templates are not cached. ClearRegexCache empties
// the cache.
//
// Parameters:
//   - tpl: The path template string to convert into a regular expression.
//
// Returns:
//   - *regexp.Regexp: A compiled regular expression that matches the template.
//   - error: An error if the template is invalid or the regular expression cannot be compiled.
//
// Example:
//
//	regex, err := GetRegexForPath("/orders/{id:[0-9]+}/items/{itemId}")
//	// regex: ^/orders/([0-9]+)/items/([^/]+)$
//	// err: nil
func GetRegexForPath(tpl string) (*regexp.Regexp, error) {
	// Return the cached regex for this template when one exists.
	if cached, ok := regexCache.Load(tpl); ok {
		return cached.(pathRegexEntry).regex, nil
	}

	entry, errCompile := compilePathRegexEntry(tpl)
	if errCompile != nil {
		return nil, errCompile
	}

	// A concurrent call for the same template may have stored its entry first.
	// LoadOrStore returns the stored winner; this call discards its own copy.
	actual, _ := regexCache.LoadOrStore(tpl, entry)
	return actual.(pathRegexEntry).regex, nil
}

// GetBraceIndicesForPath returns the brace indices of the given path template,
// the same values BraceIndices computes. The indices come from the compile
// cache, so repeated calls for one template do not re-scan it. Callers must
// treat the returned slice as read-only. An invalid template returns an error.
func GetBraceIndicesForPath(tpl string) ([]int, error) {
	if cached, ok := regexCache.Load(tpl); ok {
		return cached.(pathRegexEntry).braceIdxs, nil
	}

	entry, errCompile := compilePathRegexEntry(tpl)
	if errCompile != nil {
		return nil, errCompile
	}

	actual, _ := regexCache.LoadOrStore(tpl, entry)
	return actual.(pathRegexEntry).braceIdxs, nil
}

// pathRegexEntry holds one compile result for a path template: the compiled
// regex and the brace indices of the template placeholders.
type pathRegexEntry struct {
	regex     *regexp.Regexp
	braceIdxs []int
}

// compilePathRegexEntry builds and compiles a regular expression for the given
// path template. It holds the original, uncached implementation.
func compilePathRegexEntry(tpl string) (pathRegexEntry, error) {
	// Check if it is well-formed.
	idxs, errBraces := BraceIndices(tpl)
	if errBraces != nil {
		return pathRegexEntry{}, errBraces
	}

	// Backup the original.
	template := tpl

	// bytes.Buffer and strings.Builder writes cannot fail: the buffer grows
	// on demand and panics on overflow instead of returning an error, so the
	// pattern is assembled with plain writes and no error checks.
	var pattern strings.Builder
	pattern.WriteString("^")
	var end int

	for i := 0; i < len(idxs); i += 2 {

		// Set all values we are interested in.
		raw := tpl[end:idxs[i]]
		end = idxs[i+1]
		parts := strings.SplitN(tpl[idxs[i]+1:end-1], ":", 2)
		name := parts[0]
		patt := baseDefaultPattern
		if len(parts) == 2 {
			patt = parts[1]
		}

		// Name or pattern can't be empty.
		if name == "" || patt == "" {
			return pathRegexEntry{}, fmt.Errorf("missing name or pattern in %q", tpl[idxs[i]:end])
		}

		// Build the regexp pattern.
		pattern.WriteString(regexp.QuoteMeta(raw))
		pattern.WriteByte('(')
		pattern.WriteString(patt)
		pattern.WriteByte(')')

	}

	// Add the remaining.
	raw := tpl[end:]
	pattern.WriteString(regexp.QuoteMeta(raw))

	pattern.WriteByte('$')

	patternString := pattern.String()

	if patternString == DefaultPatternRegexString {
		return pathRegexEntry{regex: DefaultPatternRegex, braceIdxs: idxs}, nil
	}

	// Compile full regexp.
	reg, errCompile := regexp.Compile(patternString)
	if errCompile != nil {
		return pathRegexEntry{}, errCompile
	}

	// Check for capturing groups which used to work in older versions
	if reg.NumSubexp() != len(idxs)/2 {
		return pathRegexEntry{}, fmt.Errorf("route %s contains capture groups in its regexp. Only non-capturing groups are accepted: e.g. (?:pattern) instead of (pattern)", template)
	}

	// Done!
	return pathRegexEntry{regex: reg, braceIdxs: idxs}, nil
}

// BraceIndices returns the indices of the opening and closing braces in a string.
//
// It scans the input string `s` and identifies the positions of matching pairs
// of braces ('{' and '}'). The function ensures that the braces are balanced
// and properly nested.
//
// If the braces are unbalanced or improperly nested, an error is returned.
//
// Parameters:
//   - s: The input string to scan for braces.
//
// Returns:
//   - []int: A slice of integers where each pair of indices represents the
//     start and end positions of a matching pair of braces.
//   - error: An error if the braces are unbalanced or improperly nested.
//
// Example:
//
//	indices, err := BraceIndices("/orders/{id}/items/{itemId}")
//	// indices: [8, 12, 19, 26]
//	// err: nil
func BraceIndices(s string) ([]int, error) {
	var level, idx int
	var idxs []int
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			if level++; level == 1 {
				idx = i
			}
		case '}':
			if level--; level == 0 {
				idxs = append(idxs, idx, i+1)
			} else if level < 0 {
				return nil, fmt.Errorf("unbalanced braces in %q", s)
			}
		}
	}
	if level != 0 {
		return nil, fmt.Errorf("unbalanced braces in %q", s)
	}
	return idxs, nil
}
