// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ICUNormalizer2Filter is a TokenFilter that normalises token text using a
// Normalizer2.
//
// Go port of org.apache.lucene.analysis.icu.ICUNormalizer2Filter
// (Apache Lucene 10.4.0).
//
// With this filter you can normalise text in the following ways:
//   - NFKC + Case Folding + removing Ignorables (the default, "nfkc_cf")
//   - Using a standard normalisation mode (NFC, NFD, NFKC, NFKD)
//   - Based on rules from a custom normalisation mapping
//
// Deviation: The Java implementation uses com.ibm.icu.text.Normalizer2 and
// calls normalizer.quickCheck(termAtt) to skip already-normalised tokens.
// The Go port uses the Normalizer2 interface; QuickCheck returns
// Normalizer2QuickCheckYes to skip tokens and Normalizer2QuickCheckMaybe to
// force normalisation.
type ICUNormalizer2Filter struct {
	*analysis.BaseTokenFilter
	normalizer Normalizer2
	termAttr   analysis.CharTermAttribute
}

// NewICUNormalizer2Filter creates a filter that applies NFKC+CaseFold
// normalisation (the Lucene default: "nfkc_cf").
func NewICUNormalizer2Filter(input analysis.TokenStream) *ICUNormalizer2Filter {
	return NewICUNormalizer2FilterWith(input, NewNFKCCaseFoldNormalizer())
}

// NewICUNormalizer2FilterWith creates a filter that applies the supplied
// normalizer.
func NewICUNormalizer2FilterWith(input analysis.TokenStream, normalizer Normalizer2) *ICUNormalizer2Filter {
	f := &ICUNormalizer2Filter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		normalizer:      normalizer,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if attr := src.GetAttribute(analysis.CharTermAttributeType); attr != nil {
			f.termAttr = attr.(analysis.CharTermAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token and normalises its text.
func (f *ICUNormalizer2Filter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr != nil {
		text := f.termAttr.String()
		if f.normalizer.QuickCheck(text) != Normalizer2QuickCheckYes {
			normalised := f.normalizer.Normalize(text)
			f.termAttr.SetValue(normalised)
		}
	}
	return true, nil
}

// GetInput returns the wrapped input TokenStream.
func (f *ICUNormalizer2Filter) GetInput() analysis.TokenStream {
	return f.BaseTokenFilter.GetInput()
}

// Ensure compile-time interface satisfaction.
var _ analysis.TokenFilter = (*ICUNormalizer2Filter)(nil)
