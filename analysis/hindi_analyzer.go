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
//
// This is a faithful port of Lucene 10.4.0's
// org.apache.lucene.analysis.hi.HindiNormalizer.normalize(char[], int)
// implementing the algorithm specified in "Word normalization in Indian
// languages" (Pingali & Varma) plus the additions from "Hindi CLIR in Thirty
// Days" (Larkey, Connell & AbdulJaleel):
//
//   - Multiple representations of the same character are collapsed
//     (nukta deletion, dead-NA + halant → anusvara, candrabindu → anusvara).
//   - Zero-width joiner / non-joiner and virama are removed.
//   - Chandra/short vowels and long vowels are folded to their canonical
//     short forms.
//
// Operates on a mutable rune buffer, preserving Lucene's in-place
// delete-and-shift semantics so that adjacent-rune lookahead
// (`s[i] == NA && s[i+1] == VIRAMA`) behaves identically.
func (n *HindiNormalizer) Normalize(input string) string {
	if input == "" {
		return ""
	}

	s := []rune(input)
	length := len(s)

	for i := 0; i < length; i++ {
		switch s[i] {
		// dead n -> bindu (NA + virama collapses to anusvara)
		case 'न':
			if i+1 < length && s[i+1] == '्' {
				s[i] = 'ं'
				length = hindiDelete(s, i+1, length)
			}
		// candrabindu -> bindu
		case 'ँ':
			s[i] = 'ं'
		// nukta deletion
		case '़':
			length = hindiDelete(s, i, length)
			i--
		case 'ऩ':
			s[i] = 'न'
		case 'ऱ':
			s[i] = 'र'
		case 'ऴ':
			s[i] = 'ळ'
		case 'क़':
			s[i] = 'क'
		case 'ख़':
			s[i] = 'ख'
		case 'ग़':
			s[i] = 'ग'
		case 'ज़':
			s[i] = 'ज'
		case 'ड़':
			s[i] = 'ड'
		case 'ढ़':
			s[i] = 'ढ'
		case 'फ़':
			s[i] = 'फ'
		case 'य़':
			s[i] = 'य'
		// zwj / zwnj -> delete
		case '‍', '‌':
			length = hindiDelete(s, i, length)
			i--
		// virama -> delete
		case '्':
			length = hindiDelete(s, i, length)
			i--
		// chandra / short -> replace
		case 'ॅ', 'ॆ':
			s[i] = 'े'
		case 'ॉ', 'ॊ':
			s[i] = 'ो'
		case 'ऍ', 'ऎ':
			s[i] = 'ए'
		case 'ऑ', 'ऒ':
			s[i] = 'ओ'
		case 'ॲ':
			s[i] = 'अ'
		// long -> short ind. vowels
		case 'आ':
			s[i] = 'अ'
		case 'ई':
			s[i] = 'इ'
		case 'ऊ':
			s[i] = 'उ'
		case 'ॠ':
			s[i] = 'ऋ'
		case 'ॡ':
			s[i] = 'ऌ'
		case 'ऐ':
			s[i] = 'ए'
		case 'औ':
			s[i] = 'ओ'
		// long -> short dep. vowels
		case 'ी':
			s[i] = 'ि'
		case 'ू':
			s[i] = 'ु'
		case 'ॄ':
			s[i] = 'ृ'
		case 'ॣ':
			s[i] = 'ॢ'
		case 'ै':
			s[i] = 'े'
		case 'ौ':
			s[i] = 'ो'
		}
	}

	return string(s[:length])
}

// hindiDelete mirrors Lucene's StemmerUtil.delete: shifts s[pos+1:length]
// left by one slot and returns the new effective length. The trailing rune
// is left in place but logically out of range.
func hindiDelete(s []rune, pos, length int) int {
	if pos < 0 || pos >= length {
		return length
	}
	if pos < length-1 {
		copy(s[pos:length-1], s[pos+1:length])
	}
	return length - 1
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
		if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
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
		if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
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
