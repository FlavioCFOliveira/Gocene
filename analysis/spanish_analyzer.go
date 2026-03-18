// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// SpanishStopWords contains common Spanish stop words.
var SpanishStopWords = []string{
	"a", "al", "algo", "algunas", "algunos", "ante", "antes", "como", "con", "contra",
	"cual", "cuando", "de", "del", "desde", "donde", "durante", "e", "el", "ella",
	"ellas", "ellos", "en", "entre", "era", "erais", "eran", "eras", "eres", "es",
	"esa", "esas", "ese", "eso", "esos", "esta", "estaba", "estabais", "estaban",
	"estabas", "estad", "estada", "estadas", "estado", "estados", "estamos", "estando",
	"estar", "estaremos", "estará", "estarán", "estarás", "estaré", "estaréis",
	"estaría", "estaríais", "estaríamos", "estarían", "estarías", "estas", "este",
	"estemos", "esto", "estos", "estoy", "estuve", "estuviera", "estuvierais",
	"estuvieran", "estuvieras", "estuvieron", "estuviese", "estuvieseis", "estuviesen",
	"estuvieses", "estuvimos", "estuviste", "estuvisteis", "estuviéramos",
	"estuviésemos", "estuvo", "fue", "fuera", "fuerais", "fueran", "fueras",
	"fueron", "fuese", "fueseis", "fuesen", "fueses", "fui", "fuimos", "fuiste",
	"fuisteis", "fuéramos", "fuésemos", "ha", "habida", "habidas", "habido", "habidos",
	"habiendo", "habremos", "habrá", "habrán", "habrás", "habré", "habréis",
	"habría", "habríais", "habríamos", "habrían", "habrías", "han", "has", "hasta",
	"hay", "haya", "hayamos", "hayan", "hayas", "hayáis", "he", "hemos", "hube",
	"hubiera", "hubierais", "hubieran", "hubieras", "hubieron", "hubiese",
	"hubieseis", "hubiesen", "hubieses", "hubimos", "hubiste", "hubisteis",
	"hubiéramos", "hubiésemos", "hubo", "la", "las", "le", "les", "lo", "los", "me",
	"mi", "mis", "mucho", "muchos", "muy", "más", "mí", "mía", "mías", "mío", "míos",
	"nada", "ni", "no", "nos", "nosotras", "nosotros", "nuestra", "nuestras",
	"nuestro", "nuestros", "o", "os", "otra", "otras", "otro", "otros", "para",
	"pero", "poco", "por", "porque", "que", "quien", "quienes", "qué", "se", "sea",
	"seamos", "sean", "seas", "seremos", "será", "serán", "serás", "seré", "seréis",
	"sería", "seríais", "seríamos", "serían", "serías", "seáis", "sido", "siendo",
	"sin", "sobre", "sois", "somos", "son", "soy", "su", "sus", "suya", "suyas",
	"suyo", "suyos", "sí", "también", "tanto", "te", "tendremos", "tendrá",
	"tendrán", "tendrás", "tendré", "tendréis", "tendría", "tendríais",
	"tendríamos", "tendrían", "tendrías", "tened", "tenemos", "tenga", "tengamos",
	"tengan", "tengas", "tengo", "tengáis", "tenida", "tenidas", "tenido",
	"tenidos", "teniendo", "tenéis", "tenía", "teníais", "teníamos", "tenían",
	"tenías", "ti", "tiene", "tienen", "tienes", "todo", "todos", "tu", "tus",
	"tuve", "tuviera", "tuvierais", "tuvieran", "tuvieras", "tuvieron", "tuviese",
	"tuvieseis", "tuviesen", "tuvieses", "tuvimos", "tuviste", "tuvisteis",
	"tuviéramos", "tuviésemos", "tuvo", "tuya", "tuyas", "tuyo", "tuyos", "tú",
	"un", "una", "uno", "unos", "vosotras", "vosotros", "vuestra", "vuestras",
	"vuestro", "vuestros", "y", "ya", "yo", "él", "éramos",
}

// SpanishAnalyzer is an analyzer for Spanish language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.es.SpanishAnalyzer.
//
// SpanishAnalyzer uses the StandardTokenizer with Spanish stop words removal
// and light stemming.
type SpanishAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewSpanishAnalyzer creates a new SpanishAnalyzer with default Spanish stop words.
func NewSpanishAnalyzer() *SpanishAnalyzer {
	stopSet := GetWordSetFromStrings(SpanishStopWords, true)
	return NewSpanishAnalyzerWithWords(stopSet)
}

// NewSpanishAnalyzerWithWords creates a SpanishAnalyzer with custom stop words.
func NewSpanishAnalyzerWithWords(stopWords *CharArraySet) *SpanishAnalyzer {
	a := &SpanishAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewSpanishLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *SpanishAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *SpanishAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *SpanishAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure SpanishAnalyzer implements Analyzer
var _ Analyzer = (*SpanishAnalyzer)(nil)
var _ AnalyzerInterface = (*SpanishAnalyzer)(nil)

// SpanishLightStemFilter implements light stemming for Spanish.
type SpanishLightStemFilter struct {
	*BaseTokenFilter
}

// NewSpanishLightStemFilter creates a new SpanishLightStemFilter.
func NewSpanishLightStemFilter(input TokenStream) *SpanishLightStemFilter {
	return &SpanishLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *SpanishLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := spanishLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// spanishLightStem applies light Spanish stemming.
func spanishLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Spanish suffixes
	switch {
	// -ción, -sión
	case length > 4 && (string(runes[length-4:]) == "ción" || string(runes[length-4:]) == "sión"):
		return string(runes[:length-3])
	// -mente
	case length > 5 && string(runes[length-5:]) == "mente":
		return string(runes[:length-4])
	// -idad, -ades
	case length > 4 && (string(runes[length-4:]) == "idad" || string(runes[length-4:]) == "ades"):
		return string(runes[:length-3])
	// -ando, -iendo
	case length > 4 && (string(runes[length-4:]) == "ando" || string(runes[length-4:]) == "iendo"):
		return string(runes[:length-3])
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

// SpanishLightStemFilterFactory creates SpanishLightStemFilter instances.
type SpanishLightStemFilterFactory struct{}

// NewSpanishLightStemFilterFactory creates a new SpanishLightStemFilterFactory.
func NewSpanishLightStemFilterFactory() *SpanishLightStemFilterFactory {
	return &SpanishLightStemFilterFactory{}
}

// Create creates a new SpanishLightStemFilter.
func (f *SpanishLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewSpanishLightStemFilter(input)
}

// Ensure SpanishLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*SpanishLightStemFilterFactory)(nil)
