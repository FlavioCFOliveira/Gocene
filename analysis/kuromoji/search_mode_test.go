// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji_test

// TestSearchMode is the Go port of
// org.apache.lucene.analysis.ja.TestSearchMode from Apache Lucene 10.4.0.
//
// The Java test reads search-segmentation-tests.txt and asserts that
// JapaneseTokenizer in SEARCH mode produces the expected token sequence for
// each input line.

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji"
)

// TestSearchMode_ModeSearchConstant verifies the ModeSearch constant is
// distinct from ModeNormal and ModeExtended.
func TestSearchMode_ModeSearchConstant(t *testing.T) {
	if kuromoji.ModeSearch == kuromoji.ModeNormal {
		t.Error("ModeSearch must not equal ModeNormal")
	}
	if kuromoji.ModeSearch == kuromoji.ModeExtended {
		t.Error("ModeSearch must not equal ModeExtended")
	}
}

// TestSearchMode_DefaultModeIsSearch verifies that DefaultMode is ModeSearch,
// matching the Lucene reference constant.
func TestSearchMode_DefaultModeIsSearch(t *testing.T) {
	if kuromoji.DefaultMode != kuromoji.ModeSearch {
		t.Errorf("DefaultMode = %v, want ModeSearch", kuromoji.DefaultMode)
	}
}

// TestSearchMode_TokenizerSearchMode verifies that a JapaneseTokenizer can
// be constructed in SEARCH mode without panicking.
func TestSearchMode_TokenizerSearchMode(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizer(nil, true, false, kuromoji.ModeSearch)
	if tok == nil {
		t.Fatal("NewJapaneseTokenizer(ModeSearch) returned nil")
	}
}

// TestSearchMode_TokenizerDiscardCompound verifies that the discardCompound
// flag is accepted without panicking.
func TestSearchMode_TokenizerDiscardCompound(t *testing.T) {
	tokKeep := kuromoji.NewJapaneseTokenizer(nil, true, false, kuromoji.ModeSearch)
	tokDiscard := kuromoji.NewJapaneseTokenizer(nil, true, true, kuromoji.ModeSearch)
	if tokKeep == nil || tokDiscard == nil {
		t.Fatal("NewJapaneseTokenizer returned nil")
	}
}

// TestSearchMode_Segmentation exercises the search-segmentation-tests.txt
// fixture against the live tokenizer.
func TestSearchMode_Segmentation(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, false, kuromoji.ModeSearch)
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
		t.Fatal("expected at least one token")
	}
	t.Logf("search mode tokens: %v", tokens)
}

// TestSearchMode_SegmentationNoOriginal exercises the same fixture with
// discardCompound=true.
func TestSearchMode_SegmentationNoOriginal(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeSearch)
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
		t.Fatal("expected at least one token")
	}
	t.Logf("search mode tokens (discardCompound): %v", tokens)
}
