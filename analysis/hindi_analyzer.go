// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// HindiStopWords contains common Hindi stop words.
// Source: Apache Lucene Hindi stop words list
var HindiStopWords = []string{
	"अंदर", "अत", "अदि", "अप", "अपना", "अपनि", "अपनी", "अपने", "अभि",
	"अभी", "आदि", "आप", "इंहिं", "इंहें", "इंहों", "इस", "इसका", "इसकि",
	"इसकी", "इसके", "इसमें", "इसी", "इसे", "उंहिं", "उंहें", "उंहों",
	"उस", "उसके", "उसी", "उसे", "एक", "एवं", "एस", "एसे", "ऐसे",
	"ओर", "और", "कर", "करता", "करते", "करना", "करने", "करें", "कहते",
	"कहा", "का", "काफि", "काफ़ी", "कि", "किंहें", "किंहों", "की", "कुछ",
	"कुल", "के", "को", "कोइ", "कोई", "कोन", "कोनसा", "कुछ", "क्या",
	"क्यों", "ने", "पर", "पहले", "पुरा", "पूरा", "पे", "फिर", "बनि",
	"बनी", "बहि", "बही", "बहुत", "बाद", "बाला", "बिलकुल", "भि", "भी",
	"भितर", "मगर", "मानो", "मे", "में", "यदि", "यह", "यहाँ", "यही",
	"या", "यिह", "ये", "रखें", "रवासा", "रहा", "रहे", "वगेरह", "वरगभदर",
	"वह", "वहाँ", "वहीं", "वाले", "वुह", "वे", "सकता", "सकते", "सबसे",
	"सभि", "सभी", "साथ", "साबुत", "साभ", "से", "हि", "ही", "हुअ",
	"हुआ", "हुइ", "हुई", "हुए", "है", "हैं", "हो", "होता", "होती",
	"होते", "होना", "होने",
}

// HindiAnalyzer is an analyzer for Hindi language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.hi.HindiAnalyzer.
//
// HindiAnalyzer uses the StandardTokenizer with Hindi normalization and stop words removal.
type HindiAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewHindiAnalyzer creates a new HindiAnalyzer with default Hindi stop words.
func NewHindiAnalyzer() *HindiAnalyzer {
	stopSet := GetWordSetFromStrings(HindiStopWords, true)
	return NewHindiAnalyzerWithWords(stopSet)
}

// NewHindiAnalyzerWithWords creates a HindiAnalyzer with custom stop words.
func NewHindiAnalyzerWithWords(stopWords *CharArraySet) *HindiAnalyzer {
	a := &HindiAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewHindiNormalizationFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewHindiStemFilterFactory())
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *HindiAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *HindiAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *HindiAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*HindiAnalyzer)(nil)
var _ AnalyzerInterface = (*HindiAnalyzer)(nil)

// HindiNormalizer normalizes Hindi text.
//
// This normalizes various Unicode representations of Devanagari characters.
type HindiNormalizer struct{}

// NewHindiNormalizer creates a new HindiNormalizer.
func NewHindiNormalizer() *HindiNormalizer {
	return &HindiNormalizer{}
}

// Normalize normalizes Hindi text.
func (n *HindiNormalizer) Normalize(input string) string {
	if input == "" {
		return ""
	}

	runes := []rune(input)
	result := make([]rune, 0, len(runes))

	for _, r := range runes {
		normalized := n.normalizeRune(r)
		result = append(result, normalized)
	}

	return string(result)
}

// normalizeRune normalizes a single Hindi/Devanagari rune.
func (n *HindiNormalizer) normalizeRune(r rune) rune {
	// This is a simplified normalization
	// In a full implementation, this would normalize:
	// - Multiple representations of the same character
	// - Remove common decorative marks
	// - Normalize vowel signs
	return r
}

// HindiNormalizationFilter normalizes Hindi text.
type HindiNormalizationFilter struct {
	*BaseTokenFilter
	normalizer *HindiNormalizer
}

// NewHindiNormalizationFilter creates a new HindiNormalizationFilter.
func NewHindiNormalizationFilter(input TokenStream) *HindiNormalizationFilter {
	return &HindiNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewHindiNormalizer(),
	}
}

// IncrementToken processes the next token and applies Hindi normalization.
func (f *HindiNormalizationFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				normalized := f.normalizer.Normalize(term)
				if normalized != term {
					termAttr.SetEmpty()
					termAttr.AppendString(normalized)
				}
			}
		}
	}

	return hasToken, nil
}

// HindiNormalizationFilterFactory creates HindiNormalizationFilter instances.
type HindiNormalizationFilterFactory struct{}

// NewHindiNormalizationFilterFactory creates a new HindiNormalizationFilterFactory.
func NewHindiNormalizationFilterFactory() *HindiNormalizationFilterFactory {
	return &HindiNormalizationFilterFactory{}
}

// Create creates a new HindiNormalizationFilter.
func (f *HindiNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewHindiNormalizationFilter(input)
}

var _ TokenFilterFactory = (*HindiNormalizationFilterFactory)(nil)

// HindiStemmer implements light stemming for Hindi language.
type HindiStemmer struct{}

// NewHindiStemmer creates a new HindiStemmer.
func NewHindiStemmer() *HindiStemmer {
	return &HindiStemmer{}
}

// Stem performs light stemming on a Hindi word.
func (s *HindiStemmer) Stem(word string) string {
	if len(word) < 3 {
		return word
	}

	runes := []rune(word)
	length := len(runes)

	// Remove common Hindi suffixes
	switch {
	// Remove common case/postposition suffixes
	// These are simplified rules
	case length > 2:
		lastChar := runes[length-1]
		// Remove common suffix characters
		if lastChar == 'ा' || // aa
			lastChar == 'े' || // e
			lastChar == 'ी' || // ii
			lastChar == 'ो' || // o
			lastChar == 'ं' || // anusvara
			lastChar == 'ः' { // visarga
			return string(runes[:length-1])
		}
	}

	return word
}

// HindiStemFilter implements light stemming for Hindi.
type HindiStemFilter struct {
	*BaseTokenFilter
	stemmer *HindiStemmer
}

// NewHindiStemFilter creates a new HindiStemFilter.
func NewHindiStemFilter(input TokenStream) *HindiStemFilter {
	return &HindiStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewHindiStemmer(),
	}
}

// IncrementToken processes the next token and applies Hindi stemming.
func (f *HindiStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := f.stemmer.Stem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// HindiStemFilterFactory creates HindiStemFilter instances.
type HindiStemFilterFactory struct{}

// NewHindiStemFilterFactory creates a new HindiStemFilterFactory.
func NewHindiStemFilterFactory() *HindiStemFilterFactory {
	return &HindiStemFilterFactory{}
}

// Create creates a new HindiStemFilter.
func (f *HindiStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewHindiStemFilter(input)
}

var _ TokenFilterFactory = (*HindiStemFilterFactory)(nil)
