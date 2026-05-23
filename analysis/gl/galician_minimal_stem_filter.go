// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package gl

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// GalicianMinimalStemFilter is a TokenFilter that applies
// GalicianMinimalStemmer to each non-keyword token.
//
// This is the Go port of
// org.apache.lucene.analysis.gl.GalicianMinimalStemFilter from
// Apache Lucene 10.4.0.
type GalicianMinimalStemFilter struct {
	*analysis.BaseTokenFilter

	stemmer     *GalicianMinimalStemmer
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewGalicianMinimalStemFilter wraps input with Galician minimal stemming.
func NewGalicianMinimalStemFilter(input analysis.TokenStream) (*GalicianMinimalStemFilter, error) {
	stemmer, err := NewGalicianMinimalStemmer()
	if err != nil {
		return nil, err
	}
	bf := analysis.NewBaseTokenFilter(input)
	src := bf.GetAttributeSource()

	var termAttr analysis.CharTermAttribute
	var keywordAttr analysis.KeywordAttribute

	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		termAttr = a.(analysis.CharTermAttribute)
	}
	if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
		keywordAttr = a.(analysis.KeywordAttribute)
	}

	return &GalicianMinimalStemFilter{
		BaseTokenFilter: bf,
		stemmer:         stemmer,
		termAttr:        termAttr,
		keywordAttr:     keywordAttr,
	}, nil
}

// IncrementToken advances the stream, applying stemming to non-keyword tokens.
func (f *GalicianMinimalStemFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		return true, nil
	}

	if f.termAttr == nil {
		return true, nil
	}

	// Operate on a rune buffer derived from the current term.
	term := []rune(f.termAttr.String())
	// Oversized by 1 as the RSLP stemmer requires.
	buf := make([]rune, len(term)+1)
	copy(buf, term)

	newLen := f.stemmer.Stem(buf, len(term))
	f.termAttr.SetValue(string(buf[:newLen]))
	return true, nil
}

// Ensure GalicianMinimalStemFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*GalicianMinimalStemFilter)(nil)

// ─── Factory ─────────────────────────────────────────────────────────────────

// GalicianMinimalStemFilterFactory creates GalicianMinimalStemFilter
// instances. It accepts an empty parameter map.
//
// This is the Go port of
// org.apache.lucene.analysis.gl.GalicianMinimalStemFilterFactory from
// Apache Lucene 10.4.0.
type GalicianMinimalStemFilterFactory struct{}

// NewGalicianMinimalStemFilterFactory constructs the factory.
// Returns an error if any unknown parameters are present.
func NewGalicianMinimalStemFilterFactory(args map[string]string) (*GalicianMinimalStemFilterFactory, error) {
	return &GalicianMinimalStemFilterFactory{}, nil
}

// Create wraps input in a GalicianMinimalStemFilter.
func (f *GalicianMinimalStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	filter, err := NewGalicianMinimalStemFilter(input)
	if err != nil {
		// Resource load errors are fatal; panic to surface the configuration
		// problem early.
		panic("galician: load stemmer: " + err.Error())
	}
	return filter
}

// Ensure GalicianMinimalStemFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*GalicianMinimalStemFilterFactory)(nil)
