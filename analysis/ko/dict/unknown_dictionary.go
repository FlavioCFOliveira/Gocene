// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// UnknownDictionary is the dictionary for unknown-word handling during
// morphological analysis.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.UnknownDictionary from Apache Lucene
// 10.4.0.
//
// Deviation: the Java original loads pre-built binary resources from the JAR
// classpath. The Go port provides the type with constructors; resource loading
// is deferred to the nori codec sprint.
type UnknownDictionary struct {
	// morphAtts provides morphological attributes for unknown words.
	morphAtts *UnknownMorphData
	// charDef provides character category data.
	charDef *CharacterDefinition
	// targetMap maps character class IDs to lists of byte offsets in buffer.
	targetMap [][]int
}

// NewUnknownDictionary creates an UnknownDictionary with the given components.
func NewUnknownDictionary(
	morphAtts *UnknownMorphData,
	charDef *CharacterDefinition,
	targetMap [][]int,
) *UnknownDictionary {
	return &UnknownDictionary{morphAtts: morphAtts, charDef: charDef, targetMap: targetMap}
}

// GetMorphAttributes returns the UnknownMorphData for this dictionary.
func (d *UnknownDictionary) GetMorphAttributes() *UnknownMorphData { return d.morphAtts }

// GetCharacterDefinition returns the CharacterDefinition for this dictionary.
func (d *UnknownDictionary) GetCharacterDefinition() *CharacterDefinition { return d.charDef }

// LookupWordIDs populates wordIDRef with the list of wordIDs for the given
// character class ID.
func (d *UnknownDictionary) LookupWordIDs(characterClass int, wordIDRef *[]int) {
	if characterClass >= 0 && characterClass < len(d.targetMap) {
		ids := d.targetMap[characterClass]
		if cap(*wordIDRef) < len(ids) {
			*wordIDRef = make([]int, len(ids))
		} else {
			*wordIDRef = (*wordIDRef)[:len(ids)]
		}
		copy(*wordIDRef, ids)
	} else {
		*wordIDRef = (*wordIDRef)[:0]
	}
}

// defaultUnknownDictionary is the zero-value singleton.
var defaultUnknownDictionary = &UnknownDictionary{
	morphAtts: NewUnknownMorphData(nil, nil),
	charDef:   GetCharacterDefinitionInstance(),
}

// GetUnknownDictionaryInstance returns the default UnknownDictionary singleton.
//
// Deviation: the Java original loads binary data from the JAR classpath via a
// lazy singleton. The Go port returns an empty instance; full binary loading is
// deferred to the nori codec sprint.
func GetUnknownDictionaryInstance() *UnknownDictionary {
	return defaultUnknownDictionary
}
