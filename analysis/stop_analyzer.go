// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// StopAnalyzer is an analyzer that filters LetterTokenizer with LowerCaseFilter and StopFilter.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.StopAnalyzer.
//
// StopAnalyzer tokenizes text using LetterTokenizer, converts tokens to lowercase,
// and removes stop words. This is useful for general text analysis where common
// words should be excluded.
//
// Example:
//
//	analyzer := NewStopAnalyzer()
//	tokens, _ := analyzer.Analyze("The quick brown fox")
//	// tokens: ["quick", "brown", "fox"] ("the" is removed as stop word)
type StopAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// EnglishStopWords contains common English stop words.
// These are the default stop words used by Lucene's StopAnalyzer.
var EnglishStopWords = []string{
	"a", "an", "and", "are", "as", "at", "be", "but", "by",
	"for", "if", "in", "into", "is", "it", "no", "not", "of",
	"on", "or", "such", "that", "the", "their", "then", "there",
	"these", "they", "this", "to", "was", "will", "with",
}

// NewStopAnalyzer creates a new StopAnalyzer with default English stop words.
func NewStopAnalyzer() *StopAnalyzer {
	stopSet := GetWordSetFromStrings(EnglishStopWords, true)
	return NewStopAnalyzerWithWords(stopSet)
}

// NewStopAnalyzerWithWords creates a new StopAnalyzer with custom stop words.
func NewStopAnalyzerWithWords(stopWords *CharArraySet) *StopAnalyzer {
	a := &StopAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	// Set up the analysis chain: LetterTokenizer -> LowerCaseFilter -> StopFilter
	a.TokenizerFactory = NewLetterTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *StopAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Close releases resources.
func (a *StopAnalyzer) Close() error {
	return a.BaseAnalyzer.Close()
}

// GetStopWords returns the stop words used by this analyzer.
func (a *StopAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// Ensure StopAnalyzer implements Analyzer
var _ Analyzer = (*StopAnalyzer)(nil)
var _ AnalyzerInterface = (*StopAnalyzer)(nil)

// StopAnalyzerFactory creates StopAnalyzer instances.
type StopAnalyzerFactory struct {
	stopWords *CharArraySet
}

// NewStopAnalyzerFactory creates a new StopAnalyzerFactory with default stop words.
func NewStopAnalyzerFactory() *StopAnalyzerFactory {
	return &StopAnalyzerFactory{
		stopWords: GetWordSetFromStrings(EnglishStopWords, true),
	}
}

// NewStopAnalyzerFactoryWithWords creates a new StopAnalyzerFactory with custom stop words.
func NewStopAnalyzerFactoryWithWords(stopWords *CharArraySet) *StopAnalyzerFactory {
	return &StopAnalyzerFactory{
		stopWords: stopWords,
	}
}

// Create creates a new StopAnalyzer.
func (f *StopAnalyzerFactory) Create() AnalyzerInterface {
	return NewStopAnalyzerWithWords(f.stopWords)
}

// Ensure StopAnalyzerFactory implements AnalyzerFactory
var _ AnalyzerFactory = (*StopAnalyzerFactory)(nil)