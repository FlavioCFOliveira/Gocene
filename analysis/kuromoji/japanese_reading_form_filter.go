// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/tokenattributes"
)

const (
	hiraganaStart = 'ぁ'
	hiraganaEnd   = 'ゖ'
)

// JapaneseReadingFormFilter replaces the term attribute with the reading of a
// token in katakana (default) or romaji form.
//
// When a token has no reading and contains hiragana, the hiragana characters
// are shifted to their katakana equivalents and used as the reading.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseReadingFormFilter from Apache Lucene
// 10.4.0.
type JapaneseReadingFormFilter struct {
	*analysis.BaseTokenFilter
	termAttr    analysis.CharTermAttribute
	readingAttr tokenattributes.ReadingAttribute
	useRomaji   bool
}

// NewJapaneseReadingFormFilter creates a filter that emits the reading of each
// token. When useRomaji is true, the reading is romanized; otherwise it is
// emitted as katakana.
func NewJapaneseReadingFormFilter(input analysis.TokenStream, useRomaji bool) *JapaneseReadingFormFilter {
	f := &JapaneseReadingFormFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		useRomaji:       useRomaji,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(tokenattributes.ReadingAttributeType); a != nil {
			f.readingAttr = a.(tokenattributes.ReadingAttribute)
		}
	}
	return f
}

// NewJapaneseReadingFormFilterKatakana creates a filter that emits katakana
// readings (default mode).
func NewJapaneseReadingFormFilterKatakana(input analysis.TokenStream) *JapaneseReadingFormFilter {
	return NewJapaneseReadingFormFilter(input, false)
}

// IncrementToken advances to the next token and replaces its term with the
// reading (katakana or romaji).
func (f *JapaneseReadingFormFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}

	var reading string
	if f.readingAttr != nil {
		reading = f.readingAttr.Reading()
	}

	// When the token is OOV and contains hiragana, synthesise reading by
	// converting hiragana to katakana.
	if reading == "" && containsHiragana(f.termAttr.String()) {
		term := []rune(f.termAttr.String())
		buf := make([]rune, len(term))
		for i, r := range term {
			if r >= hiraganaStart && r <= hiraganaEnd {
				buf[i] = r + 0x60
			} else {
				buf[i] = r
			}
		}
		reading = string(buf)
	}

	if f.useRomaji {
		if reading == "" {
			// OOV without hiragana: romanize the term text directly
			var sb strings.Builder
			dict.GetRomanizationInto(&sb, f.termAttr.String())
			f.termAttr.SetValue(sb.String())
		} else {
			var sb strings.Builder
			dict.GetRomanizationInto(&sb, reading)
			f.termAttr.SetValue(sb.String())
		}
	} else if reading != "" {
		f.termAttr.SetValue(reading)
	}
	return true, nil
}

// containsHiragana reports whether s contains any hiragana character.
func containsHiragana(s string) bool {
	for _, r := range s {
		if r >= hiraganaStart && r <= hiraganaEnd {
			return true
		}
	}
	return false
}

// Ensure JapaneseReadingFormFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapaneseReadingFormFilter)(nil)
