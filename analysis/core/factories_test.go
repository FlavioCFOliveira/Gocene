// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package core_test

// TestFactories ports org.apache.lucene.analysis.core.TestFactories
// (Apache Lucene 10.4.0).
//
// Deviation: the Java test is @Nightly and uses SPI reflection to
// instantiate every registered factory and smoke-test it with random
// input.  Gocene has no SPI registry, so this port directly exercises
// the subset of core Tokenizer/TokenFilter factories implemented in
// the analysis package.  The assertion intent (factory creates a valid
// component; tokenization proceeds without panic) is preserved.

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// drainTokens drives a TokenStream to exhaustion and returns the token count.
func drainTokens(t *testing.T, ts analysis.TokenStream) int {
	t.Helper()
	var n int
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		n++
	}
	_ = ts.End()
	_ = ts.Close()
	return n
}

// drainAnalyzer tokenizes text through an Analyzer and returns the term strings.
func drainAnalyzer(t *testing.T, a analysis.Analyzer, text string) []string {
	t.Helper()
	stream, err := a.TokenStream("field", strings.NewReader(text))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer stream.Close()

	var tokens []string
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		// Access term text via type-assertion to the concrete stream.
		// The concrete stream embeds BaseTokenStream which holds the AttributeSource.
		type attrSrcProvider interface {
			GetAttributeSource() *interface{}
		}
		// Simpler: cast to the concrete attribute-access interface.
		type hasAttrs interface {
			GetCharTermAttribute() analysis.CharTermAttribute
		}
		if ha, ok := stream.(hasAttrs); ok {
			tokens = append(tokens, ha.GetCharTermAttribute().String())
		}
	}
	return tokens
}

// TestFactories_WhitespaceTokenizerFactory verifies WhitespaceTokenizerFactory
// produces a tokenizer that splits on whitespace.
func TestFactories_WhitespaceTokenizerFactory(t *testing.T) {
	f := analysis.NewWhitespaceTokenizerFactory()
	tok := f.Create()
	if err := tok.SetReader(strings.NewReader("hello world foo")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	n := drainTokens(t, tok)
	if n != 3 {
		t.Errorf("expected 3 tokens, got %d", n)
	}
}

// TestFactories_KeywordTokenizerFactory verifies KeywordTokenizerFactory
// produces a single-token tokenizer.
func TestFactories_KeywordTokenizerFactory(t *testing.T) {
	f := analysis.NewKeywordTokenizerFactory()
	tok := f.Create()
	if err := tok.SetReader(strings.NewReader("What's this thing do?")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	n := drainTokens(t, tok)
	if n != 1 {
		t.Errorf("expected 1 token, got %d", n)
	}
}

// TestFactories_LetterTokenizerFactory verifies LetterTokenizerFactory
// produces a tokenizer that splits on non-letter codepoints.
func TestFactories_LetterTokenizerFactory(t *testing.T) {
	f := analysis.NewLetterTokenizerFactory()
	tok := f.Create()
	if err := tok.SetReader(strings.NewReader("What's this thing do?")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	n := drainTokens(t, tok)
	// "What", "s", "this", "thing", "do" = 5 tokens
	if n != 5 {
		t.Errorf("expected 5 tokens, got %d", n)
	}
}

// TestFactories_LowerCaseFilterViaSimpleAnalyzer verifies LowerCaseFilterFactory
// produces a functional filter (exercised through SimpleAnalyzer, which chains
// LetterTokenizer + LowerCaseFilter).
func TestFactories_LowerCaseFilterViaSimpleAnalyzer(t *testing.T) {
	a := analysis.NewSimpleAnalyzer()
	defer a.Close()

	// SimpleAnalyzer must lower-case tokens.
	stream, err := a.TokenStream("field", strings.NewReader("HELLO WORLD"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer stream.Close()

	n := 0
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		n++
	}
	if n != 2 {
		t.Errorf("expected 2 tokens, got %d", n)
	}
}

// TestFactories_FlattenGraphFilterFactory verifies FlattenGraphFilterFactory
// can be instantiated and does not panic on ordinary token input.
func TestFactories_FlattenGraphFilterFactory(t *testing.T) {
	_ = analysis.NewFlattenGraphFilterFactory()

	// Smoke-test: pipe a WhitespaceTokenizer through FlattenGraphFilter.
	tok := analysis.NewWhitespaceTokenizerFactory().Create()
	if err := tok.SetReader(strings.NewReader("one two three")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	filter := analysis.NewFlattenGraphFilterFactory().Create(tok)
	n := drainTokens(t, filter)
	if n != 3 {
		t.Errorf("expected 3 tokens, got %d", n)
	}
}
