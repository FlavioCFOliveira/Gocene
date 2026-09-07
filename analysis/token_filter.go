// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/util"
)

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
type TokenFilter = api.TokenFilter

// BaseTokenFilter provides a base implementation for TokenFilter.
//
// Embed this struct in concrete TokenFilter implementations to inherit
// common functionality.
type BaseTokenFilter struct {
	BaseTokenStream

	// input is the wrapped TokenStream
	input TokenStream
}

// NewBaseTokenFilter creates a new BaseTokenFilter wrapping the given
// input. The filter shares the [util.AttributeSource] with its input
// when input exposes one; otherwise a fresh AttributeSource is created.
func NewBaseTokenFilter(input TokenStream) *BaseTokenFilter {
	bf := &BaseTokenFilter{
		input: input,
	}

	if hasAttrSrc, ok := input.(interface {
		GetAttributeSource() *util.AttributeSource
	}); ok {
		bf.AttributeSource = hasAttrSrc.GetAttributeSource()
	} else {
		bf.AttributeSource = util.NewAttributeSource()
	}

	return bf
}

// GetInput returns the wrapped input TokenStream.
func (f *BaseTokenFilter) GetInput() TokenStream {
	return f.input
}

// Unwrap returns the wrapped input TokenStream.
func (f *BaseTokenFilter) Unwrap() TokenStream {
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

// Reset resets the token stream to the beginning.
// Delegates to the input TokenStream if it supports Reset.
func (f *BaseTokenFilter) Reset() error {
	if f.input != nil {
		if resetter, ok := f.input.(TokenStreamWithReset); ok {
			return resetter.Reset()
		}
	}
	return nil
}

// LowerCaseFilterFactory creates LowerCaseFilter instances.
type LowerCaseFilterFactory struct {
	BaseTokenFilterFactory
}

// NewLowerCaseFilterFactory creates a new LowerCaseFilterFactory.
func NewLowerCaseFilterFactory() *LowerCaseFilterFactory {
	return &LowerCaseFilterFactory{}
}

// Create creates a LowerCaseFilter wrapping the given input.
func (f *LowerCaseFilterFactory) Create(input TokenStream) TokenFilter {
	return NewLowerCaseFilter(input)
}

// Ensure LowerCaseFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*LowerCaseFilterFactory)(nil)
