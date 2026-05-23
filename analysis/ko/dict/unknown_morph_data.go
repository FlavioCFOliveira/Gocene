// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// UnknownMorphData provides morphological information for unknown-dictionary
// entries. It extends TokenInfoMorphData but always returns nil for reading
// and morphemes.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.UnknownMorphData from Apache Lucene
// 10.4.0.
type UnknownMorphData struct {
	TokenInfoMorphData
}

// NewUnknownMorphData creates an UnknownMorphData from the given packed buffer
// and POS tag table.
func NewUnknownMorphData(buffer []byte, posDict []POSTag) *UnknownMorphData {
	return &UnknownMorphData{TokenInfoMorphData: TokenInfoMorphData{buffer: buffer, posDict: posDict}}
}

// GetReading always returns empty string for unknown words.
func (m *UnknownMorphData) GetReading(_ int) string { return "" }

// GetMorphemes always returns nil for unknown words.
func (m *UnknownMorphData) GetMorphemes(_ int, _ []rune, _, _ int) []Morpheme { return nil }

// Ensure UnknownMorphData implements KoMorphData.
var _ KoMorphData = (*UnknownMorphData)(nil)
