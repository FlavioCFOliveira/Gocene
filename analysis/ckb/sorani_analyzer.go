// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package ckb provides an Analyzer for Sorani Kurdish.
package ckb

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// SoraniStopWords contains default stop words for Sorani (Central Kurdish),
// derived from the Apache Lucene resource file
// org/apache/lucene/analysis/ckb/stopwords.txt at release 10.4.0.
var SoraniStopWords = []string{
	"و", "کە", "ی", "کرد", "ئەوەی", "سەر", "دوو", "هەروەها", "لەو",
	"دەکات", "چەند", "هەر", "ئەو", "ئەم", "من", "ئێمە", "تۆ", "ئێوە",
	"ئەوان", "بە", "پێ", "بەبێ", "بەدەم", "بەلای", "بەپێی", "بەرلە",
	"بەرەوی", "بەرەوە", "بەردەم", "بێ", "بێجگە", "بۆ", "دە", "تێ",
	"دەگەڵ", "دوای", "جگە", "لە", "لێ", "لەبەر", "لەبەینی", "لەبابەت",
	"لەبارەی", "لەباتی", "لەبن", "لەبرێتی", "لەدەم", "لەگەڵ", "لەلایەن",
	"لەناو", "لەنێو", "لەپێناوی", "لەرەوی", "لەرێ", "لەرێگا", "لەسەر",
	"لەژێر", "ناو", "نێوان", "پاش", "پێش", "وەک",
}

// SoraniAnalyzer is an Analyzer for Sorani Kurdish.
//
// This is the Go port of org.apache.lucene.analysis.ckb.SoraniAnalyzer
// from Apache Lucene 10.4.0.
//
// The analysis chain:
//  1. StandardTokenizer
//  2. SoraniNormalizationFilter
//  3. LowerCaseFilter
//  4. DecimalDigitFilter
//  5. StopFilter (default stopwords or custom)
//  6. SetKeywordMarkerFilter (only when a non-empty stem exclusion set is provided)
//  7. SoraniStemFilter
type SoraniAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords       *analysis.CharArraySet
	stemExclusionSet *analysis.CharArraySet
}

// NewSoraniAnalyzer builds a SoraniAnalyzer with the default Sorani stop words
// and an empty stem exclusion set.
func NewSoraniAnalyzer() *SoraniAnalyzer {
	stopSet := analysis.GetWordSetFromStrings(SoraniStopWords, false)
	return NewSoraniAnalyzerWithStopwords(stopSet)
}

// NewSoraniAnalyzerWithStopwords builds a SoraniAnalyzer with the given stop
// words and an empty stem exclusion set.
func NewSoraniAnalyzerWithStopwords(stopWords *analysis.CharArraySet) *SoraniAnalyzer {
	return NewSoraniAnalyzerFull(stopWords, analysis.NewCharArraySet(0, false))
}

// NewSoraniAnalyzerFull builds a SoraniAnalyzer with the given stop words and
// stem exclusion set. Tokens in the stem exclusion set are marked as keywords
// and are not stemmed.
func NewSoraniAnalyzerFull(stopWords, stemExclusionSet *analysis.CharArraySet) *SoraniAnalyzer {
	a := &SoraniAnalyzer{
		BaseAnalyzer:     analysis.NewAnalyzer(),
		stopWords:        stopWords,
		stemExclusionSet: stemExclusionSet,
	}
	a.TokenizerFactory = analysis.NewStandardTokenizerFactory()
	a.AddTokenFilter(analysis.NewSoraniNormalizationFilterFactory())
	a.AddTokenFilter(analysis.NewLowerCaseFilterFactory())
	a.AddTokenFilter(analysis.NewDecimalDigitFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	if stemExclusionSet != nil && !stemExclusionSet.IsEmpty() {
		a.AddTokenFilter(analysis.NewSetKeywordMarkerFilterFactoryWithSet(stemExclusionSet))
	}
	a.AddTokenFilter(analysis.NewSoraniStemFilterFactory())
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *SoraniAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *SoraniAnalyzer) GetStopWords() *analysis.CharArraySet {
	return a.stopWords
}

// GetStemExclusionSet returns the stem exclusion set used by this analyzer.
func (a *SoraniAnalyzer) GetStemExclusionSet() *analysis.CharArraySet {
	return a.stemExclusionSet
}

// Ensure SoraniAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*SoraniAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*SoraniAnalyzer)(nil)
