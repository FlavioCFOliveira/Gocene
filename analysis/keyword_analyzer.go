// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// KeywordAnalyzer is an analyzer that uses KeywordTokenizer.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.KeywordAnalyzer.
//
// KeywordAnalyzer treats the entire input as a single token. This is useful for:
// - Fields that should not be tokenized (e.g., IDs, exact match fields)
// - Preserving exact input values (e.g., product codes, identifiers)
// - Building custom analyzers that need to process input as a whole
//
// Example:
//
//	analyzer := NewKeywordAnalyzer()
//	tokens, _ := analyzer.Analyze("Hello, World!")
//	// tokens: ["Hello, World!"] (single token)
type KeywordAnalyzer struct {
	*BaseAnalyzer
}

// NewKeywordAnalyzer creates a new KeywordAnalyzer.
func NewKeywordAnalyzer() *KeywordAnalyzer {
	a := &KeywordAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
	}
	a.TokenizerFactory = NewKeywordTokenizerFactory()
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *KeywordAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Close releases resources.
func (a *KeywordAnalyzer) Close() error {
	return a.BaseAnalyzer.Close()
}

// Ensure KeywordAnalyzer implements Analyzer
var _ Analyzer = (*KeywordAnalyzer)(nil)
var _ AnalyzerInterface = (*KeywordAnalyzer)(nil)