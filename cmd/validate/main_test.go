// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package main

import "testing"

func TestRegexEngineOptions(t *testing.T) {
	for name, option := range regexParsingOptionsMap {
		t.Run(name, func(t *testing.T) {
			engine := &regexEngine{runtimeOption: option}
			re, err := engine.run("abc")
			if err != nil {
				t.Fatal(err)
			}
			if re.String() != "abc" || !re.MatchString("abc") || re.MatchString("xyz") {
				t.Fatal("regex option changed matching behavior")
			}
			if _, err := engine.run("["); err == nil {
				t.Fatal("invalid regex compiled")
			}
		})
	}
}

func TestRegexEngineZeroWidthRepeatLimit(t *testing.T) {
	engine := &regexEngine{runtimeOption: regexParsingOptionsMap["ecmascript"]}
	re, err := engine.run(`(?:^){60000}`)
	if err != nil {
		t.Fatal(err)
	}
	if re.MatchString("") {
		t.Fatal("excessive zero-width repetition must fail closed")
	}
	empty, err := engine.run(`(?:){60000}`)
	if err != nil {
		t.Fatal(err)
	}
	if !empty.MatchString("") {
		t.Fatal("literal empty repetition must remain valid")
	}
}
