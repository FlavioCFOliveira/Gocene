// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// StandardAnalyzer is a general-purpose analyzer.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.standard.StandardAnalyzer.
type StandardAnalyzer struct {
	stopWords []string
}

func NewStandardAnalyzer() *StandardAnalyzer {
	return NewStandardAnalyzerWithStopWords(nil)
}

func NewStandardAnalyzerWithStopWords(stopWords []string) *StandardAnalyzer {
	return &StandardAnalyzer{
		stopWords: stopWords,
	}
}

func (a *StandardAnalyzer) NewTokenizer(reader io.Reader) TokenStream {
	// In a full implementation, this would be StandardTokenizer.
	// For now, we use WhitespaceTokenizer.
	tokenizer := NewWhitespaceTokenizer()
	_ = tokenizer.SetReader(reader)
	return tokenizer
}

func (a *StandardAnalyzer) NewTokenFilter(stream TokenStream) TokenStream {
	stream = NewLowerCaseFilter(stream)
	if a.stopWords != nil {
		stream = NewStopFilter(stream, a.stopWords)
	}
	return stream
}

// TokenStream creates a TokenStream for analyzing text.
// Implements the Analyzer interface.
func (a *StandardAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	tokenizer := a.NewTokenizer(reader)
	return a.NewTokenFilter(tokenizer), nil
}

// Close releases resources held by this Analyzer.
// Implements the Analyzer interface.
func (a *StandardAnalyzer) Close() error {
	return nil
}
