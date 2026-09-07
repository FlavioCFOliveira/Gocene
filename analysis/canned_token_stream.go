// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
)

// CannedTokenStream is a TokenStream from a canned list of Tokens.
//
// This is the Go port of Lucene's org.apache.lucene.tests.analysis.CannedTokenStream.
type CannedTokenStream struct {
	*BaseTokenStream
	tokens      []*Token
	upto        int
	finalOffset int
	finalPosInc int
	offsetAtt   OffsetAttribute
	posIncrAtt  tokenattributes.PositionIncrementAttribute
	charTermAtt CharTermAttribute
}

// NewCannedTokenStream creates a new CannedTokenStream with the given tokens.
func NewCannedTokenStream(tokens ...*Token) *CannedTokenStream {
	return NewCannedTokenStreamWithFinalOffset(0, 0, tokens...)
}

// NewCannedTokenStreamWithFinalOffset creates a new CannedTokenStream with final position and offset.
// If you want trailing holes, pass a non-zero finalPosInc.
func NewCannedTokenStreamWithFinalOffset(finalPosInc, finalOffset int, tokens ...*Token) *CannedTokenStream {
	base := NewBaseTokenStream()
	ts := &CannedTokenStream{
		BaseTokenStream: base,
		tokens:          make([]*Token, len(tokens)),
		upto:            0,
		finalOffset:     finalOffset,
		finalPosInc:     finalPosInc,
	}
	copy(ts.tokens, tokens)

	// Initialize attributes
	ts.offsetAtt = NewOffsetAttribute()
	ts.posIncrAtt = tokenattributes.NewPositionIncrementAttribute()
	ts.charTermAtt = NewCharTermAttribute()

	base.AddAttribute(reflect.TypeOf((*OffsetAttribute)(nil)).Elem())
	base.AddAttribute(reflect.TypeOf((*tokenattributes.PositionIncrementAttribute)(nil)).Elem())
	base.AddAttribute(reflect.TypeOf((*CharTermAttribute)(nil)).Elem())

	return ts
}

// End implements the TokenStream end method.
func (ts *CannedTokenStream) End() error {
	ts.BaseTokenStream.End()
	ts.posIncrAtt.SetPositionIncrement(ts.finalPosInc)
	ts.offsetAtt.SetOffset(ts.finalOffset, ts.finalOffset)
	return nil
}

// Reset resets the stream to the beginning.
func (ts *CannedTokenStream) Reset() error {
	ts.upto = 0
	return ts.BaseTokenStream.Reset()
}

// IncrementToken reads the next token from the canned list.
func (ts *CannedTokenStream) IncrementToken() (bool, error) {
	if ts.upto < len(ts.tokens) {
		ts.ClearAttributes()
		token := ts.tokens[ts.upto]
		ts.upto++

		// Copy token data to attributes
		ts.charTermAtt.SetValue(token.Term)
		ts.offsetAtt.SetOffset(token.StartOffset, token.EndOffset)
		ts.posIncrAtt.SetPositionIncrement(token.PositionInc)

		return true, nil
	}
	return false, nil
}
