// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// DefaultMinimumKatakanaLength is the default minimum token length for the
// katakana stemmer. Tokens shorter than this are not stemmed.
const DefaultMinimumKatakanaLength = 4

// hiraganaKatakanaProlongedSoundMark is U+30FC, the katakana-hiragana prolonged
// sound mark (ー).
const hiraganaKatakanaProlongedSoundMark = 'ー'

// JapaneseKatakanaStemFilter normalizes common katakana spelling variations
// ending in a long sound character (U+30FC) by removing that character. Only
// katakana words longer than minimumKatakanaLength are stemmed.
//
// Note: only full-width katakana characters are supported. Use CJKWidthFilter
// to convert half-width katakana before applying this filter.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseKatakanaStemFilter from Apache Lucene
// 10.4.0.
type JapaneseKatakanaStemFilter struct {
	*analysis.BaseTokenFilter
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
	minimumLen  int
}

// NewJapaneseKatakanaStemFilter creates a filter that stems katakana tokens
// longer than minimumLength.
func NewJapaneseKatakanaStemFilter(input analysis.TokenStream, minimumLength int) *JapaneseKatakanaStemFilter {
	if minimumLength < 1 {
		panic("kuromoji: JapaneseKatakanaStemFilter minimumLength must be >= 1")
	}
	f := &JapaneseKatakanaStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		minimumLen:      minimumLength,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
			f.keywordAttr = a.(analysis.KeywordAttribute)
		}
	}
	return f
}

// NewJapaneseKatakanaStemFilterDefault creates a filter with the default
// minimum length (4).
func NewJapaneseKatakanaStemFilterDefault(input analysis.TokenStream) *JapaneseKatakanaStemFilter {
	return NewJapaneseKatakanaStemFilter(input, DefaultMinimumKatakanaLength)
}

// IncrementToken advances to the next token and stems katakana if applicable.
func (f *JapaneseKatakanaStemFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr != nil {
		isKeyword := f.keywordAttr != nil && f.keywordAttr.IsKeywordToken()
		if !isKeyword {
			term := []rune(f.termAttr.String())
			stemmed := stem(term, f.minimumLen)
			if len(stemmed) != len(term) {
				f.termAttr.SetValue(string(stemmed))
			}
		}
	}
	return true, nil
}

// stem returns the stemmed rune slice for term.
func stem(term []rune, minLen int) []rune {
	if len(term) < minLen {
		return term
	}
	if !isKatakana(term) {
		return term
	}
	if term[len(term)-1] == hiraganaKatakanaProlongedSoundMark {
		return term[:len(term)-1]
	}
	return term
}

// isKatakana returns true if every rune in term is full-width katakana.
func isKatakana(term []rune) bool {
	for _, r := range term {
		if unicode.Is(unicode.Katakana, r) {
			continue
		}
		return false
	}
	return true
}

// Ensure JapaneseKatakanaStemFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapaneseKatakanaStemFilter)(nil)
