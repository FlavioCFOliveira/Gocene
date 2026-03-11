// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TokenStream is an abstract base class for producing token streams.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TokenStream.
//
// TokenStream is the fundamental abstraction for processing text in Lucene.
// It represents a stream of tokens that can be consumed incrementally.
// The workflow is:
//
// 1. Create TokenStream with a source (e.g., Tokenizer or another TokenStream)
// 2. Call IncrementToken() repeatedly until it returns false
// 3. Call End() to perform end-of-stream operations
// 4. Call Close() to release resources
//
// TokenStream uses an AttributeSource to manage attributes.
type TokenStream interface {
	// IncrementToken advances to the next token in the stream.
	// Returns true if a token is available, false if at end of stream.
	IncrementToken() (bool, error)

	// End performs end-of-stream operations.
	// Called after the last token has been consumed.
	End() error

	// Close releases resources held by this TokenStream.
	Close() error
}

// BaseTokenStream provides a base implementation for TokenStream.
//
// Embed this struct in concrete TokenStream implementations to inherit
// common functionality.
type BaseTokenStream struct {
	// attributes holds the attribute source for this token stream
	attributes *AttributeSource
}

// NewBaseTokenStream creates a new BaseTokenStream.
func NewBaseTokenStream() *BaseTokenStream {
	return &BaseTokenStream{
		attributes: NewAttributeSource(),
	}
}

// GetAttributeSource returns the AttributeSource for this token stream.
func (ts *BaseTokenStream) GetAttributeSource() *AttributeSource {
	return ts.attributes
}

// AddAttribute adds an attribute implementation to this token stream.
func (ts *BaseTokenStream) AddAttribute(attr AttributeImpl) {
	ts.attributes.AddAttribute(attr)
}

// GetAttribute returns an attribute by type name.
func (ts *BaseTokenStream) GetAttribute(name string) AttributeImpl {
	return ts.attributes.GetAttribute(name)
}

// ClearAttributes clears all attributes.
func (ts *BaseTokenStream) ClearAttributes() {
	ts.attributes.ClearAttributes()
}

// IncrementToken advances to the next token.
// Subclasses must override this method.
func (ts *BaseTokenStream) IncrementToken() (bool, error) {
	return false, nil
}

// End performs end-of-stream operations.
func (ts *BaseTokenStream) End() error {
	return nil
}

// Close releases resources.
func (ts *BaseTokenStream) Close() error {
	return nil
}
