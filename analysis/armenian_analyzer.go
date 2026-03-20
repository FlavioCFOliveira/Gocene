// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// ArmenianStopWords contains common Armenian stop words.
// These are high-frequency words that provide little semantic value.
// Source: Apache Lucene Armenian stop words list
var ArmenianStopWords = []string{
	// Demonstratives
	"այդ", "այլ", "այն", "այս",
	// Pronouns
	"դու", "դուք", "ես", "իր", "նա", "նրանք",
	"ու", "ում",
	// Verb forms (to be)
	"եմ", "են", "ենք", "ես", "եք", "է", "էի", "էին", "էինք",
	"էիր", "էիք", "էր",
	// Prepositions/Postpositions
	"ըստ", "ին", "մեջ", "վրա", "համար", "հետ", "հետո",
	// Conjunctions
	"իսկ", "կամ", "նաև", "ու", "և",
	// Particles
	"թ", "ի", "ն", "որ", "որը", "որոնք", "որպես",
	// Other common words
	"մենք", "մի", "նրա", "պիտի",
}

// ArmenianAnalyzer is an analyzer for Armenian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.hy.ArmenianAnalyzer.
//
// ArmenianAnalyzer uses the StandardTokenizer with Armenian stop words removal.
// Note: Armenian does not use a stemmer in Lucene's implementation.
type ArmenianAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewArmenianAnalyzer creates a new ArmenianAnalyzer with default Armenian stop words.
func NewArmenianAnalyzer() *ArmenianAnalyzer {
	stopSet := GetWordSetFromStrings(ArmenianStopWords, true)
	return NewArmenianAnalyzerWithWords(stopSet)
}

// NewArmenianAnalyzerWithWords creates an ArmenianAnalyzer with custom stop words.
func NewArmenianAnalyzerWithWords(stopWords *CharArraySet) *ArmenianAnalyzer {
	a := &ArmenianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	// Tokenizer -> LowerCase -> StopWords
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *ArmenianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *ArmenianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *ArmenianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure ArmenianAnalyzer implements Analyzer
var _ Analyzer = (*ArmenianAnalyzer)(nil)
var _ AnalyzerInterface = (*ArmenianAnalyzer)(nil)
