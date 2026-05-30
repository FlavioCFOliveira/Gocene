// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji_test

// TestFactories is the Go port of
// org.apache.lucene.analysis.ja.TestFactories from Apache Lucene 10.4.0.
//
// The Java test exercises the SPI factory registry (TokenizerFactory,
// TokenFilterFactory, CharFilterFactory) by reflecting over all registered
// factories and smoke-testing each one with random data.  Gocene does not yet
// have an SPI factory registry; that wiring is deferred to the codec sprint.
//
// Deviation: all sub-tests that require a wired SPI registry or a live
// JapaneseTokenizer backed by binary dictionaries are skipped until those
// prerequisites are available.  The tests below exercise the structural
// instantiation of every factory type already defined in this package.

import (
	"io"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji"
)

// TestFactories_TokenizerFactory verifies that JapaneseTokenizerFactory
// can be constructed and that Create returns a non-nil tokenizer.
func TestFactories_TokenizerFactory(t *testing.T) {
	f := kuromoji.NewJapaneseTokenizerFactory(kuromoji.DefaultMode, nil, true, true)
	tok := f.Create(io.NopCloser(strings.NewReader("テスト")))
	if tok == nil {
		t.Fatal("JapaneseTokenizerFactory.Create returned nil")
	}
}

// TestFactories_BaseFormFilterFactory verifies construction.
func TestFactories_BaseFormFilterFactory(t *testing.T) {
	f := kuromoji.NewJapaneseBaseFormFilterFactory()
	if f == nil {
		t.Fatal("NewJapaneseBaseFormFilterFactory returned nil")
	}
}

// TestFactories_HiraganaUppercaseFilterFactory verifies construction.
func TestFactories_HiraganaUppercaseFilterFactory(t *testing.T) {
	f := kuromoji.NewJapaneseHiraganaUppercaseFilterFactory()
	if f == nil {
		t.Fatal("NewJapaneseHiraganaUppercaseFilterFactory returned nil")
	}
}

// TestFactories_KatakanaUppercaseFilterFactory verifies construction.
func TestFactories_KatakanaUppercaseFilterFactory(t *testing.T) {
	f := kuromoji.NewJapaneseKatakanaUppercaseFilterFactory()
	if f == nil {
		t.Fatal("NewJapaneseKatakanaUppercaseFilterFactory returned nil")
	}
}

// TestFactories_KatakanaStemFilterFactory verifies construction with default
// and custom minimumLength parameters.
func TestFactories_KatakanaStemFilterFactory(t *testing.T) {
	for _, length := range []int{kuromoji.DefaultMinimumKatakanaLength, 6} {
		f := kuromoji.NewJapaneseKatakanaStemFilterFactory(length)
		if f == nil {
			t.Fatalf("NewJapaneseKatakanaStemFilterFactory(%d) returned nil", length)
		}
	}
}

// TestFactories_NumberFilterFactory verifies construction.
func TestFactories_NumberFilterFactory(t *testing.T) {
	f := kuromoji.NewJapaneseNumberFilterFactory()
	if f == nil {
		t.Fatal("NewJapaneseNumberFilterFactory returned nil")
	}
}

// TestFactories_PartOfSpeechStopFilterFactory verifies construction.
func TestFactories_PartOfSpeechStopFilterFactory(t *testing.T) {
	f := kuromoji.NewJapanesePartOfSpeechStopFilterFactory(nil)
	if f == nil {
		t.Fatal("NewJapanesePartOfSpeechStopFilterFactory returned nil")
	}
}

// TestFactories_ReadingFormFilterFactory verifies construction.
func TestFactories_ReadingFormFilterFactory(t *testing.T) {
	for _, romaji := range []bool{false, true} {
		f := kuromoji.NewJapaneseReadingFormFilterFactory(romaji)
		if f == nil {
			t.Fatalf("NewJapaneseReadingFormFilterFactory(%v) returned nil", romaji)
		}
	}
}

// TestFactories_IterationMarkCharFilterFactory verifies construction.
func TestFactories_IterationMarkCharFilterFactory(t *testing.T) {
	f := kuromoji.NewJapaneseIterationMarkCharFilterFactory(true, true)
	if f == nil {
		t.Fatal("NewJapaneseIterationMarkCharFilterFactory returned nil")
	}
}

// TestFactories_CompletionFilterFactory verifies construction for both modes.
func TestFactories_CompletionFilterFactory(t *testing.T) {
	for _, mode := range []kuromoji.CompletionMode{kuromoji.CompletionModeIndex, kuromoji.CompletionModeQuery} {
		f := kuromoji.NewJapaneseCompletionFilterFactory(mode)
		if f == nil {
			t.Fatalf("NewJapaneseCompletionFilterFactory(%v) returned nil", mode)
		}
	}
}

// TestFactories_RandomData exercises factories with random data once the SPI
// registry and binary dictionaries are wired.
//
// Deviation: skipped — requires SPI factory registry + live JapaneseTokenizer
// backed by binary dictionaries.  Tracked in backlog task #2691.
func TestFactories_RandomData(t *testing.T) {
	t.Fatal("requires SPI factory registry and binary dictionary I/O (deferred to codec sprint)")
}
