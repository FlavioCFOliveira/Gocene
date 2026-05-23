// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classic

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ClassicDefaultMaxTokenLength is the default maximum token length for
// ClassicAnalyzer.
const ClassicDefaultMaxTokenLength = 255

// ClassicAnalyzer filters ClassicTokenizer with ClassicFilter, LowerCaseFilter
// and StopFilter, using a list of English stop words.
//
// This is the Go port of
// org.apache.lucene.analysis.classic.ClassicAnalyzer from
// Apache Lucene 10.4.0.
type ClassicAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords      *analysis.CharArraySet
	maxTokenLength int
}

// NewClassicAnalyzer creates a ClassicAnalyzer with default English stop words.
func NewClassicAnalyzer() *ClassicAnalyzer {
	return NewClassicAnalyzerWithStopwords(
		analysis.GetWordSetFromStrings(analysis.EnglishStopWords, false),
	)
}

// NewClassicAnalyzerWithStopwords creates a ClassicAnalyzer with the given
// stop words.
func NewClassicAnalyzerWithStopwords(stopWords *analysis.CharArraySet) *ClassicAnalyzer {
	a := &ClassicAnalyzer{
		BaseAnalyzer:   analysis.NewAnalyzer(),
		stopWords:      stopWords,
		maxTokenLength: ClassicDefaultMaxTokenLength,
	}
	a.TokenizerFactory = NewClassicTokenizerFactory()
	a.AddTokenFilter(NewClassicFilterFactory())
	a.AddTokenFilter(analysis.NewLowerCaseFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	return a
}

// SetMaxTokenLength sets the maximum token length.
func (a *ClassicAnalyzer) SetMaxTokenLength(length int) { a.maxTokenLength = length }

// GetMaxTokenLength returns the maximum token length.
func (a *ClassicAnalyzer) GetMaxTokenLength() int { return a.maxTokenLength }

// GetStopWords returns the stop words used by this analyzer.
func (a *ClassicAnalyzer) GetStopWords() *analysis.CharArraySet { return a.stopWords }

// TokenStream creates a TokenStream for analyzing text.
func (a *ClassicAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Ensure ClassicAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*ClassicAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*ClassicAnalyzer)(nil)
