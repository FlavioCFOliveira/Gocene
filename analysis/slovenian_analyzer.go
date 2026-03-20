// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// SlovenianStopWords contains common Slovenian stop words.
// Source: Apache Lucene Slovenian stop words list
var SlovenianStopWords = []string{
	"a", "ali", "april", "avgust", "b", "bi", "bil", "bila",
	"bile", "bili", "bilo", "biti", "blizu", "bo", "bojo",
	"bolj", "bom", "bomo", "boste", "boš", "brez", "c",
	"cel", "cela", "celi", "celo", "d", "da", "daleč",
	"dan", "danes", "datum", "december", "deset", "deseta",
	"deseti", "deseto", "devet", "deveta", "deveti", "deveto",
	"dni", "do", "dobro", "dokler", "dol", "dolgoročen",
	"dolgoročna", "dolgoročni", "dolgoročno", "dovolj", "drug",
	"druga", "drugi", "drugo", "dva", "dve", "e", "eden",
	"en", "ena", "ene", "eni", "enkrat", "eno", "etc",
	"f", "februar", "g", "gdč", "g", "glede", "gotov",
	"gotova", "gotovi", "gotovo", "h", "halo", "i", "idr",
	"ii", "iii", "in", "iv", "ix", "iz", "j", "januar",
	"julij", "junij", "jutri", "k", "kadarkoli", "kaj",
	"kajti", "kako", "kakor", "kamor", "kamorkoli", "kar",
	"karkoli", "katerikoli", "kdor", "kdorkoli", "ker",
	"ki", "kje", "kjer", "kjerkoli", "ko", "koder",
	"koderkoli", "koga", "komu", "kot", "kratek", "kratka",
	"kratke", "kratki", "l", "lahka", "lahke", "lahki",
	"lahko", "le", "lep", "lepa", "lepe", "lepi",
	"lepo", "leto", "m", "maj", "malce", "malo", "mar",
	"marec", "me", "med", "medtem", "mene", "mesec",
	"mi", "midva", "midve", "mnogo", "moj", "moja",
	"moje", "mora", "morajo", "moram", "moramo", "morate",
	"moraš", "morem", "mu", "n", "na", "nad", "naj",
	"naju", "nam", "nas", "nato", "nazaj", "ne", "nek",
	"neka", "nekaj", "nekatere", "nekateri", "nekatero",
	"nekdo", "neke", "nekega", "neki", "nekje", "neko",
	"nekoga", "nekoč", "nekoliko", "nekteri", "nekolikor",
	"nemara", "neposredno", "nečesa", "nečim", "nečim",
	"nečimer", "ni", "nje", "njej", "njega", "njegov",
	"njegova", "njegovo", "njemu", "njen", "njena", "njeno",
	"nji", "njih", "njihov", "njihova", "njihovo", "njim",
	"njimi", "njo", "njun", "njuna", "njuno", "no",
	"nocoj", "november", "o", "ob", "oba", "obe", "oboje",
	"od", "odkar", "odkod", "odkodkoli", "okoli", "oktober",
	"on", "onadva", "one", "oni", "onidve", "osem", "osma",
	"osmi", "osmo", "oz", "p", "pa", "po", "pod",
	"poj", "ponovno", "potem", "povsod", "pred", "prej",
	"preko", "pri", "s", "sa", "sam", "sama", "sami",
	"samo", "se", "sebe", "sebi", "sedem", "sedma",
	"sedmi", "sedmo", "sem", "september", "si", "sicer",
	"skoraj", "skozi", "slab", "so", "spet", "sreda",
	"srednji", "sta", "ste", "stvar", "sva", "t",
	"ta", "tak", "taka", "take", "taki", "tako",
	"takoj", "tam", "te", "tebe", "tebi", "tega",
	"težak", "težka", "težki", "težko", "ti", "tista",
	"tiste", "tisti", "tisto", "tj", "tja", "to",
	"toda", "torej", "tretja", "tretje", "tretji", "tri",
	"tu", "tudi", "tve", "u", "v", "vaju", "vam",
	"vas", "vaš", "vaša", "vaše", "več", "vedno",
	"velik", "velika", "velike", "veliki", "veliko",
	"vendar", "ves", "več", "vi", "vidva", "vii",
	"viii", "vse", "vsego", "vsi", "vso", "včasih",
	"včeraj", "x", "xi", "xii", "xiii", "xiv", "xv",
	"xvi", "xvii", "xviii", "xix", "xx", "z", "za",
	"zaj", "zakaj", "zapored", "zapri", "zaradi", "zato",
	"zda", "zdaj", "zelo", "zunaj", "č", "če", "često",
	"četrta", "četrtek", "četrto", "čez", "čigav", "š",
	"šest", "šesta", "šesti", "šesto", "štiri", "ž",
}

// SlovenianAnalyzer is an analyzer for Slovenian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.sl.SlovenianAnalyzer.
//
// SlovenianAnalyzer uses the StandardTokenizer with Slovenian stop words removal.
type SlovenianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewSlovenianAnalyzer creates a new SlovenianAnalyzer with default Slovenian stop words.
func NewSlovenianAnalyzer() *SlovenianAnalyzer {
	stopSet := GetWordSetFromStrings(SlovenianStopWords, true)
	return NewSlovenianAnalyzerWithWords(stopSet)
}

// NewSlovenianAnalyzerWithWords creates a SlovenianAnalyzer with custom stop words.
func NewSlovenianAnalyzerWithWords(stopWords *CharArraySet) *SlovenianAnalyzer {
	a := &SlovenianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *SlovenianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *SlovenianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *SlovenianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*SlovenianAnalyzer)(nil)
var _ AnalyzerInterface = (*SlovenianAnalyzer)(nil)
