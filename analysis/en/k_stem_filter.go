// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package en

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// KStemFilter is a high-performance KStem token filter for English.
//
// Go port of org.apache.lucene.analysis.en.KStemFilter (Apache Lucene 10.4.0).
//
// All terms must already be lowercased for this filter to work correctly.
// The filter respects the KeywordAttribute: tokens marked as keywords are
// passed through unchanged.
//
// Reference: Krovetz, R., "Viewing Morphology as an Inference Process",
// SIGIR 1993.
type KStemFilter struct {
	*analysis.BaseTokenFilter

	stemmer *kStemmer
}

// NewKStemFilter creates a new KStemFilter wrapping the given input stream.
func NewKStemFilter(input analysis.TokenStream) *KStemFilter {
	return &KStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stemmer:         newKStemmer(),
	}
}

// IncrementToken advances to the next token and applies KStem stemming.
func (f *KStemFilter) IncrementToken() (bool, error) {
	ok, err := f.BaseTokenFilter.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}

	attrSrc := f.GetAttributeSource()

	// Respect KeywordAttribute: skip stemming for keyword tokens.
	if kwAttr := attrSrc.GetAttribute(analysis.KeywordAttributeType); kwAttr != nil {
		if kwAttr.(analysis.KeywordAttribute).IsKeywordToken() {
			return true, nil
		}
	}

	if termAttrI := attrSrc.GetAttribute(analysis.CharTermAttributeType); termAttrI != nil {
		termAttr := termAttrI.(analysis.CharTermAttribute)
		term := termAttr.String()
		if stemmed := f.stemmer.stem(term); stemmed != term {
			termAttr.SetValue(stemmed)
		}
	}

	return true, nil
}

// Ensure interface compliance.
var _ analysis.TokenFilter = (*KStemFilter)(nil)
