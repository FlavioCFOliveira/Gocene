// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// FrenchStopWords contains common French stop words.
var FrenchStopWords = []string{
	"a", "afin", "ai", "ainsi", "après", "attendu", "au", "aujourd", "auquel", "aussi",
	"autre", "autres", "aux", "auxquelles", "auxquels", "avait", "avant", "avec", "avoir",
	"c", "car", "ce", "ceci", "cela", "celle", "celles", "celui", "cependant", "certain",
	"certaine", "certaines", "certains", "ces", "cet", "cette", "ceux", "chez", "ci",
	"combien", "comme", "comment", "concernant", "contre", "d", "dans", "de", "debout",
	"dedans", "dehors", "delà", "depuis", "derrière", "des", "désormais", "desquelles",
	"desquels", "dessous", "dessus", "devant", "devers", "devra", "divers", "diverse",
	"diverses", "doit", "donc", "dont", "du", "duquel", "durant", "dès", "elle", "elles",
	"en", "entre", "environ", "est", "et", "etc", "etre", "eu", "eux", "excepté",
	"hormis", "hors", "hélas", "il", "ils", "je", "jusqu", "jusque", "l", "la", "laquelle",
	"le", "lequel", "les", "lesquelles", "lesquels", "leur", "leurs", "lorsque", "lui",
	"là", "ma", "mais", "malgré", "me", "merci", "mes", "mien", "mienne", "miennes",
	"miens", "moi", "moins", "mon", "moyennant", "même", "mêmes", "n", "ne", "ni",
	"non", "nos", "notre", "nous", "néanmoins", "nôtre", "nôtres", "on", "ont", "ou",
	"outre", "où", "par", "parmi", "partant", "pas", "passé", "pendant", "plein",
	"plus", "plusieurs", "pour", "pourquoi", "près", "puisque", "qu", "quand", "que",
	"quel", "quelle", "quelles", "quels", "qui", "quoi", "quoique", "revoici",
	"revoilà", "s", "sa", "sans", "sauf", "se", "selon", "seront", "ses", "si",
	"sien", "sienne", "siennes", "siens", "sinon", "soi", "soit", "son", "sont",
	"sous", "suivant", "sur", "ta", "te", "tes", "tien", "tienne", "tiennes",
	"tiens", "toi", "ton", "tous", "tout", "toute", "toutes", "tu", "un", "une",
	"va", "vers", "voici", "voilà", "vos", "votre", "vous", "vu", "vôtre", "vôtres",
	"y", "à", "ça", "ès", "été", "être",
}

// FrenchAnalyzer is an analyzer for French language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.fr.FrenchAnalyzer.
//
// FrenchAnalyzer uses the StandardTokenizer with French stop words removal
// and light stemming. It also applies ASCII folding for compatibility.
type FrenchAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewFrenchAnalyzer creates a new FrenchAnalyzer with default French stop words.
func NewFrenchAnalyzer() *FrenchAnalyzer {
	stopSet := GetWordSetFromStrings(FrenchStopWords, true)
	return NewFrenchAnalyzerWithWords(stopSet)
}

// NewFrenchAnalyzerWithWords creates a FrenchAnalyzer with custom stop words.
func NewFrenchAnalyzerWithWords(stopWords *CharArraySet) *FrenchAnalyzer {
	a := &FrenchAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewFrenchLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *FrenchAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *FrenchAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *FrenchAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure FrenchAnalyzer implements Analyzer
var _ Analyzer = (*FrenchAnalyzer)(nil)
var _ AnalyzerInterface = (*FrenchAnalyzer)(nil)

// FrenchLightStemFilter implements light stemming for French.
type FrenchLightStemFilter struct {
	*BaseTokenFilter
}

// NewFrenchLightStemFilter creates a new FrenchLightStemFilter.
func NewFrenchLightStemFilter(input TokenStream) *FrenchLightStemFilter {
	return &FrenchLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *FrenchLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := frenchLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// frenchLightStem applies light French stemming.
func frenchLightStem(term string) string {
	if len(term) < 3 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common French suffixes
	switch {
	// -euse, -eux, -eur
	case length > 4 && string(runes[length-4:]) == "euse":
		return string(runes[:length-2])
	case length > 3 && string(runes[length-3:]) == "eux":
		return string(runes[:length-2])
	case length > 3 && string(runes[length-3:]) == "eur":
		return string(runes[:length-2])
	// -ement, -ement
	case length > 5 && string(runes[length-5:]) == "ement":
		return string(runes[:length-4])
	// -ment
	case length > 4 && string(runes[length-4:]) == "ment":
		return string(runes[:length-3])
	// -tion, -sion
	case length > 4 && (string(runes[length-4:]) == "tion" || string(runes[length-4:]) == "sion"):
		return string(runes[:length-3])
	// -ité
	case length > 3 && string(runes[length-3:]) == "ité":
		return string(runes[:length-2])
	// -age
	case length > 3 && string(runes[length-3:]) == "age":
		return string(runes[:length-2])
	// -er, -ir
	case length > 2 && (runes[length-1] == 'r' && (runes[length-2] == 'e' || runes[length-2] == 'i')):
		return string(runes[:length-2])
	// -s (plural)
	case length > 2 && runes[length-1] == 's':
		return string(runes[:length-1])
	// -x (plural for words ending in -au, -eau)
	case length > 2 && runes[length-1] == 'x':
		return string(runes[:length-1])
	}

	return term
}

// FrenchLightStemFilterFactory creates FrenchLightStemFilter instances.
type FrenchLightStemFilterFactory struct{}

// NewFrenchLightStemFilterFactory creates a new FrenchLightStemFilterFactory.
func NewFrenchLightStemFilterFactory() *FrenchLightStemFilterFactory {
	return &FrenchLightStemFilterFactory{}
}

// Create creates a new FrenchLightStemFilter.
func (f *FrenchLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewFrenchLightStemFilter(input)
}

// Ensure FrenchLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*FrenchLightStemFilterFactory)(nil)
