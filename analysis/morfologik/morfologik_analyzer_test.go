// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/test/org/apache/lucene/analysis/morfologik/TestMorfologikAnalyzer.java

package morfologik_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/morfologik"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// mockPolishStemmer is a minimal IStemmer that models the Polish dictionary
// results exercised by TestMorfologikAnalyzer. It reproduces the expected
// lemma output without depending on the binary FSA dictionary.
//
// Entries are sourced from the Java test assertions in
// TestMorfologikAnalyzer.java (Lucene 10.4.0).
type mockPolishStemmer struct{}

func (m *mockPolishStemmer) Lookup(token string) []morfologik.WordData {
	switch token {
	case "a":
		return []morfologik.WordData{{Stem: "a", Tag: ""}}
	case "liście":
		return []morfologik.WordData{
			{Stem: "liście", Tag: "subst:sg:acc:n2|subst:sg:nom:n2|subst:sg:voc:n2"},
			{Stem: "liść", Tag: "subst:pl:acc:m3|subst:pl:nom:m3|subst:pl:voc:m3"},
			{Stem: "list", Tag: "subst:sg:loc:m3|subst:sg:voc:m3"},
			{Stem: "lista", Tag: "subst:sg:dat:f|subst:sg:loc:f"},
		}
	case "danych":
		return []morfologik.WordData{
			{Stem: "dany", Tag: "adj:pl:nom"},
			{Stem: "dana", Tag: "subst:sg:gen:f"},
			{Stem: "dane", Tag: "subst:pl:gen:n2"},
			{Stem: "dać", Tag: "verb:perfective"},
		}
	case "agd":
		return []morfologik.WordData{
			{Stem: "artykuły gospodarstwa domowego", Tag: ""},
		}
	case "poznania":
		return []morfologik.WordData{
			{Stem: "poznanie", Tag: ""},
			{Stem: "poznać", Tag: ""},
		}
	}
	return nil
}

// mockPolishDictionary wraps the mock stemmer.
type mockPolishDictionary struct{}

func (d *mockPolishDictionary) NewStemmer() morfologik.IStemmer {
	return &mockPolishStemmer{}
}

// hasAttributeSource is the subset of TokenStream that exposes
// GetAttributeSource for typed attribute access.
type hasAttributeSource interface {
	GetAttributeSource() *util.AttributeSource
}

// drainTokens drains ts into a string slice. The caller must have called
// Reset() before invoking this helper.
func drainTokens(t *testing.T, ts analysis.TokenStream) []string {
	t.Helper()
	hasSrc, ok := ts.(hasAttributeSource)
	if !ok {
		t.Fatal("TokenStream does not expose GetAttributeSource")
		return nil
	}
	src := hasSrc.GetAttributeSource()
	var tokens []string
	for {
		advance, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !advance {
			break
		}
		attr := src.GetAttribute(analysis.CharTermAttributeType)
		if cta, ok := attr.(analysis.CharTermAttribute); ok && cta != nil {
			tokens = append(tokens, cta.String())
		}
	}
	return tokens
}

// analyzeTokens is a helper that creates a TokenStream from an analyzer and
// collects the resulting tokens.
func analyzeTokens(t *testing.T, a analysis.Analyzer, text string) []string {
	t.Helper()
	ts, err := a.TokenStream("dummy", strings.NewReader(text))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer ts.Close()

	type resetter interface{ Reset() error }
	if r, ok := ts.(resetter); ok {
		if err := r.Reset(); err != nil {
			t.Fatalf("Reset: %v", err)
		}
	}
	return drainTokens(t, ts)
}

// TestMorfologikAnalyzer_Constructor confirms the analyzer constructs without panic.
func TestMorfologikAnalyzer_Constructor(t *testing.T) {
	a := morfologik.NewMorfologikAnalyzer(&mockPolishDictionary{})
	if a == nil {
		t.Fatal("NewMorfologikAnalyzer returned nil")
	}
	a.Close()
}

// TestMorfologikAnalyzer_NilDictionaryPanics confirms a nil dictionary panics.
func TestMorfologikAnalyzer_NilDictionaryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil dictionary, got none")
		}
	}()
	morfologik.NewMorfologikAnalyzer(nil)
}

