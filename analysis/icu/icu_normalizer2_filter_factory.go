// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ICUNormalizer2FilterFactory creates ICUNormalizer2Filter instances.
//
// Go port of org.apache.lucene.analysis.icu.ICUNormalizer2FilterFactory
// (Apache Lucene 10.4.0).
//
// Supported configuration options (mirrors Java):
//   - form:   normalization form; one of "nfc", "nfkc", "nfkc_cf" (default),
//             "nfkc_scf", "nfd", "nfkd".
//   - mode:   "compose" (default) or "decompose".
//   - filter: a UnicodeSet; if non-nil the normalizer is wrapped in a
//             FilteredNormalizer2.
type ICUNormalizer2FilterFactory struct {
	normalizer Normalizer2
}

// NewICUNormalizer2FilterFactoryDefault creates a factory that applies
// NFKC_CaseFold normalisation (the Lucene default "nfkc_cf").
func NewICUNormalizer2FilterFactoryDefault() *ICUNormalizer2FilterFactory {
	return &ICUNormalizer2FilterFactory{normalizer: NewNFKCCaseFoldNormalizer()}
}

// NewICUNormalizer2FilterFactory creates a factory for the given form and mode.
func NewICUNormalizer2FilterFactory(form string, mode NormalizerMode) *ICUNormalizer2FilterFactory {
	return &ICUNormalizer2FilterFactory{normalizer: NewNormalizer2(form, mode)}
}

// NewICUNormalizer2FilterFactoryWithFilter creates a factory that wraps the
// normalizer with a FilteredNormalizer2 restricted to code points in filter.
func NewICUNormalizer2FilterFactoryWithFilter(
	form string,
	mode NormalizerMode,
	filter UnicodeSet,
) *ICUNormalizer2FilterFactory {
	inner := NewNormalizer2(form, mode)
	if filter != nil {
		inner = NewFilteredNormalizer2(inner, filter)
	}
	return &ICUNormalizer2FilterFactory{normalizer: inner}
}

// Create wraps input with an ICUNormalizer2Filter.
func (f *ICUNormalizer2FilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewICUNormalizer2FilterWith(input, f.normalizer)
}

// Normalize is an alias for Create.
func (f *ICUNormalizer2FilterFactory) Normalize(input analysis.TokenStream) analysis.TokenFilter {
	return f.Create(input)
}

// Ensure compile-time interface satisfaction.
var _ analysis.TokenFilterFactory = (*ICUNormalizer2FilterFactory)(nil)
