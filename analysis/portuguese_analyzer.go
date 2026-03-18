// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// PortugueseStopWords contains common Portuguese stop words.
var PortugueseStopWords = []string{
	"a", "à", "ao", "aos", "aquela", "aquelas", "aquele", "aqueles", "aquilo", "as",
	"às", "até", "com", "como", "da", "das", "de", "dela", "delas", "dele", "deles",
	"depois", "do", "dos", "e", "é", "ela", "elas", "ele", "eles", "em", "entre",
	"era", "eram", "éramos", "essa", "essas", "esse", "esses", "esta", "está",
	"estamos", "estão", "estar", "estas", "estava", "estavam", "estávamos", "este",
	"esteja", "estejam", "estejamos", "estes", "esteve", "estive", "estivemos",
	"estiver", "estivera", "estiveram", "estivéramos", "estiverem", "estivermos",
	"estivesse", "estivessem", "estivéssemos", "estou", "eu", "foi", "fomos", "for",
	"fora", "foram", "fôramos", "forem", "formos", "fosse", "fossem", "fôssemos",
	"fui", "há", "haja", "hajam", "hajamos", "hão", "havemos", "hei", "houve",
	"houvemos", "houver", "houvera", "houveram", "houvéramos", "houverem", "houvermos",
	"houverão", "houvesse", "houvessem", "houvéssemos", "isso", "isto", "já", "lhe",
	"lhes", "mais", "mas", "me", "mesmo", "meu", "meus", "minha", "minhas", "muito",
	"na", "não", "nas", "nem", "no", "nos", "nossa", "nossas", "nosso", "nossos",
	"num", "numa", "nós", "o", "os", "ou", "para", "pela", "pelas", "pelo", "pelos",
	"por", "quando", "que", "quem", "são", "se", "seja", "sejam", "sejamos", "sem",
	"ser", "será", "serão", "seremos", "serei", "seria", "seriam", "seríamos",
	"seu", "seus", "só", "somos", "sou", "sua", "suas", "também", "te", "tem",
	"tém", "temos", "tenha", "tenham", "tenhamos", "tenho", "ter", "terá", "terão",
	"teremos", "teria", "teriam", "teríamos", "teu", "teus", "teve", "tinha",
	"tinham", "tínhamos", "tive", "tivemos", "tiver", "tivera", "tiveram",
	"tivéramos", "tiverem", "tivermos", "tivesse", "tivessem", "tivéssemos", "tu",
	"tua", "tuas", "um", "uma", "você", "vocês", "vos", "à", "às",
}

// PortugueseAnalyzer is an analyzer for Portuguese language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.pt.PortugueseAnalyzer.
//
// PortugueseAnalyzer uses the StandardTokenizer with Portuguese stop words removal
// and light stemming.
type PortugueseAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewPortugueseAnalyzer creates a new PortugueseAnalyzer with default Portuguese stop words.
func NewPortugueseAnalyzer() *PortugueseAnalyzer {
	stopSet := GetWordSetFromStrings(PortugueseStopWords, true)
	return NewPortugueseAnalyzerWithWords(stopSet)
}

// NewPortugueseAnalyzerWithWords creates a PortugueseAnalyzer with custom stop words.
func NewPortugueseAnalyzerWithWords(stopWords *CharArraySet) *PortugueseAnalyzer {
	a := &PortugueseAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewPortugueseLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *PortugueseAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *PortugueseAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *PortugueseAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure PortugueseAnalyzer implements Analyzer
var _ Analyzer = (*PortugueseAnalyzer)(nil)
var _ AnalyzerInterface = (*PortugueseAnalyzer)(nil)

// PortugueseLightStemFilter implements light stemming for Portuguese.
type PortugueseLightStemFilter struct {
	*BaseTokenFilter
}

// NewPortugueseLightStemFilter creates a new PortugueseLightStemFilter.
func NewPortugueseLightStemFilter(input TokenStream) *PortugueseLightStemFilter {
	return &PortugueseLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *PortugueseLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := portugueseLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// portugueseLightStem applies light Portuguese stemming.
func portugueseLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Portuguese suffixes
	switch {
	// -ção, -ções
	case length > 3 && string(runes[length-3:]) == "ção":
		return string(runes[:length-2])
	case length > 4 && string(runes[length-4:]) == "ções":
		return string(runes[:length-3])
	// -mente
	case length > 5 && string(runes[length-5:]) == "mente":
		return string(runes[:length-4])
	// -idade, -idades
	case length > 5 && string(runes[length-5:]) == "idade":
		return string(runes[:length-4])
	case length > 6 && string(runes[length-6:]) == "idades":
		return string(runes[:length-5])
	// -ar, -er, -ir (infinitive endings)
	case length > 2 && runes[length-1] == 'r' && (runes[length-2] == 'a' || runes[length-2] == 'e' || runes[length-2] == 'i'):
		return string(runes[:length-2])
	// -o, -a, -os, -as (gender/plural)
	case length > 1 && (runes[length-1] == 'o' || runes[length-1] == 'a'):
		return string(runes[:length-1])
	case length > 2 && (string(runes[length-2:]) == "os" || string(runes[length-2:]) == "as"):
		return string(runes[:length-2])
	// -es (plural)
	case length > 2 && string(runes[length-2:]) == "es":
		return string(runes[:length-2])
	}

	return term
}

// PortugueseLightStemFilterFactory creates PortugueseLightStemFilter instances.
type PortugueseLightStemFilterFactory struct{}

// NewPortugueseLightStemFilterFactory creates a new PortugueseLightStemFilterFactory.
func NewPortugueseLightStemFilterFactory() *PortugueseLightStemFilterFactory {
	return &PortugueseLightStemFilterFactory{}
}

// Create creates a new PortugueseLightStemFilter.
func (f *PortugueseLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewPortugueseLightStemFilter(input)
}

// Ensure PortugueseLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*PortugueseLightStemFilterFactory)(nil)
