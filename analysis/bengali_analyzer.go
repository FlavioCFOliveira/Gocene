// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// BengaliStopWords contains common Bengali stop words.
// Source: Apache Lucene Bengali stop words list
var BengaliStopWords = []string{
	// Common particles and postpositions
	"অতএব", "অথবা", "অর্থাত", "অবশ্য", "অনুযায়ী", "অনেক", "অনেকে", "অনেকেই",
	"অন্তত", "অন্য", "অবশ্য", "আমাদের", "আমার", "আমি", "আপনার", "আপনি",
	// Conjunctions
	"ও", "ওই", "ওকে", "ওর", "ওরা", "কখনও", "কত", "কবে", "করতে", "করবে",
	"করবেন", "করলে", "করা", "করাই", "করায়", "করার", "করি", "করিতে", "করিয়া",
	"করে", "করেই", "করেছিলেন", "করেছেন", "করেন", "করেনি", "কাউকে", "কাছে",
	"কাছেও", "কাজে", "কিন্তু", "কী", "কে", "কেউ", "কেউই", "কেখা", "কেন",
	// Pronouns
	"কোন", "কোনও", "কোনো", "ক্লাস", "গিয়ে", "গিয়েছিল", "গেছে", "গেল", "গেলে",
	"গোটা", "চলে", "চান", "চায়", "চেয়ে", "ছাড়া", "ছিল", "জন্য", "জানতে",
	"জানা", "জানানো", "জানায়", "জানিয়ে", "জে", "যখন", "যত", "যদি", "যদিও",
	"যাবে", "যায়", "যার", "যারা", "যাওয়া", "যাওয়ার", "যিনি", "যে", "যেই",
	// Articles
	"যেতে", "যেথায়", "যেমন", "র", "রকম", "রয়েছে", "রেখে", "লক্ষ", "শুধু",
	"শুরু", "সঙ্গে", "সব", "সবার", "সমস্ত", "সহ", "সাথে", "সাধারণ", "সামনে",
	// Common verbs
	"সি", "সুতরাং", "সে", "সেই", "সেখান", "সেখানে", "সেটা", "সেটাই", "সেটাও",
	"সেটি", "স্পষ্ট", "হই", "হইতে", "হইবে", "হইয়া", "হইয়ে", "হয়", "হয়তো",
	"হয়নি", "হয়ে", "হয়েই", "হয়েছিল", "হয়েছে", "হল", "হলে", "হলেই", "হলেও",
	"হলো", "হিসাবে", "হিসাবে", "হৈ", "হৈতে", "হৈব", "হৈল", "হোক", "হোয়া",
	"হয়েছেন",
}

// BengaliAnalyzer is an analyzer for Bengali language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.bn.BengaliAnalyzer.
//
// BengaliAnalyzer uses the StandardTokenizer with Bengali stop words removal.
type BengaliAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewBengaliAnalyzer creates a new BengaliAnalyzer with default Bengali stop words.
func NewBengaliAnalyzer() *BengaliAnalyzer {
	stopSet := GetWordSetFromStrings(BengaliStopWords, true)
	return NewBengaliAnalyzerWithWords(stopSet)
}

// NewBengaliAnalyzerWithWords creates a BengaliAnalyzer with custom stop words.
func NewBengaliAnalyzerWithWords(stopWords *CharArraySet) *BengaliAnalyzer {
	a := &BengaliAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *BengaliAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *BengaliAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *BengaliAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure BengaliAnalyzer implements Analyzer
var _ Analyzer = (*BengaliAnalyzer)(nil)
var _ AnalyzerInterface = (*BengaliAnalyzer)(nil)
