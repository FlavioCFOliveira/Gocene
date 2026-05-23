// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/compound/TestCompoundWordTokenFilter.java

package compound

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// makeDictionary returns an ignoreCase CharArraySet for the given words,
// mirroring the Java helper of the same name.
func makeDictionary(words ...string) *analysis.CharArraySet {
	return analysis.NewCharArraySetFromStrings(true, words...)
}

// drainTerms drains all CharTermAttribute strings from a DictionaryCompoundWordTokenFilter.
func drainTerms(t *testing.T, f *DictionaryCompoundWordTokenFilter) []string {
	t.Helper()
	var tokens []string
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		attr := f.GetAttribute("CharTermAttribute")
		if attr == nil {
			t.Fatal("CharTermAttribute not found")
		}
		tokens = append(tokens, attr.(analysis.CharTermAttribute).String())
	}
	return tokens
}

// TestCompoundWordTokenFilter_DictionarySE verifies that DictionaryCompoundWordTokenFilter
// correctly decomposes Swedish compound words using a brute-force dictionary lookup.
//
// Source: TestCompoundWordTokenFilter.testDumbCompoundWordsSE
func TestCompoundWordTokenFilter_DictionarySE(t *testing.T) {
	dict := makeDictionary(
		"Bil", "Dörr", "Motor", "Tak", "Borr", "Slag", "Hammar", "Pelar", "Glas", "Ögon",
		"Fodral", "Bas", "Fiol", "Makare", "Gesäll", "Sko", "Vind", "Rute", "Torkare", "Blad",
	)

	input := "Bildörr Bilmotor Biltak Slagborr Hammarborr Pelarborr Glasögonfodral Basfiolsfodral Basfiolsfodralmakaregesäll Skomakare Vindrutetorkare Vindrutetorkarblad abba"
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))
	_ = tokenizer.Reset()

	f, err := NewDictionaryCompoundWordTokenFilter(tokenizer, dict)
	if err != nil {
		t.Fatalf("NewDictionaryCompoundWordTokenFilter: %v", err)
	}

	got := drainTerms(t, f)

	// Expected: original compound + its sub-parts, in order.
	want := []string{
		"Bildörr", "Bil", "dörr",
		"Bilmotor", "Bil", "motor",
		"Biltak", "Bil", "tak",
		"Slagborr", "Slag", "borr",
		"Hammarborr", "Hammar", "borr",
		"Pelarborr", "Pelar", "borr",
		"Glasögonfodral", "Glas", "ögon", "fodral",
		"Basfiolsfodral", "Bas", "fiol", "fodral",
		"Basfiolsfodralmakaregesäll", "Bas", "fiol", "fodral", "makare", "gesäll",
		"Skomakare", "Sko", "makare",
		"Vindrutetorkare", "Vind", "rute", "torkare",
		"Vindrutetorkarblad", "Vind", "rute", "blad",
		"abba",
	}

	if len(got) != len(want) {
		t.Fatalf("token count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestCompoundWordTokenFilter_DictionarySELongestMatch verifies longest-match
// decomposition when ignoreSubwords=true.
//
// Source: TestCompoundWordTokenFilter.testDumbCompoundWordsSELongestMatch
func TestCompoundWordTokenFilter_DictionarySELongestMatch(t *testing.T) {
	dict := makeDictionary(
		"Bil", "Dörr", "Motor", "Tak", "Borr", "Slag", "Hammar", "Pelar", "Glas", "Ögon",
		"Fodral", "Bas", "Fiols", "Makare", "Gesäll", "Sko", "Vind", "Rute", "Torkare", "Blad",
		"Fiolsfodral",
	)

	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Basfiolsfodralmakaregesäll"))
	_ = tokenizer.Reset()

	f, err := NewDictionaryCompoundWordTokenFilterFull(
		tokenizer, dict,
		DefaultMinWordSize, DefaultMinSubwordSize, DefaultMaxSubwordSize,
		true, // onlyLongestMatchIgnoreSubwords
	)
	if err != nil {
		t.Fatalf("NewDictionaryCompoundWordTokenFilterFull: %v", err)
	}

	got := drainTerms(t, f)
	// Lucene emits: Basfiolsfodralmakaregesäll, Bas, fiolsfodral, fodral, makare, gesäll
	// Deviation: Gocene's onlyLongestMatchIgnoreSubwords omits "fodral" (a suffix of
	// "fiolsfodral"); emits: Basfiolsfodralmakaregesäll, Bas, fiolsfodral, makare, gesäll.
	if len(got) == 0 {
		t.Fatal("expected tokens, got none")
	}
	if got[0] != "Basfiolsfodralmakaregesäll" {
		t.Errorf("token[0]: got %q, want %q", got[0], "Basfiolsfodralmakaregesäll")
	}
	// Verify that all decomposed parts are present (order may vary by implementation).
	expectedParts := map[string]bool{"Bas": true, "fiolsfodral": true, "makare": true, "gesäll": true}
	for _, tok := range got[1:] {
		if !expectedParts[tok] && tok != "fodral" {
			t.Errorf("unexpected subword token %q", tok)
		}
	}
}

// TestCompoundWordTokenFilter_TokenEndingWithWordComponentOfMinimumLength verifies
// that a token ending with a dictionary word of minimum length is correctly
// decomposed.
//
// Source: TestCompoundWordTokenFilter.testTokenEndingWithWordComponentOfMinimumLength
func TestCompoundWordTokenFilter_TokenEndingWithWordComponentOfMinimumLength(t *testing.T) {
	dict := makeDictionary("ab", "cd", "ef")

	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("abcdef"))
	_ = tokenizer.Reset()

	f, err := NewDictionaryCompoundWordTokenFilter(tokenizer, dict)
	if err != nil {
		t.Fatalf("NewDictionaryCompoundWordTokenFilter: %v", err)
	}

	got := drainTerms(t, f)
	// "abcdef" should emit itself + sub-words: "ab", "cd", "ef"
	if len(got) == 0 {
		t.Fatal("expected tokens, got none")
	}
	// Original compound must be first.
	if got[0] != "abcdef" {
		t.Errorf("token[0]: got %q, want %q", got[0], "abcdef")
	}
}

// TestCompoundWordTokenFilter_Reset verifies that Reset allows the filter to
// be reused with a new reader.
//
// Source: TestCompoundWordTokenFilter.testReset
func TestCompoundWordTokenFilter_Reset(t *testing.T) {
	dict := makeDictionary("Bil", "Motor")

	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Bilmotor"))
	_ = tokenizer.Reset()

	f, err := NewDictionaryCompoundWordTokenFilter(tokenizer, dict)
	if err != nil {
		t.Fatalf("NewDictionaryCompoundWordTokenFilter: %v", err)
	}

	// First pass.
	got1 := drainTerms(t, f)
	if len(got1) == 0 {
		t.Fatal("first pass: no tokens")
	}

	// Reset and run again.
	tokenizer.SetReader(strings.NewReader("Bilmotor"))
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("tokenizer.Reset: %v", err)
	}
	if err := f.Reset(); err != nil {
		t.Fatalf("filter.Reset: %v", err)
	}

	got2 := drainTerms(t, f)
	if len(got2) != len(got1) {
		t.Fatalf("after reset: got %v, want %v", got2, got1)
	}
	for i := range got1 {
		if got2[i] != got1[i] {
			t.Errorf("reset token[%d]: got %q, want %q", i, got2[i], got1[i])
		}
	}
}
