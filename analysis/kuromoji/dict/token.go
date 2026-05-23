// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

// Token is a morphological token produced by JapaneseTokenizer. It extends the
// base morph.Token with Japanese-specific morphological data.
//
// This is the Go port of org.apache.lucene.analysis.ja.Token from Apache
// Lucene 10.4.0.
type Token struct {
	// SurfaceForm is the raw character slice from the rolling buffer.
	SurfaceForm []rune
	// Offset is the start position of the surface form within SurfaceForm.
	Offset int
	// Length is the number of runes in the surface form.
	Length int
	// StartOffset is the start offset in the original character stream.
	StartOffset int
	// EndOffset is the end offset in the original character stream.
	EndOffset int
	// MorphID is the dictionary word ID for morphological data lookup.
	MorphID int
	// Type is the token classification (KNOWN, UNKNOWN, USER).
	Type morph.TokenType
	// MorphData provides morphological attributes for this token.
	MorphData JaMorphData
	// posLen is the position length (number of positions this token spans).
	posLen int
}

// NewToken creates a new Token.
func NewToken(
	surfaceForm []rune,
	offset, length, startOffset, endOffset, morphID int,
	tokenType morph.TokenType,
	morphData JaMorphData,
) *Token {
	return &Token{
		SurfaceForm: surfaceForm,
		Offset:      offset,
		Length:      length,
		StartOffset: startOffset,
		EndOffset:   endOffset,
		MorphID:     morphID,
		Type:        tokenType,
		MorphData:   morphData,
		posLen:      1,
	}
}

// SetPositionLength sets the position length of the token.
func (t *Token) SetPositionLength(n int) { t.posLen = n }

// GetPositionLength returns the position length of the token.
func (t *Token) GetPositionLength() int { return t.posLen }

// String returns a debug representation of the token.
func (t *Token) String() string {
	surface := string(t.SurfaceForm[t.Offset : t.Offset+t.Length])
	leftID := -1
	if t.MorphData != nil {
		leftID = t.MorphData.LeftID(t.MorphID)
	}
	return fmt.Sprintf(
		"Token(%q offset=%d length=%d posLen=%d type=%s morphId=%d leftID=%d)",
		surface, t.StartOffset, t.Length, t.posLen, t.Type, t.MorphID, leftID,
	)
}

// Reading returns the reading (katakana) of the token, or empty string.
func (t *Token) Reading() string {
	if t.MorphData == nil {
		return ""
	}
	surface := t.SurfaceForm[t.Offset : t.Offset+t.Length]
	return t.MorphData.Reading(t.MorphID, surface, 0, len(surface))
}

// Pronunciation returns the pronunciation of the token, or empty string.
func (t *Token) Pronunciation() string {
	if t.MorphData == nil {
		return ""
	}
	surface := t.SurfaceForm[t.Offset : t.Offset+t.Length]
	return t.MorphData.Pronunciation(t.MorphID, surface, 0, len(surface))
}

// PartOfSpeech returns the part-of-speech tag of the token.
func (t *Token) PartOfSpeech() string {
	if t.MorphData == nil {
		return ""
	}
	return t.MorphData.PartOfSpeech(t.MorphID)
}

// InflectionType returns the inflection type of the token, or empty string.
func (t *Token) InflectionType() string {
	if t.MorphData == nil {
		return ""
	}
	return t.MorphData.InflectionType(t.MorphID)
}

// InflectionForm returns the inflection form of the token, or empty string.
func (t *Token) InflectionForm() string {
	if t.MorphData == nil {
		return ""
	}
	return t.MorphData.InflectionForm(t.MorphID)
}

// BaseForm returns the dictionary base form of the token, or empty string.
func (t *Token) BaseForm() string {
	if t.MorphData == nil {
		return ""
	}
	surface := t.SurfaceForm[t.Offset : t.Offset+t.Length]
	return t.MorphData.BaseForm(t.MorphID, surface, 0, len(surface))
}

// IsKnown reports whether this token is from the system dictionary.
func (t *Token) IsKnown() bool { return t.Type == morph.TokenTypeKnown }

// IsUnknown reports whether this token is an unknown word.
func (t *Token) IsUnknown() bool { return t.Type == morph.TokenTypeUnknown }

// IsUser reports whether this token is from the user dictionary.
func (t *Token) IsUser() bool { return t.Type == morph.TokenTypeUser }
