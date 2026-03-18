// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// NorwegianStopWords contains common Norwegian stop words.
var NorwegianStopWords = []string{
	"alle", "at", "av", "bare", "begge", "ble", "blei", "bli", "blir", "blitt",
	"både", "båe", "da", "de", "deg", "dei", "deim", "deira", "deires", "dem",
	"den", "denne", "der", "dere", "deres", "det", "dette", "di", "din", "disse",
	"ditt", "du", "dykk", "dykkar", "då", "eg", "ein", "eit", "eitt", "eller",
	"elles", "en", "enn", "er", "et", "ett", "etter", "for", "fordi", "fra",
	"før", "ha", "hadde", "han", "hans", "har", "hennar", "henne", "hennes",
	"her", "hjå", "ho", "hoe", "honom", "hoss", "hossen", "hun", "hva", "hvem",
	"hver", "hvilke", "hvilken", "hvis", "hvor", "hvordan", "hvorfor", "i", "ikke",
	"ikkje", "ingen", "ingi", "inkje", "inn", "inni", "ja", "jeg", "kan", "kom",
	"korleis", "korso", "kun", "kunne", "kva", "kvar", "kvarhelst", "kven",
	"kvi", "kvifor", "man", "mange", "me", "med", "medan", "meg", "meget",
	"mellom", "men", "mi", "min", "mine", "mitt", "mot", "mykje", "mye", "nå",
	"når", "ned", "no", "noe", "noen", "noka", "noko", "nokon", "nokor",
	"nokre", "og", "også", "om", "opp", "oss", "over", "på", "samme", "seg",
	"selv", "si", "sia", "sidan", "siden", "sin", "sine", "sitt", "sjøl",
	"skal", "skulle", "slik", "so", "som", "somme", "somt", "så", "sånn",
	"til", "um", "upp", "ut", "uten", "var", "vart", "varte", "ved", "vere",
	"vert", "verta", "vette", "vi", "vil", "ville", "vore", "vors", "vort",
	"vår", "være", "vært", "å",
}

// NorwegianAnalyzer is an analyzer for Norwegian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.no.NorwegianAnalyzer.
//
// NorwegianAnalyzer uses the StandardTokenizer with Norwegian stop words removal
// and light stemming.
type NorwegianAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewNorwegianAnalyzer creates a new NorwegianAnalyzer with default Norwegian stop words.
func NewNorwegianAnalyzer() *NorwegianAnalyzer {
	stopSet := GetWordSetFromStrings(NorwegianStopWords, true)
	return NewNorwegianAnalyzerWithWords(stopSet)
}

// NewNorwegianAnalyzerWithWords creates a NorwegianAnalyzer with custom stop words.
func NewNorwegianAnalyzerWithWords(stopWords *CharArraySet) *NorwegianAnalyzer {
	a := &NorwegianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewNorwegianLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *NorwegianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *NorwegianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *NorwegianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure NorwegianAnalyzer implements Analyzer
var _ Analyzer = (*NorwegianAnalyzer)(nil)
var _ AnalyzerInterface = (*NorwegianAnalyzer)(nil)

// NorwegianLightStemFilter implements light stemming for Norwegian.
type NorwegianLightStemFilter struct {
	*BaseTokenFilter
}

// NewNorwegianLightStemFilter creates a new NorwegianLightStemFilter.
func NewNorwegianLightStemFilter(input TokenStream) *NorwegianLightStemFilter {
	return &NorwegianLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *NorwegianLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := norwegianLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// norwegianLightStem applies light Norwegian stemming.
func norwegianLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Norwegian suffixes
	switch {
	// -het, -heten (abstract nouns)
	case length > 4 && string(runes[length-4:]) == "heten":
		return string(runes[:length-3])
	case length > 3 && string(runes[length-3:]) == "het":
		return string(runes[:length-2])
	// -else, -elsen
	case length > 4 && string(runes[length-4:]) == "else":
		return string(runes[:length-3])
	case length > 5 && string(runes[length-5:]) == "elsen":
		return string(runes[:length-4])
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

// NorwegianLightStemFilterFactory creates NorwegianLightStemFilter instances.
type NorwegianLightStemFilterFactory struct{}

// NewNorwegianLightStemFilterFactory creates a new NorwegianLightStemFilterFactory.
func NewNorwegianLightStemFilterFactory() *NorwegianLightStemFilterFactory {
	return &NorwegianLightStemFilterFactory{}
}

// Create creates a new NorwegianLightStemFilter.
func (f *NorwegianLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewNorwegianLightStemFilter(input)
}

// Ensure NorwegianLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*NorwegianLightStemFilterFactory)(nil)
