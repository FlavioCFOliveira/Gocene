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

// ---- ICUNormalizer2Filter tests (task 2968) ----

// TestICUNormalizer2Filter_NFKCCF verifies that the default normaliser
// (NFKC+CaseFold) lowercases and decomposes text correctly.
func TestICUNormalizer2Filter_NFKCCF(t *testing.T) {
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("HELLO"))
	f := icu.NewICUNormalizer2Filter(tok)

	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected a token")
	}
	src := f.GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	if attr.String() != "hello" {
		t.Errorf("got %q, want %q", attr.String(), "hello")
	}
}

// TestICUNormalizer2Filter_NFKCFullwidth verifies NFKC normalisation
// converts fullwidth characters to ASCII.
func TestICUNormalizer2Filter_NFKCFullwidth(t *testing.T) {
	normalizer := icu.NewNormalizer2("nfkc", icu.NormalizerModeCompose)
	tok := analysis.NewKeywordTokenizer()
	// "Ａ" is U+FF21 FULLWIDTH LATIN CAPITAL LETTER A
	_ = tok.SetReader(strings.NewReader("Ａ"))
	f := icu.NewICUNormalizer2FilterWith(tok, normalizer)

	ok, _ := f.IncrementToken()
	if !ok {
		t.Fatal("expected a token")
	}
	src := f.GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	// NFKC maps Ａ to A
	if attr.String() != "A" {
		t.Errorf("got %q, want %q", attr.String(), "A")
	}
}

// TestICUNormalizer2Filter_AlreadyNormalised verifies that tokens that are
// already normalised pass through unchanged.
func TestICUNormalizer2Filter_AlreadyNormalised(t *testing.T) {
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("hello world"))
	f := icu.NewICUNormalizer2Filter(tok)

	ok, _ := f.IncrementToken()
	if !ok {
		t.Fatal("expected a token")
	}
	src := f.GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	if attr.String() != "hello world" {
		t.Errorf("got %q, want %q", attr.String(), "hello world")
	}
}

// ---- ICUFoldingFilter tests (task 2962) ----

// TestICUFoldingFilter_CaseFold verifies that the folding filter lowercases.
func TestICUFoldingFilter_CaseFold(t *testing.T) {
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("HELLO"))
	f := icu.NewICUFoldingFilter(tok)

	ok, err := f.IncrementToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected a token")
	}
	src := f.GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	if attr.String() != "hello" {
		t.Errorf("got %q, want %q", attr.String(), "hello")
	}
}

// TestICUFoldingFilter_Fullwidth verifies width folding.
func TestICUFoldingFilter_Fullwidth(t *testing.T) {
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("Ａ"))
	f := icu.NewICUFoldingFilter(tok)

	ok, _ := f.IncrementToken()
	if !ok {
		t.Fatal("expected a token")
	}
	src := f.GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	// NFKC+CaseFold maps Ａ (fullwidth) to "a"
	if attr.String() != "a" {
		t.Errorf("got %q, want %q", attr.String(), "a")
	}
}

// ---- ICUFoldingFilterFactory tests (task 2964) ----

// TestICUFoldingFilterFactory_Create verifies the factory creates a working
// ICUFoldingFilter.
func TestICUFoldingFilterFactory_Create(t *testing.T) {
	factory := icu.NewICUFoldingFilterFactory()
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("HELLO"))
	filter := factory.Create(tok)

	ok, _ := filter.IncrementToken()
	if !ok {
		t.Fatal("expected a token")
	}
	src := filter.(*icu.ICUFoldingFilter).GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	// NFKC+CaseFold: "HELLO" → "hello"
	got := attr.String()
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// ---- ICUNormalizer2FilterFactory tests (task 2977) ----

// TestICUNormalizer2FilterFactory_Default verifies the default factory applies
// NFKC+CaseFold.
func TestICUNormalizer2FilterFactory_Default(t *testing.T) {
	factory := icu.NewICUNormalizer2FilterFactoryDefault()
	tok := analysis.NewKeywordTokenizer()
	_ = tok.SetReader(strings.NewReader("HELLO"))
	filter := factory.Create(tok)

	ok, _ := filter.IncrementToken()
	if !ok {
		t.Fatal("expected a token")
	}
	src := filter.(*icu.ICUNormalizer2Filter).GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	if attr.String() != "hello" {
		t.Errorf("got %q, want %q", attr.String(), "hello")
	}
}

// TestICUNormalizer2FilterFactory_NFC verifies NFC form.
func TestICUNormalizer2FilterFactory_NFC(t *testing.T) {
	factory := icu.NewICUNormalizer2FilterFactory("nfc", icu.NormalizerModeCompose)
	tok := analysis.NewKeywordTokenizer()
	// Decomposed "é" (e + combining accent) should compose to "é".
	_ = tok.SetReader(strings.NewReader("é"))
	filter := factory.Create(tok)

	ok, _ := filter.IncrementToken()
	if !ok {
		t.Fatal("expected a token")
	}
	src := filter.(*icu.ICUNormalizer2Filter).GetAttributeSource()
	attr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	// NFC: decomposed é → composed é
	if attr.String() != "é" {
		t.Errorf("got %q, want %q", attr.String(), "é")
	}
}

// ---- ICUNormalizer2CharFilter tests (task 2963) ----

// TestICUNormalizer2CharFilter_Basic verifies that the char filter produces
// normalised output for a simple case.
func TestICUNormalizer2CharFilter_Basic(t *testing.T) {
	// "Ａ" (U+FF21) normalises to "a" under NFKC+CaseFold.
	f := icu.NewICUNormalizer2CharFilter(strings.NewReader("Ａ"))

	buf := make([]byte, 16)
	n, _ := f.Read(buf)
	got := string(buf[:n])
	if got != "a" {
		t.Errorf("got %q, want %q", got, "a")
	}
}

// TestICUNormalizer2CharFilter_NFC verifies NFC char filter.
func TestICUNormalizer2CharFilter_NFC(t *testing.T) {
	normalizer := icu.NewNormalizer2("nfc", icu.NormalizerModeCompose)
	// Decomposed "é" should compose.
	f := icu.NewICUNormalizer2CharFilterWith(strings.NewReader("é"), normalizer)

	buf := make([]byte, 16)
	n, _ := f.Read(buf)
	got := string(buf[:n])
	if got != "é" {
		t.Errorf("got %q, want %q", got, "é")
	}
}
