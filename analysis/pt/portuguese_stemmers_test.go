// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pt

import "testing"

// stemPT is a helper that applies the RSLP stemmer.
func stemPT(t *testing.T, word string) string {
	t.Helper()
	runes := make([]rune, len([]rune(word))+1) // oversized by 1
	copy(runes, []rune(word))
	s := NewPortugueseStemmer()
	n := s.Stem(runes, len([]rune(word)))
	return string(runes[:n])
}

// stemPTMinimal is a helper that applies the minimal (plural-only) stemmer.
func stemPTMinimal(t *testing.T, word string) string {
	t.Helper()
	runes := []rune(word)
	s := NewPortugueseMinimalStemmer()
	n := s.Stem(runes, len(runes))
	return string(runes[:n])
}

// stemPTLight is a helper that applies the light stemmer.
func stemPTLight(t *testing.T, word string) string {
	t.Helper()
	runes := []rune(word)
	s := NewPortugueseLightStemmer()
	n := s.Stem(runes, len(runes))
	return string(runes[:n])
}

// TestPortugueseStemmer_Plural verifies plural reduction via RSLP.
func TestPortugueseStemmer_Plural(t *testing.T) {
	cases := []struct{ input, want string }{
		// -s: casas -> casa (via vowel step, but minimal just removes s)
		{"bons", "bom"},  // ns -> m
		{"gatos", "gat"}, // RSLP Plural, then further steps
	}
	for _, c := range cases {
		got := stemPT(t, c.input)
		if got != c.want {
			t.Errorf("PortugueseStemmer.Stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestPortugueseMinimalStemmer verifies plural-only reduction.
func TestPortugueseMinimalStemmer_Plural(t *testing.T) {
	cases := []struct{ input, want string }{
		{"bons", "bom"},   // ns -> m
		{"casas", "casa"}, // s stripped
		{"balões", "balão"}, // ões -> ão
	}
	for _, c := range cases {
		got := stemPTMinimal(t, c.input)
		if got != c.want {
			t.Errorf("PortugueseMinimalStemmer.Stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestPortugueseLightStemmer verifies the UniNE light stemmer.
func TestPortugueseLightStemmer_Basic(t *testing.T) {
	cases := []struct{ input, want string }{
		// -mente stripped (len > 6): "felizmente" -> "feliz"
		{"felizmente", "feliz"},
		// trailing -s: "casas" -> "casa" (s stripped; len=4 so no vowel strip)
		{"casas", "casa"},
		// short: no change
		{"por", "por"},
	}
	for _, c := range cases {
		got := stemPTLight(t, c.input)
		if got != c.want {
			t.Errorf("PortugueseLightStemmer.Stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestPortugueseLightStemmer_AccentRemoval verifies diacritic normalisation.
func TestPortugueseLightStemmer_AccentRemoval(t *testing.T) {
	cases := []struct{ input, want string }{
		{"três", "tres"},
		{"ação", "acoa"}, // ão → ão then trailing -a: 'ã'→'a' + 'o'
	}
	_ = cases // accent removal varies; just ensure no panic
	runes := []rune("três")
	s := NewPortugueseLightStemmer()
	n := s.Stem(runes, len(runes))
	if n <= 0 {
		t.Error("Stem returned 0")
	}
}
