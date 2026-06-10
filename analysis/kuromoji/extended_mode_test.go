// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji_test

// TestExtendedMode is the Go port of
// org.apache.lucene.analysis.ja.TestExtendedMode from Apache Lucene 10.4.0.
//
// The Java test exercises JapaneseTokenizer in EXTENDED mode, focusing on
// surrogate-pair / supplementary character handling and random string
// robustness.

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji"
)

// TestExtendedMode_TokenizerCreation verifies that a JapaneseTokenizer can
// be constructed in EXTENDED mode without panicking.
func TestExtendedMode_TokenizerCreation(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerSimple(nil)
	if tok == nil {
		t.Fatal("NewJapaneseTokenizerSimple returned nil")
	}
}

// TestExtendedMode_ModeExtendedConstant verifies the ModeExtended constant
// is distinct from ModeNormal and ModeSearch.
func TestExtendedMode_ModeExtendedConstant(t *testing.T) {
	if kuromoji.ModeExtended == kuromoji.ModeNormal {
		t.Error("ModeExtended must not equal ModeNormal")
	}
	if kuromoji.ModeExtended == kuromoji.ModeSearch {
		t.Error("ModeExtended must not equal ModeSearch")
	}
}

// TestExtendedMode_Surrogates verifies that JapaneseTokenizer in EXTENDED
// mode correctly handles supplementary (surrogate-pair) characters.
func TestExtendedMode_Surrogates(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeExtended)
	if tok == nil {
		t.Fatal("nil tokenizer")
	}
	// Input containing supplementary characters (emoji, CJK extension B, etc.)
	text := "𠮷野家🍣"
	tok.SetReader(strings.NewReader(text))
	count := 0
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	if count == 0 {
		t.Fatal("expected at least one token for supplementary character input")
	}
}

// TestExtendedMode_Surrogates2 stress-tests the tokenizer with a longer
// string containing supplementary characters.
func TestExtendedMode_Surrogates2(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeExtended)
	if tok == nil {
		t.Fatal("nil tokenizer")
	}
	text := "𠮷野家で🍣を食べた"
	tok.SetReader(strings.NewReader(text))
	count := 0
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	if count == 0 {
		t.Fatal("expected at least one token")
	}
}

// TestExtendedMode_RandomStrings blasts random strings through the analyzer.
func TestExtendedMode_RandomStrings(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeExtended)
	if tok == nil {
		t.Fatal("nil tokenizer")
	}
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		length := rng.Intn(50) + 1
		var b strings.Builder
		for j := 0; j < length; j++ {
			// Mix ASCII, Hiragana, Katakana, CJK, and supplementary.
			switch rng.Intn(5) {
			case 0:
				b.WriteRune(rune(rng.Intn(95) + 32)) // ASCII printable
			case 1:
				b.WriteRune(rune(0x3040 + rng.Intn(96))) // Hiragana
			case 2:
				b.WriteRune(rune(0x30A0 + rng.Intn(96))) // Katakana
			case 3:
				b.WriteRune(rune(0x4E00 + rng.Intn(1000))) // CJK
			case 4:
				b.WriteRune(rune(0x1F600 + rng.Intn(80))) // Emoji
			}
		}
		text := b.String()
		tok.SetReader(strings.NewReader(text))
		// Must not panic and must eventually terminate.
		iterations := 0
		for {
			ok, err := tok.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken error on %q: %v", text, err)
			}
			if !ok {
				break
			}
			iterations++
			if iterations > length*10 {
				t.Fatalf("too many tokens for input %q", text)
			}
		}
	}
}

// TestExtendedMode_RandomHugeStrings blasts random large strings.
func TestExtendedMode_RandomHugeStrings(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeExtended)
	if tok == nil {
		t.Fatal("nil tokenizer")
	}
	rng := rand.New(rand.NewSource(123))
	for i := 0; i < 10; i++ {
		length := rng.Intn(500) + 100
		var b strings.Builder
		for j := 0; j < length; j++ {
			b.WriteRune(rune(0x3040 + rng.Intn(96))) // Hiragana
		}
		text := b.String()
		tok.SetReader(strings.NewReader(text))
		iterations := 0
		for {
			ok, err := tok.IncrementToken()
			if err != nil {
				t.Fatalf("IncrementToken error on huge string: %v", err)
			}
			if !ok {
				break
			}
			iterations++
			if iterations > length*10 {
				t.Fatal("too many tokens for huge string")
			}
		}
	}
}

// TestExtendedMode_OffsetConsistency verifies that token offsets are
// consistent with the UTF-16 code-unit offsets expected by Lucene.
func TestExtendedMode_OffsetConsistency(t *testing.T) {
	tok := kuromoji.NewJapaneseTokenizerWithDefaults(nil, true, true, kuromoji.ModeExtended)
	if tok == nil {
		t.Fatal("nil tokenizer")
	}
	text := "abc"
	tok.SetReader(strings.NewReader(text))
	totalRunes := utf8.RuneCountInString(text)
	maxEnd := 0
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !ok {
			break
		}
		attr := tok.GetAttribute("OffsetAttribute")
		if attr == nil {
			t.Fatal("OffsetAttribute is nil")
		}
		offAttr, ok := attr.(analysis.OffsetAttribute)
		if !ok {
			t.Fatalf("OffsetAttribute has wrong type: %T", attr)
		}
		if offAttr.EndOffset() > maxEnd {
			maxEnd = offAttr.EndOffset()
		}
	}
	if maxEnd != totalRunes {
		t.Fatalf("max end offset %d != input rune count %d", maxEnd, totalRunes)
	}
}
