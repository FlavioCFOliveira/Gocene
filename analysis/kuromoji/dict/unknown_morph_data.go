// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// UnknownMorphData provides morphological information for unknown-word
// dictionary entries. Reading, inflection type, and inflection form are always
// nil for unknown words.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.UnknownMorphData from Apache Lucene
// 10.4.0.
type UnknownMorphData struct {
	TokenInfoMorphData
}

// NewUnknownMorphData constructs an UnknownMorphData from packed binary data
// and POS tables.
func NewUnknownMorphData(buffer []byte, posDict, inflTypeDict, inflFormDict []string) *UnknownMorphData {
	return &UnknownMorphData{
		TokenInfoMorphData: TokenInfoMorphData{
			buffer:       buffer,
			posDict:      posDict,
			inflTypeDict: inflTypeDict,
			inflFormDict: inflFormDict,
		},
	}
}

// Reading always returns empty string for unknown words.
func (m *UnknownMorphData) Reading(_ int, _ []rune, _, _ int) string { return "" }

// InflectionType always returns empty string for unknown words.
func (m *UnknownMorphData) InflectionType(_ int) string { return "" }

// InflectionForm always returns empty string for unknown words.
func (m *UnknownMorphData) InflectionForm(_ int) string { return "" }

// Ensure UnknownMorphData implements JaMorphData.
var _ JaMorphData = (*UnknownMorphData)(nil)
