// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "github.com/FlavioCFOliveira/Gocene/analysis/morph"

// UnknownDictionary is the dictionary used for unknown-word handling during
// morphological analysis.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.UnknownDictionary from Apache Lucene
// 10.4.0.
//
// Deviation: the Java original loads pre-built binary resources from the JAR
// classpath. The Go port accepts the constituent data at construction time;
// loading from embedded resources is deferred to the codec sprint.
type UnknownDictionary struct {
	// base holds the packed binary entry data (targetMap + buffer).
	base morph.BinaryDictionary
	// morphAttrs provides the morphological attributes for unknown words.
	morphAttrs *UnknownMorphData
	// charDef provides character category data for unknown-word grouping.
	charDef *CharacterDefinition
}

// NewUnknownDictionary creates an UnknownDictionary with the given components.
func NewUnknownDictionary(
	base morph.BinaryDictionary,
	morphAttrs *UnknownMorphData,
	charDef *CharacterDefinition,
) *UnknownDictionary {
	return &UnknownDictionary{
		base:       base,
		morphAttrs: morphAttrs,
		charDef:    charDef,
	}
}

// GetMorphAttributes returns the UnknownMorphData for this dictionary.
func (d *UnknownDictionary) GetMorphAttributes() *UnknownMorphData { return d.morphAttrs }

// GetCharacterDefinition returns the CharacterDefinition for this dictionary.
func (d *UnknownDictionary) GetCharacterDefinition() *CharacterDefinition { return d.charDef }

// LookupWordIDs populates wordIDs with the list of word IDs for the given
// character class. The slice is overwritten on each call.
func (d *UnknownDictionary) LookupWordIDs(characterClass int, wordIDs *[]int) {
	ids := d.base.Lookup(characterClass)
	if cap(*wordIDs) < len(ids) {
		*wordIDs = make([]int, len(ids))
	} else {
		*wordIDs = (*wordIDs)[:len(ids)]
	}
	copy(*wordIDs, ids)
}

// LeftID delegates to morphAttrs.
func (d *UnknownDictionary) LeftID(wordID int) int {
	if d.morphAttrs == nil {
		return 0
	}
	return d.morphAttrs.LeftID(wordID)
}

// RightID delegates to morphAttrs.
func (d *UnknownDictionary) RightID(wordID int) int {
	if d.morphAttrs == nil {
		return 0
	}
	return d.morphAttrs.RightID(wordID)
}

// WordCost delegates to morphAttrs.
func (d *UnknownDictionary) WordCost(wordID int) int {
	if d.morphAttrs == nil {
		return 0
	}
	return d.morphAttrs.WordCost(wordID)
}

// PartOfSpeech delegates to morphAttrs.
func (d *UnknownDictionary) PartOfSpeech(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.PartOfSpeech(wordID)
}

// Reading delegates to morphAttrs.
func (d *UnknownDictionary) Reading(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.Reading(wordID, surface, off, length)
}

// Pronunciation delegates to morphAttrs.
func (d *UnknownDictionary) Pronunciation(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.Pronunciation(wordID, surface, off, length)
}

// BaseForm delegates to morphAttrs.
func (d *UnknownDictionary) BaseForm(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.BaseForm(wordID, surface, off, length)
}

// InflectionType delegates to morphAttrs.
func (d *UnknownDictionary) InflectionType(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.InflectionType(wordID)
}

// InflectionForm delegates to morphAttrs.
func (d *UnknownDictionary) InflectionForm(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.InflectionForm(wordID)
}

// Ensure UnknownDictionary implements JaMorphData.
var _ JaMorphData = (*UnknownDictionary)(nil)
