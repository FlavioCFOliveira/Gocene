// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// WhitespaceAnalyzer is an analyzer that tokenizes text at whitespace.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.WhitespaceAnalyzer.
//
// WhitespaceAnalyzer divides text at whitespace characters (as defined by
// unicode.IsSpace) and makes no other modifications to the tokens.
// Unlike StandardAnalyzer, it does NOT:
//   - Convert tokens to lowercase
//   - Remove stop words
//   - Apply any filters
//
// This makes it useful when you want to preserve the exact case of tokens,
// such as when indexing product codes, identifiers, or other case-sensitive
// text.
//
// Example:
//
//	Input: "Hello World TEST"
//	Output tokens: "Hello", "World", "TEST"
type WhitespaceAnalyzer struct {
	*BaseAnalyzer
}

// NewWhitespaceAnalyzer creates a new WhitespaceAnalyzer.
func NewWhitespaceAnalyzer() *WhitespaceAnalyzer {
	return &WhitespaceAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
	}
}

// TokenStream creates a TokenStream for analyzing text.
// Returns a TokenStream that tokenizes at whitespace without any filtering.
func (a *WhitespaceAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	// Create the tokenizer - no filters applied
	tokenizer := NewWhitespaceTokenizer()
	if err := tokenizer.SetReader(reader); err != nil {
		return nil, err
	}

	return tokenizer, nil
}

// Ensure WhitespaceAnalyzer implements Analyzer
var _ Analyzer = (*WhitespaceAnalyzer)(nil)

// LowerCaseWhitespaceAnalyzer is an analyzer that tokenizes at whitespace
// and converts tokens to lowercase.
//
// This analyzer is similar to WhitespaceAnalyzer but adds LowerCaseFilter.
// It tokenizes text at whitespace boundaries and lowercases all tokens.
//
// Example:
//
//	Input: "Hello World TEST"
//	Output tokens: "hello", "world", "test"
type LowerCaseWhitespaceAnalyzer struct {
	*BaseAnalyzer
}

// NewLowerCaseWhitespaceAnalyzer creates a new LowerCaseWhitespaceAnalyzer.
func NewLowerCaseWhitespaceAnalyzer() *LowerCaseWhitespaceAnalyzer {
	return &LowerCaseWhitespaceAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
	}
}

// TokenStream creates a TokenStream for analyzing text.
// Returns a TokenStream that tokenizes at whitespace and lowercases tokens.
func (a *LowerCaseWhitespaceAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	// Create the tokenizer
	tokenizer := NewWhitespaceTokenizer()
	if err := tokenizer.SetReader(reader); err != nil {
		return nil, err
	}

	// Apply lowercase filter
	lowerCaseFilter := NewLowerCaseFilter(tokenizer)

	return lowerCaseFilter, nil
}

// Ensure LowerCaseWhitespaceAnalyzer implements Analyzer
var _ Analyzer = (*LowerCaseWhitespaceAnalyzer)(nil)
