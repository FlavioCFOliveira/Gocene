// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// SwedishStopWords contains common Swedish stop words.
var SwedishStopWords = []string{
	"aderton", "adertonde", "adjö", "aldrig", "alla", "allas", "allt", "alltid",
	"alltså", "än", "andra", "andras", "annan", "annat", "ännu", "artonde",
	"artonn", "åtminstone", "att", "åtta", "åttio", "åttionde", "åttonde", "av",
	"även", "båda", "bådas", "bakom", "bara", "bäst", "bättre", "behöva",
	"behövas", "behövde", "behövt", "beslut", "beslutat", "beslutit", "bland",
	"blev", "bli", "blir", "blivit", "bort", "borta", "bra", "då", "dag", "dagar",
	"dagarna", "dagen", "där", "därför", "de", "del", "delen", "dem", "den",
	"deras", "dess", "det", "detta", "dig", "din", "dina", "ditt", "dokument",
	"dokumentet", "dom", "du", "efter", "eftersom", "elfte", "eller", "elva",
	"en", "enkel", "enkelt", "enkla", "enligt", "er", "era", "ert", "ett",
	"ettusen", "få", "fanns", "får", "fått", "fem", "femte", "femtio",
	"femtionde", "femton", "femtonde", "fick", "fin", "finnas", "finns",
	"fjärde", "fjorton", "fjortonde", "fler", "flera", "flesta", "följande",
	"för", "före", "förlåt", "förra", "första", "fram", "framför", "från",
	"fyra", "fyrtio", "fyrtionde", "gå", "går", "gärna", "gått", "genast",
	"genom", "gick", "gjorde", "gjort", "god", "goda", "godare", "godast",
	"gör", "göra", "gott", "ha", "hade", "haft", "han", "hans", "har", "här",
	"heller", "helst", "henne", "hennes", "hit", "hög", "höger", "högre",
	"högst", "hon", "honom", "hundra", "hur", "i", "ibland", "icke", "idag",
	"ide", "igår", "igen", "imorgon", "in", "inför", "inga", "ingen",
	"ingenting", "inget", "innan", "inne", "inom", "inte", "inuti", "ja",
	"jag", "jämfört", "kan", "kanske", "knappast", "kom", "komma", "kommer",
	"kommit", "krävs", "kunna", "kunnat", "kvar", "länge", "längre",
	"längst", "lätt", "lättare", "lättast", "legat", "ligga", "ligger",
	"lika", "likställd", "likställda", "lilla", "lite", "liten", "litet",
	"man", "många", "måste", "med", "mellan", "men", "mer", "mera", "mest",
	"mig", "min", "mina", "mindre", "minst", "mitt", "mittemot", "möjlig",
	"möjligen", "möjligt", "mot", "mycket", "någon", "någonting", "något",
	"några", "när", "nästa", "ned", "nederst", "nedersta", "nedre", "nej",
	"ner", "ni", "nio", "nionde", "nittio", "nittionde", "nitton",
	"nittonde", "nödvändig", "nödvändiga", "nödvändigt", "nödvändigtvis",
	"nog", "noll", "nr", "nu", "nummer", "och", "också", "ofta", "oftast",
	"olika", "olikt", "om", "oss", "på", "rakt", "rätt", "redan", "så",
	"sade", "säga", "säger", "sagt", "samma", "sämre", "sämst", "sedan",
	"senare", "senast", "sent", "sex", "sextio", "sextionde", "sexton",
	"sextonde", "sig", "sin", "sina", "sist", "sista", "siste", "sitt",
	"själv", "sjätte", "sju", "sjunde", "sjuttio", "sjuttionde", "sjutton",
	"sjuttonde", "ska", "skall", "skulle", "slutligen", "små", "smått",
	"snart", "som", "stor", "stora", "större", "störst", "stort", "tack",
	"tidig", "tidigare", "tidigast", "tidigt", "till", "tills", "tillsammans",
	"tio", "tionde", "tjugo", "tjugoen", "tjugoett", "tjugonde", "tjugotre",
	"tjugotvå", "tjungo", "tolfte", "tolv", "tre", "tredje", "trettio",
	"trettionde", "tretton", "trettonde", "två", "tvåhundra", "under",
	"upp", "ur", "ursäkt", "ut", "utan", "ute", "vad", "vänster", "vänstra",
	"var", "vår", "vara", "våra", "varför", "varifrån", "varit", "varken",
	"värre", "varsågod", "vart", "vårt", "vem", "vems", "verkligen", "vi",
	"vid", "vilka", "vilken", "vilket", "vill",
}

