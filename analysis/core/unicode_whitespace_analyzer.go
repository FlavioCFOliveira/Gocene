// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package core

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// UnicodeWhitespaceAnalyzer is an Analyzer that uses [UnicodeWhitespaceTokenizer].
//
// This is the Go port of
// org.apache.lucene.analysis.core.UnicodeWhitespaceAnalyzer from Apache
// Lucene 10.4.0.
type UnicodeWhitespaceAnalyzer struct {
	*analysis.BaseAnalyzer
}

// NewUnicodeWhitespaceAnalyzer creates a new UnicodeWhitespaceAnalyzer.
func NewUnicodeWhitespaceAnalyzer() *UnicodeWhitespaceAnalyzer {
	a := &UnicodeWhitespaceAnalyzer{
		BaseAnalyzer: analysis.NewAnalyzer(),
	}
	a.TokenizerFactory = &unicodeWhitespaceTokenizerFactory{}
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *UnicodeWhitespaceAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Close releases resources.
func (a *UnicodeWhitespaceAnalyzer) Close() error {
	return a.BaseAnalyzer.Close()
}

// Ensure UnicodeWhitespaceAnalyzer implements analysis.Analyzer.
var _ analysis.Analyzer = (*UnicodeWhitespaceAnalyzer)(nil)

// unicodeWhitespaceTokenizerFactory is the internal TokenizerFactory.
type unicodeWhitespaceTokenizerFactory struct{}

func (f *unicodeWhitespaceTokenizerFactory) Create() analysis.Tokenizer {
	return NewUnicodeWhitespaceTokenizer()
}
