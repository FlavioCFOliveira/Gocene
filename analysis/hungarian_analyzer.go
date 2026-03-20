// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// HungarianStopWords contains common Hungarian stop words.
// Source: Apache Lucene Hungarian stop words list
var HungarianStopWords = []string{
	"a", "abba", "abban", "abból", "addig", "ahhoz", "ahogy", "ahol", "akár",
	"aki", "akik", "akkor", "alapján", "alatt", "alá", "alól", "által",
	"általában", "ám", "amely", "amelyből", "amelyek", "amelyekben",
	"amelyeket", "amelyet", "amelyik", "amelynek", "ami", "amíg", "amit",
	"amolyan", "amott", "annak", "annál", "arra", "arról", "át", "az",
	"azok", "azokat", "azokból", "azon", "azonban", "azonnal", "azt",
	"aztán", "azzá", "azért", "bár", "be", "belül", "benne", "bár",
	"cikk", "cikkek", "cikkeket", "csak", "de", "de", "e", "é", "eddig",
	"egy", "egyes", "egyetlen", "egyik", "egyéb", "egyik", "egymás",
	"egyre", "egész", "ehhez", "ekkor", "el", "elég", "ellen", "ellenére",
	"elmondta", "első", "elő", "előbb", "először", "előtt", "emellett",
	"én", "éppen", "erre", "és", "és", "ezen", "ezért", "ezután", "fel",
	"felé", "hanem", "hiszen", "hogy", "hogyan", "igen", "ill", "ill.",
	"illetve", "ilyen", "ilyenkor", "ismét", "ison", "itt", "j", "jó",
	"jól", "jólesik", "kell", "kellene", "kelljen", "keressünk", "keresztül",
	"késő", "később", "későn", "két", "kétszer", "kívül", "közben",
	"közé", "közt", "közül", "külön", "le", "legalább", "legyen", "lehet",
	"lehetetlen", "lehetőleg", "lenne", "lennék", "lennének", "lesz",
	"leszek", "less", "lett", "lettek", "lettem", "lettünk", "lévő",
	"ma", "maga", "magad", "magam", "magát", "máig", "már", "más",
	"másik", "mások", "mást", "meg", "még", "mellett", "mely", "melyek",
	"melyik", "mennyi", "mert", "mi", "míg", "miért", "mikor", "milyen",
	"min", "mind", "minden", "mindenes", "mindig", "mint", "mintha",
	"miss", "mit", "mivel", "most", "nagy", "nagyobb", "nagyon", "nála",
	"néha", "néhány", "nélkül", "nem", "nincs", "nyújt", "oda", "ok",
	"ő", "ők", "őket", "olyan", "onnan", "ott", "pedig", "például",
	"persze", "rá", "s", "saját", "sok", "sokáig", "sokszor", "számára",
	"szemben", "szerint", "szinte", "szóval", "talán", "tartalmaz",
	"tartalmaznak", "te", "téged", "tegnap", "tényleg", "ti", "több",
	"tovább", "továbbá", "többi", "túl", "úgy", "úgyis", "úgynevezett",
	"új", "újabb", "újra", "után", "utána", "utolsó", "vagy", "vagyis",
	"vagyok", "valaki", "valakié", "valakinek", "valakit", "valamelyik",
	"valami", "valaminek", "valamint", "való", "valóban", "valónak",
	"van", "vannak", "végig", "végül", "végülis", "vele", "vissza", "volna",
	"volnának", "volt", "voltak", "voltam", "voltunk", "ön", "önök",
	"önöké", "önöknek", "önöket", "önökön", "önre", "össze", "úgyhogy",
}

// HungarianAnalyzer is an analyzer for Hungarian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.hu.HungarianAnalyzer.
//
// HungarianAnalyzer uses the StandardTokenizer with Hungarian stop words removal and light stemming.
type HungarianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewHungarianAnalyzer creates a new HungarianAnalyzer with default Hungarian stop words.
func NewHungarianAnalyzer() *HungarianAnalyzer {
	stopSet := GetWordSetFromStrings(HungarianStopWords, true)
	return NewHungarianAnalyzerWithWords(stopSet)
}

// NewHungarianAnalyzerWithWords creates a HungarianAnalyzer with custom stop words.
func NewHungarianAnalyzerWithWords(stopWords *CharArraySet) *HungarianAnalyzer {
	a := &HungarianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewHungarianLightStemFilterFactory())
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *HungarianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *HungarianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *HungarianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*HungarianAnalyzer)(nil)
var _ AnalyzerInterface = (*HungarianAnalyzer)(nil)

