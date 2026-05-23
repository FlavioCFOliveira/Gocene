// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "github.com/FlavioCFOliveira/Gocene/analysis/morph"

// TokenInfoDictionary is the main system dictionary for kuromoji. It maps
// token surface forms to word IDs via a TokenInfoFST and stores morphological
// attributes in a packed binary buffer.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.TokenInfoDictionary from Apache Lucene
// 10.4.0.
//
// Deviation: the Java original loads pre-built binary resources from the JAR
// classpath, including an FST file. The Go port accepts the constituent data
// at construction time; resource loading is deferred to the codec sprint.
type TokenInfoDictionary struct {
	// base holds the packed binary entry data.
	base morph.BinaryDictionary
	// fst maps byte sequences to word ID lists.
	fst *morph.TokenInfoFST
	// morphAttrs provides morphological attributes for system words.
	morphAttrs *TokenInfoMorphData
}

// NewTokenInfoDictionary creates a TokenInfoDictionary with the given
// components.
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

// GetFST returns the TokenInfoFST used for surface-form lookup.
func (d *TokenInfoDictionary) GetFST() *morph.TokenInfoFST { return d.fst }

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
