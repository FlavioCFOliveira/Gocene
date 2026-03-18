// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// ItalianStopWords contains common Italian stop words.
var ItalianStopWords = []string{
	"a", "abbia", "abbiamo", "abbiano", "abbiate", "ad", "agl", "agli", "ai", "al",
	"all", "alla", "alle", "allo", "anche", "avemmo", "avendo", "avesse", "avessero",
	"avessi", "avessimo", "aveste", "avesti", "avete", "aveva", "avevamo", "avevano",
	"avevate", "avevi", "avevo", "avrai", "avranno", "avrebbe", "avrebbero",
	"avrei", "avremmo", "avremo", "avrete", "avresti", "avuto", "c", "che", "chi",
	"ci", "coi", "col", "come", "con", "contro", "cosa", "cui", "da", "dagl",
	"dagli", "dai", "dal", "dall", "dalla", "dalle", "dallo", "degl", "degli", "dei",
	"del", "dell", "della", "delle", "dello", "di", "dove", "e", "è", "ebbe",
	"ebbero", "ebbi", "ed", "egli", "ella", "essendo", "facendo", "fai", "fanno",
	"farà", "farai", "faranno", "farebbe", "farebbero", "farei", "faremmo", "faremo",
	"fareste", "faresti", "farò", "fece", "fecero", "feci", "fosse", "fossero",
	"fossi", "fossimo", "foste", "fosti", "fu", "fui", "fummo", "furono", "gli",
	"ha", "hai", "hanno", "ho", "i", "il", "in", "io", "l", "la", "le", "lei",
	"li", "lo", "loro", "lui", "ma", "mi", "mia", "mie", "miei", "mio", "ne",
	"negl", "negli", "nei", "nel", "nell", "nella", "nelle", "nello", "noi", "non",
	"nostra", "nostre", "nostri", "nostro", "o", "per", "perché", "più", "quale",
	"quanta", "quante", "quanti", "quanto", "quella", "quelle", "quelli", "quello",
	"questa", "queste", "questi", "questo", "sarà", "sarai", "saranno", "sarebbe",
	"sarebbero", "sarei", "saremmo", "saremo", "sareste", "saresti", "sarò", "sia",
	"siamo", "siano", "siate", "siete", "sono", "sta", "stai", "stanno", "starà",
	"starai", "staranno", "starebbe", "starebbero", "starei", "staremmo", "staremo",
	"stareste", "staresti", "starò", "stava", "stavamo", "stavano", "stavate",
	"stavi", "stavo", "stemmo", "stesse", "stessero", "stessi", "stessimo", "steste",
	"stesti", "stette", "stettero", "stetti", "stia", "stiamo", "stiano", "stiate",
	"sto", "su", "sua", "sue", "sugl", "sugli", "sui", "sul", "sull", "sulla",
	"sulle", "sullo", "suo", "suoi", "ti", "tra", "tu", "tua", "tue", "tuo",
	"tuoi", "tutti", "tutto", "un", "una", "uno", "voi", "vostra", "vostre",
	"vostri", "vostro",
}

// ItalianAnalyzer is an analyzer for Italian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.it.ItalianAnalyzer.
//
// ItalianAnalyzer uses the StandardTokenizer with Italian stop words removal
// and light stemming.
type ItalianAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewItalianAnalyzer creates a new ItalianAnalyzer with default Italian stop words.
func NewItalianAnalyzer() *ItalianAnalyzer {
	stopSet := GetWordSetFromStrings(ItalianStopWords, true)
	return NewItalianAnalyzerWithWords(stopSet)
}

// NewItalianAnalyzerWithWords creates an ItalianAnalyzer with custom stop words.
func NewItalianAnalyzerWithWords(stopWords *CharArraySet) *ItalianAnalyzer {
	a := &ItalianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewItalianLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *ItalianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *ItalianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *ItalianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure ItalianAnalyzer implements Analyzer
var _ Analyzer = (*ItalianAnalyzer)(nil)
var _ AnalyzerInterface = (*ItalianAnalyzer)(nil)

// ItalianLightStemFilter implements light stemming for Italian.
type ItalianLightStemFilter struct {
	*BaseTokenFilter
}

// NewItalianLightStemFilter creates a new ItalianLightStemFilter.
func NewItalianLightStemFilter(input TokenStream) *ItalianLightStemFilter {
	return &ItalianLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *ItalianLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := italianLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// italianLightStem applies light Italian stemming.
func italianLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Italian suffixes
	switch {
	// -zione, -zioni
	case length > 5 && string(runes[length-5:]) == "zione":
		return string(runes[:length-4])
	case length > 5 && string(runes[length-5:]) == "zioni":
		return string(runes[:length-4])
	// -mento, -menti
	case length > 5 && string(runes[length-5:]) == "mento":
		return string(runes[:length-4])
	case length > 5 && string(runes[length-5:]) == "menti":
		return string(runes[:length-4])
	// -tà (abstract nouns)
	case length > 2 && runes[length-1] == 'à':
		return string(runes[:length-1])
	// -are, -ere, -ire (infinitive endings)
	case length > 3 && runes[length-1] == 'e' && (runes[length-2] == 'r' && (runes[length-3] == 'a' || runes[length-3] == 'e' || runes[length-3] == 'i')):
		return string(runes[:length-3])
	// -o, -a, -i, -e (gender/plural)
	case length > 1 && (runes[length-1] == 'o' || runes[length-1] == 'a'):
		return string(runes[:length-1])
	case length > 1 && (runes[length-1] == 'i' || runes[length-1] == 'e'):
		return string(runes[:length-1])
	}

	return term
}

// ItalianLightStemFilterFactory creates ItalianLightStemFilter instances.
type ItalianLightStemFilterFactory struct{}

// NewItalianLightStemFilterFactory creates a new ItalianLightStemFilterFactory.
func NewItalianLightStemFilterFactory() *ItalianLightStemFilterFactory {
	return &ItalianLightStemFilterFactory{}
}

// Create creates a new ItalianLightStemFilter.
func (f *ItalianLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewItalianLightStemFilter(input)
}

// Ensure ItalianLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*ItalianLightStemFilterFactory)(nil)
