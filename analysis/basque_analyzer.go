// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
)

// BasqueStopWords contains common Basque stop words.
// Source: Apache Lucene Basque stop words list
var BasqueStopWords = []string{
	// Articles
	"a", "al", "an", "ara", "aren", "arren", "artean", "as", "aste",
	// Pronouns
	"baino", "bat", "batean", "batek", "bati", "batzuk", "bera", "beraiek",
	"berau", "berauek", "bere", "beretan", "berori", "beroriek", "beste",
	"bezala", "da", "dago", "dira", "du", "dute", "edo", "egin", "ere",
	// Prepositions
	"eta", "eurak", "ez", "gainera", "gu", "gutxi", "guzti", "haiei",
	"haiek", "hainbeste", "hala", "han", "handik", "hango", "hara", "hari",
	"hark", "hartan", "hau", "hauei", "hauek", "hauren", "hemen", "hemendik",
	"hemengo", "hi", "hona", "honek", "honela", "honetan", "honi", "hor",
	"hori", "horiek", "horietan", "horra", "hortik", "hura", "izan", "ni",
	// Conjunctions
	"nola", "non", "nondik", "nongo", "nor", "nora", "zein", "zen",
	"zenbait", "zenbat", "zer", "zergatik", "ziren", "zituen", "zu", "zuek",
	"zuen", "zuten",
}

// BasqueAnalyzer is an analyzer for Basque language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.eu.BasqueAnalyzer.
//
// BasqueAnalyzer uses the StandardTokenizer with Basque stop words removal.
type BasqueAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewBasqueAnalyzer creates a new BasqueAnalyzer with default Basque stop words.
func NewBasqueAnalyzer() *BasqueAnalyzer {
	stopSet := GetWordSetFromStrings(BasqueStopWords, true)
	return NewBasqueAnalyzerWithWords(stopSet)
}

// NewBasqueAnalyzerWithWords creates a BasqueAnalyzer with custom stop words.
func NewBasqueAnalyzerWithWords(stopWords *CharArraySet) *BasqueAnalyzer {
	a := &BasqueAnalyzer{
		BaseAnalyzer: NewAnalyzer(GlobalReuseStrategy),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.CreateComponents = func(fieldName string) *TokenStreamComponents {
		src := NewStandardTokenizer()
		var tok TokenStream = NewLowerCaseFilter(src)
		tok = NewStopFilterWithWords(tok, stopWords)

		return &TokenStreamComponents{
			Source: func(r io.Reader) error {
				src.SetReader(r)
				return nil
			},
			Sink: tok,
		}
	}

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *BasqueAnalyzer) TokenStream(fieldName string, reader io.Reader) (api.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *BasqueAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *BasqueAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure BasqueAnalyzer implements Analyzer
var _ api.Analyzer = (*BasqueAnalyzer)(nil)
