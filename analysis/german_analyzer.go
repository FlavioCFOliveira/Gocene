// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// GermanStopWords contains common German stop words.
var GermanStopWords = []string{
	"aber", "alle", "allem", "allen", "aller", "alles", "als", "also", "am", "an",
	"ander", "andere", "anderem", "anderen", "anderer", "anderes", "anderm", "andern",
	"anderr", "anders", "auch", "auf", "aus", "bei", "bin", "bis", "bist", "da",
	"damit", "dann", "der", "den", "des", "dem", "die", "das", "daß", "derselbe",
	"derselben", "denselben", "desselben", "demselben", "dieselbe", "dieselben",
	"dasselbe", "dazu", "dein", "deine", "deinem", "deinen", "deiner", "deines",
	"denn", "derer", "dessen", "dich", "dir", "du", "dies", "diese", "diesem",
	"diesen", "dieser", "dieses", "doch", "dort", "durch", "ein", "eine", "einem",
	"einen", "einer", "eines", "einig", "einige", "einigem", "einigen", "einiger",
	"einiges", "einmal", "er", "ihn", "ihm", "es", "etwas", "euer", "eure", "eurem",
	"euren", "eurer", "eures", "für", "gegen", "gewesen", "hab", "habe", "haben",
	"hat", "hatte", "hatten", "hier", "hin", "hinter", "ich", "mich", "mir", "ihr",
	"ihre", "ihrem", "ihren", "ihrer", "ihres", "euch", "im", "in", "indem", "ins",
	"ist", "jede", "jedem", "jeden", "jeder", "jedes", "jene", "jenem", "jenen",
	"jener", "jenes", "jetzt", "kann", "kein", "keine", "keinem", "keinen", "keiner",
	"keines", "können", "könnte", "machen", "man", "manche", "manchem", "manchen",
	"mancher", "manches", "mein", "meine", "meinem", "meinen", "meiner", "meines",
	"mit", "nach", "nichts", "noch", "nun", "nur", "ob", "oder", "ohne", "sehr",
	"sein", "seine", "seinem", "seinen", "seiner", "seines", "selbst", "sich",
	"sie", "ihnen", "sind", "so", "solche", "solchem", "solchen", "solcher",
	"solches", "soll", "sollte", "sondern", "sonst", "über", "um", "und", "uns",
	"unse", "unsem", "unsen", "unser", "unses", "unter", "viel", "vom", "von",
	"vor", "während", "war", "waren", "warst", "was", "weg", "weil", "weiter",
	"welche", "welchem", "welchen", "welcher", "welches", "wenn", "werde", "werden",
	"wie", "wieder", "will", "wir", "wird", "wirst", "wo", "wollen", "wollte",
	"würde", "würden", "zu", "zum", "zur", "zwar", "zwischen",
}

// GermanAnalyzer is an analyzer for German language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.de.GermanAnalyzer.
//
// GermanAnalyzer uses the StandardTokenizer with German stop words removal
// and light stemming. It also applies ASCII folding for compatibility.
type GermanAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewGermanAnalyzer creates a new GermanAnalyzer with default German stop words.
func NewGermanAnalyzer() *GermanAnalyzer {
	stopSet := GetWordSetFromStrings(GermanStopWords, true)
	return NewGermanAnalyzerWithWords(stopSet)
}

// NewGermanAnalyzerWithWords creates a GermanAnalyzer with custom stop words.
func NewGermanAnalyzerWithWords(stopWords *CharArraySet) *GermanAnalyzer {
	a := &GermanAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewGermanLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *GermanAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *GermanAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *GermanAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure GermanAnalyzer implements Analyzer
var _ Analyzer = (*GermanAnalyzer)(nil)
var _ AnalyzerInterface = (*GermanAnalyzer)(nil)

// GermanLightStemFilter implements light stemming for German.
type GermanLightStemFilter struct {
	*BaseTokenFilter
}

// NewGermanLightStemFilter creates a new GermanLightStemFilter.
func NewGermanLightStemFilter(input TokenStream) *GermanLightStemFilter {
	return &GermanLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *GermanLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := germanLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// germanLightStem applies light German stemming.
func germanLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common German suffixes
	switch {
	// -ungen, -ungen
	case length > 5 && string(runes[length-5:]) == "ungen":
		return string(runes[:length-4])
	// -ungen
	case length > 5 && string(runes[length-5:]) == "ungen":
		return string(runes[:length-4])
	// -chen, -lein (diminutives)
	case length > 4 && (string(runes[length-4:]) == "chen" || string(runes[length-4:]) == "lein"):
		return string(runes[:length-4])
	// -ern
	case length > 3 && string(runes[length-3:]) == "ern":
		return string(runes[:length-2])
	// -em, -en, -er, -es
	case length > 2 && (runes[length-1] == 'm' || runes[length-1] == 'n' || runes[length-1] == 'r' || runes[length-1] == 's'):
		// Check for preceding 'e'
		if runes[length-2] == 'e' {
			return string(runes[:length-2])
		}
	// -e
	case length > 1 && runes[length-1] == 'e':
		return string(runes[:length-1])
	}

	return term
}

// GermanLightStemFilterFactory creates GermanLightStemFilter instances.
type GermanLightStemFilterFactory struct{}

// NewGermanLightStemFilterFactory creates a new GermanLightStemFilterFactory.
func NewGermanLightStemFilterFactory() *GermanLightStemFilterFactory {
	return &GermanLightStemFilterFactory{}
}

// Create creates a new GermanLightStemFilter.
func (f *GermanLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGermanLightStemFilter(input)
}

// Ensure GermanLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*GermanLightStemFilterFactory)(nil)
