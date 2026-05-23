// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "encoding/binary"

// Binary entry flag constants used to encode optional morphological fields.
//
// These are the Go ports of the corresponding constants in
// org.apache.lucene.analysis.ko.dict.TokenInfoMorphData from Apache Lucene
// 10.4.0.
const (
	// HasSinglePOS indicates that the entry has a single part of speech
	// (leftPOS equals rightPOS).
	HasSinglePOS = 1
	// HasReading indicates the entry has reading data.
	HasReading = 2
)

// TokenInfoMorphData provides morphological information for system-dictionary
// entries. Entries are packed into a contiguous byte slice.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.TokenInfoMorphData from Apache Lucene
// 10.4.0.
type TokenInfoMorphData struct {
	// buffer holds the packed binary dictionary data (big-endian shorts/chars).
	buffer []byte
	// posDict maps left-ID indices to POSTag values.
	posDict []POSTag
}

// NewTokenInfoMorphData creates a TokenInfoMorphData from the given packed
// buffer and POS tag table.
func NewTokenInfoMorphData(buffer []byte, posDict []POSTag) *TokenInfoMorphData {
	return &TokenInfoMorphData{buffer: buffer, posDict: posDict}
}

func (m *TokenInfoMorphData) getShort(offset int) int16 {
	if offset+1 >= len(m.buffer) {
		return 0
	}
	return int16(binary.BigEndian.Uint16(m.buffer[offset:]))
}

func (m *TokenInfoMorphData) getUShort(offset int) uint16 {
	if offset+1 >= len(m.buffer) {
		return 0
	}
	return binary.BigEndian.Uint16(m.buffer[offset:])
}

// LeftID returns the left connection ID for the entry at morphID.
func (m *TokenInfoMorphData) LeftID(morphID int) int {
	return int(m.getUShort(morphID)) >> 2
}

// RightID returns the right connection ID for the entry at morphID.
func (m *TokenInfoMorphData) RightID(morphID int) int {
	return int(m.getUShort(morphID+2)) >> 2
}

// WordCost returns the word cost for the entry at morphID.
func (m *TokenInfoMorphData) WordCost(morphID int) int {
	return int(m.getShort(morphID + 4))
}

// GetPOSType returns the POSType encoded in the lower 2 bits of the first
// short.
func (m *TokenInfoMorphData) GetPOSType(morphID int) POSType {
	value := byte(m.getUShort(morphID) & 3)
	return ResolveTypeByByte(value)
}

// GetLeftPOS returns the left POSTag for morphID.
func (m *TokenInfoMorphData) GetLeftPOS(morphID int) POSTag {
	id := m.LeftID(morphID)
	if id < len(m.posDict) {
		return m.posDict[id]
	}
	return POSTagUNKNOWN
}

// GetRightPOS returns the right POSTag for morphID.
func (m *TokenInfoMorphData) GetRightPOS(morphID int) POSTag {
	posType := m.GetPOSType(morphID)
	if posType == POSTypeMorpheme || posType == POSTypeCompound || m.hasSinglePOS(morphID) {
		return m.GetLeftPOS(morphID)
	}
	if morphID+6 < len(m.buffer) {
		return ResolveTagByByte(m.buffer[morphID+6])
	}
	return POSTagUNKNOWN
}

// GetReading returns the reading string for morphID, or empty string if none.
func (m *TokenInfoMorphData) GetReading(morphID int) string {
	if m.hasReadingData(morphID) {
		return m.readString(morphID + 6)
	}
	return ""
}

// GetMorphemes returns the morpheme decomposition for morphID.
func (m *TokenInfoMorphData) GetMorphemes(morphID int, surfaceForm []rune, off, length int) []Morpheme {
	posType := m.GetPOSType(morphID)
	if posType == POSTypeMorpheme {
		return nil
	}
	offset := morphID + 6
	hasSinglePos := m.hasSinglePOS(morphID)
	if !hasSinglePos {
		offset++ // skip rightPOS byte
	}
	if offset >= len(m.buffer) {
		return nil
	}
	count := int(m.buffer[offset])
	offset++
	if count == 0 {
		return nil
	}
	morphemes := make([]Morpheme, count)
	surfaceOffset := 0
	leftPOS := m.GetLeftPOS(morphID)
	for i := 0; i < count; i++ {
		var tag POSTag
		if hasSinglePos {
			tag = leftPOS
		} else {
			if offset >= len(m.buffer) {
				break
			}
			tag = ResolveTagByByte(m.buffer[offset])
			offset++
		}
		var form string
		if posType == POSTypeInflect {
			form = m.readString(offset)
			offset += len([]rune(form))*2 + 1
		} else {
			if offset >= len(m.buffer) {
				break
			}
			formLen := int(m.buffer[offset])
			offset++
			end := off + surfaceOffset + formLen
			if end <= len(surfaceForm) {
				form = string(surfaceForm[off+surfaceOffset : end])
			}
			surfaceOffset += formLen
		}
		morphemes[i] = Morpheme{PosTag: tag, SurfaceForm: form}
	}
	return morphemes
}

func (m *TokenInfoMorphData) hasSinglePOS(morphID int) bool {
	return m.getUShort(morphID+2)&HasSinglePOS != 0
}

func (m *TokenInfoMorphData) hasReadingData(morphID int) bool {
	return m.getUShort(morphID+2)&HasReading != 0
}

func (m *TokenInfoMorphData) readString(offset int) string {
	if offset >= len(m.buffer) {
		return ""
	}
	strLen := int(m.buffer[offset])
	offset++
	if strLen == 0 {
		return ""
	}
	chars := make([]rune, strLen)
	for i := 0; i < strLen; i++ {
		pos := offset + i*2
		if pos+1 < len(m.buffer) {
			chars[i] = rune(binary.BigEndian.Uint16(m.buffer[pos:]))
		}
	}
	return string(chars)
}

// Ensure TokenInfoMorphData implements KoMorphData.
var _ KoMorphData = (*TokenInfoMorphData)(nil)
