// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// SlovakStopWords contains common Slovak stop words.
// Source: Apache Lucene Slovak stop words list
var SlovakStopWords = []string{
	"a", "aby", "aj", "ak", "ako", "ale", "alebo", "and", "ani",
	"áno", "asi", "až", "bez", "bude", "budem", "budeš", "budeme",
	"budete", "budú", "by", "bol", "bola", "boli", "bolo", "byť",
	"čo", "či", "ďalšia", "ďalšie", "ďalší", "dnes", "do", "ho",
	"i", "ja", "je", "jeho", "jej", "ich", "im", "iná", "iné",
	"iný", "k", "kam", "každá", "každé", "každí", "každý",
	"kde", "keď", "kto", "ktorá", "ktoré", "ktorí", "ktorou",
	"ktorý", "ktorých", "ku", "lebo", "len", "ma", "má", "máte",
	"mať", "medzi", "menej", "mňa", "môcť", "môj", "môže",
	"my", "na", "nad", "nám", "náš", "naši", "ne", "než",
	"nič", "nie", "nová", "nové", "noví", "nový", "o", "od",
	"odo", "of", "on", "ona", "ono", "oni", "ony", "po",
	"pod", "podľa", "pokiaľ", "potom", "práve", "pre", "prečo",
	"preto", "pretože", "pri", "s", "sa", "so", "si", "sú",
	"svoj", "svoje", "svojich", "svojím", "svojími", "ta",
	"tá", "tak", "takže", "táto", "teda", "tej", "tento",
	"tieto", "tí", "to", "toho", "tohto", "tom", "tomto",
	"toto", "tým", "týmto", "tvoj", "tvoje", "tvoji", "ty",
	"už", "v", "vám", "váš", "vaši", "veľmi", "vo", "však",
	"všetci", "všetky", "všetko", "vy", "z", "za", "zo",
	"že", "aby", "aj", "ak", "ako", "ale", "alebo", "ani",
	"áno", "asi", "až", "bez", "bude", "budem", "budeš",
	"budeme", "budete", "budú", "by", "bol", "bola", "boli",
	"bolo", "byť", "čo", "či", "ďalšia", "ďalšie", "ďalší",
	"dnes", "do", "ho", "i", "ja", "je", "jeho", "jej",
	"ich", "im", "iná", "iné", "iný", "k", "kam", "každá",
	"každé", "každí", "každý", "kde", "keď", "kto", "ktorá",
	"ktoré", "ktorí", "ktorou", "ktorý", "ktorých", "ku",
	"lebo", "len", "ma", "má", "máte", "mať", "medzi",
	"menej", "mňa", "môcť", "môj", "môže", "my", "na",
	"nad", "nám", "náš", "naši", "ne", "než", "nič",
	"nie", "nová", "nové", "noví", "nový", "o", "od",
	"odo", "of", "on", "ona", "ono", "oni", "ony",
	"po", "pod", "podľa", "pokiaľ", "potom", "práve", "pre",
	"prečo", "preto", "pretože", "pri", "s", "sa", "so",
	"si", "sú", "svoj", "svoje", "svojich", "svojím",
	"svojími", "ta", "tá", "tak", "takže", "táto", "teda",
	"tej", "tento", "tieto", "tí", "to", "toho", "tohto",
	"tom", "tomto", "toto", "tým", "týmto", "tvoj", "tvoje",
	"tvoji", "ty", "už", "v", "vám", "váš", "vaši",
	"veľmi", "vo", "však", "všetci", "všetky", "všetko",
	"vy", "z", "za", "zo", "že",
}

// SlovakAnalyzer is an analyzer for Slovak language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.sk.SlovakAnalyzer.
//
// SlovakAnalyzer uses the StandardTokenizer with Slovak stop words removal.
type SlovakAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewSlovakAnalyzer creates a new SlovakAnalyzer with default Slovak stop words.
func NewSlovakAnalyzer() *SlovakAnalyzer {
	stopSet := GetWordSetFromStrings(SlovakStopWords, true)
	return NewSlovakAnalyzerWithWords(stopSet)
}

// NewSlovakAnalyzerWithWords creates a SlovakAnalyzer with custom stop words.
func NewSlovakAnalyzerWithWords(stopWords *CharArraySet) *SlovakAnalyzer {
	a := &SlovakAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *SlovakAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *SlovakAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *SlovakAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*SlovakAnalyzer)(nil)
var _ AnalyzerInterface = (*SlovakAnalyzer)(nil)
