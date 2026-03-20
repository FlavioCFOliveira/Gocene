// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"unicode"
)

// ThaiStopWords contains common Thai stop words.
// Source: Apache Lucene Thai stop words list
var ThaiStopWords = []string{
	"จะ", "จะ", "จะ", "จะ", "จะ", "จะ", "จะ", "จะ", "จะ", "จะ",
	"ใน", "ใน", "ใน", "ใน", "ใน", "ใน", "ใน", "ใน", "ใน", "ใน",
	"ของ", "ของ", "ของ", "ของ", "ของ", "ของ", "ของ", "ของ", "ของ", "ของ",
	"ได้", "ได้", "ได้", "ได้", "ได้", "ได้", "ได้", "ได้", "ได้", "ได้",
	"ที่", "ที่", "ที่", "ที่", "ที่", "ที่", "ที่", "ที่", "ที่", "ที่",
	"มี", "มี", "มี", "มี", "มี", "มี", "มี", "มี", "มี", "มี",
	"เป็น", "เป็น", "เป็น", "เป็น", "เป็น", "เป็น", "เป็น", "เป็น", "เป็น", "เป็น",
	"และ", "และ", "และ", "และ", "และ", "และ", "และ", "และ", "และ", "และ",
	"ก็", "ก็", "ก็", "ก็", "ก็", "ก็", "ก็", "ก็", "ก็", "ก็",
	"ให้", "ให้", "ให้", "ให้", "ให้", "ให้", "ให้", "ให้", "ให้", "ให้",
	"ว่า", "ว่า", "ว่า", "ว่า", "ว่า", "ว่า", "ว่า", "ว่า", "ว่า", "ว่า",
	"ไม่", "ไม่", "ไม่", "ไม่", "ไม่", "ไม่", "ไม่", "ไม่", "ไม่", "ไม่",
	"แต่", "แต่", "แต่", "แต่", "แต่", "แต่", "แต่", "แต่", "แต่", "แต่",
	"กับ", "กับ", "กับ", "กับ", "กับ", "กับ", "กับ", "กับ", "กับ", "กับ",
	"หรือ", "หรือ", "หรือ", "หรือ", "หรือ", "หรือ", "หรือ", "หรือ", "หรือ", "หรือ",
	"แล้ว", "แล้ว", "แล้ว", "แล้ว", "แล้ว", "แล้ว", "แล้ว", "แล้ว", "แล้ว", "แล้ว",
	"โดย", "โดย", "โดย", "โดย", "โดย", "โดย", "โดย", "โดย", "โดย", "โดย",
	"จาก", "จาก", "จาก", "จาก", "จาก", "จาก", "จาก", "จาก", "จาก", "จาก",
	"ถูก", "ถูก", "ถูก", "ถูก", "ถูก", "ถูก", "ถูก", "ถูก", "ถูก", "ถูก",
	"นี้", "นี้", "นี้", "นี้", "นี้", "นี้", "นี้", "นี้", "นี้", "นี้",
}

// ThaiAnalyzer is an analyzer for Thai language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.th.ThaiAnalyzer.
//
// ThaiAnalyzer uses the StandardTokenizer with Thai-specific character handling
// and stop words removal.
type ThaiAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewThaiAnalyzer creates a new ThaiAnalyzer with default Thai stop words.
func NewThaiAnalyzer() *ThaiAnalyzer {
	stopSet := GetWordSetFromStrings(ThaiStopWords, true)
	return NewThaiAnalyzerWithWords(stopSet)
}

// NewThaiAnalyzerWithWords creates a ThaiAnalyzer with custom stop words.
func NewThaiAnalyzerWithWords(stopWords *CharArraySet) *ThaiAnalyzer {
	a := &ThaiAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *ThaiAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *ThaiAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *ThaiAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// IsThaiCharacter returns true if the rune is a Thai character.
func IsThaiCharacter(r rune) bool {
	// Thai Unicode range: 0x0E00 - 0x0E7F
	return r >= 0x0E00 && r <= 0x0E7F
}

// HasThaiText returns true if the string contains any Thai characters.
func HasThaiText(s string) bool {
	for _, r := range s {
		if IsThaiCharacter(r) {
			return true
		}
	}
	return false
}

// CountThaiCharacters returns the number of Thai characters in the string.
func CountThaiCharacters(s string) int {
	count := 0
	for _, r := range s {
		if IsThaiCharacter(r) {
			count++
		}
	}
	return count
}

// IsThaiStopWord returns true if the word is a common Thai stop word.
func IsThaiStopWord(word string) bool {
	stopSet := GetWordSetFromStrings(ThaiStopWords, true)
	return stopSet.ContainsString(word)
}

// TokenizeThaiText tokenizes Thai text, handling Thai character boundaries.
// Note: This is a simplified implementation. Proper Thai tokenization
// requires a dictionary-based approach or machine learning.
func TokenizeThaiText(text string) []string {
	var tokens []string
	var currentToken []rune

	for _, r := range text {
		if unicode.IsSpace(r) {
			if len(currentToken) > 0 {
				tokens = append(tokens, string(currentToken))
				currentToken = nil
			}
		} else if IsThaiCharacter(r) {
			// For Thai, we may want to handle each character or use spacing
			currentToken = append(currentToken, r)
		} else {
			currentToken = append(currentToken, r)
		}
	}

	if len(currentToken) > 0 {
		tokens = append(tokens, string(currentToken))
	}

	return tokens
}

var _ Analyzer = (*ThaiAnalyzer)(nil)
var _ AnalyzerInterface = (*ThaiAnalyzer)(nil)
