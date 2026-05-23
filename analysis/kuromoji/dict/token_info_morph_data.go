// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"encoding/binary"
)

// Binary entry flag constants used to encode optional morphological fields.
//
// These are the Go ports of the corresponding public static int constants in
// org.apache.lucene.analysis.ja.dict.TokenInfoMorphData from Apache Lucene
// 10.4.0.
const (
	// HasBaseform indicates the entry has base-form data.
	HasBaseform = 1
	// HasReading indicates the entry has reading data.
	HasReading = 2
	// HasPronunciation indicates the entry has pronunciation data.
	HasPronunciation = 4
)

// TokenInfoMorphData provides morphological information for system-dictionary
// entries. Entries are packed into a contiguous byte slice; field offsets are
// computed from a per-entry header short.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.TokenInfoMorphData from Apache Lucene
// 10.4.0.
type TokenInfoMorphData struct {
	// buffer holds the packed binary dictionary data.
	buffer []byte
	// posDict maps left-ID indices to POS strings.
	posDict []string
	// inflTypeDict maps left-ID indices to inflection-type strings.
	inflTypeDict []string
	// inflFormDict maps left-ID indices to inflection-form strings.
	inflFormDict []string
}

// NewTokenInfoMorphData constructs a TokenInfoMorphData from the given packed
// buffer and POS tables.
func NewTokenInfoMorphData(buffer []byte, posDict, inflTypeDict, inflFormDict []string) *TokenInfoMorphData {
	return &TokenInfoMorphData{
		buffer:       buffer,
		posDict:      posDict,
		inflTypeDict: inflTypeDict,
		inflFormDict: inflFormDict,
	}
}

// headerShort returns the flags/ID short at the start of entry wordID.
func (m *TokenInfoMorphData) headerShort(wordID int) uint16 {
	if wordID+1 >= len(m.buffer) {
		return 0
	}
	return binary.BigEndian.Uint16(m.buffer[wordID:])
}

// LeftID returns the left connection ID of entry wordID.
func (m *TokenInfoMorphData) LeftID(wordID int) int {
	return int(m.headerShort(wordID)&0xffff) >> 3
}

// RightID returns the right connection ID of entry wordID.
func (m *TokenInfoMorphData) RightID(wordID int) int {
	return int(m.headerShort(wordID)&0xffff) >> 3
}

// WordCost returns the word cost of entry wordID.
func (m *TokenInfoMorphData) WordCost(wordID int) int {
	if wordID+3 >= len(m.buffer) {
		return 0
	}
	return int(int16(binary.BigEndian.Uint16(m.buffer[wordID+2:])))
}

// PartOfSpeech returns the POS string for entry wordID.
func (m *TokenInfoMorphData) PartOfSpeech(wordID int) string {
	idx := m.LeftID(wordID)
	if idx < len(m.posDict) {
		return m.posDict[idx]
	}
	return ""
}

// InflectionType returns the inflection-type string for entry wordID.
func (m *TokenInfoMorphData) InflectionType(wordID int) string {
	idx := m.LeftID(wordID)
	if idx < len(m.inflTypeDict) {
		return m.inflTypeDict[idx]
	}
	return ""
}

// InflectionForm returns the inflection-form string for entry wordID.
func (m *TokenInfoMorphData) InflectionForm(wordID int) string {
	idx := m.LeftID(wordID)
	if idx < len(m.inflFormDict) {
		return m.inflFormDict[idx]
	}
	return ""
}

// BaseForm returns the base form for entry wordID.
func (m *TokenInfoMorphData) BaseForm(wordID int, surface []rune, off, length int) string {
	if !m.hasBaseFormData(wordID) {
		return ""
	}
	offset := baseFormOffset(wordID)
	if offset >= len(m.buffer) {
		return ""
	}
	data := int(m.buffer[offset])
	offset++
	prefix := data >> 4
	suffix := data & 0xF
	text := make([]rune, prefix+suffix)
	copy(text, surface[off:off+prefix])
	for i := 0; i < suffix; i++ {
		pos := offset + (i << 1)
		if pos+1 < len(m.buffer) {
			text[prefix+i] = rune(binary.BigEndian.Uint16(m.buffer[pos:]))
		}
	}
	return string(text)
}

// Reading returns the reading (katakana) for entry wordID.
func (m *TokenInfoMorphData) Reading(wordID int, surface []rune, off, length int) string {
	if m.hasReadingData(wordID) {
		offset := m.readingOffset(wordID)
		if offset >= len(m.buffer) {
			return ""
		}
		readingData := int(m.buffer[offset])
		offset++
		return m.readString(offset, readingData>>1, (readingData&1) == 1)
	}
	// synthesise: shift hiragana to katakana
	text := make([]rune, length)
	for i := 0; i < length; i++ {
		ch := surface[off+i]
		if ch > 0x3040 && ch < 0x3097 {
			text[i] = ch + 0x60
		} else {
			text[i] = ch
		}
	}
	return string(text)
}

// Pronunciation returns the pronunciation for entry wordID.
func (m *TokenInfoMorphData) Pronunciation(wordID int, surface []rune, off, length int) string {
	if m.hasPronunciationData(wordID) {
		offset := m.pronunciationOffset(wordID)
		if offset >= len(m.buffer) {
			return ""
		}
		pronunciationData := int(m.buffer[offset])
		offset++
		return m.readString(offset, pronunciationData>>1, (pronunciationData&1) == 1)
	}
	return m.Reading(wordID, surface, off, length)
}

func baseFormOffset(wordID int) int { return wordID + 4 }

func (m *TokenInfoMorphData) hasBaseFormData(wordID int) bool {
	return m.headerShort(wordID)&HasBaseform != 0
}

func (m *TokenInfoMorphData) hasReadingData(wordID int) bool {
	return m.headerShort(wordID)&HasReading != 0
}

func (m *TokenInfoMorphData) hasPronunciationData(wordID int) bool {
	return m.headerShort(wordID)&HasPronunciation != 0
}

func (m *TokenInfoMorphData) readingOffset(wordID int) int {
	offset := baseFormOffset(wordID)
	if m.hasBaseFormData(wordID) && offset < len(m.buffer) {
		baseFormLength := int(m.buffer[offset]) & 0xF
		offset++
		offset += baseFormLength << 1
	}
	return offset
}

func (m *TokenInfoMorphData) pronunciationOffset(wordID int) int {
	if !m.hasReadingData(wordID) {
		return m.readingOffset(wordID)
	}
	offset := m.readingOffset(wordID)
	if offset >= len(m.buffer) {
		return offset
	}
	readingData := int(m.buffer[offset])
	offset++
	var readingLength int
	if (readingData & 1) == 0 {
		readingLength = readingData & 0xFE
	} else {
		readingLength = readingData >> 1
	}
	return offset + readingLength
}

func (m *TokenInfoMorphData) readString(offset, length int, kana bool) string {
	text := make([]rune, length)
	if kana {
		for i := 0; i < length; i++ {
			if offset+i < len(m.buffer) {
				text[i] = rune(0x30A0 + int(m.buffer[offset+i]&0xff))
			}
		}
	} else {
		for i := 0; i < length; i++ {
			pos := offset + (i << 1)
			if pos+1 < len(m.buffer) {
				text[i] = rune(binary.BigEndian.Uint16(m.buffer[pos:]))
			}
		}
	}
	return string(text)
}

// Ensure TokenInfoMorphData implements JaMorphData.
var _ JaMorphData = (*TokenInfoMorphData)(nil)
