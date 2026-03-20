// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// SerbianStopWords contains common Serbian stop words.
// Source: Apache Lucene Serbian stop words list
var SerbianStopWords = []string{
	"a", "ako", "ali", "baš", "bez", "bi", "bila", "bili", "bilo",
	"bio", "biti", "biće", "blizu", "boje", "buđav", "ce", "ča",
	"čas", "ćemo", "ćete", "ću", "ćeš", "da", "dana", "danas",
	"do", "dobar", "dobiti", "dodati", "dok", "dole", "doći",
	"drugi", "drugovi", "duž", "ga", "gde", "gdeće", "gladni",
	"gotovo", "govoriti", "hde", "hijerarhija", "hoće", "hoću",
	"hteti", "htio", "hvala", "i", "iako", "ide", "ih", "iju",
	"ili", "im", "imaju", "inače", "io", "iš", "išta", "itd",
	"ići", "iz", "izgleda", "iznad", "izvan", "izvoli", "ja",
	"jadan", "jah", "jasno", "je", "jedan", "jedini", "jednom",
	"jeste", "još", "ju", "juče", "kako", "kao", "katalog", "kaze",
	"kaže", "kazeš", "kemija", "ki", "klijent", "koji", "koliko",
	"kome", "kroz", "kuda", "koji", "koja", "koje", "koju",
	"lako", "levo", "li", "ljud", "mali", "manje", "me", "meni",
	"mesec", "mi", "milja", "mnogo", "moći", "mogao", "moja",
	"moje", "moji", "mora", "moram", "morao", "možda", "muku",
	"na", "nad", "nadam", "nakon", "nam", "nama", "nas", "naš",
	"naša", "naše", "naši", "ne", "nego", "neka", "neki", "neko",
	"nema", "nemam", "nešto", "neću", "ni", "nije", "nikada",
	"nikoga", "nikoje", "nikom", "nimalo", "nista", "ništa",
	"njega", "njegov", "njegova", "njegovo", "njemu", "njen",
	"njena", "njeno", "njih", "njihov", "njihova", "njihovo",
	"njim", "njima", "njoj", "nju", "no", "o", "od", "odakle",
	"odmah", "odnost", "oko", "omogućiti", "on", "ona", "one",
	"oni", "onim", "onima", "ono", "onoj", "onom", "onu", "onog",
	"oprosti", "osim", "ostali", "ova", "ovaj", "ovamo", "ove",
	"ovi", "ovim", "ovima", "ovo", "pa", "pak", "pitati", "po",
	"početak", "početi", "pod", "poj", "pone", "ponovo", "počinje",
	"pored", "povodom", "pravo", "pre", "preko", "prema", "prvi",
	"put", "radije", "raspored", "reći", "s", "sa", "sada",
	"samo", "se", "sebe", "sebi", "shodno", "si", "smo", "sob",
	"some", "spreman", "srp", "stvar", "su", "sutra", "svi",
	"svi", "svim", "svima", "svoj", "svoja", "svoje", "svoju",
	"ta", "tada", "taj", "tako", "takođe", "tamo", "te", "tek",
	"tema", "ti", "tim", "time", "tome", "totalno", "treba",
	"tri", "tuda", "tvoj", "tvoja", "tvoje", "tvoji", "u",
	"ubuduće", "udruženje", "uopšte", "upravo", "usled", "uvek",
	"vaš", "vaša", "vaše", "vaši", "već", "večeras", "veoma",
	"verovali", "vi", "više", "vođenje", "vrh", "za", "zaista",
	"zajedno", "započinje", "zapravo", "zar", "zbog", "zdravo",
	"želeti", "želeo", "želi", "zelim", "znati", "ćemo", "ćeš",
	"ćete", "će", "čak", "članovi", "šta", "što", "žao",
}

// SerbianAnalyzer is an analyzer for Serbian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.sr.SerbianAnalyzer.
//
// SerbianAnalyzer uses the StandardTokenizer with Serbian stop words removal.
type SerbianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewSerbianAnalyzer creates a new SerbianAnalyzer with default Serbian stop words.
func NewSerbianAnalyzer() *SerbianAnalyzer {
	stopSet := GetWordSetFromStrings(SerbianStopWords, true)
	return NewSerbianAnalyzerWithWords(stopSet)
}

// NewSerbianAnalyzerWithWords creates a SerbianAnalyzer with custom stop words.
func NewSerbianAnalyzerWithWords(stopWords *CharArraySet) *SerbianAnalyzer {
	a := &SerbianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *SerbianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *SerbianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *SerbianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*SerbianAnalyzer)(nil)
var _ AnalyzerInterface = (*SerbianAnalyzer)(nil)
