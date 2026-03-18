// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// RussianStopWords contains common Russian stop words.
var RussianStopWords = []string{
	"а", "без", "более", "бы", "был", "была", "были", "было", "быть", "в", "вам",
	"вас", "весь", "во", "вот", "все", "всего", "всех", "вы", "где", "да", "для",
	"до", "его", "ее", "ей", "ему", "если", "есть", "еще", "ж", "же", "за", "зачем",
	"и", "из", "или", "им", "их", "к", "как", "кем", "ко", "когда", "кого", "ком",
	"кому", "которая", "которого", "которой", "которые", "который", "которых", "кроме",
	"кто", "куда", "ли", "лучше", "между", "меня", "мне", "много", "мной", "мог",
	"может", "мои", "мой", "моя", "мы", "на", "над", "надо", "нам", "нас", "наш",
	"не", "него", "нее", "нет", "ни", "ним", "них", "но", "ну", "о", "об", "один",
	"он", "она", "они", "оно", "от", "перед", "по", "под", "при", "про", "с", "сам",
	"свое", "своего", "своей", "свои", "свой", "свою", "себе", "себя", "со", "так",
	"также", "такой", "там", "те", "тем", "теперь", "то", "того", "тоже", "той",
	"только", "том", "тот", "тут", "ты", "у", "уж", "уже", "хотя", "чего", "чем",
	"чему", "что", "чтобы", "чье", "чья", "эта", "эти", "этим", "этих", "это",
	"этого", "этой", "этом", "этот", "эту", "я",
}

// RussianAnalyzer is an analyzer for Russian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ru.RussianAnalyzer.
//
// RussianAnalyzer uses the StandardTokenizer with Russian stop words removal
// and light stemming.
type RussianAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewRussianAnalyzer creates a new RussianAnalyzer with default Russian stop words.
func NewRussianAnalyzer() *RussianAnalyzer {
	stopSet := GetWordSetFromStrings(RussianStopWords, true)
	return NewRussianAnalyzerWithWords(stopSet)
}

// NewRussianAnalyzerWithWords creates a RussianAnalyzer with custom stop words.
func NewRussianAnalyzerWithWords(stopWords *CharArraySet) *RussianAnalyzer {
	a := &RussianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewRussianLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *RussianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *RussianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *RussianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure RussianAnalyzer implements Analyzer
var _ Analyzer = (*RussianAnalyzer)(nil)
var _ AnalyzerInterface = (*RussianAnalyzer)(nil)

// RussianLightStemFilter implements light stemming for Russian.
type RussianLightStemFilter struct {
	*BaseTokenFilter
}

// NewRussianLightStemFilter creates a new RussianLightStemFilter.
func NewRussianLightStemFilter(input TokenStream) *RussianLightStemFilter {
	return &RussianLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *RussianLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := russianLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// russianLightStem applies light Russian stemming.
func russianLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Russian suffixes
	switch {
	// -ость, -ости, -остей (abstract nouns)
	case length > 4 && string(runes[length-4:]) == "ость":
		return string(runes[:length-3])
	case length > 4 && string(runes[length-4:]) == "ости":
		return string(runes[:length-3])
	// -ение, -ения, -ений
	case length > 4 && string(runes[length-4:]) == "ение":
		return string(runes[:length-3])
	case length > 4 && string(runes[length-4:]) == "ения":
		return string(runes[:length-3])
	// -ать, -ять, -ить, -еть (infinitive endings)
	case length > 2 && runes[length-1] == 'ь' && (runes[length-2] == 'т' || runes[length-2] == 'я' || runes[length-2] == 'и' || runes[length-2] == 'е'):
		return string(runes[:length-2])
	// -ов, -ев, -ыв (verb suffixes)
	case length > 2 && (string(runes[length-2:]) == "ов" || string(runes[length-2:]) == "ев" || string(runes[length-2:]) == "ыв"):
		return string(runes[:length-2])
	// -ый, -ий, -ая, -яя, -ое, -ее (adjective endings)
	case length > 2 && (runes[length-1] == 'й' || runes[length-1] == 'я' || runes[length-1] == 'е'):
		return string(runes[:length-2])
	// -а, -я, -о, -е, -ы, -и (noun endings)
	case length > 1 && (runes[length-1] == 'а' || runes[length-1] == 'я' || runes[length-1] == 'о' || runes[length-1] == 'е' || runes[length-1] == 'ы' || runes[length-1] == 'и'):
		return string(runes[:length-1])
	}

	return term
}

// RussianLightStemFilterFactory creates RussianLightStemFilter instances.
type RussianLightStemFilterFactory struct{}

// NewRussianLightStemFilterFactory creates a new RussianLightStemFilterFactory.
func NewRussianLightStemFilterFactory() *RussianLightStemFilterFactory {
	return &RussianLightStemFilterFactory{}
}

// Create creates a new RussianLightStemFilter.
func (f *RussianLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewRussianLightStemFilter(input)
}

// Ensure RussianLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*RussianLightStemFilterFactory)(nil)
