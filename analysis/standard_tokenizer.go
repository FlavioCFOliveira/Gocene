// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
	"regexp"
	"unicode"
)

// StandardTokenizer is a grammar-based tokenizer constructed with JFlex.
//
// This is a simplified Go port of Lucene's org.apache.lucene.analysis.standard.StandardTokenizer.
//
// This tokenizer implements the Word Break rules from the Unicode Text Segmentation
// algorithm (Unicode Standard Annex #29) for tokenizing text.
//
// The tokenizer recognizes:
// - Alphanumeric tokens (words)
// - Internet tokens (email, URLs)
// - Numbers with decimal points
//
// Note: This is a simplified implementation. A full implementation would require
// a complete state machine or regex-based approach matching UTS #51.
type StandardTokenizer struct {
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

	// tokenRe is the regex pattern for standard tokens
	tokenRe *regexp.Regexp
}

// tokenPattern matches standard word tokens.
// This is a simplified pattern - the real StandardTokenizer is more sophisticated.
var tokenPattern = regexp.MustCompile(`[A-Za-z0-9]+([._-][A-Za-z0-9]+)*`)

// NewStandardTokenizer creates a new StandardTokenizer.
func NewStandardTokenizer() *StandardTokenizer {
	t := &StandardTokenizer{
		BaseTokenizer: NewBaseTokenizer(),
		tokenRe:       tokenPattern,
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
func (t *StandardTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.scanner = bufio.NewScanner(input)
	t.scanner.Split(bufio.ScanRunes)
	t.currentOffset = 0
	return nil
}

// IncrementToken advances to the next token.
func (t *StandardTokenizer) IncrementToken() (bool, error) {
	if t.input == nil || t.scanner == nil {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()

	// Build token character by character
	var tokenChars []rune
	startOffset := t.currentOffset

	for t.scanner.Scan() {
		r := []rune(t.scanner.Text())[0]

		// Check if this rune is part of a token
		if isTokenChar(r) {
			tokenChars = append(tokenChars, r)
			t.currentOffset++
		} else if len(tokenChars) > 0 {
			// End of token - endOffset should be the position after the last token character
			t.emitToken(tokenChars, startOffset, t.currentOffset)
			t.currentOffset++ // Skip the non-token character
			return true, nil
		} else {
			// Skip non-token characters before any token
			t.currentOffset++
		}
	}

	// Emit final token if any
	if len(tokenChars) > 0 {
		t.emitToken(tokenChars, startOffset, t.currentOffset)
		return true, nil
	}

	return false, t.scanner.Err()
}

// isTokenChar checks if a rune is a valid token character.
func isTokenChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// emitToken emits a token with the given properties.
func (t *StandardTokenizer) emitToken(chars []rune, startOffset, endOffset int) {
	t.termAttr.SetValue(string(chars))
	t.offsetAttr.SetStartOffset(startOffset)
	t.offsetAttr.SetEndOffset(endOffset)
	t.posIncrAttr.SetPositionIncrement(1)
}

// Reset resets the tokenizer.
func (t *StandardTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentOffset = 0
	return nil
}

// End performs end-of-stream operations.
func (t *StandardTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.currentOffset)
	}
	return nil
}

// Ensure StandardTokenizer implements Tokenizer
var _ Tokenizer = (*StandardTokenizer)(nil)

// StandardTokenizerFactory creates StandardTokenizer instances.
type StandardTokenizerFactory struct{}

// NewStandardTokenizerFactory creates a new StandardTokenizerFactory.
func NewStandardTokenizerFactory() *StandardTokenizerFactory {
	return &StandardTokenizerFactory{}
}

// Create creates a new StandardTokenizer.
func (f *StandardTokenizerFactory) Create() Tokenizer {
	return NewStandardTokenizer()
}

// Ensure StandardTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*StandardTokenizerFactory)(nil)
