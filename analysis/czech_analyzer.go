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

// CzechStemFilter implements light stemming for Czech.
type CzechStemFilter struct {
	*BaseTokenFilter
}

// NewCzechStemFilter creates a new CzechStemFilter.
func NewCzechStemFilter(input TokenStream) *CzechStemFilter {
	return &CzechStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *CzechStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := czechLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// czechLightStem applies light Czech stemming.
func czechLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	runes := []rune(term)
	length := len(runes)

	// Remove common Czech suffixes
	switch {
	// -ovat, -ovávat (verb suffixes)
	case length > 5 && string(runes[length-5:]) == "ovat":
		return string(runes[:length-3])
	case length > 7 && string(runes[length-7:]) == "ovávat":
		return string(runes[:length-5])
	// -ost, -osti (noun suffixes)
	case length > 4 && string(runes[length-4:]) == "osti":
		return string(runes[:length-2])
	case length > 3 && string(runes[length-3:]) == "ost":
		return string(runes[:length-1])
	// -né (adjective suffix)
	case length > 3 && string(runes[length-3:]) == "né":
		return string(runes[:length-2])
	// -ých (plural adjective)
	case length > 3 && string(runes[length-3:]) == "ých":
		return string(runes[:length-2])
	// -ým (instrumental adjective)
	case length > 3 && string(runes[length-3:]) == "ým":
		return string(runes[:length-2])
	// -s (plural) - only for longer words
	case length > 5 && runes[length-1] == 's':
		return string(runes[:length-1])
	}

	return term
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
