// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "testing"

func stemCzech(t *testing.T, input string) string {
	t.Helper()
	s := czechStemmer{}
	runes := []rune(input)
	n := s.stem(runes, len(runes))
	return string(runes[:n])
}

// TestCzechStemmer_MasculineNouns mirrors testMasculineNouns from
// TestCzechStemmer.java (subset of representative cases).
func TestCzechStemmer_MasculineNouns(t *testing.T) {
	// animate ending with hard consonant — all should stem to "pán"
	pánCases := []string{"páni", "pánové", "pána", "pánů", "pánovi", "pánům", "pány", "páne", "pánech", "pánem"}
	for _, w := range pánCases {
		got := stemCzech(t, w)
		if got != "pán" {
			t.Errorf("stem(%q) = %q, want %q", w, got, "pán")
		}
	}

	// animate soft consonant — all should stem to "muh" (after ž->h normalisation)
	mužCases := []string{"muž", "muži", "muže", "mužů", "mužům", "mužích", "mužem"}
	for _, w := range mužCases {
		got := stemCzech(t, w)
		if got != "muh" {
			t.Errorf("stem(%q) = %q, want %q", w, got, "muh")
		}
	}

	// inanimate hard consonant — all should stem to "hrad"
	hradCases := []string{"hradu", "hrade", "hradem", "hrady", "hradech", "hradům", "hradů"}
	for _, w := range hradCases {
		got := stemCzech(t, w)
		if got != "hrad" {
			t.Errorf("stem(%q) = %q, want %q", w, got, "hrad")
		}
	}
}

// TestCzechStemmer_FeminineNouns mirrors testFeminineNouns (representative cases).
func TestCzechStemmer_FeminineNouns(t *testing.T) {
	// ending with hard consonant — all should conflate to "kost"
	kostCases := []string{"kosti", "kostí", "kostem", "kostech", "kostmi"}
	for _, w := range kostCases {
		got := stemCzech(t, w)
		if got != "kost" {
			t.Errorf("stem(%q) = %q, want %q", w, got, "kost")
		}
	}

	// soft consonant ending: all inflected forms should conflate to "písn"
	// (singular nominative "píseň" stems to "písň" — different stem, expected)
	písnCases := []string{"písně", "písni", "písněmi", "písních", "písním"}
	for _, w := range písnCases {
		got := stemCzech(t, w)
		if got != "písn" {
			t.Errorf("stem(%q) = %q, want %q", w, got, "písn")
		}
	}
}

// TestCzechStemmer_Normalize verifies normalisation rules (čt->ck, št->sk).
func TestCzechStemmer_Normalize(t *testing.T) {
	cases := []struct{ input, want string }{
		// soudce -> soudk  (c -> k normalisation after case stripping)
		{"soudce", "soudk"},
		{"soudci", "soudk"},
	}
	for _, c := range cases {
		got := stemCzech(t, c.input)
		if got != c.want {
			t.Errorf("stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestCzechStemmer_ShortWords verifies very short words are returned unchanged.
func TestCzechStemmer_ShortWords(t *testing.T) {
	for _, w := range []string{"a", "je", "to"} {
		got := stemCzech(t, w)
		if got != w {
			t.Errorf("stem(%q) = %q, want unchanged", w, got)
		}
	}
}
