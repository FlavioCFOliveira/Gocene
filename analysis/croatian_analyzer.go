// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// CroatianStopWords contains common Croatian stop words.
// Source: Apache Lucene Croatian stop words list
var CroatianStopWords = []string{
	"a", "ako", "ali", "bi", "bila", "bili", "bilo", "biti", "bumo", "da",
	"do", "docim", "dolazak", "duž", "ga", "gdje", "i", "iako", "ili",
	"iz", "ja", "je", "jedna", "jedne", "jedno", "jer", "jesam", "jesi",
	"jesmo", "jest", "jeste", "jesu", "jim", "joj", "još", "ju", "kada",
	"kako", "kao", "koja", "koje", "koji", "kojima", "koju", "kroz", "li",
	"me", "mene", "meni", "mi", "mimo", "moja", "moje", "moji", "mu",
	"na", "nad", "nakon", "nam", "nama", "nas", "naš", "naša", "naše",
	"našeg", "naši", "ne", "neće", "nećemo", "nećeš", "nećete", "nego",
	"neka", "neki", "nekog", "neku", "nema", "netko", "nešto", "ni",
	"nije", "nikoga", "nikoje", "nikoju", "nismo", "niste", "nisu", "njega",
	"njegov", "njegova", "njegovo", "njemu", "njezin", "njezina", "njezino",
	"njih", "njihov", "njihova", "njihovo", "njim", "njima", "njoj", "nju",
	"no", "o", "od", "odakle", "odmah", "on", "ona", "oni", "ono", "onaj",
	"onažnji", "onda", "osim", "ostali", "ovaj", "ovažnji", "ovako",
	"ovdje", "ovim", "ovima", "postoji", "potom", "poželjan", "prvi",
	"radije", "sam", "samo", "saznati", "sve", "svi", "svog", "svoj",
	"svoja", "svoje", "svoju", "ta", "tada", "taj", "tako", "te", "tebe",
	"tebi", "ti", "to", "toj", "tome", "tu", "tvoj", "tvoja", "tvoje",
	"u", "uz", "uzeti", "vam", "vama", "vas", "vaš", "vaša", "vaše",
	"već", "vi", "vjerojatno", "vrlo", "za", "zašto", "zar", "zato",
	"zbog", "želeći", "žena", "žene", "ženi", "ženu",
}

// CroatianAnalyzer is an analyzer for Croatian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.hr.CroatianAnalyzer.
//
// CroatianAnalyzer uses the StandardTokenizer with Croatian stop words removal.
type CroatianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewCroatianAnalyzer creates a new CroatianAnalyzer with default Croatian stop words.
func NewCroatianAnalyzer() *CroatianAnalyzer {
	stopSet := GetWordSetFromStrings(CroatianStopWords, true)
	return NewCroatianAnalyzerWithWords(stopSet)
}

// NewCroatianAnalyzerWithWords creates a CroatianAnalyzer with custom stop words.
func NewCroatianAnalyzerWithWords(stopWords *CharArraySet) *CroatianAnalyzer {
	a := &CroatianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *CroatianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *CroatianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *CroatianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*CroatianAnalyzer)(nil)
var _ AnalyzerInterface = (*CroatianAnalyzer)(nil)
