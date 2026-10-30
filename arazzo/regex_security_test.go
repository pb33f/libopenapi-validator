// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"errors"
	"testing"

	"github.com/dlclark/regexp2/v2"
)

func TestInputRegexZeroWidthRepeatLimit(t *testing.T) {
	// 60,000 repetitions exceed the default 100,000-slot backtracking limit
	// while keeping this regression safe to run against the old engine.
	for _, pattern := range []string{`(?:^){60000}`, `(?:^){60000}?`} {
		t.Run(pattern, func(t *testing.T) {
			v := contractValidation(nil)
			state := &inputRegexpState{v: v}
			re, err := state.compile(pattern)
			if err != nil {
				t.Fatal(err)
			}
			if re.MatchString("") || !errors.Is(state.err, regexp2.ErrBacktrackingStackLimit) {
				t.Fatalf("expected bounded match failure, got %v", state.err)
			}
			reportInputCompile(v, state, state.err, "/workflows/0/inputs")
			var tool *Error
			if !errors.As(v.err, &tool) || tool.Kind != ErrorOperational || !errors.Is(v.err, regexp2.ErrBacktrackingStackLimit) {
				t.Fatalf("regex limit cause was lost: %v", v.err)
			}
		})
	}
}

func TestInputRegexEmptyRepeat(t *testing.T) {
	state := &inputRegexpState{v: contractValidation(nil)}
	re, err := state.compile(`(?:){60000}`)
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("") || state.err != nil {
		t.Fatalf("literal empty repetition must remain valid: %v", state.err)
	}
}
