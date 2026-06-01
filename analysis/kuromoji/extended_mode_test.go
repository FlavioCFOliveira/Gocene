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
//
// Deviation: all sub-tests that rely on IncrementToken producing tokens (which
// requires binary dictionaries to be loaded) are skipped until that
// prerequisite is available.  The structural tests (tokenizer and analyzer
// construction in EXTENDED mode) run immediately.

import (
	"testing"

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
//
// Deviation: the binary dictionary is now loaded but the Viterbi segmentation
// algorithm is not yet implemented. IncrementToken returns an empty stream.
// Full token-producing tests are deferred until Viterbi lands.
func TestExtendedMode_Surrogates(t *testing.T) {
	t.Fatal("Viterbi segmentation algorithm not yet implemented — JapaneseTokenizer.IncrementToken is a stub")
}

// TestExtendedMode_Surrogates2 stress-tests the tokenizer with random
// Unicode strings, verifying no surrogate pairs are split.
//
// Deviation: same Viterbi prerequisite as TestExtendedMode_Surrogates.
func TestExtendedMode_Surrogates2(t *testing.T) {
	t.Fatal("Viterbi segmentation algorithm not yet implemented — JapaneseTokenizer.IncrementToken is a stub")
}

// TestExtendedMode_RandomStrings blasts random strings through the analyzer.
//
// Deviation: same Viterbi prerequisite.
func TestExtendedMode_RandomStrings(t *testing.T) {
	t.Fatal("Viterbi segmentation algorithm not yet implemented — JapaneseTokenizer.IncrementToken is a stub")
}

// TestExtendedMode_RandomHugeStrings blasts random large strings.
//
// Deviation: same Viterbi prerequisite.
func TestExtendedMode_RandomHugeStrings(t *testing.T) {
	t.Fatal("Viterbi segmentation algorithm not yet implemented — JapaneseTokenizer.IncrementToken is a stub")
}
