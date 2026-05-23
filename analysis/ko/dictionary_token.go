// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

// DictionaryToken is a token stored in a KoMorphData dictionary.
//
// This is the Go port of org.apache.lucene.analysis.ko.DictionaryToken from
// Apache Lucene 10.4.0.
type DictionaryToken struct {
	Token
	wordID    int
	morphAtts dict.KoMorphData
}

// NewDictionaryToken creates a DictionaryToken.
func NewDictionaryToken(
	tokenType morph.TokenType,
	morphAtts dict.KoMorphData,
	wordID int,
	surfaceForm []rune,
	offset, length, startOffset, endOffset int,
) *DictionaryToken {
	return &DictionaryToken{
		Token:     *newToken(surfaceForm, offset, length, startOffset, endOffset, tokenType),
		wordID:    wordID,
		morphAtts: morphAtts,
	}
}

// String returns a debug representation.
func (t *DictionaryToken) String() string {
	leftID := -1
	if t.morphAtts != nil {
		leftID = t.morphAtts.LeftID(t.wordID)
	}
	return fmt.Sprintf(
		"DictionaryToken(%q pos=%d length=%d posLen=%d type=%s wordID=%d leftID=%d)",
		t.GetSurfaceFormString(), t.startOffset, t.length, t.posLen, t.tokenType, t.wordID, leftID,
	)
}

// IsKnown reports whether this token is from the system dictionary.
func (t *DictionaryToken) IsKnown() bool { return t.tokenType == morph.TokenTypeKnown }

// IsUnknown reports whether this token is an unknown word.
func (t *DictionaryToken) IsUnknown() bool { return t.tokenType == morph.TokenTypeUnknown }

// IsUser reports whether this token is from the user dictionary.
func (t *DictionaryToken) IsUser() bool { return t.tokenType == morph.TokenTypeUser }

// GetPOSType returns the POS type from the dictionary.
func (t *DictionaryToken) GetPOSType() POSType {
	if t.morphAtts == nil {
		return POSTypeMorpheme
	}
	return t.morphAtts.GetPOSType(t.wordID)
}

// GetLeftPOS returns the left POS tag from the dictionary.
func (t *DictionaryToken) GetLeftPOS() POSTag {
	if t.morphAtts == nil {
		return POSTagUNKNOWN
	}
	return t.morphAtts.GetLeftPOS(t.wordID)
}

// GetRightPOS returns the right POS tag from the dictionary.
func (t *DictionaryToken) GetRightPOS() POSTag {
	if t.morphAtts == nil {
		return POSTagUNKNOWN
	}
	return t.morphAtts.GetRightPOS(t.wordID)
}

// GetReading returns the reading (Hanja → Hangul) from the dictionary.
func (t *DictionaryToken) GetReading() string {
	if t.morphAtts == nil {
		return ""
	}
	return t.morphAtts.GetReading(t.wordID)
}

// GetMorphemes returns the morpheme decomposition from the dictionary.
func (t *DictionaryToken) GetMorphemes() []dict.Morpheme {
	if t.morphAtts == nil {
		return nil
	}
	return t.morphAtts.GetMorphemes(t.wordID, t.surfaceForm, t.offset, t.length)
}
