// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/ko/tokenattributes"
)

// DefaultStopTags is the default set of POS tags removed by
// KoreanPartOfSpeechStopFilter.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanPartOfSpeechStopFilter.DEFAULT_STOP_TAGS
// from Apache Lucene 10.4.0.
var DefaultStopTags = map[dict.POSTag]struct{}{
	dict.POSTagEP:  {},
	dict.POSTagEF:  {},
	dict.POSTagEC:  {},
	dict.POSTagETN: {},
	dict.POSTagETM: {},
	dict.POSTagIC:  {},
	dict.POSTagJKS: {},
	dict.POSTagJKC: {},
	dict.POSTagJKG: {},
	dict.POSTagJKO: {},
	dict.POSTagJKB: {},
	dict.POSTagJKV: {},
	dict.POSTagJKQ: {},
	dict.POSTagJX:  {},
	dict.POSTagJC:  {},
	dict.POSTagMAG: {},
	dict.POSTagMAJ: {},
	dict.POSTagMM:  {},
	dict.POSTagSP:  {},
	dict.POSTagSSC: {},
	dict.POSTagSSO: {},
	dict.POSTagSC:  {},
	dict.POSTagSE:  {},
	dict.POSTagXPN: {},
	dict.POSTagXSA: {},
	dict.POSTagXSN: {},
	dict.POSTagXSV: {},
	dict.POSTagUNA: {},
	dict.POSTagNA:  {},
	dict.POSTagVSV: {},
}

// KoreanPartOfSpeechStopFilter removes tokens whose left POS tag is in the
// configured stop-tag set.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanPartOfSpeechStopFilter from Apache
// Lucene 10.4.0.
type KoreanPartOfSpeechStopFilter struct {
	*analysis.BaseTokenFilter

	stopTags map[dict.POSTag]struct{}
	posAttr  tokenattributes.PartOfSpeechAttribute
}

// NewKoreanPartOfSpeechStopFilter creates a filter using the default stop tags.
func NewKoreanPartOfSpeechStopFilter(input analysis.TokenStream) *KoreanPartOfSpeechStopFilter {
	return NewKoreanPartOfSpeechStopFilterWithTags(input, DefaultStopTags)
}

// NewKoreanPartOfSpeechStopFilterWithTags creates a filter using a custom stop
// tag set.
func NewKoreanPartOfSpeechStopFilterWithTags(
	input analysis.TokenStream,
	stopTags map[dict.POSTag]struct{},
) *KoreanPartOfSpeechStopFilter {
	f := &KoreanPartOfSpeechStopFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stopTags:        stopTags,
	}
	if src := f.BaseTokenFilter.GetAttributeSource(); src != nil {
		if a := src.GetAttribute(tokenattributes.PartOfSpeechAttributeType); a != nil {
			f.posAttr, _ = a.(tokenattributes.PartOfSpeechAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token that is not in the stop-tag set.
func (f *KoreanPartOfSpeechStopFilter) IncrementToken() (bool, error) {
	for {
		ok, err := f.BaseTokenFilter.IncrementToken()
		if !ok || err != nil {
			return ok, err
		}
		if f.accept() {
			return true, nil
		}
	}
}

// accept reports whether the current token should be passed through.
func (f *KoreanPartOfSpeechStopFilter) accept() bool {
	if f.posAttr == nil {
		return true
	}
	leftPOS := f.posAttr.GetLeftPOS()
	if leftPOS == dict.POSTagUNKNOWN {
		return true
	}
	_, blocked := f.stopTags[leftPOS]
	return !blocked
}

// Ensure KoreanPartOfSpeechStopFilter satisfies TokenStream.
var _ analysis.TokenStream = (*KoreanPartOfSpeechStopFilter)(nil)
