// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ICUFoldingFilter is a TokenFilter that applies search-term folding to
// Unicode text, applying foldings from UTR#30 Character Foldings.
//
// Go port of org.apache.lucene.analysis.icu.ICUFoldingFilter
// (Apache Lucene 10.4.0).
//
// This filter applies the following foldings to Unicode text:
//   - Accent removal, Case folding, Canonical duplicates folding
//   - Dashes, Diacritics, Greek letterforms, Han Radical, Hebrew Alternates
//   - Jamo, Letterforms, Math symbol, Multigraph Expansions, Native digits
//   - No-break, Overline, Positional forms, Small forms, Space
//   - Spacing Accents, Subscript, Superscript, Suzhou Numeral, Symbol
//   - Underline, Vertical forms, Width folding
//
// Additionally, Default Ignorables are removed and text is normalised to NFKC.
//
// Deviation: The Java original loads the UTR#30 normaliser from the embedded
// resource "utr30.nrm" (a compiled ICU data file) and uses ICU4J's Normalizer2
// to apply it. Go has no equivalent UTR#30 data loader. ICUFoldingFilter
// therefore uses NFKC+CaseFold normalisation (the closest standard-library
// equivalent). The ICU4J UTR#30 normaliser applies additional foldings beyond
// NFKC_CF; callers requiring full UTR#30 compatibility must supply a custom
// Normalizer2 implementation via NewICUFoldingFilterWith.
//
// DefaultFoldingNormalizer is the normaliser used by ICUFoldingFilter when no
// custom normaliser is supplied. It is NFKC+CaseFold.
var DefaultFoldingNormalizer Normalizer2 = NewNFKCCaseFoldNormalizer()

// ICUFoldingFilter wraps ICUNormalizer2Filter with the default folding
// normaliser.
type ICUFoldingFilter struct {
	*ICUNormalizer2Filter
}

// NewICUFoldingFilter creates a new ICUFoldingFilter on the specified input
// using DefaultFoldingNormalizer.
func NewICUFoldingFilter(input analysis.TokenStream) *ICUFoldingFilter {
	return &ICUFoldingFilter{
		ICUNormalizer2Filter: NewICUNormalizer2FilterWith(input, DefaultFoldingNormalizer),
	}
}

// NewICUFoldingFilterWith creates a new ICUFoldingFilter using the specified
// normaliser (allows callers to supply a custom UTR#30-compatible normaliser).
func NewICUFoldingFilterWith(input analysis.TokenStream, normalizer Normalizer2) *ICUFoldingFilter {
	return &ICUFoldingFilter{
		ICUNormalizer2Filter: NewICUNormalizer2FilterWith(input, normalizer),
	}
}

// Ensure compile-time interface satisfaction.
var _ analysis.TokenFilter = (*ICUFoldingFilter)(nil)
