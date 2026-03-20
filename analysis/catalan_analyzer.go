// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// CatalanStopWords contains common Catalan stop words.
// Source: Apache Lucene Catalan stop words list
var CatalanStopWords = []string{
	// Articles
	"a", "així", "això", "al", "aleshores", "algun", "alguna", "algunes",
	"alguns", "alhora", "allà", "allí", "allò", "als", "als quals", "amb",
	"ambdues", "ambdós", "anar", "ans", "any", "aquell", "aquella",
	"aquelles", "aquells", "aquest", "aquesta", "aquestes", "aquests",
	"aquí", "arreu", "cada", "com", "contra", "d'en", "d'una", "d'un",
	"dalt", "de", "del", "dels", "des", "des de", "després", "deu", "diferents",
	"din", "dins", "dins de", "div", "doncs", "dos", "durant", "e", "eh",
	"el", "el qual", "els", "els quals", "em", "en", "enlloc", "ens",
	"entre", "eren", "es", "és", "esta", "està", "estàs", "estat", "estava",
	"estaven", "estem", "esteu", "et", "etc", "ets", "fa", "faig", "fan",
	"fent", "fer", "feu", "fi", "fora", "gairebé", "ha", "hagi", "hagin",
	"haguem", "hagués", "hagueren", "haguessis", "han", "has", "hauria",
	"haurien", "havem", "haver", "havíem", "hi", "i", "igual", "iguals",
	"ja", "l'hi", "la", "les", "li", "llavors", "més", "meu", "meus",
	"meva", "meves", "mig", "mateix", "mateixa", "mateixes", "mateixos",
	"me'n", "menys", "mentre", "mi", "molt", "molta", "moltes", "molts",
	"mon", "mons", "na", "ni", "ningú", "no", "no res", "nogensmenys",
	"només", "nosaltres", "nostre", "nostra", "nostres", "o", "oh", "on",
	"països", "pel", "pels", "per", "per a", "però", "perquè", "pertot",
	"poc", "poca", "pocs", "poques", "potser", "preu", "propi", "qual",
	"quals", "quan", "quant", "que", "quelcom", "qüestió", "qui", "quin",
	"quina", "quines", "quins","regir", "respecte", "sí", "sobre", "sobretot",
	"solament", "sols", "son", "són", "sota", "sou", "sovint", "t'ha",
	"t'han", "t'hem", "ta", "tal", "tals", "també", "tampoc", "tan",
	"tanta", "tantes", "tant", "tants", "teu", "teus", "teva", "teves",
	"ton", "tons", "tot", "tota", "totes", "tots", "un", "una", "unes",
	"uns", "val", "vàrem", "vàreu", "vos", "vosaltres", "vostre", "vostra",
	"vostres", "vull", "vam"}

// CatalanAnalyzer is an analyzer for Catalan language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ca.CatalanAnalyzer.
//
// CatalanAnalyzer uses the StandardTokenizer with Catalan stop words removal.
type CatalanAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewCatalanAnalyzer creates a new CatalanAnalyzer with default Catalan stop words.
func NewCatalanAnalyzer() *CatalanAnalyzer {
	stopSet := GetWordSetFromStrings(CatalanStopWords, true)
	return NewCatalanAnalyzerWithWords(stopSet)
}

// NewCatalanAnalyzerWithWords creates a CatalanAnalyzer with custom stop words.
func NewCatalanAnalyzerWithWords(stopWords *CharArraySet) *CatalanAnalyzer {
	a := &CatalanAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *CatalanAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *CatalanAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *CatalanAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*CatalanAnalyzer)(nil)
var _ AnalyzerInterface = (*CatalanAnalyzer)(nil)
