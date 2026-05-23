// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package gl

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// GalicianStemFilter is a TokenFilter that applies GalicianStemmer
// to each non-keyword token.
//
// This is the Go port of
// org.apache.lucene.analysis.gl.GalicianStemFilter from
// Apache Lucene 10.4.0.
type GalicianStemFilter struct {
	*analysis.BaseTokenFilter

	stemmer     *GalicianStemmer
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewGalicianStemFilter wraps input with full Galician stemming.
func NewGalicianStemFilter(input analysis.TokenStream) (*GalicianStemFilter, error) {
	stemmer, err := NewGalicianStemmer()
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

	return &GalicianStemFilter{
		BaseTokenFilter: bf,
		stemmer:         stemmer,
		termAttr:        termAttr,
		keywordAttr:     keywordAttr,
	}, nil
}

// IncrementToken advances the stream, applying stemming to non-keyword tokens.
func (f *GalicianStemFilter) IncrementToken() (bool, error) {
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
	// Oversized by 1: worst case '*çom' -> '*ción' (same length in runes, but
	// the Galician vowel step may momentarily need an extra slot).
	term := []rune(f.termAttr.String())
	buf := make([]rune, len(term)+1)
	copy(buf, term)

	newLen := f.stemmer.Stem(buf, len(term))
	f.termAttr.SetValue(string(buf[:newLen]))
	return true, nil
}

// Ensure GalicianStemFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*GalicianStemFilter)(nil)

// ─── Factory ─────────────────────────────────────────────────────────────────

// GalicianStemFilterFactory creates GalicianStemFilter instances.
// It accepts an empty parameter map.
//
// This is the Go port of
// org.apache.lucene.analysis.gl.GalicianStemFilterFactory from
// Apache Lucene 10.4.0.
type GalicianStemFilterFactory struct{}

// NewGalicianStemFilterFactory constructs the factory.
// Returns an error if any unknown parameters are present.
func NewGalicianStemFilterFactory(args map[string]string) (*GalicianStemFilterFactory, error) {
	return &GalicianStemFilterFactory{}, nil
}

// Create wraps input in a GalicianStemFilter.
func (f *GalicianStemFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	filter, err := NewGalicianStemFilter(input)
	if err != nil {
		// Resource load errors are fatal; panic to surface the configuration
		// problem early.
		panic("galician: load stemmer: " + err.Error())
	}
	return filter
}

// Ensure GalicianStemFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*GalicianStemFilterFactory)(nil)
