// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// BrazilianPortugueseStopWords contains common Brazilian Portuguese stop words.
// Source: Apache Lucene Brazilian Portuguese stop words list
var BrazilianPortugueseStopWords = []string{
	// Articles
	"a", "as", "o", "os",
	// Prepositions
	"à", "ao", "aos", "às", "ante", "após", "até", "com", "conforme",
	"contra", "de", "desde", "durante", "em", "entre", "para", "perante",
	"por", "sem", "sob", "sobre", "trás",
	// Pronouns
	"aquele", "aqueles", "aquela", "aquelas", "aquilo", "algum", "alguma",
	"alguns", "algumas", "alguma", "algo", "alguém", "cada", "certo",
	"certa", "certos", "certas", "cujo", "cuja", "cujos", "cujas",
	"demais", "diferente", "diferentes", "diversos", "diversas", "ela",
	"elas", "ele", "eles", "embora", "essa", "essas", "esse", "esses",
	"esta", "estas", "este", "estes", "eu", "isso", "isto", "já",
	"lhe", "lhes", "mais", "mas", "me", "menos", "mesmo", "mesma",
	"mesmos", "mesmas", "meu", "meus", "minha", "minhas", "muito",
	"muitos", "muita", "muitas", "nenhum", "nenhuma", "nenhuns",
	"nenhumas", "no", "nos", "na", "nas", "nossa", "nossas", "nosso",
	"nossos", "num", "numa", "nuns", "numas", "o", "os", "ou", "outro",
	"outra", "outros", "outras", "pelo", "pelos", "pela", "pelas",
	"pouco", "pouca", "poucos", "poucas", "próprio", "própria",
	"próprios", "próprias", "qual", "qualquer", "quaisquer", "quando",
	"quanto", "quantos", "quanta", "quantas", "que", "quem", "se",
	"seu", "seus", "sua", "suas", "tal", "tão", "tanto", "tantos",
	"tanta", "tantas", "te", "teu", "teus", "toda", "todas", "todo",
	"todos","tudo", "um", "uma", "uns", "umas", "você", "vocês",
	// Conjunctions
	"e", "nem", "ou", "ora", "já", "que", "porque", "pois", "mas",
	"porém", "todavia", "contudo", "entretanto", "então", "logo",
	"portanto", "por isso", "consequentemente", "assim", "desse modo",
	"desse jeito", "desse forma", "embora", "conquanto", "ainda que",
	"mesmo que", "se bem que", "quando", "enquanto", "se", "caso",
	"desde que", "contanto que", "a menos que", "a não ser que",
	// Common adverbs
	"aqui", "aí", "ali", "lá", "cá", "acolá", "lá", "bem", "mal",
	"assim", "adrede", "melhor", "pior", "depressa", "devagar",
	"acinte", "por acaso", "às vezes", "nunca", "jamais", "sempre",
	"ora", "já", "depois", "antes", "tarde", "cedo", "agora",
	// Common verbs
	"é", "são", "era", "eram", "foi", "foram", "ser", "sendo",
	"sido", "está", "estão", "estava", "estavam", "esteve",
	"estiveram", "está", "estar", "estando", "estado", "tem",
	"têm", "tinha", "tinham", "teve", "tiveram", "ter", "tendo",
	"tido", "há", "hão", "havia", "haviam", "houve", "houveram",
	"haver", "havendo", "havido", "faz", "fazem", "fazia", "faziam",
	"fez", "fizeram", "fazer", "fazendo", "feito", "pode", "podem",
	"podia", "podiam", "pôde", "puderam", "poder", "podendo", "podido",
	"vai", "vão", "ia", "iam", "foi", "foram", "ir", "indo", "ido",
	"dá", "dão", "dava", "davam", "deu", "deram", "dar", "dando",
	"dado", "sai", "saem", "saía", "saíam", "saiu", "saíram", "sair",
	"saindo", "saído", "fica", "ficam", "ficava", "ficavam", "ficou",
	"ficaram", "ficar", "ficando", "ficado", "vai", "vão", "ia",
	"iam", "foi", "foram", "ir", "indo", "ido", "vem", "vêm",
	"vinha", "vinham", "veio", "vieram", "vir", "vindo", "vindo",
}

