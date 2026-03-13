// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// SimpleAnalyzer is an analyzer that tokenizes text at non-letters
// and then lowercases the tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.SimpleAnalyzer.
//
// SimpleAnalyzer combines a LetterTokenizer with a LowerCaseFilter to produce
// lowercase word tokens. It does not remove stop words, making it a simple
// but effective analyzer for basic text processing.
//
// The tokenization process:
//  1. LetterTokenizer: Splits text into tokens at non-letter boundaries
//  2. LowerCaseFilter: Converts all tokens to lowercase
//
// Example:
//
//	Input: "Hello, World! 123 TEST."
//	Output tokens: "hello", "world", "test"
//
// This analyzer is suitable for simple search use cases where you want
// case-insensitive matching on word tokens without stop word removal.
type SimpleAnalyzer struct {
	*BaseAnalyzer
}

// NewSimpleAnalyzer creates a new SimpleAnalyzer.
func NewSimpleAnalyzer() *SimpleAnalyzer {
	return &SimpleAnalyzer{
		BaseAnalyzer: NewBaseAnalyzer(),
	}
}

// TokenStream creates a TokenStream for analyzing text.
// Returns a TokenStream that tokenizes at non-letter boundaries and
// converts tokens to lowercase.
func (a *SimpleAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	// Create the tokenizer
	tokenizer := NewLetterTokenizer()
	if err := tokenizer.SetReader(reader); err != nil {
		return nil, err
	}

	// Create the filter chain: Tokenizer -> LowerCaseFilter
	lowerCaseFilter := NewLowerCaseFilter(tokenizer)

	return lowerCaseFilter, nil
}

// Ensure SimpleAnalyzer implements Analyzer
var _ Analyzer = (*SimpleAnalyzer)(nil)
