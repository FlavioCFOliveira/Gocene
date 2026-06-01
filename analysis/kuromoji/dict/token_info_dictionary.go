// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
	gofst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// TokenInfoDictionary is the main system dictionary for kuromoji. It maps
// token surface forms to word IDs via a TokenInfoFST and stores morphological
// attributes in a packed binary buffer.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.TokenInfoDictionary from Apache Lucene
// 10.4.0.
type TokenInfoDictionary struct {
	// base holds the packed binary entry data.
	base morph.BinaryDictionary
	// fst is the legacy morph placeholder; use realFST for real arc traversal.
	fst *morph.TokenInfoFST
	// realFST is the loaded FST[int64] from the embedded binary resource.
	// It is nil until GetTokenInfoDictionaryInstance() has been called.
	realFST *gofst.FST[int64]
	// morphAttrs provides morphological attributes for system words.
	morphAttrs *TokenInfoMorphData
}

// NewTokenInfoDictionary creates a TokenInfoDictionary with the given
// components (used by tests and the CSV-based builder path).
func NewTokenInfoDictionary(
	base morph.BinaryDictionary,
	fst *morph.TokenInfoFST,
	morphAttrs *TokenInfoMorphData,
) *TokenInfoDictionary {
	return &TokenInfoDictionary{
		base:       base,
		fst:        fst,
		morphAttrs: morphAttrs,
	}
}

// newTokenInfoDictionaryFromBinary is the internal constructor used by the
// embedded-resource loader; it wires up the real FST alongside the morph
// placeholder.
func newTokenInfoDictionaryFromBinary(
	base morph.BinaryDictionary,
	realFST *gofst.FST[int64],
	morphAttrs *TokenInfoMorphData,
) *TokenInfoDictionary {
	return &TokenInfoDictionary{
		base:       base,
		fst:        morph.NewTokenInfoFST(),
		realFST:    realFST,
		morphAttrs: morphAttrs,
	}
}

// GetFST returns the morph-level TokenInfoFST placeholder.
func (d *TokenInfoDictionary) GetFST() *morph.TokenInfoFST { return d.fst }

// GetRealFST returns the loaded FST[int64] from the embedded binary resource,
// or nil if the dictionary was constructed without binary data.
func (d *TokenInfoDictionary) GetRealFST() *gofst.FST[int64] { return d.realFST }

// GetMorphAttributes returns the TokenInfoMorphData for this dictionary.
func (d *TokenInfoDictionary) GetMorphAttributes() *TokenInfoMorphData { return d.morphAttrs }

// LeftID delegates to morphAttrs.
func (d *TokenInfoDictionary) LeftID(wordID int) int {
	if d.morphAttrs == nil {
		return 0
	}
	return d.morphAttrs.LeftID(wordID)
}

// RightID delegates to morphAttrs.
func (d *TokenInfoDictionary) RightID(wordID int) int {
	if d.morphAttrs == nil {
		return 0
	}
	return d.morphAttrs.RightID(wordID)
}

// WordCost delegates to morphAttrs.
func (d *TokenInfoDictionary) WordCost(wordID int) int {
	if d.morphAttrs == nil {
		return 0
	}
	return d.morphAttrs.WordCost(wordID)
}

// PartOfSpeech delegates to morphAttrs.
func (d *TokenInfoDictionary) PartOfSpeech(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.PartOfSpeech(wordID)
}

// Reading delegates to morphAttrs.
func (d *TokenInfoDictionary) Reading(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.Reading(wordID, surface, off, length)
}

// Pronunciation delegates to morphAttrs.
func (d *TokenInfoDictionary) Pronunciation(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.Pronunciation(wordID, surface, off, length)
}

// BaseForm delegates to morphAttrs.
func (d *TokenInfoDictionary) BaseForm(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.BaseForm(wordID, surface, off, length)
}

// InflectionType delegates to morphAttrs.
func (d *TokenInfoDictionary) InflectionType(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.InflectionType(wordID)
}

// InflectionForm delegates to morphAttrs.
func (d *TokenInfoDictionary) InflectionForm(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.InflectionForm(wordID)
}

// Ensure TokenInfoDictionary implements JaMorphData.
var _ JaMorphData = (*TokenInfoDictionary)(nil)
