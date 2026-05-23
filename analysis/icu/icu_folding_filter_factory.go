// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ICUFoldingFilterFactory creates ICUFoldingFilter instances.
//
// Go port of org.apache.lucene.analysis.icu.ICUFoldingFilterFactory
// (Apache Lucene 10.4.0).
//
// Supports an optional "filter" parameter accepting a UnicodeSet; when
// provided, the folding normaliser is wrapped in a FilteredNormalizer2 so
// that code points outside the set are left unchanged.
//
// Deviation: The Java factory accepts a UnicodeSet pattern string and parses
// it via com.ibm.icu.text.UnicodeSet. This Go factory accepts a pre-built
// UnicodeSet or nil.
type ICUFoldingFilterFactory struct {
	normalizer Normalizer2
}

// NewICUFoldingFilterFactory creates a factory that applies UTR#30 folding
// (approximated as NFKC+CaseFold in Gocene).
func NewICUFoldingFilterFactory() *ICUFoldingFilterFactory {
	return &ICUFoldingFilterFactory{normalizer: DefaultFoldingNormalizer}
}

// NewICUFoldingFilterFactoryWithFilter creates a factory that additionally
// restricts folding to code points in the given UnicodeSet.
func NewICUFoldingFilterFactoryWithFilter(filter UnicodeSet) *ICUFoldingFilterFactory {
	var n Normalizer2 = DefaultFoldingNormalizer
	if filter != nil {
		n = NewFilteredNormalizer2(DefaultFoldingNormalizer, filter)
	}
	return &ICUFoldingFilterFactory{normalizer: n}
}

// Create wraps input with an ICUFoldingFilter using this factory's normaliser.
func (f *ICUFoldingFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewICUFoldingFilterWith(input, f.normalizer)
}

// Normalize is an alias for Create.
func (f *ICUFoldingFilterFactory) Normalize(input analysis.TokenStream) analysis.TokenFilter {
	return f.Create(input)
}

// Ensure compile-time interface satisfaction.
var _ analysis.TokenFilterFactory = (*ICUFoldingFilterFactory)(nil)
