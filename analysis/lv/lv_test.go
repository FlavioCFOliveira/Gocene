// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lv

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// stemRunes is a helper that stems the given string using LatvianStemmer.
func stemRunes(st *LatvianStemmer, term string) string {
	runes := []rune(term)
	// Extra capacity for unpalatalize kš→kst expansion.
	buf := make([]rune, len(runes)+4)
	copy(buf, runes)
	n := st.Stem(buf, len(runes))
	return string(buf[:n])
}

// collectTokens runs a whitespace-tokenized pipeline through LatvianStemFilter.
func collectTokens(t *testing.T, text string) []string {
	t.Helper()
	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader(text)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	f := NewLatvianStemFilter(tok)
	defer f.Close()

	src := f.GetAttributeSource()
	var termAttr analysis.CharTermAttribute
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		termAttr = a.(analysis.CharTermAttribute)
	}

	var terms []string
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if termAttr != nil {
			terms = append(terms, termAttr.String())
		}
	}
	return terms
}

// checkOneTerm verifies that a single term stems to the expected value.
func checkOneTerm(t *testing.T, input, want string) {
	t.Helper()
	st := &LatvianStemmer{}
	got := stemRunes(st, input)
	if got != want {
		t.Errorf("Stem(%q) = %q; want %q", input, got, want)
	}
}

// ---------------------------------------------------------------------------
// TestLatvianStemmer — declension I nouns
// Source: TestLatvianStemmer.testNouns1
// ---------------------------------------------------------------------------

func TestLatvianStemmer_DeclI(t *testing.T) {
	cases := [][2]string{
		{"tēvs", "tēv"},
		{"tēvi", "tēv"},
		{"tēva", "tēv"},
		{"tēvam", "tēv"},
		{"tēvā", "tēv"},
	}
	for _, tc := range cases {
		t.Run(tc[0], func(t *testing.T) {
			checkOneTerm(t, tc[0], tc[1])
		})
	}
}

// ---------------------------------------------------------------------------
// TestLatvianStemmer — declension II nouns (palatalization)
// Source: TestLatvianStemmer.testNouns2
// ---------------------------------------------------------------------------

func TestLatvianStemmer_DeclII_Palatalization(t *testing.T) {
	cases := [][2]string{
		{"lācis", "lāc"},
		{"lāči", "lāc"},
		{"lāča", "lāc"},
		{"lāčiem", "lāc"},
	}
	for _, tc := range cases {
		t.Run(tc[0], func(t *testing.T) {
			checkOneTerm(t, tc[0], tc[1])
		})
	}
}

// ---------------------------------------------------------------------------
// TestLatvianStemFilter — filter correctly passes short words
// ---------------------------------------------------------------------------

func TestLatvianStemFilter_ShortWords(t *testing.T) {
	got := collectTokens(t, "es tu")
	if len(got) != 2 {
		t.Fatalf("expected 2 tokens, got %v", got)
	}
	// Very short words may or may not be stemmed; just verify they pass through.
	for _, tok := range got {
		if len(tok) == 0 {
			t.Errorf("empty token in output")
		}
	}
}

// ---------------------------------------------------------------------------
// TestLatvianStemFilterFactory — factory produces a working filter
// ---------------------------------------------------------------------------

func TestLatvianStemFilterFactory_Create(t *testing.T) {
	f := NewLatvianStemFilterFactory()
	tok := analysis.NewWhitespaceTokenizer()
	_ = tok.SetReader(strings.NewReader("tēvs"))
	filter := f.Create(tok)
	defer filter.Close()

	// Verify we can call IncrementToken without panic.
	ok, err := filter.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken: %v", err)
	}
	if !ok {
		t.Fatal("expected at least one token")
	}
}
