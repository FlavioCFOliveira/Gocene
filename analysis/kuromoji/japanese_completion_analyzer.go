// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
)

// JapaneseCompletionAnalyzer is an analyzer for Japanese completion suggester.
// It combines JapaneseTokenizer, JapaneseCompletionFilter, and
// LowerCaseFilter into a single chain, preceded by CJK-width normalization.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseCompletionAnalyzer from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original delegates reader init to CJKWidthCharFilter.
// This Go port omits that reader wrapper (deferred to the CJK sprint) and
// applies the analyzer chain directly.
type JapaneseCompletionAnalyzer struct {
	analysis.BaseAnalyzer
	mode           CompletionMode
	userDictionary *dict.UserDictionary
}

// NewJapaneseCompletionAnalyzer creates a JapaneseCompletionAnalyzer with
// the given user dictionary and completion mode.
func NewJapaneseCompletionAnalyzer(userDictionary *dict.UserDictionary, mode CompletionMode) *JapaneseCompletionAnalyzer {
	return &JapaneseCompletionAnalyzer{mode: mode, userDictionary: userDictionary}
}

// NewJapaneseCompletionAnalyzerDefault creates a JapaneseCompletionAnalyzer
// with default settings (no user dictionary, index mode).
func NewJapaneseCompletionAnalyzerDefault() *JapaneseCompletionAnalyzer {
	return NewJapaneseCompletionAnalyzer(nil, DefaultCompletionMode)
}

// CreateComponents builds the analysis chain for the given field name.
func (a *JapaneseCompletionAnalyzer) CreateComponents(_ string) *analysis.TokenStreamComponents {
	tokenizer := NewJapaneseTokenizer(a.userDictionary, true, true, ModeNormal)
	var stream analysis.TokenStream = NewJapaneseCompletionFilter(tokenizer, a.mode)
	stream = analysis.NewLowerCaseFilter(stream)
	return analysis.NewTokenStreamComponents(tokenizer, stream)
}

// Ensure JapaneseCompletionAnalyzer implements analysis.Analyzer.
var _ analysis.Analyzer = (*JapaneseCompletionAnalyzer)(nil)
