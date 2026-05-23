// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package es

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// SpanishPluralStemFilter applies [SpanishPluralStemmer] to each token.
//
// Tokens marked as keywords (via KeywordAttribute) are passed through unchanged.
//
// This is the Go port of
// org.apache.lucene.analysis.es.SpanishPluralStemFilter from Apache Lucene
// 10.4.0.
type SpanishPluralStemFilter struct {
	*analysis.BaseTokenFilter

	stemmer     *SpanishPluralStemmer
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewSpanishPluralStemFilter creates a new SpanishPluralStemFilter.
func NewSpanishPluralStemFilter(input analysis.TokenStream) *SpanishPluralStemFilter {
	f := &SpanishPluralStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stemmer:         NewSpanishPluralStemmer(),
	}
	src := f.GetAttributeSource()
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr = a.(analysis.CharTermAttribute)
	} else {
		f.termAttr = analysis.NewCharTermAttribute()
		src.AddAttributeImpl(f.termAttr)
	}
	if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
		f.keywordAttr = a.(analysis.KeywordAttribute)
	}
	return f
}

// IncrementToken advances to the next token.
func (f *SpanishPluralStemFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.keywordAttr == nil || !f.keywordAttr.IsKeywordToken() {
		term := f.termAttr.String()
		runes := []rune(term)
		newLen := f.stemmer.Stem(runes, len(runes))
		if newLen != len(runes) {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(string(runes[:newLen]))
		}
	}
	return true, nil
}

// Reset resets the filter.
func (f *SpanishPluralStemFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		return r.Reset()
	}
	return nil
}

// ── SpanishPluralStemFilterFactory ───────────────────────────────────────────

// SpanishPluralStemFilterFactory creates [SpanishPluralStemFilter] instances.
//
// This is the Go port of
// org.apache.lucene.analysis.es.SpanishPluralStemFilterFactory from Apache
// Lucene 10.4.0.
type SpanishPluralStemFilterFactory struct{}

// NewSpanishPluralStemFilterFactory creates a new SpanishPluralStemFilterFactory.
func NewSpanishPluralStemFilterFactory() *SpanishPluralStemFilterFactory {
	return &SpanishPluralStemFilterFactory{}
}

// Create returns a new SpanishPluralStemFilter wrapping input.
func (f *SpanishPluralStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewSpanishPluralStemFilter(input)
}

// Ensure factory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*SpanishPluralStemFilterFactory)(nil)
