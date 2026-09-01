// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// Token represents a single token in a stream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Token.
type Token struct {
	Term    string
	StartOffset int
	EndOffset   int
	Position    int
	PositionInc int
}

// TokenStream is an interface for a stream of tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TokenStream.
type TokenStream interface {
	// Reset resets the stream to its initial state.
	Reset() error
	// Next returns the next token.
	Next() (Token, bool)
	// Close closes the stream.
	Close() error
}

// Analyzer is an interface for analyzing text into tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Analyzer.
type Analyzer interface {
	// NewTokenizer creates a tokenizer for the given reader.
	NewTokenizer(reader io.Reader) TokenStream
	// NewTokenFilter creates a token filter for the given stream.
	NewTokenFilter(stream TokenStream) TokenStream
}

// Tokenizer is an interface for tokenizing text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Tokenizer.
type Tokenizer interface {
	TokenStream
}

// TokenFilter is an interface for filtering tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TokenFilter.
type TokenFilter interface {
	TokenStream
}
