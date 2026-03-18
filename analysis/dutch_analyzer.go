// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// DutchStopWords contains common Dutch stop words.
var DutchStopWords = []string{
	"aan", "al", "alles", "als", "altijd", "andere", "ben", "bij", "daar", "dan",
	"dat", "de", "der", "deze", "die", "dit", "doch", "doen", "door", "dus", "een",
	"eens", "en", "er", "ge", "geen", "geweest", "haar", "had", "heb", "hebben",
	"heeft", "hem", "het", "hier", "hij", "hoe", "hun", "iemand", "iets", "ik",
	"in", "is", "ja", "je", "kan", "kon", "kunnen", "maar", "me", "meer", "men",
	"met", "mij", "mijn", "moet", "na", "naar", "niet", "niets", "nog", "nu", "of",
	"om", "omdat", "ons", "ook", "op", "over", "reeds", "te", "tegen", "toch",
	"toen", "tot", "u", "uit", "uw", "van", "veel", "voor", "want", "waren",
	"was", "wat", "we", "wel", "werd", "wezen", "wie", "wij", "wil", "worden",
	"zal", "ze", "zelf", "zich", "zij", "zijn", "zo", "zonder", "zou",
}

// DutchAnalyzer is an analyzer for Dutch language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.nl.DutchAnalyzer.
//
// DutchAnalyzer uses the StandardTokenizer with Dutch stop words removal
// and light stemming.
type DutchAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewDutchAnalyzer creates a new DutchAnalyzer with default Dutch stop words.
func NewDutchAnalyzer() *DutchAnalyzer {
	stopSet := GetWordSetFromStrings(DutchStopWords, true)
	return NewDutchAnalyzerWithWords(stopSet)
}

// NewDutchAnalyzerWithWords creates a DutchAnalyzer with custom stop words.
func NewDutchAnalyzerWithWords(stopWords *CharArraySet) *DutchAnalyzer {
	a := &DutchAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewDutchLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *DutchAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *DutchAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *DutchAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure DutchAnalyzer implements Analyzer
var _ Analyzer = (*DutchAnalyzer)(nil)
var _ AnalyzerInterface = (*DutchAnalyzer)(nil)

// DutchLightStemFilter implements light stemming for Dutch.
type DutchLightStemFilter struct {
	*BaseTokenFilter
}

// NewDutchLightStemFilter creates a new DutchLightStemFilter.
func NewDutchLightStemFilter(input TokenStream) *DutchLightStemFilter {
	return &DutchLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *DutchLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := dutchLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// dutchLightStem applies light Dutch stemming.
func dutchLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Dutch suffixes
	switch {
	// -heid, -heden (abstract nouns)
	case length > 4 && string(runes[length-4:]) == "heid":
		return string(runes[:length-3])
	// -te, -ten
	case length > 3 && string(runes[length-3:]) == "ten":
		return string(runes[:length-2])
	// -de, -den
	case length > 3 && string(runes[length-3:]) == "den":
		return string(runes[:length-2])
	// -en (plural/infinitive)
	case length > 2 && runes[length-1] == 'n' && runes[length-2] == 'e':
		return string(runes[:length-2])
	// -t (verb ending)
	case length > 1 && runes[length-1] == 't':
		return string(runes[:length-1])
	}

	return term
}

// DutchLightStemFilterFactory creates DutchLightStemFilter instances.
type DutchLightStemFilterFactory struct{}

// NewDutchLightStemFilterFactory creates a new DutchLightStemFilterFactory.
func NewDutchLightStemFilterFactory() *DutchLightStemFilterFactory {
	return &DutchLightStemFilterFactory{}
}

// Create creates a new DutchLightStemFilter.
func (f *DutchLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDutchLightStemFilter(input)
}

// Ensure DutchLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*DutchLightStemFilterFactory)(nil)
