// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// userWordCost is the fixed word cost for user-dictionary entries.
const userWordCost = -100000

// userLeftID is the fixed left connection ID (NNG left).
const userLeftID = 1781

// UserMorphData provides morphological information for user-dictionary entries.
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.UserMorphData from
// Apache Lucene 10.4.0.
type UserMorphData struct {
	// segmentations holds per-entry segment lengths, or nil for simple nouns.
	segmentations [][]int
	// rightIDs holds the right connection ID for each entry.
	rightIDs []int16
}

// NewUserMorphData creates a UserMorphData from the given segmentation table
// and right-ID table.
func NewUserMorphData(segmentations [][]int, rightIDs []int16) *UserMorphData {
	return &UserMorphData{segmentations: segmentations, rightIDs: rightIDs}
}

// LeftID returns the fixed NNG left connection ID.
func (m *UserMorphData) LeftID(_ int) int { return userLeftID }

// RightID returns the right connection ID for the given morphID.
func (m *UserMorphData) RightID(morphID int) int {
	if morphID < len(m.rightIDs) {
		return int(m.rightIDs[morphID])
	}
	return 0
}

// WordCost returns the fixed user-dictionary word cost.
func (m *UserMorphData) WordCost(_ int) int { return userWordCost }

// GetPOSType returns MORPHEME for simple nouns and COMPOUND for segmented
// entries.
func (m *UserMorphData) GetPOSType(morphID int) POSType {
	if morphID < len(m.segmentations) && m.segmentations[morphID] != nil {
		return POSTypeCompound
	}
	return POSTypeMorpheme
}

// GetLeftPOS always returns NNG for user entries.
func (m *UserMorphData) GetLeftPOS(_ int) POSTag { return POSTagNNG }

// GetRightPOS always returns NNG for user entries.
func (m *UserMorphData) GetRightPOS(_ int) POSTag { return POSTagNNG }

// GetReading always returns empty string for user entries.
func (m *UserMorphData) GetReading(_ int) string { return "" }

// GetMorphemes returns the morpheme decomposition for compound user entries,
// or nil for simple entries.
func (m *UserMorphData) GetMorphemes(morphID int, surfaceForm []rune, off, length int) []Morpheme {
	if morphID >= len(m.segmentations) || m.segmentations[morphID] == nil {
		return nil
	}
	segs := m.segmentations[morphID]
	morphemes := make([]Morpheme, len(segs))
	offset := 0
	for i, segLen := range segs {
		end := off + offset + segLen
		var form string
		if end <= len(surfaceForm) {
			form = string(surfaceForm[off+offset : end])
		}
		morphemes[i] = Morpheme{PosTag: POSTagNNG, SurfaceForm: form}
		offset += segLen
	}
	return morphemes
}

// Ensure UserMorphData implements KoMorphData.
var _ KoMorphData = (*UserMorphData)(nil)
