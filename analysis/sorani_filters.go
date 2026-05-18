// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// SoraniNormalizationFilter is a TokenFilter that applies
// SoraniNormalizer to each token's text.
//
// This is the Go port of
// org.apache.lucene.analysis.ckb.SoraniNormalizationFilter from
// Apache Lucene 10.4.0.
type SoraniNormalizationFilter struct {
	*BaseTokenFilter

	normalizer *SoraniNormalizer
	termAttr   CharTermAttribute
}

// NewSoraniNormalizationFilter wraps input with the normaliser.
func NewSoraniNormalizationFilter(input TokenStream) *SoraniNormalizationFilter {
	f := &SoraniNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewSoraniNormalizer(),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken applies Sorani normalisation to the current token's
// term text.
func (f *SoraniNormalizationFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.termAttr != nil {
		buf := f.termAttr.Buffer()[:f.termAttr.Length()]
		out := f.normalizer.NormalizeBytes(buf)
		f.termAttr.SetEmpty()
		f.termAttr.Append(out)
	}
	return true, nil
}

// Ensure SoraniNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*SoraniNormalizationFilter)(nil)

// SoraniNormalizationFilterFactory creates
// SoraniNormalizationFilter instances.
type SoraniNormalizationFilterFactory struct{}

// NewSoraniNormalizationFilterFactory returns a fresh factory.
func NewSoraniNormalizationFilterFactory() *SoraniNormalizationFilterFactory {
	return &SoraniNormalizationFilterFactory{}
}

// Create returns a SoraniNormalizationFilter wrapping input.
func (f *SoraniNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSoraniNormalizationFilter(input)
}

// Ensure SoraniNormalizationFilterFactory implements
// TokenFilterFactory.
var _ TokenFilterFactory = (*SoraniNormalizationFilterFactory)(nil)

// SoraniStemFilter is a TokenFilter that applies SoraniStemmer to
// every non-keyword token.
//
// This is the Go port of
// org.apache.lucene.analysis.ckb.SoraniStemFilter from Apache Lucene
// 10.4.0.
type SoraniStemFilter struct {
	*BaseTokenFilter

	stemmer     *SoraniStemmer
	termAttr    CharTermAttribute
	keywordAttr KeywordAttribute
}

// NewSoraniStemFilter wraps input with the Sorani stemmer.
func NewSoraniStemFilter(input TokenStream) *SoraniStemFilter {
	f := &SoraniStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewSoraniStemmer(),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(KeywordAttributeType); a != nil {
			f.keywordAttr = a.(KeywordAttribute)
		}
	}
	return f
}

// IncrementToken stems the current token unless the KeywordAttribute
// marks it as a keyword.
func (f *SoraniStemFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.termAttr == nil {
		return true, nil
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	text := f.termAttr.String()
	stem := f.stemmer.StemString(text)
	if stem != text {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(stem)
	}
	return true, nil
}

// Ensure SoraniStemFilter implements TokenFilter.
var _ TokenFilter = (*SoraniStemFilter)(nil)

// SoraniStemFilterFactory creates SoraniStemFilter instances.
type SoraniStemFilterFactory struct{}

// NewSoraniStemFilterFactory returns a fresh factory.
func NewSoraniStemFilterFactory() *SoraniStemFilterFactory {
	return &SoraniStemFilterFactory{}
}

// Create returns a SoraniStemFilter wrapping input.
func (f *SoraniStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSoraniStemFilter(input)
}

// Ensure SoraniStemFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*SoraniStemFilterFactory)(nil)
