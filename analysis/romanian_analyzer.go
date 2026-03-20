// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// RomanianStopWords contains common Romanian stop words.
// Source: Apache Lucene Romanian stop words list
var RomanianStopWords = []string{
	"a", "abia", "acea", "aceasta", "această", "aceea", "aceeasi",
	"aceia", "aceiași", "acela", "acelasi", "acele", "acelea",
	"aceluiași", "acest", "acesta", "aceste", "acestea", "acestei",
	"acestes", "acestor", "acestora", "acestui", "acestuia", "acestuiasi",
	"acolo", "acord", "acum", "adica", "ai", "aia", "aibă", "aici",
	"aiurea", "al", "ala", "alaturi", "ale", "alea", "alt", "alta",
	"altă", "altfel", "altii", "altilor", "altor", "altora", "altui",
	"altul", "am", "anume", "apoi", "ar", "are", "as", "asa", "asadar",
	"asemenea", "asta", "astăzi", "asupra", "atare", "atat", "atata",
	"atatea", "atatia", "atatia", "ati", "atit", "atita", "atitea",
	"atitia", "atunci", "au", "avea", "avem", "aveti", "aveți", "azi",
	"ba", "bine", "bucur", "bună", "ca", "cam", "cand", "capat",
	"care", "careia", "carora", "caruia", "cat", "catre", "caut",
	"ce", "cea", "ceea", "cei", "ceilalti", "cel", "ceva", "chiar",
	"ci", "cind", "cine", "cineva", "cit", "cita", "cite", "citeva",
	"citi", "citiva", "conform", "conforme", "conformitate", "conformității",
	"conformității", "conform", "conforme", "cu", "cum", "cumva",
	"da", "dacă", "dar", "datorită", "de", "deasupra", "deci", "decit",
	"deja", "deoarece", "departe", "despre", "deși", "din", "dinaintea",
	"dincolo", "dintr", "dintr-", "dintre", "doar", "doresc", "doriti",
	"doriti", "două", "dră", "dumneavoastră", "ea", "ei", "el", "ele",
	"era", "este", "eu", "exact", "face", "fata", "fă", "fără",
	"fata", "fel", "fi", "fie", "fiecare", "fii", "foarte", "fost",
	"frumos", "ftai", "ga", "gata", "graț", "grație", "grație",
	"h", "halbă", "iar", "ieri", "ii", "il", "îi", "îl", "împreună",
	"în", "încît", "între", "întru", "îți", "la", "lângă", "le",
	"li", "lîngă", "lor", "lui", "mă", "mai", "mâine", "mereu",
	"mi", "mie", "mîine", "mine", "mod", "mult", "multă", "mulți",
	"mulțumesc", "ne", "nevoie", "ni", "nici", "niciodată", "nicăieri",
	"nimeni", "nimeri", "nimic", "niste", "niște", "noastre", "noastră",
	"noi", "nostru", "nou", "nouă", "nu", "numai", "o", "opt",
	"or", "ori", "oricare", "oricât", "orice", "oricine", "oriunde",
	"până", "pentru", "peste", "pina", "plus", "poate", "pot", "prea",
	"prin", "prima", "primul", "prin", "priveste", "putini", "rău",
	"rog", "sa", "sa-mi", "sa-ti", "să", "să-mi", "să-ti", "săi",
	"sale", "sau", "său", "se", "si", "sînt", "sînte", "sîntem",
	"spate", "spre", "sub", "sunt", "suntem", "sunteți", "sută",
	"ta", "tăi", "tale", "tău", "te", "ti", "timp", "tine", "toată",
	"toate", "tocmai", "tot", "toti", "totul", "totusi", "toți",
	"trei", "treia", "treilea", "tu", "tuturor", "un", "una", "unde",
	"unei", "uneia", "unele", "unele", "uneori", "unii", "unor",
	"unora", "unu", "unui", "unuia", "unul", "va", "vă", "vi",
	"voastre", "voastră", "voi", "vostru", "vouă", "vreme", "vreo",
	"vreun", "zece", "zi", "zice",
}

// RomanianAnalyzer is an analyzer for Romanian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ro.RomanianAnalyzer.
//
// RomanianAnalyzer uses the StandardTokenizer with Romanian stop words removal.
type RomanianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewRomanianAnalyzer creates a new RomanianAnalyzer with default Romanian stop words.
func NewRomanianAnalyzer() *RomanianAnalyzer {
	stopSet := GetWordSetFromStrings(RomanianStopWords, true)
	return NewRomanianAnalyzerWithWords(stopSet)
}

// NewRomanianAnalyzerWithWords creates a RomanianAnalyzer with custom stop words.
func NewRomanianAnalyzerWithWords(stopWords *CharArraySet) *RomanianAnalyzer {
	a := &RomanianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *RomanianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *RomanianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *RomanianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*RomanianAnalyzer)(nil)
var _ AnalyzerInterface = (*RomanianAnalyzer)(nil)
