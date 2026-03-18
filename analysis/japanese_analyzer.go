// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// JapaneseStopWords contains common Japanese stop words.
var JapaneseStopWords = []string{
	"の", "に", "は", "を", "た", "が", "で", "て", "と", "し", "れ", "さ", "ある", "いる",
	"も", "する", "から", "な", "こと", "として", "い", "や", "れる", "など", "なっ", "ない",
	"この", "ため", "その", "あっ", "これ", "ある", "こと", "これら", "それ", "どの",
	"または", "および", "について", "により", "において", "による", "について",
	"もの", "とき", "ため", "ながら", "あと", "ほか", "ほど", "よう", "そう", "すべて",
	"あまり", "あれ", "あわせ", "いくつ", "いくら", "いずれ", "いっ", "かつ", "かつて",
	"かなり", "かも", "から", "が", "き", "く", "ここ", "ごと", "さらに", "しかし",
	"しかも", "したがって", "すでに", "すべて", "そして", "その", "それ", "ただ",
	"たび", "ため", "だ", "だけ", "だに", "だの", "つ", "て", "で", "と", "ながら",
	"ならび", "なり", "なる", "の", "は", "ば", "へ", "ほど", "また", "または",
	"まで", "も", "もの", "や", "よう", "より", "ら", "る", "れ", "を", "ん",
}

// JapaneseAnalyzer is an analyzer for Japanese language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ja.JapaneseAnalyzer.
//
// JapaneseAnalyzer uses the StandardTokenizer with Japanese stop words removal.
// Note: For proper Japanese text segmentation, a specialized tokenizer like
// Kuromoji would be needed. This implementation provides basic support using
// the StandardTokenizer.
type JapaneseAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewJapaneseAnalyzer creates a new JapaneseAnalyzer with default Japanese stop words.
func NewJapaneseAnalyzer() *JapaneseAnalyzer {
	stopSet := GetWordSetFromStrings(JapaneseStopWords, true)
	return NewJapaneseAnalyzerWithWords(stopSet)
}

// NewJapaneseAnalyzerWithWords creates a JapaneseAnalyzer with custom stop words.
func NewJapaneseAnalyzerWithWords(stopWords *CharArraySet) *JapaneseAnalyzer {
	a := &JapaneseAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	// Note: For proper Japanese, a specialized tokenizer should be used
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *JapaneseAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *JapaneseAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *JapaneseAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure JapaneseAnalyzer implements Analyzer
var _ Analyzer = (*JapaneseAnalyzer)(nil)
var _ AnalyzerInterface = (*JapaneseAnalyzer)(nil)

// JapaneseAnalyzerFactory creates JapaneseAnalyzer instances.
type JapaneseAnalyzerFactory struct {
	stopWords *CharArraySet
}

// NewJapaneseAnalyzerFactory creates a new JapaneseAnalyzerFactory with default stop words.
func NewJapaneseAnalyzerFactory() *JapaneseAnalyzerFactory {
	return &JapaneseAnalyzerFactory{
		stopWords: GetWordSetFromStrings(JapaneseStopWords, true),
	}
}

// NewJapaneseAnalyzerFactoryWithWords creates a new JapaneseAnalyzerFactory with custom stop words.
func NewJapaneseAnalyzerFactoryWithWords(stopWords *CharArraySet) *JapaneseAnalyzerFactory {
	return &JapaneseAnalyzerFactory{
		stopWords: stopWords,
	}
}

// Create creates a new JapaneseAnalyzer.
func (f *JapaneseAnalyzerFactory) Create() AnalyzerInterface {
	return NewJapaneseAnalyzerWithWords(f.stopWords)
}

// Ensure JapaneseAnalyzerFactory implements AnalyzerFactory
var _ AnalyzerFactory = (*JapaneseAnalyzerFactory)(nil)