// HungarianLightStemmer implements light stemming for Hungarian language.
type HungarianLightStemmer struct{}

// NewHungarianLightStemmer creates a new HungarianLightStemmer.
func NewHungarianLightStemmer() *HungarianLightStemmer {
	return &HungarianLightStemmer{}
}

// Stem performs light stemming on a Hungarian word.
func (s *HungarianLightStemmer) Stem(word string) string {
	if len(word) < 4 {
		return word
	}

	runes := []rune(word)
	length := len(runes)

	// Hungarian has many case suffixes
	switch {
	// Plural and case suffixes
	// -ok, -ek, -ök (plural nominative)
	case length > 3 && (string(runes[length-2:]) == "ok" ||
		string(runes[length-2:]) == "ek" ||
		string(runes[length-2:]) == "ök"):
		return string(runes[:length-2])
	// -ak, -ik (plural for some stems)
	case length > 3 && (string(runes[length-2:]) == "ak" ||
		string(runes[length-2:]) == "ik"):
		return string(runes[:length-2])
	// -nak, -nek (dative)
	case length > 4 && (string(runes[length-3:]) == "nak" ||
		string(runes[length-3:]) == "nek"):
		return string(runes[:length-3])
	// -val, -vel (instrumental)
	case length > 4 && (string(runes[length-3:]) == "val" ||
		string(runes[length-3:]) == "vel"):
		return string(runes[:length-3])
	// -hoz, -hez, -höz (directional)
	case length > 4 && (string(runes[length-3:]) == "hoz" ||
		string(runes[length-3:]) == "hez" ||
		string(runes[length-3:]) == "höz"):
		return string(runes[:length-3])
	// -ban, -ben (inessive)
	case length > 4 && (string(runes[length-3:]) == "ban" ||
		string(runes[length-3:]) == "ben"):
		return string(runes[:length-3])
	// -ból, -ből (elative)
	case length > 4 && (string(runes[length-3:]) == "ból" ||
		string(runes[length-3:]) == "ből"):
		return string(runes[:length-3])
	// -nál, -nél (adessive)
	case length > 4 && (string(runes[length-3:]) == "nál" ||
		string(runes[length-3:]) == "nél"):
		return string(runes[:length-3])
	// -tól, -től (delative)
	case length > 4 && (string(runes[length-3:]) == "tól" ||
		string(runes[length-3:]) == "től"):
		return string(runes[:length-3])
	// -ba, -be (illative)
	case length > 3 && (string(runes[length-2:]) == "ba" ||
		string(runes[length-2:]) == "be"):
		return string(runes[:length-2])
	// -ra, -re (sublative)
	case length > 3 && (string(runes[length-2:]) == "ra" ||
		string(runes[length-2:]) == "re"):
		return string(runes[:length-2])
	// -on, -ön (superessive)
	case length > 3 && (string(runes[length-2:]) == "on" ||
		string(runes[length-2:]) == "ön"):
		return string(runes[:length-2])
	// -an, -en (inessive for some stems)
	case length > 3 && (string(runes[length-2:]) == "an" ||
		string(runes[length-2:]) == "en"):
		return string(runes[:length-2])
	// -t (accusative) - only for longer words
	case length > 4 && runes[length-1] == 't':
		return string(runes[:length-1])
	}

	return word
}

// HungarianLightStemFilter implements light stemming for Hungarian.
type HungarianLightStemFilter struct {
	*BaseTokenFilter
	stemmer *HungarianLightStemmer
}

// NewHungarianLightStemFilter creates a new HungarianLightStemFilter.
func NewHungarianLightStemFilter(input TokenStream) *HungarianLightStemFilter {
	return &HungarianLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewHungarianLightStemmer(),
	}
}

// IncrementToken processes the next token and applies Hungarian light stemming.
func (f *HungarianLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := f.stemmer.Stem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// HungarianLightStemFilterFactory creates HungarianLightStemFilter instances.
type HungarianLightStemFilterFactory struct{}

// NewHungarianLightStemFilterFactory creates a new HungarianLightStemFilterFactory.
func NewHungarianLightStemFilterFactory() *HungarianLightStemFilterFactory {
	return &HungarianLightStemFilterFactory{}
}

// Create creates a new HungarianLightStemFilter.
func (f *HungarianLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewHungarianLightStemFilter(input)
}

var _ TokenFilterFactory = (*HungarianLightStemFilterFactory)(nil)
