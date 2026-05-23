// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pt

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ──────────────────────────────────────────────────────────────────────────────
// PortugueseMinimalStemFilter
// ──────────────────────────────────────────────────────────────────────────────

// PortugueseMinimalStemFilter applies PortugueseMinimalStemmer to each token.
// Tokens marked as keywords are left unchanged.
//
// Go port of org.apache.lucene.analysis.pt.PortugueseMinimalStemFilter
// (Apache Lucene 10.4.0).
type PortugueseMinimalStemFilter struct {
	*analysis.BaseTokenFilter
	stemmer     *PortugueseMinimalStemmer
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewPortugueseMinimalStemFilter wraps input with a PortugueseMinimalStemFilter.
func NewPortugueseMinimalStemFilter(input analysis.TokenStream) *PortugueseMinimalStemFilter {
	f := &PortugueseMinimalStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stemmer:         NewPortugueseMinimalStemmer(),
	}
	src := f.GetAttributeSource()
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr = a.(analysis.CharTermAttribute)
	}
	if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
		f.keywordAttr = a.(analysis.KeywordAttribute)
	}
	return f
}

// IncrementToken advances to the next token and applies Portuguese minimal stemming.
func (f *PortugueseMinimalStemFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	if f.termAttr != nil {
		runes := []rune(f.termAttr.String())
		newLen := f.stemmer.Stem(runes, len(runes))
		f.termAttr.SetValue(string(runes[:newLen]))
	}
	return true, nil
}

// Ensure interface compliance.
var _ analysis.TokenFilter = (*PortugueseMinimalStemFilter)(nil)

// ──────────────────────────────────────────────────────────────────────────────
// PortugueseMinimalStemFilterFactory
// ──────────────────────────────────────────────────────────────────────────────

// PortugueseMinimalStemFilterFactory creates PortugueseMinimalStemFilter instances.
//
// Go port of org.apache.lucene.analysis.pt.PortugueseMinimalStemFilterFactory
// (Apache Lucene 10.4.0).
type PortugueseMinimalStemFilterFactory struct{}

// NewPortugueseMinimalStemFilterFactory creates a new factory.
func NewPortugueseMinimalStemFilterFactory() *PortugueseMinimalStemFilterFactory {
	return &PortugueseMinimalStemFilterFactory{}
}

// Create creates a new PortugueseMinimalStemFilter.
func (f *PortugueseMinimalStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewPortugueseMinimalStemFilter(input)
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*PortugueseMinimalStemFilterFactory)(nil)

// ──────────────────────────────────────────────────────────────────────────────
// PortugueseStemFilter
// ──────────────────────────────────────────────────────────────────────────────

// PortugueseStemFilter applies PortugueseStemmer (full RSLP) to each token.
// Tokens marked as keywords are left unchanged.
//
// Go port of org.apache.lucene.analysis.pt.PortugueseStemFilter (Apache Lucene
// 10.4.0).
type PortugueseStemFilter struct {
	*analysis.BaseTokenFilter
	stemmer     *PortugueseStemmer
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewPortugueseStemFilter wraps input with a PortugueseStemFilter.
func NewPortugueseStemFilter(input analysis.TokenStream) *PortugueseStemFilter {
	f := &PortugueseStemFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stemmer:         NewPortugueseStemmer(),
	}
	src := f.GetAttributeSource()
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr = a.(analysis.CharTermAttribute)
	}
	if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
		f.keywordAttr = a.(analysis.KeywordAttribute)
	}
	return f
}

// IncrementToken advances to the next token and applies full RSLP stemming.
func (f *PortugueseStemFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	if f.termAttr != nil {
		runes := []rune(f.termAttr.String())
		// Worst-case expansion: 'ã'→'ão' adds 1 rune.
		oversized := make([]rune, len(runes)+1)
		copy(oversized, runes)
		newLen := f.stemmer.Stem(oversized, len(runes))
		f.termAttr.SetValue(string(oversized[:newLen]))
	}
	return true, nil
}

// Ensure interface compliance.
var _ analysis.TokenFilter = (*PortugueseStemFilter)(nil)

// ──────────────────────────────────────────────────────────────────────────────
// PortugueseStemFilterFactory
// ──────────────────────────────────────────────────────────────────────────────

// PortugueseStemFilterFactory creates PortugueseStemFilter instances.
//
// Go port of org.apache.lucene.analysis.pt.PortugueseStemFilterFactory
// (Apache Lucene 10.4.0).
type PortugueseStemFilterFactory struct{}

// NewPortugueseStemFilterFactory creates a new factory.
func NewPortugueseStemFilterFactory() *PortugueseStemFilterFactory {
	return &PortugueseStemFilterFactory{}
}

// Create creates a new PortugueseStemFilter.
func (f *PortugueseStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewPortugueseStemFilter(input)
}

// Ensure interface compliance.
var _ analysis.TokenFilterFactory = (*PortugueseStemFilterFactory)(nil)
