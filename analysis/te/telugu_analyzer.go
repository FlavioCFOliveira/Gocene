// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package te provides analysis components for Telugu language text.
package te

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TeluguAnalyzer is an analyzer for Telugu language text.
//
// Port of org.apache.lucene.analysis.te.TeluguAnalyzer (Apache Lucene 10.4.0).
//
// The analysis chain applies:
//  1. StandardTokenizer — splits on whitespace/punctuation
//  2. LowerCaseFilter — folds to lowercase
//  3. TeluguNormalizationFilter — normalizes Telugu characters
//  4. TeluguStemFilter — stems Telugu words
type TeluguAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords *analysis.CharArraySet
}

// NewTeluguAnalyzer creates a TeluguAnalyzer with default stop words.
func NewTeluguAnalyzer() *TeluguAnalyzer {
	return NewTeluguAnalyzerWithWords(analysis.NewCharArraySet(0, true))
}

// NewTeluguAnalyzerWithWords creates a TeluguAnalyzer with custom stop words.
func NewTeluguAnalyzerWithWords(stopWords *analysis.CharArraySet) *TeluguAnalyzer {
	a := &TeluguAnalyzer{
		BaseAnalyzer: analysis.NewAnalyzer(),
		stopWords:    stopWords,
	}

	a.TokenizerFactory = analysis.NewStandardTokenizerFactory()
	a.AddTokenFilter(analysis.NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewTeluguNormalizationFilterFactory())
	a.AddTokenFilter(NewTeluguStemFilterFactory())
	if stopWords != nil && stopWords.Size() > 0 {
		a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	}

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *TeluguAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *TeluguAnalyzer) GetStopWords() *analysis.CharArraySet {
	return a.stopWords
}

var _ analysis.Analyzer = (*TeluguAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*TeluguAnalyzer)(nil)
