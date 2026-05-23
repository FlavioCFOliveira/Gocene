// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/lt/TestLithuanianStemming.java

package snowball

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/snowball/ext"
)

// stemLithuanian passes word through a WhitespaceTokenizer → SnowballFilter
// with LithuanianStemmer and returns the stem.
func stemLithuanian(t *testing.T, word string) string {
	t.Helper()
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(word))
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	f := NewSnowballFilter(tokenizer, ext.NewLithuanianStemmer())
	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken: %v", err)
	}
	if !ok {
		t.Fatal("no token")
	}
	attr := f.GetAttribute("CharTermAttribute")
	if attr == nil {
		t.Fatal("CharTermAttribute not found")
	}
	return attr.(analysis.CharTermAttribute).String()
}

// checkOneTerm verifies that word stems to expected.
func checkOneTerm(t *testing.T, word, expected string) {
	t.Helper()
	got := stemLithuanian(t, word)
	if got != expected {
		t.Errorf("stem(%q) = %q, want %q", word, got, expected)
	}
}

// TestLithuanianStemming_NounsI tests declension I noun stems.
// Source: TestLithuanianStemming.testNounsI
func TestLithuanianStemming_NounsI(t *testing.T) {
	// -as declension
	checkOneTerm(t, "vaikas", "vaik")   // nom. sing.
	checkOneTerm(t, "vaikai", "vaik")   // nom. pl.
	checkOneTerm(t, "vaiko", "vaik")    // gen. sg.
	checkOneTerm(t, "vaikų", "vaik")    // gen. pl.
	checkOneTerm(t, "vaikui", "vaik")   // dat. sg.
	checkOneTerm(t, "vaikams", "vaik")  // dat. pl.
	checkOneTerm(t, "vaiką", "vaik")    // acc. sg.
	checkOneTerm(t, "vaikus", "vaik")   // acc. pl.
	checkOneTerm(t, "vaiku", "vaik")    // ins. sg.
	checkOneTerm(t, "vaikais", "vaik")  // ins. pl.
	checkOneTerm(t, "vaike", "vaik")    // loc. sg.
	checkOneTerm(t, "vaikuose", "vaik") // loc. pl.
}

// TestLithuanianStemming_NounsII tests declension II noun stems.
// Source: TestLithuanianStemming.testNounsII
func TestLithuanianStemming_NounsII(t *testing.T) {
	// -a declension
	checkOneTerm(t, "motina", "motin")   // nom. sing.
	checkOneTerm(t, "motinos", "motin")  // nom. pl.
	checkOneTerm(t, "motinų", "motin")   // gen. pl.
	checkOneTerm(t, "motinai", "motin")  // dat. sg.
	checkOneTerm(t, "motinoms", "motin") // dat. pl.
	checkOneTerm(t, "motiną", "motin")   // acc. sg.
	checkOneTerm(t, "motinas", "motin")  // acc. pl.
}

// TestLithuanianStemming_AdjI tests adjective declension I stems.
// Source: TestLithuanianStemming.testAdjI
func TestLithuanianStemming_AdjI(t *testing.T) {
	checkOneTerm(t, "geras", "ger")   // nom. sg. masc.
	checkOneTerm(t, "geri", "ger")    // nom. pl. masc.
	checkOneTerm(t, "gero", "ger")    // gen. sg. masc.
	checkOneTerm(t, "gerų", "ger")    // gen. pl. masc.
	checkOneTerm(t, "gera", "ger")    // nom. sg. fem.
	checkOneTerm(t, "geros", "ger")   // nom. pl. fem.
	checkOneTerm(t, "gerą", "ger")    // acc. sg. fem.
	checkOneTerm(t, "geras", "ger")   // acc. pl. fem.
	checkOneTerm(t, "geroje", "ger")  // loc. sg. fem.
	checkOneTerm(t, "gerose", "ger")  // loc. pl. fem.
}

// TestLithuanianStemming_HighFrequency tests high-frequency Lithuanian terms.
// Source: TestLithuanianStemming.testHighFrequencyTerms
func TestLithuanianStemming_HighFrequency(t *testing.T) {
	// Common Lithuanian function words and frequency terms should stem reasonably.
	tests := []struct{ in, want string }{
		{"ir", "ir"},     // "and" — function word, no stemming
		{"kad", "kad"},   // "that" — function word
		{"tai", "tai"},   // "this/that" — demonstrative
		{"kaip", "kaip"}, // "how" — function word
	}
	for _, tc := range tests {
		checkOneTerm(t, tc.in, tc.want)
	}
}
