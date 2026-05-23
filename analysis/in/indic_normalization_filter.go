// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package in

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// IndicNormalizationFilter applies IndicNormalizer to each token.
//
// Go port of org.apache.lucene.analysis.in.IndicNormalizationFilter
// (Apache Lucene 10.4.0).
type IndicNormalizationFilter struct {
	*analysis.BaseTokenFilter
	normalizer *IndicNormalizer
}

// NewIndicNormalizationFilter creates a new IndicNormalizationFilter.
func NewIndicNormalizationFilter(input analysis.TokenStream) *IndicNormalizationFilter {
	return &IndicNormalizationFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		normalizer:      NewIndicNormalizer(),
	}
}

// IncrementToken processes the next token and normalises its term.
func (f *IndicNormalizationFilter) IncrementToken() (bool, error) {
	ok, err := f.BaseTokenFilter.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}

	attrSrc := f.GetAttributeSource()
	if termAttrI := attrSrc.GetAttribute(analysis.CharTermAttributeType); termAttrI != nil {
		termAttr := termAttrI.(analysis.CharTermAttribute)
		runes := []rune(termAttr.String())
		newLen := f.normalizer.Normalize(runes, len(runes))
		termAttr.SetValue(string(runes[:newLen]))
	}
	return true, nil
}

// Ensure interface compliance.
var _ analysis.TokenFilter = (*IndicNormalizationFilter)(nil)
