// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
	"unicode"
)

// WhitespaceTokenizer divides text at whitespace.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.WhitespaceTokenizer.
//
// This tokenizer splits tokens on whitespace characters as defined by
// unicode.IsSpace. It does not perform any other tokenization rules.
// The resulting tokens are not modified (no lowercasing, no stop word removal).
type WhitespaceTokenizer struct {
	*BaseTokenizer

	// scanner reads from the input
	scanner *bufio.Scanner

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute

	// currentOffset tracks the current position in input
	currentOffset int

	// currentToken holds token being built
	currentToken []rune

	// tokenStartOffset holds the start offset of current token
	tokenStartOffset int
}

// NewWhitespaceTokenizer creates a new WhitespaceTokenizer.
func NewWhitespaceTokenizer() *WhitespaceTokenizer {
	t := &WhitespaceTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
	}

	// Add attributes
	t.termAttr = NewCharTermAttribute()
	t.offsetAttr = NewOffsetAttribute()
	t.posIncrAttr = NewPositionIncrementAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)

	return t
}

// SetReader sets the input source for this Tokenizer.
func (t *WhitespaceTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.scanner = bufio.NewScanner(input)
	t.scanner.Split(bufio.ScanRunes)
	t.currentOffset = 0
	t.currentToken = nil
	t.tokenStartOffset = 0
	return nil
}

// IncrementToken advances to the next token.
func (t *WhitespaceTokenizer) IncrementToken() (bool, error) {
	if t.input == nil || t.scanner == nil {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Process characters until we find a non-whitespace token
	for t.scanner.Scan() {
		r := []rune(t.scanner.Text())[0]

		// Check if this is whitespace
		if unicode.IsSpace(r) {
			// If we have a token built, emit it
			if len(t.currentToken) > 0 {
				t.emitToken()
				t.currentOffset++
				return true, nil
			}
			// Otherwise skip whitespace
			t.currentOffset++
			t.tokenStartOffset = t.currentOffset
		} else {
			// Add to current token
			if len(t.currentToken) == 0 {
				t.tokenStartOffset = t.currentOffset
			}
			t.currentToken = append(t.currentToken, r)
			t.currentOffset++
		}
	}

	// Emit final token if any
	if len(t.currentToken) > 0 {
		t.emitToken()
		return true, nil
	}

	return false, t.scanner.Err()
}

// emitToken emits the current token and resets the buffer.
func (t *WhitespaceTokenizer) emitToken() {
	t.termAttr.SetValue(string(t.currentToken))
	t.offsetAttr.SetStartOffset(t.tokenStartOffset)
	t.offsetAttr.SetEndOffset(t.currentOffset)
	t.posIncrAttr.SetPositionIncrement(1)

	// Reset token buffer
	t.currentToken = nil
	t.tokenStartOffset = t.currentOffset
}

// Reset resets the tokenizer.
func (t *WhitespaceTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentOffset = 0
	t.currentToken = nil
	t.tokenStartOffset = 0
	return nil
}

// End performs end-of-stream operations.
func (t *WhitespaceTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.currentOffset)
	}
	return nil
}

// Ensure WhitespaceTokenizer implements Tokenizer
var _ Tokenizer = (*WhitespaceTokenizer)(nil)
