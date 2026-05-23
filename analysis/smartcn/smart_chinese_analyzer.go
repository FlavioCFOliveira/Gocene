// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	"io"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// DefaultStopwordFileComment is the line-comment prefix used in stopwords.txt.
const DefaultStopwordFileComment = "//"

// defaultStopWords is the list of punctuation stop words bundled with the
// SmartChineseAnalyzer, mirroring the content of stopwords.txt in Lucene 10.4.0.
var defaultStopWords = []string{
	",", ".", "`", "-", "_", "=", "?", "'", "|", `"`,
	"(", ")", "{", "}", "[", "]", "<", ">", "*", "#",
	"&", "^", "$", "@", "!", "~", ":", ";", "+", `\`,
	"《", "》", "—", "－", "，", "。", "、", "：", "；", "！",
	"·", "？", "“", "”", "）", "（", "【", "】", "［", "］",
	"●", "　",
}

// defaultStopSetOnce guards lazy initialisation of defaultStopSet.
var defaultStopSetOnce sync.Once

// defaultStopSet is the lazily initialised unmodifiable set of default stop words.
var defaultStopSet *analysis.UnmodifiableCharArraySet

// GetDefaultStopSet returns an unmodifiable instance of the default stop-word set.
// The set is initialised once and shared across all callers.
func GetDefaultStopSet() *analysis.UnmodifiableCharArraySet {
	defaultStopSetOnce.Do(func() {
		base := analysis.GetWordSetFromStrings(defaultStopWords, false)
		defaultStopSet = analysis.NewUnmodifiableCharArraySet(base)
	})
	return defaultStopSet
}

// SmartChineseAnalyzer is an analyzer for Chinese or mixed Chinese-English text.
// The analyzer uses probabilistic knowledge to find the optimal word segmentation
// for Simplified Chinese text. The text is first broken into sentences, then each
// sentence is segmented into words.
//
// Segmentation is based upon the Hidden Markov Model. A large training corpus was
// used to calculate Chinese word frequency probability.
//
// This analyzer requires a dictionary to provide statistical data.
// SmartChineseAnalyzer has an included dictionary out-of-box.
//
// The included dictionary data is from ICTCLAS1.0.
//
// Go port of org.apache.lucene.analysis.cn.smart.SmartChineseAnalyzer
// (Apache Lucene 10.4.0).
//
// Deviation: Java's createComponents / normalize pattern maps to a single
// TokenStream method; Go has no Analyzer base-class reuse machinery for
// cross-package tokenizers, so the chain is built inline.
type SmartChineseAnalyzer struct {
	// stopWords is the set of stop-word strings to filter.
	// nil means no stop filtering.
	stopWords []string
}

// NewSmartChineseAnalyzer creates a SmartChineseAnalyzer using the default
// stop-word list (punctuation).
func NewSmartChineseAnalyzer() *SmartChineseAnalyzer {
	return &SmartChineseAnalyzer{stopWords: defaultStopWords}
}

// NewSmartChineseAnalyzerWithDefault creates a SmartChineseAnalyzer, optionally
// using the default stop-word list.
//
// If useDefaultStopWords is false, punctuation will not be removed from the text.
func NewSmartChineseAnalyzerWithDefault(useDefaultStopWords bool) *SmartChineseAnalyzer {
	if useDefaultStopWords {
		return &SmartChineseAnalyzer{stopWords: defaultStopWords}
	}
	return &SmartChineseAnalyzer{stopWords: nil}
}

// NewSmartChineseAnalyzerWithStopWords creates a SmartChineseAnalyzer with the
// provided stop-word set.
//
// Pass nil or an empty set to disable stop-word filtering (punctuation will be
// indexed).
func NewSmartChineseAnalyzerWithStopWords(stopWords *analysis.CharArraySet) *SmartChineseAnalyzer {
	if stopWords == nil || stopWords.IsEmpty() {
		return &SmartChineseAnalyzer{stopWords: nil}
	}
	words := stopWords.Items()
	return &SmartChineseAnalyzer{stopWords: words}
}

// TokenStream builds the analysis chain:
//
//	HMMChineseTokenizer → PorterStemFilter → [StopFilter]
//
// The LowerCaseFilter is not applied on the main path because SegTokenFilter
// (inside HMMChineseTokenizer) already lowercases Basic Latin text.
// LowerCaseFilter is applied only in the normalize path (not modelled here).
func (a *SmartChineseAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	_ = fieldName
	tok, err := NewHMMChineseTokenizer()
	if err != nil {
		return nil, err
	}
	if err := tok.SetReader(reader); err != nil {
		return nil, err
	}

	var stream analysis.TokenStream = analysis.NewPorterStemFilter(tok)

	if len(a.stopWords) > 0 {
		stream = analysis.NewStopFilter(stream, a.stopWords)
	}

	return stream, nil
}

// GetStopWords returns the stop-word list used by this analyzer.
func (a *SmartChineseAnalyzer) GetStopWords() []string {
	out := make([]string, len(a.stopWords))
	copy(out, a.stopWords)
	return out
}

// Close releases resources held by this analyzer.
// SmartChineseAnalyzer holds no persistent resources; this is a no-op.
func (a *SmartChineseAnalyzer) Close() error {
	return nil
}

// Ensure SmartChineseAnalyzer implements analysis.Analyzer.
var _ analysis.Analyzer = (*SmartChineseAnalyzer)(nil)
