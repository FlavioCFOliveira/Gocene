// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "strings"

// UserMorphData provides morphological information for user-dictionary entries.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.UserMorphData from Apache Lucene 10.4.0.
type UserMorphData struct {
	// data holds the feature strings indexed by wordID offset from
	// CustomDictionaryWordIDOffset.
	data []string
}

// User-dictionary fixed connection and cost constants.
const (
	UserMorphWordCost = -100000
	UserMorphLeftID   = 5
	UserMorphRightID  = 5
	// CustomDictionaryWordIDOffset is the base offset applied to all user
	// word IDs to distinguish them from system dictionary entries.
	CustomDictionaryWordIDOffset = 100000000
	// InternalSeparator is the string used to separate feature fields in stored
	// user-dictionary entries. Mirrors Java's U+0000 NUL character.
	InternalSeparator = "\x00"
)

// NewUserMorphData creates a UserMorphData backed by data.
func NewUserMorphData(data []string) *UserMorphData { return &UserMorphData{data: data} }

// LeftID returns the fixed left connection ID for user entries.
func (m *UserMorphData) LeftID(_ int) int { return UserMorphLeftID }

// RightID returns the fixed right connection ID for user entries.
func (m *UserMorphData) RightID(_ int) int { return UserMorphRightID }

// WordCost returns the fixed word cost for user entries.
func (m *UserMorphData) WordCost(_ int) int { return UserMorphWordCost }

// Reading returns the reading feature for a user-dictionary word.
func (m *UserMorphData) Reading(wordID int, _ []rune, _, _ int) string {
	return m.getFeature(wordID, 0)
}

// PartOfSpeech returns the POS feature for a user-dictionary word.
func (m *UserMorphData) PartOfSpeech(wordID int) string { return m.getFeature(wordID, 1) }

// BaseForm always returns empty string for user entries.
func (m *UserMorphData) BaseForm(_ int, _ []rune, _, _ int) string { return "" }

// Pronunciation always returns empty string for user entries.
func (m *UserMorphData) Pronunciation(_ int, _ []rune, _, _ int) string { return "" }

// InflectionType always returns empty string for user entries.
func (m *UserMorphData) InflectionType(_ int) string { return "" }

// InflectionForm always returns empty string for user entries.
func (m *UserMorphData) InflectionForm(_ int) string { return "" }

func (m *UserMorphData) getFeature(wordID, field int) string {
	idx := wordID - CustomDictionaryWordIDOffset
	if idx < 0 || idx >= len(m.data) {
		return ""
	}
	parts := strings.Split(m.data[idx], InternalSeparator)
	if field < len(parts) {
		return parts[field]
	}
	return ""
}

// Ensure UserMorphData implements JaMorphData.
var _ JaMorphData = (*UserMorphData)(nil)
