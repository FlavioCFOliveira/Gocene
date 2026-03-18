// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// DanishStopWords contains common Danish stop words.
var DanishStopWords = []string{
	"af", "alle", "andet", "andre", "at", "begge", "da", "de", "den", "denne", "der",
	"deres", "det", "dette", "dig", "din", "dog", "du", "efter", "eller", "en",
	"end", "er", "et", "for", "fra", "ham", "han", "hans", "har", "hendes", "her",
	"hos", "hun", "hvad", "hvem", "hver", "hvilken", "hvis", "hvor", "hvordan",
	"hvorfor", "i", "ikke", "ind", "ingen", "intet", "jeg", "jeres", "kan", "kom",
	"kommer", "lav", "lidt", "lille", "man", "mand", "mange", "med", "meget", "men",
	"mens", "mere", "mig", "ned", "nej", "ni", "nogen", "noget", "ny", "nyt", "nær",
	"næste", "næsten", "og", "også", "okay", "om", "op", "os", "otte", "over",
	"på", "se", "seks", "ses", "som", "stor", "store", "syv", "ti", "til", "to",
	"tre", "ud", "under", "var", "ved", "vi", "vil", "ville", "vores", "åtte",
	"øvrig",
}

// DanishAnalyzer is an analyzer for Danish language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.da.DanishAnalyzer.
//
// DanishAnalyzer uses the StandardTokenizer with Danish stop words removal
// and light stemming.
type DanishAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewDanishAnalyzer creates a new DanishAnalyzer with default Danish stop words.
func NewDanishAnalyzer() *DanishAnalyzer {
	stopSet := GetWordSetFromStrings(DanishStopWords, true)
	return NewDanishAnalyzerWithWords(stopSet)
}

// NewDanishAnalyzerWithWords creates a DanishAnalyzer with custom stop words.
func NewDanishAnalyzerWithWords(stopWords *CharArraySet) *DanishAnalyzer {
	a := &DanishAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewDanishLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *DanishAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *DanishAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *DanishAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure DanishAnalyzer implements Analyzer
var _ Analyzer = (*DanishAnalyzer)(nil)
var _ AnalyzerInterface = (*DanishAnalyzer)(nil)

// DanishLightStemFilter implements light stemming for Danish.
type DanishLightStemFilter struct {
	*BaseTokenFilter
}

// NewDanishLightStemFilter creates a new DanishLightStemFilter.
func NewDanishLightStemFilter(input TokenStream) *DanishLightStemFilter {
	return &DanishLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *DanishLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := danishLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// danishLightStem applies light Danish stemming.
func danishLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Danish suffixes
	switch {
	// -hed, -heden (abstract nouns)
	case length > 3 && string(runes[length-3:]) == "hed":
		return string(runes[:length-2])
	// -else, -elsen
	case length > 4 && string(runes[length-4:]) == "else":
		return string(runes[:length-3])
	// -erne, -eren (definite plural)
	case length > 4 && (string(runes[length-4:]) == "erne" || string(runes[length-4:]) == "eren"):
		return string(runes[:length-3])
	// -ene (definite plural)
	case length > 3 && string(runes[length-3:]) == "ene":
		return string(runes[:length-2])
	// -er, -en, -et
	case length > 2 && (runes[length-1] == 'r' || runes[length-1] == 'n' || runes[length-1] == 't'):
		if runes[length-2] == 'e' {
			return string(runes[:length-2])
		}
	// -e
	case length > 1 && runes[length-1] == 'e':
		return string(runes[:length-1])
	}

	return term
}

// DanishLightStemFilterFactory creates DanishLightStemFilter instances.
type DanishLightStemFilterFactory struct{}

// NewDanishLightStemFilterFactory creates a new DanishLightStemFilterFactory.
func NewDanishLightStemFilterFactory() *DanishLightStemFilterFactory {
	return &DanishLightStemFilterFactory{}
}

// Create creates a new DanishLightStemFilter.
func (f *DanishLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDanishLightStemFilter(input)
}

// Ensure DanishLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*DanishLightStemFilterFactory)(nil)
