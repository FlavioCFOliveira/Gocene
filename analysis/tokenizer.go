// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"io"

	"github.com/FlavioCFOliveira/Gocene/util"
)

var (
	errIllegalState = errors.New("TokenStream contract violation: reset()/close() call missing, reset() called multiple times, or subclass does not call super.reset(). Please see Javadocs of TokenStream class for more information about the correct consuming workflow")
)

// illegalStateReader is used to detect violations of the TokenStream contract.
type illegalStateReader struct{}

func (r *illegalStateReader) Read(p []byte) (n int, err error) {
	return 0, errIllegalState
}

func (r *illegalStateReader) Close() error {
	return nil
}

var illegalStateReaderInstance = &illegalStateReader{}

// Tokenizer is a TokenStream whose input is a Reader.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Tokenizer.
//
// Tokenizer is an abstract base for tokenizers. Subclasses must override
// IncrementToken to produce tokens from the input reader.
type Tokenizer interface {
	TokenStream

	// SetReader sets the input source for this Tokenizer.
	SetReader(input io.Reader)
}

// BaseTokenizer provides the common implementation for Tokenizers.
//
// Embed this struct in concrete Tokenizer implementations to inherit
// the Lucene Tokenizer behavior.
type BaseTokenizer struct {
	BaseTokenStream

	// input is the text source for this Tokenizer.
	input io.Reader

	// inputPending is the reader that will be assigned to input during reset().
	inputPending io.Reader
}

// NewBaseTokenizer creates a new BaseTokenizer with no input, awaiting a call to SetReader.
func NewBaseTokenizer() *BaseTokenizer {
	return NewBaseTokenizerWithFactory(util.DefaultAttributeFactoryInstance)
}

// NewBaseTokenizerWithFactory creates a new BaseTokenizer with no input,
// awaiting a call to SetReader, using the supplied attribute factory.
func NewBaseTokenizerWithFactory(factory util.AttributeFactory) *BaseTokenizer {
	if factory == nil {
		panic("BaseTokenizer factory must not be nil")
	}
	return &BaseTokenizer{
		BaseTokenStream: *NewBaseTokenStreamWithFactory(factory),
		input:           illegalStateReaderInstance,
		inputPending:    illegalStateReaderInstance,
	}
}

// Close closes the input reader and resets the Tokenizer state.
//
// The default implementation closes the input Reader, so concrete implementations
// overriding this method should call BaseTokenizer.Close().
func (t *BaseTokenizer) Close() error {
	var err error
	if closer, ok := t.input.(io.Closer); ok {
		err = closer.Close()
	}

	// Don't hold onto Reader after close, so GC can reclaim
	t.input = illegalStateReaderInstance
	t.inputPending = illegalStateReaderInstance

	return err
}

// CorrectOffset returns the corrected offset. If the input is a CharFilter,
// it calls CorrectOffset on the filter; otherwise, it returns the current offset.
func (t *BaseTokenizer) CorrectOffset(currentOff int) int {
	if cf, ok := t.input.(CharFilter); ok {
		return cf.CorrectOffset(currentOff)
	}
	return currentOff
}

// SetReader sets a new reader on the Tokenizer.
//
// Typically, an analyzer will use this to reuse a previously created tokenizer.
// Panics if input is nil or if the TokenStream contract is violated (close() call missing).
func (t *BaseTokenizer) SetReader(input io.Reader) {
	if input == nil {
		panic("input must not be null")
	}
	if t.input != illegalStateReaderInstance {
		panic("TokenStream contract violation: close() call missing")
	}
	t.inputPending = input
	t.setReaderTestPoint()
}

// Reset resets the Tokenizer to a clean state and assigns the pending reader to the active input.
func (t *BaseTokenizer) Reset() error {
	if err := t.BaseTokenStream.Reset(); err != nil {
		return err
	}
	t.input = t.inputPending
	t.inputPending = illegalStateReaderInstance
	return nil
}

// setReaderTestPoint is an internal method used for testing.
func (t *BaseTokenizer) setReaderTestPoint() {}
