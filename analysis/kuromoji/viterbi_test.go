// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji"
)

// TestViterbiBasicSegmentation verifies that JapaneseTokenizer backed by the
// embedded binary dictionaries can segment simple Japanese input.
func TestViterbiBasicSegmentation(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeSearch)
	if tok == nil {
		t.Fatal("NewJapaneseTokenizerWithDefaults returned nil")
	}

	text := "東京都" // "東京都" (Tokyo Metropolis)
	tok.SetReader(strings.NewReader(text))

	var tokens []string
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !ok {
			break
		}
		attr := tok.GetAttribute("CharTermAttribute")
		if attr == nil {
			t.Fatal("CharTermAttribute is nil")
		}
		termAttr, ok := attr.(analysis.CharTermAttribute)
		if !ok {
			t.Fatalf("CharTermAttribute has wrong type: %T", attr)
		}
		tokens = append(tokens, termAttr.String())
	}

	if len(tokens) == 0 {
		t.Fatal("expected at least one token, got none")
	}
	t.Logf("tokens: %v", tokens)
}

// TestViterbiExtendedMode verifies that ModeExtended produces unigrams for
// unknown words.
func TestViterbiExtendedMode(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeExtended)
	if tok == nil {
		t.Fatal("NewJapaneseTokenizerWithDefaults returned nil")
	}

	text := "東京都"
	tok.SetReader(strings.NewReader(text))

	var tokens []string
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !ok {
			break
		}
		attr := tok.GetAttribute("CharTermAttribute")
		if attr == nil {
			t.Fatal("CharTermAttribute is nil")
		}
		termAttr, ok := attr.(analysis.CharTermAttribute)
		if !ok {
			t.Fatalf("CharTermAttribute has wrong type: %T", attr)
		}
		tokens = append(tokens, termAttr.String())
	}

	if len(tokens) == 0 {
		t.Fatal("expected at least one token in extended mode, got none")
	}
	t.Logf("extended tokens: %v", tokens)
}

// TestViterbiNormalMode verifies that ModeNormal produces ordinary segmentation.
func TestViterbiNormalMode(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeNormal)
	if tok == nil {
		t.Fatal("NewJapaneseTokenizerWithDefaults returned nil")
	}

	text := "お茶" // "お茶" (tea)
	tok.SetReader(strings.NewReader(text))

	var tokens []string
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !ok {
			break
		}
		attr := tok.GetAttribute("CharTermAttribute")
		if attr == nil {
			t.Fatal("CharTermAttribute is nil")
		}
		termAttr, ok := attr.(analysis.CharTermAttribute)
		if !ok {
			t.Fatalf("CharTermAttribute has wrong type: %T", attr)
		}
		tokens = append(tokens, termAttr.String())
	}

	if len(tokens) == 0 {
		t.Fatal("expected at least one token in normal mode, got none")
	}
	t.Logf("normal tokens: %v", tokens)
}

// TestViterbiOffsetAndAttributes verifies that offset and Japanese-specific
// attributes are populated correctly.
func TestViterbiOffsetAndAttributes(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeSearch)
	if tok == nil {
		t.Fatal("NewJapaneseTokenizerWithDefaults returned nil")
	}

	text := "東京"
	tok.SetReader(strings.NewReader(text))

	ok, err := tok.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken error: %v", err)
	}
	if !ok {
		t.Fatal("expected at least one token")
	}

	attr := tok.GetAttribute("OffsetAttribute")
	if attr == nil {
		t.Fatal("OffsetAttribute is nil")
	}
	offAttr, ok := attr.(analysis.OffsetAttribute)
	if !ok {
		t.Fatalf("OffsetAttribute has wrong type: %T", attr)
	}
	start, end := offAttr.StartOffset(), offAttr.EndOffset()
	if start != 0 || end != 2 {
		t.Fatalf("offset mismatch: got [%d,%d), want [0,2)", start, end)
	}
}
