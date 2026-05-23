// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package commongrams_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/commongrams"
)

// makeTokenizer builds a StandardTokenizer over the given text.
func makeTokenizer(t *testing.T, text string) analysis.TokenStream {
	t.Helper()
	tok := analysis.NewStandardTokenizer()
	if err := tok.SetReader(strings.NewReader(text)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	lc := analysis.NewLowerCaseFilter(tok)
	return lc
}

// TestCommonGramsFilter_BasicBigrams verifies that common words cause bigrams
// to be emitted at position increment 0, and rare words are emitted normally.
func TestCommonGramsFilter_BasicBigrams(t *testing.T) {
	commonWords := analysis.GetWordSetFromStrings([]string{"the", "quick"}, false)
	input := makeTokenizer(t, "the quick brown fox")
	cgf := commongrams.NewCommonGramsFilter(input, commonWords)

	src := cgf.GetAttributeSource()
	var termAttr analysis.CharTermAttribute
	var typeAttr analysis.TypeAttribute
	var posIncAttr analysis.PositionIncrementAttribute
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		termAttr = a.(analysis.CharTermAttribute)
	}
	if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
		typeAttr = a.(analysis.TypeAttribute)
	}
	if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
		posIncAttr = a.(analysis.PositionIncrementAttribute)
	}

	type token struct {
		term   string
		typ    string
		posInc int
	}
	var tokens []token
	for {
		ok, err := cgf.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		tok := token{}
		if termAttr != nil {
			tok.term = termAttr.String()
		}
		if typeAttr != nil {
			tok.typ = typeAttr.GetType()
		}
		if posIncAttr != nil {
			tok.posInc = posIncAttr.GetPositionIncrement()
		}
		tokens = append(tokens, tok)
	}

	// Expected: "the" (posInc=1), "the_quick" (posInc=0,gram), "quick" (posInc=1),
	// "quick_brown" (posInc=0,gram), "brown" (posInc=1), "fox" (posInc=1).
	// Verify at minimum that we have grams for the common words.
	gramCount := 0
	for _, tok := range tokens {
		if tok.typ == commongrams.GramType {
			gramCount++
		}
	}
	if gramCount == 0 {
		t.Errorf("expected at least one gram token, got none; tokens: %v", tokens)
	}
	// Verify gram tokens have posInc=0.
	for _, tok := range tokens {
		if tok.typ == commongrams.GramType && tok.posInc != 0 {
			t.Errorf("gram token %q has posInc=%d, want 0", tok.term, tok.posInc)
		}
	}
}

// TestCommonGramsFilter_GramSeparator verifies bigrams use underscore separator.
func TestCommonGramsFilter_GramSeparator(t *testing.T) {
	commonWords := analysis.GetWordSetFromStrings([]string{"the"}, false)
	input := makeTokenizer(t, "the fox")
	cgf := commongrams.NewCommonGramsFilter(input, commonWords)

	src := cgf.GetAttributeSource()
	var termAttr analysis.CharTermAttribute
	var typeAttr analysis.TypeAttribute
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		termAttr = a.(analysis.CharTermAttribute)
	}
	if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
		typeAttr = a.(analysis.TypeAttribute)
	}

	var grams []string
	for {
		ok, err := cgf.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if typeAttr != nil && typeAttr.GetType() == commongrams.GramType {
			if termAttr != nil {
				grams = append(grams, termAttr.String())
			}
		}
	}

	if len(grams) == 0 {
		t.Fatal("expected at least one gram, got none")
	}
	if grams[0] != "the_fox" {
		t.Errorf("gram[0] = %q, want %q", grams[0], "the_fox")
	}
}

// TestCommonGramsFilter_NoCommonWords verifies that with no common words, no
// bigrams are emitted.
func TestCommonGramsFilter_NoCommonWords(t *testing.T) {
	commonWords := analysis.NewCharArraySet(0, false)
	input := makeTokenizer(t, "the quick brown fox")
	cgf := commongrams.NewCommonGramsFilter(input, commonWords)

	src := cgf.GetAttributeSource()
	var typeAttr analysis.TypeAttribute
	if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
		typeAttr = a.(analysis.TypeAttribute)
	}

	for {
		ok, err := cgf.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if typeAttr != nil && typeAttr.GetType() == commongrams.GramType {
			t.Errorf("unexpected gram token with empty common-words set")
		}
	}
}

// TestCommonGramsQueryFilter_Basic verifies that CommonGramsQueryFilter
// suppresses unigrams that are part of bigrams.
func TestCommonGramsQueryFilter_Basic(t *testing.T) {
	commonWords := analysis.GetWordSetFromStrings([]string{"the", "in"}, false)
	input := makeTokenizer(t, "the rain in spain falls")
	cgf := commongrams.NewCommonGramsFilter(input, commonWords)
	qf := commongrams.NewCommonGramsQueryFilter(cgf)

	src := qf.GetAttributeSource()
	var termAttr analysis.CharTermAttribute
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		termAttr = a.(analysis.CharTermAttribute)
	}

	var terms []string
	for {
		ok, err := qf.IncrementToken()
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

	// Expected output contains bigrams for common-word adjacencies and
	// standalone terms for non-common words that are not in a bigram.
	if len(terms) == 0 {
		t.Fatal("expected tokens, got none")
	}
	// Verify "the" and "in" alone are NOT in output (they are members of bigrams).
	for _, term := range terms {
		if term == "the" || term == "in" {
			t.Errorf("unexpected standalone common word %q in query filter output", term)
		}
	}
	// Verify "falls" IS in output (not a common word, not part of a bigram).
	found := false
	for _, term := range terms {
		if term == "falls" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected standalone term 'falls' in output, got %v", terms)
	}
}

// TestCommonGramsFilterFactory_Create verifies factory wiring.
func TestCommonGramsFilterFactory_Create(t *testing.T) {
	commonWords := analysis.GetWordSetFromStrings([]string{"the"}, false)
	factory := commongrams.NewCommonGramsFilterFactoryWithWords(commonWords)
	input := makeTokenizer(t, "the fox")
	filter := factory.Create(input)
	if filter == nil {
		t.Fatal("factory.Create returned nil")
	}
}

// TestCommonGramsQueryFilterFactory_Create verifies factory wiring.
func TestCommonGramsQueryFilterFactory_Create(t *testing.T) {
	factory := commongrams.NewCommonGramsQueryFilterFactory(map[string]string{})
	input := makeTokenizer(t, "the fox")
	filter := factory.Create(input)
	if filter == nil {
		t.Fatal("factory.Create returned nil")
	}
}
