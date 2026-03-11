// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// StandardAnalyzer filters StandardTokenizer with LowerCaseFilter and StopFilter.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.standard.StandardAnalyzer.
//
// StandardAnalyzer is a general-purpose analyzer that tokenizes text using the
// StandardTokenizer, converts tokens to lowercase, and removes English stop words.
//
// It is the most commonly used analyzer for general text search.
type StandardAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords []string
}

// NewStandardAnalyzer creates a new StandardAnalyzer with English stop words.
func NewStandardAnalyzer() *StandardAnalyzer {
	return &StandardAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(),
		stopWords:    EnglishStopWords,
	}
}

// NewStandardAnalyzerWithStopWords creates a StandardAnalyzer with custom stop words.
func NewStandardAnalyzerWithStopWords(stopWords []string) *StandardAnalyzer {
	return &StandardAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(),
		stopWords:    stopWords,
	}
}

// TokenStream creates a TokenStream for analyzing text.
func (a *StandardAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	// Create the tokenizer
	tokenizer := NewStandardTokenizer()
	if err := tokenizer.SetReader(reader); err != nil {
		return nil, err
	}

	// Create the filter chain: Tokenizer -> LowerCaseFilter -> StopFilter
	lowerCaseFilter := NewLowerCaseFilter(tokenizer)
	stopFilter := NewStopFilter(lowerCaseFilter, a.stopWords)

	return stopFilter, nil
}

// GetStopWords returns the stop words used by this analyzer.
func (a *StandardAnalyzer) GetStopWords() []string {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *StandardAnalyzer) SetStopWords(stopWords []string) {
	a.stopWords = stopWords
}

// Ensure StandardAnalyzer implements Analyzer
var _ Analyzer = (*StandardAnalyzer)(nil)
