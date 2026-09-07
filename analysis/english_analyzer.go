// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"github.com/FlavioCFOliveira/Gocene/analysis/api"
)

// EnglishAnalyzer is an analyzer for English language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.en.EnglishAnalyzer.
//
// EnglishAnalyzer uses the StandardTokenizer with English stop words removal
// and Porter stemming. It also applies ASCII folding for compatibility.
//
// The analysis chain:
//  1. StandardTokenizer - tokenizes text following UTS#39 rules
//  2. LowerCaseFilter - converts to lowercase
//  3. StopFilter - removes English stop words
//  4. PorterStemFilter - applies Porter stemming algorithm
//
// Example:
//
//	analyzer := NewEnglishAnalyzer()
//	stream := analyzer.TokenStream("field", strings.NewReader("running quickly"))
//	// tokens: "run", "quick"
type EnglishAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet

	// enableStemming controls whether stemming is applied
	enableStemming bool
}

// NewEnglishAnalyzer creates a new EnglishAnalyzer with default English stop words.
func NewEnglishAnalyzer() *EnglishAnalyzer {
	stopSet := GetWordSetFromStrings(EnglishStopWords, true)
	return NewEnglishAnalyzerWithWords(stopSet)
}

// NewEnglishAnalyzerWithWords creates an EnglishAnalyzer with custom stop words.
func NewEnglishAnalyzerWithWords(stopWords *CharArraySet) *EnglishAnalyzer {
	a := &EnglishAnalyzer{
		BaseAnalyzer:   NewAnalyzer(GlobalReuseStrategy),
		stopWords:      stopWords,
		enableStemming: true,
	}

	// Set up the analysis chain
	a.CreateComponents = func(fieldName string) *TokenStreamComponents {
		src := NewStandardTokenizer()
		var tok TokenStream = NewLowerCaseFilter(src)
		tok = NewStopFilterWithWords(tok, stopWords)
		if a.enableStemming {
			tok = NewPorterStemFilter(tok)
		}

		return &TokenStreamComponents{
			source: func(r io.Reader) error {
				src.SetReader(r)
				return nil
			},
			sink: tok,
		}
	}

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *EnglishAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *EnglishAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *EnglishAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// IsStemmingEnabled returns whether stemming is enabled.
func (a *EnglishAnalyzer) IsStemmingEnabled() bool {
	return a.enableStemming
}

// SetStemmingEnabled sets whether stemming is enabled.
func (a *EnglishAnalyzer) SetStemmingEnabled(enabled bool) {
	a.enableStemming = enabled
}

// Ensure EnglishAnalyzer implements api.Analyzer
var _ api.Analyzer = (*EnglishAnalyzer)(nil)

// EnglishAnalyzerFactory creates EnglishAnalyzer instances.
type EnglishAnalyzerFactory struct {
	stopWords *CharArraySet
}

// NewEnglishAnalyzerFactory creates a new EnglishAnalyzerFactory with default stop words.
func NewEnglishAnalyzerFactory() *EnglishAnalyzerFactory {
	return &EnglishAnalyzerFactory{
		stopWords: GetWordSetFromStrings(EnglishStopWords, true),
	}
}

// NewEnglishAnalyzerFactoryWithWords creates a new EnglishAnalyzerFactory with custom stop words.
func NewEnglishAnalyzerFactoryWithWords(stopWords *CharArraySet) *EnglishAnalyzerFactory {
	return &EnglishAnalyzerFactory{
		stopWords: stopWords,
	}
}

// Create creates a new EnglishAnalyzer.
func (f *EnglishAnalyzerFactory) Create() api.Analyzer {
	return NewEnglishAnalyzerWithWords(f.stopWords)
}

// Ensure EnglishAnalyzerFactory implements AnalyzerFactory
var _ AnalyzerFactory = (*EnglishAnalyzerFactory)(nil)

// PorterStemFilterFactory creates PorterStemFilter instances.
type PorterStemFilterFactory struct {
	BaseTokenFilterFactory
}

// NewPorterStemFilterFactory creates a new PorterStemFilterFactory.
func NewPorterStemFilterFactory() *PorterStemFilterFactory {
	return &PorterStemFilterFactory{}
}

// Create creates a new PorterStemFilter.
func (f *PorterStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewPorterStemFilter(input)
}

// Ensure PorterStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*PorterStemFilterFactory)(nil)
