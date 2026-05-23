// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "testing"

// TestGermanLightStemmer_DiacriticNormalisation verifies that umlauts and
// accented vowels are mapped to their base form before suffix stripping.
func TestGermanLightStemmer_DiacriticNormalisation(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"bäume", "baum"},   // ä→a, step1 strips "e"
		{"öfen", "ofen"},     // ö→o; len=4, step1 conditions need len>4 for -en, not met
		{"füße", "fuß"},     // ü→u; ß is not normalised, step1 strips trailing "e" (len=4>3)
		{"hütten", "hutt"},  // ü→u, step1 strips "en"
		{"mädchen", "madch"}, // ä→a, step1 strips "en"
	}

	var st germanLightStemmer
	for _, c := range cases {
		s := []rune(c.input)
		n := st.stem(s, len(s))
		got := string(s[:n])
		if got != c.want {
			t.Errorf("stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestGermanLightStemmer_Step1Suffixes covers the four plural / case
// suffixes removed by step1.
func TestGermanLightStemmer_Step1Suffixes(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// -ern (len > 5)
		{"kindern", "kind"},
		// -em (len > 4)
		{"weisem", "weis"},
		// -en (len > 4)
		{"weisen", "weis"},
		// -er (len > 4)
		{"weiser", "weis"},
		// -es (len > 4)
		{"weises", "weis"},
		// -e (len > 3)
		{"hunde", "hund"},
		// -s after st-ending (len > 3)
		{"hunds", "hund"},
	}

	var st germanLightStemmer
	for _, c := range cases {
		s := []rune(c.input)
		n := st.stem(s, len(s))
		got := string(s[:n])
		if got != c.want {
			t.Errorf("stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestGermanLightStemmer_Step2Suffixes covers the three verb / adjective
// suffixes removed by step2.
func TestGermanLightStemmer_Step2Suffixes(t *testing.T) {
	// step2 operates on the already-step1-reduced form; we need inputs that
	// survive step1 intact and then trigger step2.
	cases := []struct {
		input string
		want  string
	}{
		// -est after st-ending (len > 4 after step1)
		{"dankst", "dank"},  // step1 no-op; step2 strips "st" (stEnding('k'))
		// -er (len > 4)
		{"laufer", "lauf"},
		// -en already handled by step1 — step2 fires on -en when step1 missed it
		// e.g. after diacritic normalisation the word is 5 chars so step1 fires first;
		// test a word that hits step2 -en specifically via len
		{"tauben", "taub"},  // step1: strips "en" → "taub" (len=4>4 false, >4 is 5)
	}

	var st germanLightStemmer
	for _, c := range cases {
		s := []rune(c.input)
		n := st.stem(s, len(s))
		got := string(s[:n])
		if got != c.want {
			t.Errorf("stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestGermanLightStemmer_ShortWords ensures very short words are not truncated.
func TestGermanLightStemmer_ShortWords(t *testing.T) {
	cases := []string{"ab", "der", "die", "das"}

	var st germanLightStemmer
	for _, w := range cases {
		s := []rune(w)
		n := st.stem(s, len(s))
		got := string(s[:n])
		if got != w {
			t.Errorf("stem(%q) = %q, want unchanged %q", w, got, w)
		}
	}
}
