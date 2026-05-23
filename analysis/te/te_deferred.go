// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package te

import (
	"io"
	"sync"

	indicpkg "github.com/FlavioCFOliveira/Gocene/analysis/in"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// DefaultStopWordFile is the name of the bundled Telugu stopword file.
const DefaultStopWordFile = "stopwords.txt"

// TeluguStopWords is the default Telugu stop-word list.
// Source: Apache Lucene 10.4.0,
// analysis/common/src/resources/org/apache/lucene/analysis/te/stopwords.txt
var TeluguStopWords = []string{
	"చేయగలిగింది", "గురించి", "పై", "ప్రకారం", "అనుగుణంగా",
	"అడ్డంగా", "నిజంగా", "తర్వాత", "మళ్ళీ", "వ్యతిరేకంగా",
	"కాదు", "అందరూ", "అనుమతించు", "అనుమతిస్తుంది", "దాదాపు",
	"మాత్రమే", "వెంట", "ఇప్పటికే", "కూడా", "అయితే",
	"ఎప్పుడు", "వద్ద", "మధ్య", "ఒక", "మరియు",
	"మరొక", "ఏ", "ఎవరో ఒకరు", "ఏమైనప్పటికి", "ఎవరైనా",
	"ఏదైనా", "ఎక్కడైనా", "వేరుగా", "కనిపిస్తాయి", "మెచ్చుకో",
	"తగిన", "ఉన్నారు", "చుట్టూ", "గా", "ఒక ప్రక్కన",
	"అడగండి", "అడగడం", "సంబంధం", "అందుబాటులో", "దూరంగా",
}

var (
	teluguDefaultStopSetOnce sync.Once
	teluguDefaultStopSet     *analysis.CharArraySet
)

// GetDefaultStopSet returns an unmodifiable instance of the default Telugu
// stop-word set.
func GetDefaultStopSet() *analysis.CharArraySet {
	teluguDefaultStopSetOnce.Do(func() {
		teluguDefaultStopSet = analysis.GetWordSetFromStrings(TeluguStopWords, false)
	})
	return teluguDefaultStopSet
}

// TeluguAnalyzer is an analyzer for Telugu.
//
// Pipeline: StandardTokenizer → DecimalDigitFilter → [SetKeywordMarkerFilter]
// → IndicNormalizationFilter → TeluguNormalizationFilter → StopFilter →
// TeluguStemFilter.
//
// Go port of org.apache.lucene.analysis.te.TeluguAnalyzer (Apache Lucene
// 10.4.0).
type TeluguAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords        *analysis.CharArraySet
	stemExclusionSet *analysis.CharArraySet
}

// NewTeluguAnalyzer creates a TeluguAnalyzer with default stop words and no
// stem-exclusion set.
func NewTeluguAnalyzer() *TeluguAnalyzer {
	return NewTeluguAnalyzerFull(GetDefaultStopSet(), analysis.NewCharArraySet(0, false))
}

// NewTeluguAnalyzerWithStopWords creates a TeluguAnalyzer with custom stop
// words and no stem-exclusion set.
func NewTeluguAnalyzerWithStopWords(stopWords *analysis.CharArraySet) *TeluguAnalyzer {
	return NewTeluguAnalyzerFull(stopWords, analysis.NewCharArraySet(0, false))
}

// NewTeluguAnalyzerFull creates a TeluguAnalyzer with explicit stop words and
// stem-exclusion set.
func NewTeluguAnalyzerFull(stopWords, stemExclusionSet *analysis.CharArraySet) *TeluguAnalyzer {
	a := &TeluguAnalyzer{
		BaseAnalyzer:     analysis.NewAnalyzer(),
		stopWords:        stopWords,
		stemExclusionSet: stemExclusionSet,
	}
	a.TokenizerFactory = analysis.NewStandardTokenizerFactory()
	a.AddTokenFilter(analysis.NewDecimalDigitFilterFactory())
	if stemExclusionSet != nil && !stemExclusionSet.IsEmpty() {
		a.AddTokenFilter(analysis.NewSetKeywordMarkerFilterFactoryWithSet(stemExclusionSet))
	}
	a.AddTokenFilter(indicpkg.NewIndicNormalizationFilterFactory())
	a.AddTokenFilter(NewTeluguNormalizationFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewTeluguStemFilterFactory())
	return a
}

// GetDefaultStopWords returns the default stop-word set.
func (a *TeluguAnalyzer) GetDefaultStopWords() *analysis.CharArraySet {
	return GetDefaultStopSet()
}

// GetStopWords returns the stop-word set used by this analyzer.
func (a *TeluguAnalyzer) GetStopWords() *analysis.CharArraySet { return a.stopWords }

// GetStemExclusionSet returns the stem-exclusion set.
func (a *TeluguAnalyzer) GetStemExclusionSet() *analysis.CharArraySet { return a.stemExclusionSet }

// TokenStream creates a TokenStream for the given reader.
func (a *TeluguAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Ensure TeluguAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*TeluguAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*TeluguAnalyzer)(nil)
