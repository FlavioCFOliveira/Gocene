// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// LatvianStopWords contains common Latvian stop words.
// Source: Apache Lucene Latvian stop words list
var LatvianStopWords = []string{
	"ārpus", "ar", "arī", "augšpus", "bet", "bez", "bija", "biju",
	"bijām", "bijāt", "būs", "būšu", "būsi", "būsiet", "būsim",
	"būt", "būšu", "caur", "diemžēl", "diezin", "droši", "dēļ",
	"es", "esam", "esat", "esi", "esmu", "gan", "gar", "iekām",
	"iekāms", "iekš", "iekšpus", "ik", "ir", "it", "itin", "iz",
	"jau", "jā", "jel", "jo", "jums", "jūs", "jūsu", "ka", "kam",
	"kaut", "kolīdz", "kopš", "kur", "kuram", "kurā", "kuras",
	"kurai", "kura", "kurš", "kuru", "kurus", "lai", "lejpus",
	"līdz", "līdzko", "ne", "nebūt", "nedz", "nevis", "nezin",
	"no", "nopus", "nu", "nē", "pa", "par", "pat", "pie",
	"pirms", "pret", "priekš", "starp", "tad", "tā", "tāpēc",
	"tām", "tās", "tālab", "tāpēc", "te", "tik", "tikai", "tiklab",
	"to", "tātad", "pie", "tā", "tādēļ", "un", "uz", "vai", "var",
	"varēja", "varējām", "varējāt", "varēju", "varēji", "varēt",
	"vien", "virs", "virspus", "viņa", "viņas", "viņām", "viņā",
	"viņām", "viņš", "viņos", "viņiem", "viņu", "zem",
}

// LatvianAnalyzer is an analyzer for Latvian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.lv.LatvianAnalyzer.
//
// LatvianAnalyzer uses the StandardTokenizer with Latvian stop words removal.
type LatvianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewLatvianAnalyzer creates a new LatvianAnalyzer with default Latvian stop words.
func NewLatvianAnalyzer() *LatvianAnalyzer {
	stopSet := GetWordSetFromStrings(LatvianStopWords, true)
	return NewLatvianAnalyzerWithWords(stopSet)
}

// NewLatvianAnalyzerWithWords creates a LatvianAnalyzer with custom stop words.
func NewLatvianAnalyzerWithWords(stopWords *CharArraySet) *LatvianAnalyzer {
	a := &LatvianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *LatvianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *LatvianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *LatvianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*LatvianAnalyzer)(nil)
var _ AnalyzerInterface = (*LatvianAnalyzer)(nil)
