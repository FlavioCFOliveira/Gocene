// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "github.com/FlavioCFOliveira/Gocene/analysis/morph"

// KoMorphData represents Korean morphological information stored in a
// dictionary entry.
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.KoMorphData from
// Apache Lucene 10.4.0.
type KoMorphData interface {
	morph.MorphData

	// GetPOSType returns the POSType of the specified word (morpheme,
	// compound, inflect or pre-analysis).
	GetPOSType(morphID int) POSType

	// GetLeftPOS returns the left POSTag of the specified word.
	GetLeftPOS(morphID int) POSTag

	// GetRightPOS returns the right POSTag of the specified word.
	GetRightPOS(morphID int) POSTag

	// GetReading returns the reading of the specified word (mainly used for
	// Hanja to Hangul conversion). Returns empty string when not available.
	GetReading(morphID int) string

	// GetMorphemes returns the morpheme decomposition of the specified word.
	// Returns nil for simple morphemes.
	GetMorphemes(morphID int, surfaceForm []rune, off, length int) []Morpheme
}
