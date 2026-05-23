// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/icu"
)

// simpleReplaceTransliterator is a trivial Transliterator for testing that
// replaces occurrences of old with new.
type simpleReplaceTransliterator struct {
	old, new string
}

func (t *simpleReplaceTransliterator) Transliterate(src string) string {
	return strings.ReplaceAll(src, t.old, t.new)
}

// identityTransliterator passes input through unchanged.
type identityTransliterator struct{}

func (t *identityTransliterator) Transliterate(src string) string { return src }

// buildICUTransformTokenStream creates a keyword-like token stream with a
// single token containing text, then wraps it with an ICUTransformFilter.
func buildICUTransformTokenStream(text string, transform icu.Transliterator) (*analysis.KeywordTokenizer, *icu.ICUTransformFilter) {
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader(text))
	filter := icu.NewICUTransformFilter(tok, transform)
	return tok, filter
}

// TestICUTransformFilter_Identity verifies that an identity transliterator
// leaves tokens unchanged (mirrors TestICUTransformFilter.testBasicFunctionality
// structural contract).
func TestICUTransformFilter_Identity(t *testing.T) {
	transform := &identityTransliterator{}
	_, filter := buildICUTransformTokenStream("hello", transform)

	_ = filter // reset handled by fresh tokenizer
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("hello"))
	f := icu.NewICUTransformFilter(tok, transform)

	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected at least one token")
	}

	src := f.GetAttributeSource()
	if src == nil {
		t.Fatal("attribute source is nil")
	}
	attr := src.GetAttribute(analysis.CharTermAttributeType)
	if attr == nil {
		t.Fatal("CharTermAttribute is nil")
	}
	got := attr.(analysis.CharTermAttribute).String()
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// TestICUTransformFilter_SimpleReplace verifies that a transliterator that
// replaces 'a' with 'b' transforms token text (mirrors the custom rule tests).
func TestICUTransformFilter_SimpleReplace(t *testing.T) {
	transform := &simpleReplaceTransliterator{old: "a", new: "b"}
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("abacada"))
	f := icu.NewICUTransformFilter(tok, transform)

	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected at least one token")
	}

	src := f.GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	got := attr.String()
	want := "bbbcbdb"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestICUTransformFilter_Empty verifies that an empty token is handled
// (mirrors testEmptyTerm).
func TestICUTransformFilter_Empty(t *testing.T) {
	transform := &identityTransliterator{}
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader(""))
	f := icu.NewICUTransformFilter(tok, transform)

	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		// KeywordTokenizer may or may not emit a token for empty input; either
		// outcome is acceptable.
		return
	}
	src := f.GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	if attr.String() != "" {
		t.Errorf("got %q, want empty string", attr.String())
	}
}

// TestICUTransformFilter_MultipleTokens verifies that multiple tokens are
// all transformed.
func TestICUTransformFilter_MultipleTokens(t *testing.T) {
	transform := &simpleReplaceTransliterator{old: "x", new: "y"}
	tokenizer := analysis.NewWhitespaceTokenizer()
	_ = tokenizer.SetReader(strings.NewReader("fox box"))
	f := icu.NewICUTransformFilter(tokenizer, transform)

	var tokens []string
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			break
		}
		src := f.GetAttributeSource()
		attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
		tokens = append(tokens, attr.String())
	}

	want := []string{"foy", "boy"}
	if len(tokens) != len(want) {
		t.Fatalf("got %v tokens, want %v", len(tokens), len(want))
	}
	for i, w := range want {
		if tokens[i] != w {
			t.Errorf("token[%d]: got %q, want %q", i, tokens[i], w)
		}
	}
}

// TestICUTransformFilterFactory_Create verifies that the factory produces a
// working ICUTransformFilter.
func TestICUTransformFilterFactory_Create(t *testing.T) {
	transform := &simpleReplaceTransliterator{old: "e", new: "a"}
	factory := icu.NewICUTransformFilterFactory(transform)

	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("hello"))
	filter := factory.Create(tok)

	ok, err := filter.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected a token")
	}
	// Verify through the concrete type.
	f := filter.(*icu.ICUTransformFilter)
	attrSrc := f.GetAttributeSource()
	attr := attrSrc.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	if attr.String() != "hallo" {
		t.Errorf("got %q, want %q", attr.String(), "hallo")
	}
}
