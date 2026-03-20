// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// ArabicStopWords contains common Arabic stop words.
// These are high-frequency words that provide little semantic value.
var ArabicStopWords = []string{
	// Articles and prepositions
	"ال", "و", "ف", "ب", "ك", "ل", "من", "في", "على", "إلى",
	"عن", "مع", "هذا", "هذه", "هذان", "هاتان", "هؤلاء", "ذلك",
	"تلك", "أولئك", "هو", "هي", "هما", "هم", "هن", "أنا",
	"أنت", "أنتِ", "أنتما", "أنتم", "أنتن", "نحن", "إياه",
	"إياها", "إياهما", "إياهم", "إياهن", "إياك", "إياكما",
	"إياكم", "إياكن", "إيانا", "إياي",
	// Common particles
	"قد", "لا", "ما", "لم", "لن", "ليس", "إن", "أن", "إنه",
	"إنها", "أنه", "أنها", "إذ", "إذا", "لما", "لولا", "لو",
	"كلما", "مهما", "أينما", "حيثما", "كيفما", "أي",
	// Verbs and auxiliaries
	"كان", "كانت", "كانتا", "كانوا", "كن", "صار", "صارت",
	"أصبح", "أضحى", "أمسى", "بات", "ظل", "عاد", "ليست",
	"لست", "لستم", "لستما", "لسن", "ليسوا",
	// Common conjunctions
	"ثم", "أو", "أم", "بل", "حتى", "لكن", "لكنه", "لكنها",
	"لكنك", "لكنكم", "لكنكن", "لكننا", "لكنني", "لكنهما",
	// Demonstratives
	"الذي", "التي", "اللذان", "اللتان", "الذين", "اللواتي",
	"اللائي", "اللتين",
}

// ArabicAnalyzer is an analyzer for Arabic language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ar.ArabicAnalyzer.
//
// ArabicAnalyzer uses the StandardTokenizer with Arabic normalization,
// stop words removal, and Arabic-specific stemming.
type ArabicAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewArabicAnalyzer creates a new ArabicAnalyzer with default Arabic stop words.
func NewArabicAnalyzer() *ArabicAnalyzer {
	stopSet := GetWordSetFromStrings(ArabicStopWords, true)
	return NewArabicAnalyzerWithWords(stopSet)
}

// NewArabicAnalyzerWithWords creates an ArabicAnalyzer with custom stop words.
func NewArabicAnalyzerWithWords(stopWords *CharArraySet) *ArabicAnalyzer {
	a := &ArabicAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	// Tokenizer -> LowerCase -> ArabicNormalization -> StopWords -> ArabicStemming
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewArabicNormalizationFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewArabicStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *ArabicAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *ArabicAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *ArabicAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure ArabicAnalyzer implements Analyzer
var _ Analyzer = (*ArabicAnalyzer)(nil)
var _ AnalyzerInterface = (*ArabicAnalyzer)(nil)
