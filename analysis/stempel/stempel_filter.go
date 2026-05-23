// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package stempel

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// DefaultMinLength is the minimum length of input words to be processed.
// Shorter words are returned unchanged.
const DefaultMinLength = 3

// StempelFilter transforms a token stream using a Stempel stemmer.
//
// The input must already be in lower case (apply LowerCaseFilter upstream).
//
// This is the Go port of
// org.apache.lucene.analysis.stempel.StempelFilter (Lucene 10.4.0).
type StempelFilter struct {
	*analysis.BaseTokenFilter

	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
	stemmer     *StempelStemmer
	minLength   int
}

// NewStempelFilter wraps input with the stempel stem filter using the default
// minimum length (3).
func NewStempelFilter(input analysis.TokenStream, stemmer *StempelStemmer) *StempelFilter {
	return NewStempelFilterWithMinLength(input, stemmer, DefaultMinLength)
}

// NewStempelFilterWithMinLength wraps input with the stempel stem filter.
// Words shorter than minLength characters are returned unchanged.
func NewStempelFilterWithMinLength(input analysis.TokenStream, stemmer *StempelStemmer, minLength int) *StempelFilter {
	if stemmer == nil {
		panic("stemmer must not be nil")
	}
	if minLength < 1 {
		panic("minLength must be >= 1")
	}
	f := &StempelFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stemmer:         stemmer,
		minLength:       minLength,
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

// IncrementToken returns the next stemmed token.
func (f *StempelFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.termAttr == nil {
		return true, nil
	}
	// Skip keyword-marked tokens and short tokens.
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}
	if f.termAttr.Length() < f.minLength {
		return true, nil
	}
	word := []rune(string(f.termAttr.Buffer()[:f.termAttr.Length()]))
	stem := f.stemmer.Stem(word)
	if stem != nil {
		f.termAttr.SetEmpty()
		f.termAttr.Append([]byte(string(stem)))
	}
	return true, nil
}

// Ensure StempelFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*StempelFilter)(nil)
