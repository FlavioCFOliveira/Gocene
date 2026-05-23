// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package snowball

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// SnowballStemmer is the interface that Snowball-generated stemmers must satisfy.
//
// This is the Go equivalent of org.tartarus.snowball.SnowballStemmer from the
// Apache Lucene snowball dependency.
type SnowballStemmer interface {
	// SetCurrent sets the word to be stemmed.
	SetCurrent(word string)
	// Stem performs the stemming algorithm and returns whether the word was modified.
	Stem() bool
	// GetCurrent returns the (possibly stemmed) word.
	GetCurrent() string
}

// SnowballFilter applies a Snowball stemmer to each non-keyword token.
//
// This is the Go port of
// org.apache.lucene.analysis.snowball.SnowballFilter from
// Apache Lucene 10.4.0.
//
// Note: SnowballFilter expects lowercased text.
type SnowballFilter struct {
	*analysis.BaseTokenFilter

	stemmer     SnowballStemmer
	termAttr    analysis.CharTermAttribute
	keywordAttr analysis.KeywordAttribute
}

// NewSnowballFilter creates a SnowballFilter wrapping input with stemmer.
func NewSnowballFilter(input analysis.TokenStream, stemmer SnowballStemmer) *SnowballFilter {
	f := &SnowballFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stemmer:         stemmer,
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

// IncrementToken advances to the next token. Non-keyword tokens are stemmed.
func (f *SnowballFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if !ok || err != nil {
		return ok, err
	}
	if f.keywordAttr == nil || !f.keywordAttr.IsKeywordToken() {
		if f.termAttr != nil {
			term := f.termAttr.String()
			f.stemmer.SetCurrent(term)
			f.stemmer.Stem()
			stemmed := f.stemmer.GetCurrent()
			if stemmed != term {
				f.termAttr.SetEmpty()
				f.termAttr.AppendString(stemmed)
			}
		}
	}
	return true, nil
}

// Ensure SnowballFilter implements TokenFilter.
var _ analysis.TokenFilter = (*SnowballFilter)(nil)

// SnowballPorterFilterFactory creates SnowballFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.snowball.SnowballPorterFilterFactory from
// Apache Lucene 10.4.0.
//
// Deviation: the Java reference resolves the stemmer class by reflection from
// the Tartarus snowball library. This Go port accepts a SnowballStemmer
// directly, as the Tartarus Go port is not yet available. Language-specific
// stemmer construction is deferred to the snowball ext sprint.
type SnowballPorterFilterFactory struct {
	stemmer        SnowballStemmer
	protectedWords *analysis.CharArraySet
}

// NewSnowballPorterFilterFactory creates a factory wrapping stemmer.
func NewSnowballPorterFilterFactory(stemmer SnowballStemmer) *SnowballPorterFilterFactory {
	return &SnowballPorterFilterFactory{stemmer: stemmer}
}

// NewSnowballPorterFilterFactoryFull creates a factory with protected words.
func NewSnowballPorterFilterFactoryFull(
	stemmer SnowballStemmer,
	protectedWords *analysis.CharArraySet,
) *SnowballPorterFilterFactory {
	return &SnowballPorterFilterFactory{stemmer: stemmer, protectedWords: protectedWords}
}

// Create creates a SnowballFilter, optionally preceded by a
// SetKeywordMarkerFilter for the protected words.
func (f *SnowballPorterFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	if f.protectedWords != nil && !f.protectedWords.IsEmpty() {
		input = analysis.NewSetKeywordMarkerFilter(input, f.protectedWords)
	}
	return NewSnowballFilter(input, f.stemmer)
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*SnowballPorterFilterFactory)(nil)
