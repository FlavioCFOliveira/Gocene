// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// GalicianStopWords contains common Galician stop words.
// Source: Apache Lucene Galician stop words list
var GalicianStopWords = []string{
	"a", "aínda", "alí", "ambos", "aquel", "aquela", "aquelas", "aqueles",
	"aquilo", "aquí", "ás", "así", "á", "ben", "cando", "che", "co",
	"coa", "coas", "con", "connosco", "convosco", "cos", "cun", "cuns",
	"da", "dalgunha", "dalgunhas", "dalgún", "dalgúns", "das", "de", "del",
	"dela", "delas", "deles", "desde", "deste", "destes", "desta", "destas",
	"destas", "do", "dos", "dunha", "dunhas", "dun", "duns", "e", "el",
	"ela", "elas", "eles", "en", "era", "eran", "esa", "esas", "ese",
	"eses", "esta", "estaba", "estamos", "están", "estás", "estes",
	"este", "eu", "é", "facer", "fai", "fose", "fósese", "gran", "hai",
	"iso", "isto", "la", "las", "lle", "lles", "lo", "los", "mais",
	"me", "meu", "meus", "min", "miña", "miñas", "moi", "na", "nas",
	"neste", "nestes", "nesta", "nestas", "nin", "no", "non", "nos",
	"nosa", "nosas", "noso", "nosos", "nun", "nunha", "nuns", "nunhas",
	"o", "os", "ou", "ó", "ós", "para", "pela", "polas", "polo", "polos",
	"por", "pode", "podes", "podo", "pois", "pola", "pola", "que", "se",
	"sen", "seu", "seus", "sido", "súa", "súas", "tamén", "tan", "tamen",
	"tede", "tedes", "teñen", "teño", "ten", "temos", "ten", "ter",
	"teu", "teus", "ti", "tida", "tido", "tña", "tñas", "tñen", "tñer",
	"tña", "un", "unha", "unhas", "uns", "vaia", "vai", "xa", "zo",
}

// GalicianAnalyzer is an analyzer for Galician language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.gl.GalicianAnalyzer.
//
// GalicianAnalyzer uses the StandardTokenizer with Galician stop words removal and light stemming.
type GalicianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewGalicianAnalyzer creates a new GalicianAnalyzer with default Galician stop words.
func NewGalicianAnalyzer() *GalicianAnalyzer {
	stopSet := GetWordSetFromStrings(GalicianStopWords, true)
	return NewGalicianAnalyzerWithWords(stopSet)
}

// NewGalicianAnalyzerWithWords creates a GalicianAnalyzer with custom stop words.
func NewGalicianAnalyzerWithWords(stopWords *CharArraySet) *GalicianAnalyzer {
	a := &GalicianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewGalicianStemFilterFactory())
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *GalicianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *GalicianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *GalicianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*GalicianAnalyzer)(nil)
var _ AnalyzerInterface = (*GalicianAnalyzer)(nil)

// GalicianStemFilter implements light stemming for Galician.
type GalicianStemFilter struct {
	*BaseTokenFilter
}

// NewGalicianStemFilter creates a new GalicianStemFilter.
func NewGalicianStemFilter(input TokenStream) *GalicianStemFilter {
	return &GalicianStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies Galician stemming.
func (f *GalicianStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := galicianLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// galicianLightStem applies light Galician stemming.
func galicianLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	runes := []rune(term)
	length := len(runes)

	// Galician is similar to Portuguese but with some differences
	switch {
	// -mente (adverb suffix)
	case length > 5 && string(runes[length-5:]) == "mente":
		return string(runes[:length-4])
	// -ción, -sión
	case length > 4 && (string(runes[length-4:]) == "ción" ||
		string(runes[length-4:]) == "sión"):
		return string(runes[:length-3])
	// -idade, -idades
	case length > 5 && string(runes[length-5:]) == "idade":
		return string(runes[:length-4])
	case length > 6 && string(runes[length-6:]) == "idades":
		return string(runes[:length-5])
	// -eza, -ezas
	case length > 3 && string(runes[length-3:]) == "eza":
		return string(runes[:length-2])
	case length > 4 && string(runes[length-4:]) == "ezas":
		return string(runes[:length-3])
	// -s plural
	case length > 3 && runes[length-1] == 's':
		return string(runes[:length-1])
	}

	return term
}

// GalicianStemFilterFactory creates GalicianStemFilter instances.
type GalicianStemFilterFactory struct{}

// NewGalicianStemFilterFactory creates a new GalicianStemFilterFactory.
func NewGalicianStemFilterFactory() *GalicianStemFilterFactory {
	return &GalicianStemFilterFactory{}
}

// Create creates a new GalicianStemFilter.
func (f *GalicianStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGalicianStemFilter(input)
}

var _ TokenFilterFactory = (*GalicianStemFilterFactory)(nil)
