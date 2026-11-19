package helpers

import (
	"testing"

	"github.com/pb33f/testify/assert"
)

func TestGetRegexForPath(t *testing.T) {
	tests := []struct {
		name     string
		tpl      string
		wantErr  bool
		wantExpr string
	}{
		{
			name:     "well-formed template with default pattern",
			tpl:      "/orders/{id}",
			wantErr:  false,
			wantExpr: "^/orders/([^/]*)$",
		},
		{
			name:     "well-formed template with custom pattern",
			tpl:      "/orders/{id:[0-9]+}",
			wantErr:  false,
			wantExpr: "^/orders/([0-9]+)$",
		},
		{
			name:    "missing name in template",
			tpl:     "/orders/{:pattern}",
			wantErr: true,
		},
		{
			name:    "missing pattern in template",
			tpl:     "/orders/{name:}",
			wantErr: true,
		},
		{
			name:    "unbalanced braces in template",
			tpl:     "/orders/{id",
			wantErr: true,
		},
		{
			name:    "unbalanced braces in template",
			tpl:     "/orders/id}",
			wantErr: true,
		},
		{
			name:     "template with multiple variables",
			tpl:      "/orders/{id:[0-9]+}/items/{itemId}",
			wantErr:  false,
			wantExpr: "^/orders/([0-9]+)/items/([^/]*)$",
		},
		{
			name:     "OData formatted URL with single quotes",
			tpl:      "/entities('{id}')",
			wantErr:  false,
			wantExpr: "^/entities\\('([^/]*)'\\)$",
		},
		{
			name:     "OData formatted URL with custom pattern",
			tpl:      "/entities('{id:[0-9]+}')",
			wantErr:  false,
			wantExpr: "^/entities\\('([0-9]+)'\\)$",
		},
		{
			name:     "get default pattern",
			tpl:      "/{param}",
			wantErr:  false,
			wantExpr: "^/([^/]*)$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetRegexForPath(tt.tpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRegexForPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.wantExpr {
				t.Errorf("GetRegexForPath() = %v, want %v", got.String(), tt.wantExpr)
			}
		})
	}
}

func TestBraceIndices(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    []int
		wantErr bool
	}{
		{
			name:    "well-formed braces",
			s:       "/orders/{id}/items/{itemId}",
			want:    []int{8, 12, 19, 27},
			wantErr: false,
		},
		{
			name:    "unbalanced braces",
			s:       "/orders/{id/items/{itemId}",
			wantErr: true,
		},
		{
			name:    "unbalanced braces",
			s:       "/orders/{id}/items/{itemId",
			wantErr: true,
		},
		{
			name:    "no braces",
			s:       "/orders/id/items/itemId",
			want:    []int{},
			wantErr: false,
		},
		{
			name:    "OData formatted URL with single quotes",
			s:       "/entities('{id}')",
			want:    []int{11, 15},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BraceIndices(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("BraceIndices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !equal(got, tt.want) {
				t.Errorf("BraceIndices() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultPatternCompileCache(t *testing.T) {
	res, err := GetRegexForPath("{param}")

	assert.Nil(t, err)
	assert.Equal(t, res, DefaultPatternRegex)
	assert.Equal(t, res.String(), DefaultPatternRegexString)
}

// TestGetRegexForPathHoldsNoState pins the stateless contract: repeated
// calls for one template each compile fresh regexes. Validators cache
// compiled regexes per validator in config.ValidationOptions.RegexCache.
func TestGetRegexForPathHoldsNoState(t *testing.T) {
	first, errFirst := GetRegexForPath("/stateless/{id}")
	second, errSecond := GetRegexForPath("/stateless/{id}")

	assert.NoError(t, errFirst)
	assert.NoError(t, errSecond)
	assert.NotSame(t, first, second, "expected a fresh compile per call")
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGetRegexForPathInvalidCustomPattern(t *testing.T) {
	// an unterminated character class fails regexp compilation
	_, err := GetRegexForPath("/x/{id:[a-z}")
	assert.Error(t, err)
}

func TestGetRegexForPathCapturingGroupRejected(t *testing.T) {
	// capturing groups in custom patterns are rejected
	_, err := GetRegexForPath("/x/{id:(abc)}")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "capture groups")
}
