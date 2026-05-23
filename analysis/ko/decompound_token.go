// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

// DecompoundToken is a token generated from a compound decomposition.
//
// This is the Go port of org.apache.lucene.analysis.ko.DecompoundToken from
// Apache Lucene 10.4.0.
type DecompoundToken struct {
	Token
	posTag POSTag
}

// NewDecompoundToken creates a DecompoundToken from a morpheme surface form.
func NewDecompoundToken(
	posTag POSTag,
	surfaceForm string,
	startOffset, endOffset int,
	tokenType morph.TokenType,
) *DecompoundToken {
	runes := []rune(surfaceForm)
	t := &DecompoundToken{
		Token:  *newToken(runes, 0, len(runes), startOffset, endOffset, tokenType),
		posTag: posTag,
	}
	return t
}

// String returns a debug representation.
func (t *DecompoundToken) String() string {
	return fmt.Sprintf(
		"DecompoundToken(%q pos=%d length=%d startOffset=%d endOffset=%d)",
		t.GetSurfaceFormString(), t.startOffset, t.length, t.startOffset, t.endOffset,
	)
}

// GetPOSType always returns POSTypeMorpheme for decompound tokens.
func (t *DecompoundToken) GetPOSType() POSType { return POSTypeMorpheme }

// GetLeftPOS returns the POS tag of this morpheme.
func (t *DecompoundToken) GetLeftPOS() POSTag { return t.posTag }

// GetRightPOS returns the POS tag of this morpheme.
func (t *DecompoundToken) GetRightPOS() POSTag { return t.posTag }

// GetReading always returns empty string for decompound tokens.
func (t *DecompoundToken) GetReading() string { return "" }

// GetMorphemes always returns nil for decompound tokens.
func (t *DecompoundToken) GetMorphemes() []dict.Morpheme { return nil }
