package helpers

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func GetRegexForPath(tpl string) (*regexp.Regexp, error) {

	// Check if it is well-formed.
	idxs, errBraces := BraceIndices(tpl)
	if errBraces != nil {
		return nil, errBraces
	}

	// Backup the original.
	template := tpl

	// Now let's parse it.
	defaultPattern := "[^/]+"

	pattern := bytes.NewBufferString("^")
	var end int

	for i := 0; i < len(idxs); i += 2 {

		// Set all values we are interested in.
		raw := tpl[end:idxs[i]]
		end = idxs[i+1]
		parts := strings.SplitN(tpl[idxs[i]+1:end-1], ":", 2)
		name := parts[0]
		patt := defaultPattern
		if len(parts) == 2 {
			patt = parts[1]
		}

		// Name or pattern can't be empty.
		if name == "" || patt == "" {
			return nil, fmt.Errorf("mux: missing name or pattern in %q", tpl[idxs[i]:end])
		}

		// Build the regexp pattern.
		_, err := fmt.Fprintf(pattern, "%s(%s)", regexp.QuoteMeta(raw), patt)
		if err != nil {
			return nil, err
		}

	}

	// Add the remaining.
	raw := tpl[end:]
	pattern.WriteString(regexp.QuoteMeta(raw))

	pattern.WriteByte('$')

	// Compile full regexp.
	reg, errCompile := regexp.Compile(pattern.String())
	if errCompile != nil {
		return nil, errCompile
	}

	// Check for capturing groups which used to work in older versions
	if reg.NumSubexp() != len(idxs)/2 {
		return nil, fmt.Errorf(fmt.Sprintf("route %s contains capture groups in its regexp. ", template) + "Only non-capturing groups are accepted: e.g. (?:pattern) instead of (pattern)")
	}

	// Done!
	return reg, nil
}

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
				return nil, fmt.Errorf("mux: unbalanced braces in %q", s)
			}
		}
	}
	if level != 0 {
		return nil, fmt.Errorf("mux: unbalanced braces in %q", s)
	}
	return idxs, nil
}
