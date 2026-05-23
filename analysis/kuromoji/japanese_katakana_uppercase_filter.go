// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// katakanaUpperMappings maps small katakana (捨て仮名) to their full-size
// equivalents. ㇷ゚ (two runes) is handled separately in the loop.
var katakanaUpperMappings = map[rune]rune{
	'ァ': 'ア',
	'ィ': 'イ',
	'ゥ': 'ウ',
	'ェ': 'エ',
	'ォ': 'オ',
	'ヵ': 'カ',
	'ㇰ': 'ク',
	'ヶ': 'ケ',
	'ㇱ': 'シ',
	'ㇲ': 'ス',
	'ッ': 'ツ',
	'ㇳ': 'ト',
	'ㇴ': 'ヌ',
	'ㇵ': 'ハ',
	'ㇶ': 'ヒ',
	'ㇷ': 'フ', // standalone mapping; ㇷ゚ → プ handled in loop
	'ㇸ': 'ヘ',
	'ㇹ': 'ホ',
	'ㇺ': 'ム',
	'ャ': 'ヤ',
	'ュ': 'ユ',
	'ョ': 'ヨ',
	'ㇻ': 'ラ',
	'ㇼ': 'リ',
	'ㇽ': 'ル',
	'ㇾ': 'レ',
	'ㇿ': 'ロ',
	'ヮ': 'ワ',
}

// JapaneseKatakanaUppercaseFilter normalizes small katakana letters (捨て仮名)
// into their full-size equivalents. For instance, "ストップウォッチ" becomes
// "ストツプウオツチ".
//
// The two-character sequence ㇷ゚ (U+31F7 + U+309A) is replaced by プ (U+30D7)
// and the token length is reduced by one accordingly.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseKatakanaUppercaseFilter from Apache
// Lucene 10.4.0.
type JapaneseKatakanaUppercaseFilter struct {
	*analysis.BaseTokenFilter
	termAttr analysis.CharTermAttribute
}

// NewJapaneseKatakanaUppercaseFilter creates a new filter wrapping input.
func NewJapaneseKatakanaUppercaseFilter(input analysis.TokenStream) *JapaneseKatakanaUppercaseFilter {
	f := &JapaneseKatakanaUppercaseFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	if src := f.GetAttributeSource(); src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token and applies the katakana
// uppercase normalization.
func (f *JapaneseKatakanaUppercaseFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr != nil {
		in := []rune(f.termAttr.String())
		out := make([]rune, 0, len(in))
		for i := 0; i < len(in); i++ {
			r := in[i]
			// ㇷ゚ (U+31F7 followed by U+309A combining katakana-hiragana voiced
			// iteration mark) → プ (U+30D7)
			if r == 'ㇷ' && i+1 < len(in) && in[i+1] == '゚' {
				out = append(out, 'プ')
				i++ // consume the combining mark
				continue
			}
			if mapped, ok := katakanaUpperMappings[r]; ok {
				out = append(out, mapped)
			} else {
				out = append(out, r)
			}
		}
		if len(out) != len(in) || string(out) != f.termAttr.String() {
			f.termAttr.SetValue(string(out))
		}
	}
	return true, nil
}

// Ensure JapaneseKatakanaUppercaseFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapaneseKatakanaUppercaseFilter)(nil)
