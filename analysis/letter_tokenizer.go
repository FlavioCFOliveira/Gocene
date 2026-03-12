// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
	"unicode"
)

// LetterTokenizer is a tokenizer that divides text at non-letters.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.core.LetterTokenizer.
//
// LetterTokenizer tokenizes text by recognizing sequences of letters (as defined
// by unicode.IsLetter) as tokens. All non-letter characters act as token
// separators and are discarded.
//
// This is the most basic tokenizer that produces the simplest tokens - just
// contiguous sequences of letters. It's commonly used as the foundation for
// analyzers that need to process words in a language-agnostic way.
//
// Example:
//   Input: "Hello, World! 123 Test."
//   Output tokens: "Hello", "World", "Test"
type LetterTokenizer struct {
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

// NewLetterTokenizer creates a new LetterTokenizer.
func NewLetterTokenizer() *LetterTokenizer {
	t := &LetterTokenizer{
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
func (t *LetterTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.scanner = bufio.NewScanner(input)
	t.scanner.Split(bufio.ScanRunes)
	t.currentOffset = 0
	t.currentToken = nil
	t.tokenStartOffset = 0
	return nil
}

// IncrementToken advances to the next token.
func (t *LetterTokenizer) IncrementToken() (bool, error) {
	if t.input == nil || t.scanner == nil {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Process characters until we find a letter sequence
	for t.scanner.Scan() {
		r := []rune(t.scanner.Text())[0]

		// Check if this is a letter
		if unicode.IsLetter(r) {
			// Add to current token
			if len(t.currentToken) == 0 {
				t.tokenStartOffset = t.currentOffset
			}
			t.currentToken = append(t.currentToken, r)
		} else if len(t.currentToken) > 0 {
			// End of token - emit it
			t.emitToken()
			t.currentOffset++
			return true, nil
		}
		// Otherwise skip non-letter characters
		t.currentOffset++
	}

	// Emit final token if any
	if len(t.currentToken) > 0 {
		t.emitToken()
		return true, nil
	}

	return false, t.scanner.Err()
}

// emitToken emits the current token and resets the buffer.
func (t *LetterTokenizer) emitToken() {
	t.termAttr.SetValue(string(t.currentToken))
	t.offsetAttr.SetStartOffset(t.tokenStartOffset)
	t.offsetAttr.SetEndOffset(t.currentOffset)
	t.posIncrAttr.SetPositionIncrement(1)

	// Reset token buffer
	t.currentToken = nil
	t.tokenStartOffset = t.currentOffset
}

// Reset resets the tokenizer.
func (t *LetterTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentOffset = 0
	t.currentToken = nil
	t.tokenStartOffset = 0
	return nil
}

// End performs end-of-stream operations.
func (t *LetterTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.currentOffset)
	}
	return nil
}

// Ensure LetterTokenizer implements Tokenizer
var _ Tokenizer = (*LetterTokenizer)(nil)
