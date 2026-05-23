// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package stempel

import (
	"bytes"
	_ "embed"
	"io"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/egothor"
)

//go:embed stopwords.txt
var polishStopwordsData []byte

//go:embed stemmer_20000.tbl
var polishStemmerData []byte

var polishDefaultsOnce sync.Once
var polishDefaultStopSet *analysis.CharArraySet
var polishDefaultTable egothorTrie

func loadPolishDefaults() {
	polishDefaultsOnce.Do(func() {
		var err error
		polishDefaultStopSet, err = analysis.GetWordSetWithComment(
			bytes.NewReader(polishStopwordsData), "#",
			analysis.NewCharArraySet(64, true),
		)
		if err != nil {
			panic("unable to load Polish stop words: " + err.Error())
		}
		polishDefaultTable, err = Load(bytes.NewReader(polishStemmerData))
		if err != nil {
			panic("unable to load Polish stemmer table: " + err.Error())
		}
	})
}

// GetDefaultStopSet returns an unmodifiable instance of the default Polish
// stop words set.
func GetDefaultStopSet() *analysis.CharArraySet {
	loadPolishDefaults()
	return polishDefaultStopSet
}

// GetDefaultTable returns the default Polish stemmer Trie.
func GetDefaultTable() egothorTrie {
	loadPolishDefaults()
	return polishDefaultTable
}

// PolishAnalyzer is an Analyzer for Polish language text.
//
// Analysis chain:
//  1. StandardTokenizer
//  2. LowerCaseFilter
//  3. StopFilter (using Polish stop words)
//  4. SetKeywordMarkerFilter (only when stem exclusion set is non-empty)
//  5. StempelFilter (using the Polish stemmer table)
//
// This is the Go port of
// org.apache.lucene.analysis.pl.PolishAnalyzer (Lucene 10.4.0).
type PolishAnalyzer struct {
	*analysis.BaseAnalyzer
	stopWords        *analysis.CharArraySet
	stemExclusionSet *analysis.CharArraySet
	stemTable        egothorTrie
}

// NewPolishAnalyzer creates a PolishAnalyzer with the default stop words.
func NewPolishAnalyzer() *PolishAnalyzer {
	return NewPolishAnalyzerWithStopwords(GetDefaultStopSet())
}

// NewPolishAnalyzerWithStopwords creates a PolishAnalyzer with the given stop
// words and an empty stem exclusion set.
func NewPolishAnalyzerWithStopwords(stopwords *analysis.CharArraySet) *PolishAnalyzer {
	return NewPolishAnalyzerFull(stopwords, analysis.NewCharArraySet(0, false))
}

// NewPolishAnalyzerFull creates a PolishAnalyzer with the given stop words and
// stem exclusion set. Tokens in the exclusion set are not stemmed.
func NewPolishAnalyzerFull(stopwords, stemExclusionSet *analysis.CharArraySet) *PolishAnalyzer {
	loadPolishDefaults()
	a := &PolishAnalyzer{
		BaseAnalyzer:     analysis.NewAnalyzer(),
		stopWords:        stopwords,
		stemExclusionSet: stemExclusionSet,
		stemTable:        polishDefaultTable,
	}
	a.TokenizerFactory = analysis.NewStandardTokenizerFactory()
	a.AddTokenFilter(analysis.NewLowerCaseFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopwords))
	if stemExclusionSet != nil && !stemExclusionSet.IsEmpty() {
		a.AddTokenFilter(analysis.NewSetKeywordMarkerFilterFactoryWithSet(stemExclusionSet))
	}
	a.AddTokenFilter(newStempelFilterFactory(a.stemTable))
	return a
}

// GetStopWords returns the stop words used by this analyzer.
func (a *PolishAnalyzer) GetStopWords() *analysis.CharArraySet {
	return a.stopWords
}

// GetStemExclusionSet returns the stem exclusion set used by this analyzer.
func (a *PolishAnalyzer) GetStemExclusionSet() *analysis.CharArraySet {
	return a.stemExclusionSet
}

// TokenStream creates a TokenStream for the given field and reader.
func (a *PolishAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Ensure PolishAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*PolishAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*PolishAnalyzer)(nil)

// stempelFilterFactory is an internal TokenFilterFactory that creates a
// StempelFilter with a fixed pre-loaded trie.
type stempelFilterFactory struct {
	stemTable egothorTrie
}

func newStempelFilterFactory(trie egothorTrie) *stempelFilterFactory {
	return &stempelFilterFactory{stemTable: trie}
}

func (f *stempelFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	trie, ok := f.stemTable.(*egothor.MultiTrie2)
	if ok {
		_ = trie // satisfy use
	}
	return NewStempelFilter(input, NewStempelStemmer(f.stemTable))
}

// Ensure stempelFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*stempelFilterFactory)(nil)
