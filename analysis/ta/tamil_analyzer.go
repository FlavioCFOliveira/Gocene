// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ta

import (
	"embed"
	"io"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/in"
	"github.com/FlavioCFOliveira/Gocene/analysis/snowball"
	snowballext "github.com/FlavioCFOliveira/Gocene/snowball/ext"
)

//go:embed stopwords.txt
var tamilStopwordsFS embed.FS

var (
	tamilDefaultStopSetOnce sync.Once
	tamilDefaultStopSet     *analysis.CharArraySet
)

// defaultTamilStopSet returns the lazily-loaded default Tamil stop-word set.
func defaultTamilStopSet() *analysis.CharArraySet {
	tamilDefaultStopSetOnce.Do(func() {
		f, err := tamilStopwordsFS.Open("stopwords.txt")
		if err != nil {
			panic("ta: failed to open stopwords.txt: " + err.Error())
		}
		defer f.Close()
		set, err := analysis.GetWordSetWithComment(f, "#", analysis.NewCharArraySet(64, false))
		if err != nil {
			panic("ta: failed to load stopwords.txt: " + err.Error())
		}
		tamilDefaultStopSet = set
	})
	return tamilDefaultStopSet
}

// TamilAnalyzer is an Analyzer for Tamil language text.
//
// The analysis chain is:
//  1. StandardTokenizer
//  2. LowerCaseFilter
//  3. DecimalDigitFilter
//  4. SetKeywordMarkerFilter (if a stem exclusion set is provided)
//  5. IndicNormalizationFilter
//  6. StopFilter (Tamil stop words)
//  7. SnowballFilter (TamilStemmer)
//
// Go port of org.apache.lucene.analysis.ta.TamilAnalyzer (Apache Lucene 10.4.0).
type TamilAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords        *analysis.CharArraySet
	stemExclusionSet *analysis.CharArraySet
}

// NewTamilAnalyzer builds a TamilAnalyzer with the default stop words and no
// stem exclusion set.
func NewTamilAnalyzer() *TamilAnalyzer {
	return NewTamilAnalyzerWithStopWords(defaultTamilStopSet())
}

// NewTamilAnalyzerWithStopWords builds a TamilAnalyzer with the given stop
// words and no stem exclusion set.
func NewTamilAnalyzerWithStopWords(stopWords *analysis.CharArraySet) *TamilAnalyzer {
	return NewTamilAnalyzerFull(stopWords, analysis.NewCharArraySet(0, false))
}

// NewTamilAnalyzerFull builds a TamilAnalyzer with explicit stop words and
// stem exclusion set.
func NewTamilAnalyzerFull(stopWords *analysis.CharArraySet, stemExclusionSet *analysis.CharArraySet) *TamilAnalyzer {
	if stopWords == nil {
		stopWords = analysis.NewCharArraySet(0, false)
	}
	if stemExclusionSet == nil {
		stemExclusionSet = analysis.NewCharArraySet(0, false)
	}
	a := &TamilAnalyzer{
		BaseAnalyzer:     analysis.NewAnalyzer(),
		stopWords:        stopWords,
		stemExclusionSet: stemExclusionSet,
	}

	a.TokenizerFactory = analysis.NewStandardTokenizerFactory()
	a.AddTokenFilter(analysis.NewLowerCaseFilterFactory())
	a.AddTokenFilter(analysis.NewDecimalDigitFilterFactory())
	if !stemExclusionSet.IsEmpty() {
		a.AddTokenFilter(analysis.NewSetKeywordMarkerFilterFactoryWithSet(stemExclusionSet))
	}
	a.AddTokenFilter(in.NewIndicNormalizationFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(snowball.NewSnowballPorterFilterFactory(snowballext.NewTamilStemmer()))

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *TamilAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *TamilAnalyzer) GetStopWords() *analysis.CharArraySet {
	return a.stopWords
}

// Ensure TamilAnalyzer implements analysis.Analyzer.
var _ analysis.Analyzer = (*TamilAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*TamilAnalyzer)(nil)
