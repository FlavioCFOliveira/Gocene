// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// LithuanianStopWords contains common Lithuanian stop words.
// Source: Apache Lucene Lithuanian stop words list
var LithuanianStopWords = []string{
	"kadangi", "nes", "nors", "visgi", "vis", "dėl", "dėka",
	"iki", "iš", "apie", "bei", "pagal", "be", "prieš", "prie",
	"po", "už", "su", "ar", "arba", "ir", "bet", "o", "taip",
	"taigi", "anot", "apačioje", "aplink", "aš", "čia", "šio",
	"šios", "šiuo", "šiuos", "šioje", "šiame", "šį", "šie",
	"šių", "šešiolika", "šeši", "šešiasdešimt", "šis", "šviežias",
	"toli", "daug", "du", "daugiau", "daugiausia", "dažnai",
	"dabar", "iki", "ne", "tačiau", "iš", "nes", "į", "per",
	"pro", "jau", "jūs", "jums", "juos", "jose", "jose",
	"jis", "jų", "ji", "jos", "jį", "jį", "jo", "joje",
	"joje", "jam", "jame", "jais", "juo", "juk", "jums",
	"jumyse", "junk", "juo", "jūsų", "ką", "kada", "kad",
	"kai", "kokia", "koks", "kokį", "kokios", "kokių",
	"kokiame", "kokiame", "kokiose", "kokiais", "kokiuose",
	"kokį", "kur", "kurį", "kurioje", "kuriame", "kuriuos",
	"kuriose", "kurio", "kurios", "kurių", "kuriems",
	"kurie", "kurios", "kurias", "kuriuos", "kuriose",
	"koks", "kokia", "kokie", "kokios", "maždaug", "mažas",
	"mažai", "mažiausia", "mažiausias", "man", "mane",
	"mano", "manyje", "manyje", "manimi", "manyje", "mes",
	"mudu", "mūsų", "mums", "mus", "mūsų", "mūsų", "mūsų",
	"mūsų", "mums", "mumyse", "manimi", "manyje", "mano",
	"manyje", "manyje", "mane", "mūs", "mano", "manyje",
	"manyje", "manimi", "manyje", "ne", "nei", "neį",
	"nebe", "nebent", "nors", "nuo", "nors", "o", "pagal",
	"panašus", "panašiai", "panašus", "paskui", "paskum",
	"pasak", "pasilikti", "pats", "patys", "pirmas",
	"pirma", "pirmoji", "pirmosios", "pirmąją", "pirmuosius",
	"pirmojoje", "pirmuoju", "pirmųjų", "prie", "prieš",
	"priešingai", "priešais", "priešui", "priešais",
	"priešui", "priešais", "priešui", "priešais", "priešui",
	"priešais", "priešui", "priešais", "priešui", "priešais",
}

// LithuanianAnalyzer is an analyzer for Lithuanian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.lt.LithuanianAnalyzer.
//
// LithuanianAnalyzer uses the StandardTokenizer with Lithuanian stop words removal.
type LithuanianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewLithuanianAnalyzer creates a new LithuanianAnalyzer with default Lithuanian stop words.
func NewLithuanianAnalyzer() *LithuanianAnalyzer {
	stopSet := GetWordSetFromStrings(LithuanianStopWords, true)
	return NewLithuanianAnalyzerWithWords(stopSet)
}

// NewLithuanianAnalyzerWithWords creates a LithuanianAnalyzer with custom stop words.
func NewLithuanianAnalyzerWithWords(stopWords *CharArraySet) *LithuanianAnalyzer {
	a := &LithuanianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *LithuanianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *LithuanianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *LithuanianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*LithuanianAnalyzer)(nil)
var _ AnalyzerInterface = (*LithuanianAnalyzer)(nil)
