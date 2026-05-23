// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "github.com/FlavioCFOliveira/Gocene/analysis/morph"

// JaMorphData represents Japanese morphological information stored in a
// dictionary entry.
//
// This is the Go port of org.apache.lucene.analysis.ja.dict.JaMorphData from
// Apache Lucene 10.4.0.
type JaMorphData interface {
	morph.MorphData

	// PartOfSpeech returns the part-of-speech tag for the morpheme at morphID.
	PartOfSpeech(morphID int) string

	// Reading returns the reading (katakana) of the morpheme at morphID.
	// surface, off, and len identify the surface form in the character buffer.
	// Returns empty string when no reading is available.
	Reading(morphID int, surface []rune, off, length int) string

	// BaseForm returns the dictionary (base) form of the morpheme at morphID.
	// Returns empty string when the surface form is already the base form.
	BaseForm(morphID int, surface []rune, off, length int) string

	// Pronunciation returns the pronunciation of the morpheme at morphID.
	// Returns empty string when no pronunciation data is available.
	Pronunciation(morphID int, surface []rune, off, length int) string

	// InflectionType returns the inflection type of the morpheme at morphID,
	// or empty string when not applicable.
	InflectionType(morphID int) string

	// InflectionForm returns the inflection form of the morpheme at morphID,
	// or empty string when not applicable.
	InflectionForm(morphID int) string
}
