// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/tokenattributes"
)

// JapanesePartOfSpeechStopFilter removes tokens whose part-of-speech tag is
// in the configured stop set.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapanesePartOfSpeechStopFilter from Apache
// Lucene 10.4.0.
type JapanesePartOfSpeechStopFilter struct {
	*analysis.FilteringTokenFilter
	stopTags map[string]struct{}
	posAttr  tokenattributes.PartOfSpeechAttribute
}

// NewJapanesePartOfSpeechStopFilter creates a filter that removes tokens
// whose POS tag is contained in stopTags.
func NewJapanesePartOfSpeechStopFilter(
	input analysis.TokenStream,
	stopTags map[string]struct{},
) *JapanesePartOfSpeechStopFilter {
	if stopTags == nil {
		panic("kuromoji: JapanesePartOfSpeechStopFilter stopTags must not be nil")
	}
	f := &JapanesePartOfSpeechStopFilter{stopTags: stopTags}

	// We need to build the accept closure before constructing FilteringTokenFilter.
	// Capture posAttr via a closure that will be populated after construction.
	var posAttr tokenattributes.PartOfSpeechAttribute
	accept := func() (bool, error) {
		if posAttr == nil {
			return true, nil
		}
		pos := posAttr.PartOfSpeech()
		if pos == "" {
			return true, nil
		}
		_, blocked := stopTags[pos]
		return !blocked, nil
	}

	f.FilteringTokenFilter = analysis.NewFilteringTokenFilter(input, accept)

	// Retrieve posAttr from the shared AttributeSource after construction.
	if src := f.FilteringTokenFilter.GetAttributeSource(); src != nil {
		if a := src.GetAttribute(tokenattributes.PartOfSpeechAttributeType); a != nil {
			posAttr = a.(tokenattributes.PartOfSpeechAttribute)
			f.posAttr = posAttr
		}
	}
	return f
}

// Ensure JapanesePartOfSpeechStopFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapanesePartOfSpeechStopFilter)(nil)