// SwedishAnalyzer is an analyzer for Swedish language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.sv.SwedishAnalyzer.
//
// SwedishAnalyzer uses the StandardTokenizer with Swedish stop words removal
// and light stemming.
type SwedishAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewSwedishAnalyzer creates a new SwedishAnalyzer with default Swedish stop words.
func NewSwedishAnalyzer() *SwedishAnalyzer {
	stopSet := GetWordSetFromStrings(SwedishStopWords, true)
	return NewSwedishAnalyzerWithWords(stopSet)
}

// NewSwedishAnalyzerWithWords creates a SwedishAnalyzer with custom stop words.
func NewSwedishAnalyzerWithWords(stopWords *CharArraySet) *SwedishAnalyzer {
	a := &SwedishAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewSwedishLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *SwedishAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *SwedishAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *SwedishAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure SwedishAnalyzer implements Analyzer
var _ Analyzer = (*SwedishAnalyzer)(nil)
var _ AnalyzerInterface = (*SwedishAnalyzer)(nil)

// SwedishLightStemFilter implements light stemming for Swedish.
type SwedishLightStemFilter struct {
	*BaseTokenFilter
}

// NewSwedishLightStemFilter creates a new SwedishLightStemFilter.
func NewSwedishLightStemFilter(input TokenStream) *SwedishLightStemFilter {
	return &SwedishLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *SwedishLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := swedishLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// swedishLightStem applies light Swedish stemming.
func swedishLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Swedish suffixes
	switch {
	// -het, -heten (abstract nouns)
	case length > 4 && string(runes[length-4:]) == "heten":
		return string(runes[:length-3])
	case length > 3 && string(runes[length-3:]) == "het":
		return string(runes[:length-2])
	// -else, -elsen
	case length > 4 && string(runes[length-4:]) == "else":
		return string(runes[:length-3])
	case length > 5 && string(runes[length-5:]) == "elsen":
		return string(runes[:length-4])
	// -arna, -erna (definite plural)
	case length > 4 && (string(runes[length-4:]) == "arna" || string(runes[length-4:]) == "erna"):
		return string(runes[:length-3])
	// -ar, -er, -or (plural)
	case length > 2 && (runes[length-1] == 'r' || runes[length-1] == 'n') && runes[length-2] == 'e':
		return string(runes[:length-2])
	// -en, -et (definite)
	case length > 2 && (runes[length-1] == 'n' || runes[length-1] == 't') && runes[length-2] == 'e':
		return string(runes[:length-2])
	// -a, -e
	case length > 1 && (runes[length-1] == 'a' || runes[length-1] == 'e'):
		return string(runes[:length-1])
	}

	return term
}

// SwedishLightStemFilterFactory creates SwedishLightStemFilter instances.
type SwedishLightStemFilterFactory struct{}

// NewSwedishLightStemFilterFactory creates a new SwedishLightStemFilterFactory.
func NewSwedishLightStemFilterFactory() *SwedishLightStemFilterFactory {
	return &SwedishLightStemFilterFactory{}
}

// Create creates a new SwedishLightStemFilter.
func (f *SwedishLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSwedishLightStemFilter(input)
}

// Ensure SwedishLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*SwedishLightStemFilterFactory)(nil)
