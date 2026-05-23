// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// CzechStopWords contains common Czech stop words.
// Source: Apache Lucene Czech stop words list
var CzechStopWords = []string{
	"a", "aby", "aj", "ak", "ake", "ako", "aky", "ale", "anebo", "ani",
	"áno", "asi", "až", "bez", "bude", "budem", "budeš", "by", "byť",
	"čo", "či", "ďalší", "ďalšia", "ďalšie", "dnes", "do", "ho",
	"i", "ja", "je", "jeho", "jej", "ich", "k", "kam", "každý",
	"každá", "každé", "každí", "kde", "ked", "kedy", "keď", "kto",
	"ktorá", "ktoré", "ktorí", "ktorou", "ktorý", "ktorých", "kto",
	"ku", "lebo", "len", "ma", "má", "máte", "mať", "medzi",
	"menej", "mňa", "môcť", "môj", "môže", "my", "na", "nad",
	"nám", "náš", "naši", "nie", "nech", "než", "nič", "niektorý",
	"nové", "nový", "nová", "nové", "nový", "o", "od", "odo",
	"of", "on", "ona", "ono", "oni", "oný", "po", "pod",
	"podľa", "pokiaľ", "potom", "práve", "pre", "prečo", "preto",
	"pretože", "prvý", "prvá", "prvé", "prví", "pred", "predo",
	"pri", "pýta", "s", "sa", "so", "si", "sú", "svoj", "svoje",
	"svojich", "svojím", "svojími", "ta", "tak", "takže", "táto",
	"teda", "tej", "tento", "tieto", "tým", "týmto", "tvoj", "tvoje",
	"tvojich", "ty", "teda", "tieto", "tým", "týmto", "už", "v",
	"vám", "váš", "vaši", "veľmi", "vo", "však", "všetok", "vy",
	"z", "za", "že", "aby", "aj", "ak", "ako", "ale", "ani",
	"áno", "asi", "až", "bez", "bude", "budem", "budeš", "by",
	"bol", "bola", "boli", "bolo", "byť", "čo", "či", "ďalší",
	"ďalšia", "ďalšie", "dnes", "do", "ho", "i", "ja", "je",
	"jeho", "jej", "ich", "k", "kam", "každý", "každá", "každé",
	"každí", "kde", "keď", "kto", "ktorá", "ktoré", "ktorí",
	"ktorou", "ktorý", "ktorých", "ku", "lebo", "len", "ma",
	"má", "máte", "mať", "medzi", "menej", "mňa", "môcť", "môj",
	"môže", "my", "na", "nad", "nám", "náš", "naši", "nie",
	"nech", "než", "nič", "niektorý", "nové", "nový", "nová",
	"nové", "nový", "o", "od", "odo", "of", "on", "ona",
	"ono", "oni", "oný", "po", "pod", "podľa", "pokiaľ",
	"potom", "práve", "pre", "prečo", "preto", "pretože", "prvý",
	"prvá", "prvé", "prví", "pred", "predo", "pri", "pýta",
	"s", "sa", "so", "si", "sú", "svoj", "svoje", "svojich",
	"svojím", "svojími", "ta", "tak", "takže", "táto", "teda",
	"tej", "tento", "tieto", "tým", "týmto", "tvoj", "tvoje",
	"tvojich", "ty", "teda", "tieto", "tým", "týmto", "už",
	"v", "vám", "váš", "vaši", "veľmi", "vo", "však", "všetok",
	"vy", "z", "za", "že",
}

// CzechAnalyzer is an analyzer for Czech language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.cz.CzechAnalyzer.
//
// CzechAnalyzer uses the StandardTokenizer with Czech stop words removal and light stemming.
type CzechAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewCzechAnalyzer creates a new CzechAnalyzer with default Czech stop words.
func NewCzechAnalyzer() *CzechAnalyzer {
	stopSet := GetWordSetFromStrings(CzechStopWords, true)
	return NewCzechAnalyzerWithWords(stopSet)
}

// NewCzechAnalyzerWithWords creates a CzechAnalyzer with custom stop words.
func NewCzechAnalyzerWithWords(stopWords *CharArraySet) *CzechAnalyzer {
	a := &CzechAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewCzechStemFilterFactory())
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *CzechAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *CzechAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *CzechAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*CzechAnalyzer)(nil)
var _ AnalyzerInterface = (*CzechAnalyzer)(nil)

// CzechStemFilter implements light stemming for Czech via the full CzechStemmer
// algorithm.
//
// Go port of org.apache.lucene.analysis.cz.CzechStemFilter (Apache Lucene
// 10.4.0).
type CzechStemFilter struct {
	*BaseTokenFilter
	stemmer czechStemmer
}

// NewCzechStemFilter creates a new CzechStemFilter.
func NewCzechStemFilter(input TokenStream) *CzechStemFilter {
	return &CzechStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies Czech stemming.
func (f *CzechStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				runes := []rune(termAttr.String())
				newLen := f.stemmer.stem(runes, len(runes))
				termAttr.SetValue(string(runes[:newLen]))
			}
		}
	}

	return hasToken, nil
}

// CzechStemFilterFactory creates CzechStemFilter instances.
type CzechStemFilterFactory struct{}

// NewCzechStemFilterFactory creates a new CzechStemFilterFactory.
func NewCzechStemFilterFactory() *CzechStemFilterFactory {
	return &CzechStemFilterFactory{}
}

// Create creates a new CzechStemFilter.
func (f *CzechStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewCzechStemFilter(input)
}

var _ TokenFilterFactory = (*CzechStemFilterFactory)(nil)
