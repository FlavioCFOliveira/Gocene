// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/ko/tokenattributes"
)

// KoreanReadingFormFilter replaces the term text with the reading from
// ReadingAttribute, which is the Hangul transcription of Hanja characters.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanReadingFormFilter from Apache Lucene
// 10.4.0.
type KoreanReadingFormFilter struct {
	*analysis.BaseTokenFilter

	termAttr    analysis.CharTermAttribute
	readingAttr tokenattributes.ReadingAttribute
}

// NewKoreanReadingFormFilter creates a KoreanReadingFormFilter wrapping input.
func NewKoreanReadingFormFilter(input analysis.TokenStream) *KoreanReadingFormFilter {
	f := &KoreanReadingFormFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	src := f.BaseTokenFilter.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr, _ = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(tokenattributes.ReadingAttributeType); a != nil {
			f.readingAttr, _ = a.(tokenattributes.ReadingAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token. If the token has a non-empty
// reading, the term text is replaced with the reading.
func (f *KoreanReadingFormFilter) IncrementToken() (bool, error) {
	ok, err := f.BaseTokenFilter.IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.readingAttr != nil {
		reading := f.readingAttr.GetReading()
		if reading != "" && f.termAttr != nil {
			f.termAttr.SetValue(reading)
		}
	}
	return true, nil
}

// Ensure KoreanReadingFormFilter satisfies TokenStream.
var _ analysis.TokenStream = (*KoreanReadingFormFilter)(nil)
