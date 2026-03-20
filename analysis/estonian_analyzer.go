// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// EstonianStopWords contains common Estonian stop words.
// Source: Apache Lucene Estonian stop words list
var EstonianStopWords = []string{
	"aga", "ei", "et", "ja", "jah", "kas", "kui", "kõik", "ma", "me",
	"mida", "midagi", "mind", "minu", "mis", "mu", "mulle", "nad",
	"nagu", "neid", "on", "oled", "olen", "oli", "oma", "piiks",
	"pole", "seal", "sest", "selle", "selleks", "sellist", "seda",
	"see", "selle", "siin", "siis", "ta", "te", "ära", "üks",
}

// EstonianAnalyzer is an analyzer for Estonian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.et.EstonianAnalyzer.
//
// EstonianAnalyzer uses the StandardTokenizer with Estonian stop words removal.
type EstonianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewEstonianAnalyzer creates a new EstonianAnalyzer with default Estonian stop words.
func NewEstonianAnalyzer() *EstonianAnalyzer {
	stopSet := GetWordSetFromStrings(EstonianStopWords, true)
	return NewEstonianAnalyzerWithWords(stopSet)
}

// NewEstonianAnalyzerWithWords creates an EstonianAnalyzer with custom stop words.
func NewEstonianAnalyzerWithWords(stopWords *CharArraySet) *EstonianAnalyzer {
	a := &EstonianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *EstonianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *EstonianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *EstonianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*EstonianAnalyzer)(nil)
var _ AnalyzerInterface = (*EstonianAnalyzer)(nil)
