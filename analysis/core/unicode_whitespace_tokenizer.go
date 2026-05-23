// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package core

import (
	"bufio"
	"io"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// UnicodeWhitespaceTokenizer divides text at Unicode whitespace characters.
//
// Adjacent sequences of non-whitespace characters form tokens according to
// Unicode's White_Space property (unicode.White_Space). This differs from
// [analysis.WhitespaceTokenizer], which uses Java-style whitespace semantics.
//
// This is the Go port of
// org.apache.lucene.analysis.core.UnicodeWhitespaceTokenizer from Apache
// Lucene 10.4.0.
//
// Deviation: Java extends CharTokenizer and overrides isTokenChar; Go uses
// the same streaming scanner pattern as other Gocene tokenizers.
type UnicodeWhitespaceTokenizer struct {
	*analysis.BaseTokenizer

	scanner          *bufio.Scanner
	termAttr         analysis.CharTermAttribute
	offsetAttr       analysis.OffsetAttribute
	posIncrAttr      analysis.PositionIncrementAttribute
	currentOffset    int
	currentToken     []rune
	tokenStartOffset int
}

// NewUnicodeWhitespaceTokenizer creates a new UnicodeWhitespaceTokenizer.
func NewUnicodeWhitespaceTokenizer() *UnicodeWhitespaceTokenizer {
	t := &UnicodeWhitespaceTokenizer{
		BaseTokenizer: analysis.NewBaseTokenizer(),
	}
	t.termAttr = analysis.NewCharTermAttribute()
	t.offsetAttr = analysis.NewOffsetAttribute()
	t.posIncrAttr = analysis.NewPositionIncrementAttribute()
	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)
	return t
}

// SetReader sets the input source for this Tokenizer.
func (t *UnicodeWhitespaceTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.scanner = bufio.NewScanner(input)
	t.scanner.Split(bufio.ScanRunes)
	t.currentOffset = 0
	t.currentToken = nil
	t.tokenStartOffset = 0
	return nil
}

// IncrementToken advances to the next token.
func (t *UnicodeWhitespaceTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()
	for t.scanner.Scan() {
		r := []rune(t.scanner.Text())[0]
		if unicode.Is(unicode.White_Space, r) {
			if len(t.currentToken) > 0 {
				t.emitToken()
				t.currentOffset++
				return true, nil
			}
			t.currentOffset++
			t.tokenStartOffset = t.currentOffset
		} else {
			if len(t.currentToken) == 0 {
				t.tokenStartOffset = t.currentOffset
			}
			t.currentToken = append(t.currentToken, r)
			t.currentOffset++
		}
	}
	if len(t.currentToken) > 0 {
		t.emitToken()
		return true, nil
	}
	return false, t.scanner.Err()
}

func (t *UnicodeWhitespaceTokenizer) emitToken() {
	t.termAttr.SetValue(string(t.currentToken))
	t.offsetAttr.SetStartOffset(t.tokenStartOffset)
	t.offsetAttr.SetEndOffset(t.currentOffset)
	t.posIncrAttr.SetPositionIncrement(1)
	t.currentToken = nil
	t.tokenStartOffset = t.currentOffset
}

// Reset resets the tokenizer.
func (t *UnicodeWhitespaceTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentOffset = 0
	t.currentToken = nil
	t.tokenStartOffset = 0
	return nil
}

// End performs end-of-stream operations.
func (t *UnicodeWhitespaceTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.currentOffset)
	}
	return nil
}

// Ensure UnicodeWhitespaceTokenizer implements analysis.Tokenizer.
var _ analysis.Tokenizer = (*UnicodeWhitespaceTokenizer)(nil)
