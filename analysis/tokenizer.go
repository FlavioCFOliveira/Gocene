// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// Tokenizer is a TokenStream whose input is a Reader.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Tokenizer.
//
// Tokenizer is the source of tokens in the analysis pipeline. It reads
// characters from an input source and produces tokens. Subclasses must
// implement the IncrementToken method to tokenize the input.
type Tokenizer interface {
	TokenStream

	// SetReader sets the input source for this Tokenizer.
	// Must be called before IncrementToken is called.
	SetReader(input io.Reader) error

	// Reset resets the Tokenizer to a clean state.
	// Called before each tokenization session.
	Reset() error
}

// BaseTokenizer provides a base implementation for Tokenizer.
//
// Embed this struct in concrete Tokenizer implementations to inherit
// common functionality.
type BaseTokenizer struct {
	BaseTokenStream

	// input is the current input source
	input io.Reader

	// inputFinished tracks whether the input has been fully consumed
	inputFinished bool
}

// NewBaseTokenizer creates a new BaseTokenizer.
func NewBaseTokenizer() *BaseTokenizer {
	return &BaseTokenizer{
		BaseTokenStream: *NewBaseTokenStream(),
		input:           nil,
		inputFinished:   false,
	}
}

// SetReader sets the input source for this Tokenizer.
func (t *BaseTokenizer) SetReader(input io.Reader) error {
	t.input = input
	t.inputFinished = false
	return nil
}

// GetReader returns the current input reader.
func (t *BaseTokenizer) GetReader() io.Reader {
	return t.input
}

// Reset resets the Tokenizer to a clean state.
func (t *BaseTokenizer) Reset() error {
	t.inputFinished = false
	t.ClearAttributes()
	return nil
}

// IsInputFinished returns true if the input has been fully consumed.
func (t *BaseTokenizer) IsInputFinished() bool {
	return t.inputFinished
}

// SetInputFinished marks the input as fully consumed.
func (t *BaseTokenizer) SetInputFinished(finished bool) {
	t.inputFinished = finished
}

// End performs end-of-stream operations.
func (t *BaseTokenizer) End() error {
	// Default implementation does nothing
	return nil
}

// Close releases resources.
func (t *BaseTokenizer) Close() error {
	t.input = nil
	return nil
}

// TokenizerFactory creates Tokenizer instances.
//
// This is the Go port of Lucene's TokenizerFactory interface.
type TokenizerFactory interface {
	// Create creates a new Tokenizer.
	Create() Tokenizer
}

// LetterTokenizerFactory creates LetterTokenizer instances.
type LetterTokenizerFactory struct{}

// NewLetterTokenizerFactory creates a new LetterTokenizerFactory.
func NewLetterTokenizerFactory() *LetterTokenizerFactory {
	return &LetterTokenizerFactory{}
}

// Create creates a new LetterTokenizer.
func (f *LetterTokenizerFactory) Create() Tokenizer {
	return NewLetterTokenizer()
}

// Ensure LetterTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*LetterTokenizerFactory)(nil)
