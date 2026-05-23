// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lv

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// LatvianStemFilter applies LatvianStemmer to each token. Tokens marked as
// keywords (via KeywordAttribute) are passed through unchanged.
//
// This is the Go port of
// org.apache.lucene.analysis.lv.LatvianStemFilter from
// Apache Lucene 10.4.0.
type LatvianStemFilter struct {
	*analysis.BaseTokenFilter

	stemmer     LatvianStemmer
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewLatvianStemFilter creates a LatvianStemFilter wrapping input.
func NewLatvianStemFilter(input analysis.TokenStream) *LatvianStemFilter {
	f := &LatvianStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
			f.keywordAttr = a.(analysis.KeywordAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token, applying Latvian stemming.
func (f *LatvianStemFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}

	term := f.termAttr.String()
	runes := []rune(term)
	// Provide extra capacity so unpalatalize can access s[length].
	buf := make([]rune, len(runes)+4)
	copy(buf, runes)
	n := f.stemmer.Stem(buf, len(runes))
	if n != len(runes) || string(buf[:n]) != term {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(string(buf[:n]))
	}
	return true, nil
}

// Ensure LatvianStemFilter implements TokenFilter.
var _ analysis.TokenFilter = (*LatvianStemFilter)(nil)

// LatvianStemFilterFactory creates LatvianStemFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.lv.LatvianStemFilterFactory from
// Apache Lucene 10.4.0.
type LatvianStemFilterFactory struct{}

// NewLatvianStemFilterFactory creates a LatvianStemFilterFactory.
func NewLatvianStemFilterFactory() *LatvianStemFilterFactory { return &LatvianStemFilterFactory{} }

// Create creates a LatvianStemFilter wrapping input.
func (f *LatvianStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewLatvianStemFilter(input)
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*LatvianStemFilterFactory)(nil)