// BrazilianAnalyzer is an analyzer for Brazilian Portuguese language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.br.BrazilianAnalyzer.
//
// BrazilianAnalyzer uses the StandardTokenizer with Brazilian Portuguese stop words removal
// and light stemming.
type BrazilianAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewBrazilianAnalyzer creates a new BrazilianAnalyzer with default Brazilian Portuguese stop words.
func NewBrazilianAnalyzer() *BrazilianAnalyzer {
	stopSet := GetWordSetFromStrings(BrazilianPortugueseStopWords, true)
	return NewBrazilianAnalyzerWithWords(stopSet)
}

// NewBrazilianAnalyzerWithWords creates a BrazilianAnalyzer with custom stop words.
func NewBrazilianAnalyzerWithWords(stopWords *CharArraySet) *BrazilianAnalyzer {
	a := &BrazilianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewBrazilianStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *BrazilianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *BrazilianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *BrazilianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure BrazilianAnalyzer implements Analyzer
var _ Analyzer = (*BrazilianAnalyzer)(nil)
var _ AnalyzerInterface = (*BrazilianAnalyzer)(nil)

// BrazilianStemFilter implements light stemming for Brazilian Portuguese.
type BrazilianStemFilter struct {
	*BaseTokenFilter
}

// NewBrazilianStemFilter creates a new BrazilianStemFilter.
func NewBrazilianStemFilter(input TokenStream) *BrazilianStemFilter {
	return &BrazilianStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *BrazilianStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := brazilianLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// brazilianLightStem applies light Brazilian Portuguese stemming.
func brazilianLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	runes := []rune(term)
	length := len(runes)

	// Remove common Brazilian Portuguese suffixes
	switch {
	// -mente (adverb suffix)
	case length > 5 && string(runes[length-5:]) == "mente":
		return string(runes[:length-5])
	// -ção, -ções
	case length > 3 && string(runes[length-3:]) == "ção":
		return string(runes[:length-3])
	case length > 5 && string(runes[length-5:]) == "ções":
		return string(runes[:length-5])
	// -dade, -dades
	case length > 4 && string(runes[length-4:]) == "dade":
		return string(runes[:length-4])
	case length > 5 && string(runes[length-5:]) == "dades":
		return string(runes[:length-5])
	// -ez, -eza
	case length > 2 && string(runes[length-2:]) == "ez":
		return string(runes[:length-2])
	case length > 3 && string(runes[length-3:]) == "eza":
		return string(runes[:length-3])
	// -ico, -ica, -icos, -icas
	case length > 4 && (string(runes[length-4:]) == "icos" || string(runes[length-4:]) == "icas"):
		return string(runes[:length-4])
	case length > 3 && (string(runes[length-3:]) == "ico" || string(runes[length-3:]) == "ica"):
		return string(runes[:length-3])
	// -ismo, -ismos
	case length > 5 && string(runes[length-5:]) == "ismos":
		return string(runes[:length-5])
	case length > 4 && string(runes[length-4:]) == "ismo":
		return string(runes[:length-4])
	// -ista, -istas
	case length > 5 && string(runes[length-5:]) == "istas":
		return string(runes[:length-5])
	case length > 4 && string(runes[length-4:]) == "ista":
		return string(runes[:length-4])
	// -ável, -ível
	case length > 4 && (string(runes[length-4:]) == "ável" || string(runes[length-4:]) == "ível"):
		return string(runes[:length-4])
	// -oso, -osa, -osos, -osas
	case length > 4 && (string(runes[length-4:]) == "osos" || string(runes[length-4:]) == "osas"):
		return string(runes[:length-4])
	case length > 3 && (string(runes[length-3:]) == "oso" || string(runes[length-3:]) == "osa"):
		return string(runes[:length-3])
	// -ar, -er, -ir (infinitive endings) - only for longer words
	case length > 5 && runes[length-1] == 'r' && (runes[length-2] == 'a' || runes[length-2] == 'e' || runes[length-2] == 'i'):
		return string(runes[:length-2])
	// -s (plural) - only for longer words
	case length > 4 && runes[length-1] == 's':
		return string(runes[:length-1])
	}

	return term
}

// BrazilianStemFilterFactory creates BrazilianStemFilter instances.
type BrazilianStemFilterFactory struct{}

// NewBrazilianStemFilterFactory creates a new BrazilianStemFilterFactory.
func NewBrazilianStemFilterFactory() *BrazilianStemFilterFactory {
	return &BrazilianStemFilterFactory{}
}

// Create creates a new BrazilianStemFilter.
func (f *BrazilianStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewBrazilianStemFilter(input)
}

// Ensure BrazilianStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*BrazilianStemFilterFactory)(nil)
