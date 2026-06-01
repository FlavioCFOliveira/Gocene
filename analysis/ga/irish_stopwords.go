// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package ga provides Irish (Gaeilge) language analysis components.
package ga

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// IrishStopWords contains default Irish stop words from the Apache Lucene
// resource file org/apache/lucene/analysis/snowball/irish_stop.txt.
var IrishStopWords = []string{
	"a", "ach", "ag", "agus", "an", "aon", "ar", "arna",
	"as", "b'", "ba", "beirt", "bhur", "caoga", "ceathair", "ceathracha",
	"céad", "chuig", "cois", "cén", "cér", "chomh", "chtó", "d'", "daichead",
	"dar", "de", "deich", "deichniúr", "den", "dhá", "do", "don",
	"dtí", "dá", "dár", "dó", "é", "faoi", "faoin", "faoina",
	"faoinár", "fara", "fiche", "gach", "gan", "go", "gur", "gurb",
	"i", "iad", "idir", "in", "ina", "ins", "inár", "is",
	"le", "leis", "lena", "lenár", "mar", "mé", "mo", "muid",
	"na", "nach", "naoi", "naonúr", "ná", "ní", "níl", "nó",
	"nách", "nár", "ó", "ochtó", "ochtar", "os", "roimh", "sa",
	"seacht", "seachtó", "seachtar", "seasca", "seisear", "siad", "sibh", "sin",
	"sna", "so", "sé", "sí", "tar", "thar", "thú", "triúr",
	"trí", "tríocha", "tú", "um", "ár", "é", "éis",
}

// irishArticles is the set of Irish articles to remove via elision.
var irishArticles = analysis.GetWordSetFromStrings([]string{"d", "m", "b"}, true)

// irishHyphenations is the set of prefixes whose hyphen does not create
// a position increment gap.
var irishHyphenations = analysis.GetWordSetFromStrings([]string{"h", "n", "t"}, true)

// IrishAnalyzer is an Analyzer for Irish (Gaeilge).
//
// This is the Go port of org.apache.lucene.analysis.ga.IrishAnalyzer from
// Apache Lucene 10.4.0.
//
// Analysis chain:
//  1. StandardTokenizer
//  2. StopFilter (removes h/n/t hyphenation prefixes)
//  3. ElisionFilter (removes articles d/m/b)
//  4. IrishLowerCaseFilter
//  5. StopFilter (removes default Irish stop words)
//  6. (optional) SetKeywordMarkerFilter for stem exclusion
//  7. SnowballFilter / IrishStemmer (placeholder — Snowball Go port pending)
type IrishAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords        *analysis.CharArraySet
	stemExclusionSet *analysis.CharArraySet
}

// NewIrishAnalyzer builds an IrishAnalyzer with default stop words.
func NewIrishAnalyzer() *IrishAnalyzer {
	stopSet := analysis.GetWordSetFromStrings(IrishStopWords, true)
	return NewIrishAnalyzerWithStopwords(stopSet)
}

// NewIrishAnalyzerWithStopwords builds an IrishAnalyzer with the given stop words.
func NewIrishAnalyzerWithStopwords(stopWords *analysis.CharArraySet) *IrishAnalyzer {
	return NewIrishAnalyzerFull(stopWords, analysis.NewCharArraySet(0, false))
}

// NewIrishAnalyzerFull builds an IrishAnalyzer with stop words and stem
// exclusion set. Tokens in the exclusion set are not stemmed.
func NewIrishAnalyzerFull(stopWords, stemExclusionSet *analysis.CharArraySet) *IrishAnalyzer {
	a := &IrishAnalyzer{
		BaseAnalyzer:     analysis.NewAnalyzer(),
		stopWords:        stopWords,
		stemExclusionSet: stemExclusionSet,
	}
	a.TokenizerFactory = analysis.NewStandardTokenizerFactory()
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(irishHyphenations))
	a.AddTokenFilter(analysis.NewElisionFilterFactory(irishArticles))
	a.AddTokenFilter(analysis.NewIrishLowerCaseFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	if stemExclusionSet != nil && !stemExclusionSet.IsEmpty() {
		a.AddTokenFilter(analysis.NewSetKeywordMarkerFilterFactoryWithSet(stemExclusionSet))
	}
	// Snowball IrishStemmer is deferred to the snowball sprint.
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *IrishAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *IrishAnalyzer) GetStopWords() *analysis.CharArraySet { return a.stopWords }

// GetStemExclusionSet returns the stem exclusion set.
func (a *IrishAnalyzer) GetStemExclusionSet() *analysis.CharArraySet { return a.stemExclusionSet }

// Ensure IrishAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*IrishAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*IrishAnalyzer)(nil)
