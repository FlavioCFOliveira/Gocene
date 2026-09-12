// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package api

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Analyzer is the abstract base class for all analyzers.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Analyzer.
type Analyzer interface {
	// TokenStream creates a TokenStream for analyzing text from a Reader.
	TokenStream(fieldName string, reader io.Reader) (TokenStream, error)

	// Normalize returns a TokenStream that is a normalized version of the analysis chain for the given field.
	Normalize(fieldName string) TokenStream

	// Close releases resources held by this Analyzer.
	Close() error
}

// TokenStream is an abstract base class for producing token streams.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TokenStream.
type TokenStream interface {
	// GetAttributeSource returns the source of attributes for this stream.
	GetAttributeSource() *util.AttributeSource

	// IncrementToken advances to the next token in the stream.
	// Returns true if a token is available, false if at end of stream.
	IncrementToken() (bool, error)

	// Reset resets this stream to a clean state.
	Reset() error

	// End performs end-of-stream operations.
	// Called after the last token has been consumed.
	End() error

	// Close releases resources held by this TokenStream.
	Close() error
}

// AnalyzerFactory is the interface for factories that create Analyzer instances.
type AnalyzerFactory interface {
	Create() Analyzer
}
