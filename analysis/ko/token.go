// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

// Token is an analyzed Korean token with morphological data.
//
// This is the Go port of org.apache.lucene.analysis.ko.Token from Apache
// Lucene 10.4.0.
type Token struct {
	// surfaceForm is the raw character slice from the rolling buffer.
	surfaceForm []rune
	// offset is the start position within surfaceForm.
	offset int
	// length is the number of runes in the surface form.
	length int
	// startOffset is the start offset in the original character stream.
	startOffset int
	// endOffset is the end offset in the original character stream.
	endOffset int
	// tokenType is the token classification (KNOWN, UNKNOWN, USER).
	tokenType morph.TokenType
	// posIncrement is the position increment (default 1).
	posIncrement int
	// posLen is the position length (number of positions this token spans).
	posLen int
}

func newToken(
	surfaceForm []rune,
	offset, length, startOffset, endOffset int,
	tokenType morph.TokenType,
) *Token {
	return &Token{
		surfaceForm: surfaceForm,
		offset:      offset,
		length:      length,
		startOffset: startOffset,
		endOffset:   endOffset,
		tokenType:   tokenType,
		posIncrement: 1,
		posLen:      1,
	}
}

// GetSurfaceForm returns the raw surface form slice.
func (t *Token) GetSurfaceForm() []rune { return t.surfaceForm }

// GetOffset returns the start position within surfaceForm.
func (t *Token) GetOffset() int { return t.offset }

// GetLength returns the number of runes in the surface form.
func (t *Token) GetLength() int { return t.length }

// GetStartOffset returns the start offset in the original character stream.
func (t *Token) GetStartOffset() int { return t.startOffset }

// GetEndOffset returns the end offset in the original character stream.
func (t *Token) GetEndOffset() int { return t.endOffset }

// GetSurfaceFormString returns the surface form as a string.
func (t *Token) GetSurfaceFormString() string {
	return string(t.surfaceForm[t.offset : t.offset+t.length])
}

// SetPositionIncrement sets the position increment.
func (t *Token) SetPositionIncrement(n int) { t.posIncrement = n }

// GetPositionIncrement returns the position increment.
func (t *Token) GetPositionIncrement() int { return t.posIncrement }

// SetPositionLength sets the position length.
func (t *Token) SetPositionLength(n int) { t.posLen = n }

// GetPositionLength returns the position length.
func (t *Token) GetPositionLength() int { return t.posLen }

// GetPOSType returns the POS type of this token.
func (t *Token) GetPOSType() POSType { return POSTypeMorpheme }

// GetLeftPOS returns the left POS tag of this token.
func (t *Token) GetLeftPOS() POSTag { return POSTagUNKNOWN }

// GetRightPOS returns the right POS tag of this token.
func (t *Token) GetRightPOS() POSTag { return POSTagUNKNOWN }

// GetReading returns the reading of this token (Hanja → Hangul).
func (t *Token) GetReading() string { return "" }

// GetMorphemes returns the morpheme decomposition of this token.
func (t *Token) GetMorphemes() []dict.Morpheme { return nil }
