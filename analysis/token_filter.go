// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TokenFilter is a TokenStream that wraps another TokenStream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TokenFilter.
//
// TokenFilter is the base class for filters that modify tokens produced
// by another TokenStream (either a Tokenizer or another TokenFilter).
// Filters can modify tokens by:
// - Changing token text (e.g., LowerCaseFilter)
// - Removing tokens (e.g., StopFilter)
// - Adding/modifying attributes (e.g., SynonymFilter)
// - Combining/splitting tokens
type TokenFilter interface {
	TokenStream

	// GetInput returns the wrapped input TokenStream.
	GetInput() TokenStream
}

// BaseTokenFilter provides a base implementation for TokenFilter.
//
// Embed this struct in concrete TokenFilter implementations to inherit
// common functionality.
type BaseTokenFilter struct {
	BaseTokenStream

	// input is the wrapped TokenStream
	input TokenStream
}

// NewBaseTokenFilter creates a new BaseTokenFilter wrapping the given input.
func NewBaseTokenFilter(input TokenStream) *BaseTokenFilter {
	return &BaseTokenFilter{
		BaseTokenStream: *NewBaseTokenStream(),
		input:         input,
	}
}

// GetInput returns the wrapped input TokenStream.
func (f *BaseTokenFilter) GetInput() TokenStream {
	return f.input
}

// End performs end-of-stream operations.
// Delegates to the input TokenStream.
func (f *BaseTokenFilter) End() error {
	if f.input != nil {
		return f.input.End()
	}
	return nil
}

// Close releases resources.
// Delegates to the input TokenStream.
func (f *BaseTokenFilter) Close() error {
	if f.input != nil {
		return f.input.Close()
	}
	return nil
}
