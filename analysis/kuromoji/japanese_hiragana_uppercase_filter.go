// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// hiraganaUpperMappings maps small hiragana (捨て仮名) to their normal counterparts.
var hiraganaUpperMappings = map[rune]rune{
	'ぁ': 'あ',
	'ぃ': 'い',
	'ぅ': 'う',
	'ぇ': 'え',
	'ぉ': 'お',
	'っ': 'つ',
	'ゃ': 'や',
	'ゅ': 'ゆ',
	'ょ': 'よ',
	'ゎ': 'わ',
	'ゕ': 'か',
	'ゖ': 'け',
}

// JapaneseHiraganaUppercaseFilter normalizes small hiragana letters (捨て仮名)
// into their full-size equivalents. For instance, "ちょっとまって" becomes
// "ちよつとまつて".
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseHiraganaUppercaseFilter from Apache
// Lucene 10.4.0.
type JapaneseHiraganaUppercaseFilter struct {
	*analysis.BaseTokenFilter
	termAttr analysis.CharTermAttribute
}

// NewJapaneseHiraganaUppercaseFilter creates a new filter wrapping input.
func NewJapaneseHiraganaUppercaseFilter(input analysis.TokenStream) *JapaneseHiraganaUppercaseFilter {
	f := &JapaneseHiraganaUppercaseFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	if src := f.GetAttributeSource(); src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token and applies the hiragana
// uppercase normalization.
func (f *JapaneseHiraganaUppercaseFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr != nil {
		term := []rune(f.termAttr.String())
		changed := false
		for i, r := range term {
			if mapped, ok := hiraganaUpperMappings[r]; ok {
				term[i] = mapped
				changed = true
			}
		}
		if changed {
			f.termAttr.SetValue(string(term))
		}
	}
	return true, nil
}

// Ensure JapaneseHiraganaUppercaseFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapaneseHiraganaUppercaseFilter)(nil)
