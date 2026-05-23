// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"io"
)

// ICUNormalizer2CharFilterFactory creates ICUNormalizer2CharFilter instances.
//
// Go port of
// org.apache.lucene.analysis.icu.ICUNormalizer2CharFilterFactory
// (Apache Lucene 10.4.0).
//
// Supported configuration options (mirrors Java):
//   - form:   normalization form; one of "nfc", "nfkc", "nfkc_cf" (default),
//             "nfkc_scf", "nfd", "nfkd".
//   - mode:   "compose" (default) or "decompose".
//   - filter: A UnicodeSet; if non-nil the normalizer is wrapped in a
//             FilteredNormalizer2.
//
// Deviation: The Java factory parses its form/mode from a Map<String,String>
// and the filter from a UnicodeSet pattern string. This Go factory accepts the
// already-resolved arguments because Go has no SPI loader for pattern strings.
type ICUNormalizer2CharFilterFactory struct {
	normalizer Normalizer2
}

// NewICUNormalizer2CharFilterFactoryDefault creates a factory that applies
// NFKC_CaseFold normalisation (the Lucene default "nfkc_cf", compose).
func NewICUNormalizer2CharFilterFactoryDefault() *ICUNormalizer2CharFilterFactory {
	return &ICUNormalizer2CharFilterFactory{
		normalizer: NewNFKCCaseFoldNormalizer(),
	}
}

// NewICUNormalizer2CharFilterFactory creates a factory with the given
// normalizer.
func NewICUNormalizer2CharFilterFactory(form string, mode NormalizerMode) *ICUNormalizer2CharFilterFactory {
	return &ICUNormalizer2CharFilterFactory{
		normalizer: NewNormalizer2(form, mode),
	}
}

// NewICUNormalizer2CharFilterFactoryWithFilter creates a factory that wraps
// the normalizer with a FilteredNormalizer2.
func NewICUNormalizer2CharFilterFactoryWithFilter(
	form string,
	mode NormalizerMode,
	filter UnicodeSet,
) *ICUNormalizer2CharFilterFactory {
	inner := NewNormalizer2(form, mode)
	if filter != nil {
		inner = NewFilteredNormalizer2(inner, filter)
	}
	return &ICUNormalizer2CharFilterFactory{normalizer: inner}
}

// Create wraps the given reader with an ICUNormalizer2CharFilter.
func (f *ICUNormalizer2CharFilterFactory) Create(input io.Reader) io.Reader {
	return NewICUNormalizer2CharFilterWith(input, f.normalizer)
}

// Normalize is an alias for Create, matching the Java factory contract.
func (f *ICUNormalizer2CharFilterFactory) Normalize(input io.Reader) io.Reader {
	return f.Create(input)
}