// TestMorfologikAnalyzer_TokenStreamCreation confirms a TokenStream is created
// and IncrementToken returns at least one result for known input.
// Mirrors testSingleTokens / testMultipleTokens behavior from TestMorfologikAnalyzer.java.
func TestMorfologikAnalyzer_TokenStreamCreation(t *testing.T) {
	a := morfologik.NewMorfologikAnalyzer(&mockPolishDictionary{})
	defer a.Close()

	ts, err := a.TokenStream("f", strings.NewReader("liście"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	if ts == nil {
		t.Fatal("TokenStream returned nil")
	}
	defer ts.Close()

	type resetter interface{ Reset() error }
	if r, ok := ts.(resetter); ok {
		_ = r.Reset()
	}

	count := 0
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	// "liście" expands to 4 lemmas in the mock dictionary.
	if count != 4 {
		t.Errorf("expected 4 tokens for 'liście', got %d", count)
	}
}

// TestMorfologikAnalyzer_MultipleTokens confirms multi-token input expands
// each word to its lemmas and the total count is correct.
// Mirrors testMultipleTokens from TestMorfologikAnalyzer.java.
func TestMorfologikAnalyzer_MultipleTokens(t *testing.T) {
	a := morfologik.NewMorfologikAnalyzer(&mockPolishDictionary{})
	defer a.Close()

	ts, err := a.TokenStream("f", strings.NewReader("liście danych"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer ts.Close()

	type resetter interface{ Reset() error }
	if r, ok := ts.(resetter); ok {
		_ = r.Reset()
	}

	count := 0
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	// "liście" = 4 lemmas, "danych" = 4 lemmas → total 8.
	if count != 8 {
		t.Errorf("expected 8 tokens for 'liście danych', got %d", count)
	}
}

// TestMorfologikAnalyzer_UnknownToken confirms unknown tokens pass through
// unchanged (no dictionary hit → original token emitted once).
func TestMorfologikAnalyzer_UnknownToken(t *testing.T) {
	a := morfologik.NewMorfologikAnalyzer(&mockPolishDictionary{})
	defer a.Close()

	ts, err := a.TokenStream("f", strings.NewReader("ęóąśłżźćń"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer ts.Close()

	type resetter interface{ Reset() error }
	if r, ok := ts.(resetter); ok {
		_ = r.Reset()
	}

	count := 0
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	// Unknown token has no dictionary hit → emitted once.
	if count != 1 {
		t.Errorf("expected 1 token for unknown input, got %d", count)
	}
}

// TestMorfologikAnalyzer_LeftoverStems mirrors testLeftoverStems from
// TestMorfologikAnalyzer.java: verify filter state is reset between reuses.
func TestMorfologikAnalyzer_LeftoverStems(t *testing.T) {
	a := morfologik.NewMorfologikAnalyzer(&mockPolishDictionary{})
	defer a.Close()

	// First stream - consume only the first token of "liście" (4 lemmas).
	ts1, err := a.TokenStream("dummy", strings.NewReader("liście"))
	if err != nil {
		t.Fatalf("first TokenStream: %v", err)
	}
	defer ts1.Close()

	type resetter interface{ Reset() error }
	if r, ok := ts1.(resetter); ok {
		_ = r.Reset()
	}
	ok, err := ts1.IncrementToken()
	if err != nil {
		t.Fatalf("first IncrementToken: %v", err)
	}
	if !ok {
		t.Fatal("expected first token from 'liście'")
	}

	// Second stream - start fresh on "danych" (4 lemmas).
	ts2, err := a.TokenStream("dummy", strings.NewReader("danych"))
	if err != nil {
		t.Fatalf("second TokenStream: %v", err)
	}
	defer ts2.Close()

	if r, ok := ts2.(resetter); ok {
		_ = r.Reset()
	}
	count := 0
	for {
		ok, err := ts2.IncrementToken()
		if err != nil {
			t.Fatalf("second IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	if count != 4 {
		t.Errorf("expected 4 tokens for 'danych' on second stream, got %d", count)
	}
}
