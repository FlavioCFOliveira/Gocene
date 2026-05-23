// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package in_test

import (
	"testing"

	indicpkg "github.com/FlavioCFOliveira/Gocene/analysis/in"
)

func normalize(t *testing.T, input string) string {
	t.Helper()
	n := indicpkg.NewIndicNormalizer()
	runes := []rune(input)
	l := n.Normalize(runes, len(runes))
	return string(runes[:l])
}

// TestIndicNormalizer_Basics mirrors the Java testBasics().
func TestIndicNormalizer_Basics(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"अाॅअाॅ", "ऑऑ"},
		{"अाॆअाॆ", "ऒऒ"},
		{"अाेअाे", "ओओ"},
		{"अाैअाै", "औऔ"},
		{"अाअा", "आआ"},
		{"अाैर", "और"},
		// Bengali khanda-ta: ত + ্ + ZWJ → ৎ
		{"ত্‍", "ৎ"},
	}
	for _, c := range cases {
		got := normalize(t, c.input)
		if got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestIndicNormalizer_Identity verifies that already-normalised text is unchanged.
func TestIndicNormalizer_Identity(t *testing.T) {
	words := []string{"hello", "world", ""}
	for _, w := range words {
		got := normalize(t, w)
		if got != w {
			t.Errorf("normalize(%q) = %q, want unchanged", w, got)
		}
	}
}
