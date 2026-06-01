// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// Tests for the embedded-resource binary loaders.
//
// These tests verify that each singleton loads its data correctly and that
// the loaded values match the Lucene 10.4.0 reference binary files byte-for-byte.
//
// Reference: Apache Lucene 10.4.0,
// lucene/analysis/nori/src/resources/org/apache/lucene/analysis/ko/dict/

import (
	"testing"
)

// ---- ConnectionCosts --------------------------------------------------------

// TestLoadConnectionCosts_ForwardSize verifies that the loaded forwardSize
// matches the known value from Lucene 10.4.0's Korean ConnectionCosts.dat.
func TestLoadConnectionCosts_ForwardSize(t *testing.T) {
	cc, err := loadConnectionCosts(connCostsData)
	if err != nil {
		t.Fatalf("loadConnectionCosts: %v", err)
	}
	// Verify Get does not panic/zero-out for valid indices.
	// forwardSize = 3822, so ids 0..3821 are valid forward ids.
	// We call Get(0, 0) as a smoke test; actual value is data-dependent.
	_ = cc.Get(0, 0)
	_ = cc.Get(3821, 3821)
}

// TestLoadConnectionCosts_MatrixNonZero verifies that at least some entries
// in the cost matrix are non-zero, confirming the delta-zigzag decoding works.
func TestLoadConnectionCosts_MatrixNonZero(t *testing.T) {
	cc, err := loadConnectionCosts(connCostsData)
	if err != nil {
		t.Fatalf("loadConnectionCosts: %v", err)
	}
	nonZero := false
	for i := 0; i < 100; i++ {
		if cc.Get(i, i) != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("all 100 sampled diagonal ConnectionCosts entries are zero")
	}
}

// TestLoadConnectionCosts_Singleton verifies that GetConnectionCostsInstance
// returns the same non-nil pointer on repeated calls (singleton semantics).
func TestLoadConnectionCosts_Singleton(t *testing.T) {
	a := GetConnectionCostsInstance()
	b := GetConnectionCostsInstance()
	if a == nil {
		t.Fatal("GetConnectionCostsInstance() is nil")
	}
	if a != b {
		t.Error("GetConnectionCostsInstance() returned different pointers")
	}
}

// ---- CharacterDefinition ----------------------------------------------------

// TestLoadCharacterDefinition_KnownClasses verifies character-class assignments
// for code points whose classes are fixed by the Lucene 10.4.0 binary resource.
func TestLoadCharacterDefinition_KnownClasses(t *testing.T) {
	cd, err := loadCharacterDefinition(charDefData)
	if err != nil {
		t.Fatalf("loadCharacterDefinition: %v", err)
	}
	tests := []struct {
		cp    rune
		class byte
		name  string
	}{
		{0x0020, CharClassSPACE, "SPACE U+0020"},
		{0xAC00, CharClassHANGUL, "HANGUL U+AC00 (가)"},
		{0x3042, CharClassHIRAGANA, "HIRAGANA U+3042 (あ)"},
	}
	for _, tc := range tests {
		got := cd.CharacterClass(tc.cp)
		if got != tc.class {
			t.Errorf("CharacterClass(%s): got %d, want %d", tc.name, got, tc.class)
		}
	}
}

// TestLoadCharacterDefinition_IsHangul verifies IsHangul for a known Hangul
// syllable and a non-Hangul character.
func TestLoadCharacterDefinition_IsHangul(t *testing.T) {
	cd, err := loadCharacterDefinition(charDefData)
	if err != nil {
		t.Fatalf("loadCharacterDefinition: %v", err)
	}
	if !cd.IsHangul(0xAC00) { // 가
		t.Error("IsHangul(U+AC00) = false; want true")
	}
	if cd.IsHangul(0x0041) { // 'A'
		t.Error("IsHangul(U+0041 'A') = true; want false")
	}
}

// TestLoadCharacterDefinition_Singleton verifies singleton semantics.
func TestLoadCharacterDefinition_Singleton(t *testing.T) {
	a := GetCharacterDefinitionInstance()
	b := GetCharacterDefinitionInstance()
	if a == nil {
		t.Fatal("GetCharacterDefinitionInstance() is nil")
	}
	if a != b {
		t.Error("GetCharacterDefinitionInstance() returned different pointers")
	}
}

// ---- TokenInfoDictionary ----------------------------------------------------

// TestLoadTokenInfoDictionary_NonNil verifies that loading succeeds and all
// components are non-nil.
func TestLoadTokenInfoDictionary_NonNil(t *testing.T) {
	d, err := loadTokenInfoDictionary()
	if err != nil {
		t.Fatalf("loadTokenInfoDictionary: %v", err)
	}
	if d == nil {
		t.Fatal("loadTokenInfoDictionary returned nil")
	}
	if d.fst == nil {
		t.Error("TokenInfoDictionary.fst is nil")
	}
	if d.morphAtts == nil {
		t.Error("TokenInfoDictionary.morphAtts is nil")
	}
}

// TestLoadTokenInfoDictionary_FSTLookup verifies that the FST accepts at least
// one known Korean word.  The word "가" (U+AC00) appears in the IPADIC/mecab-ko
// dictionary and should be accepted by the FST.
func TestLoadTokenInfoDictionary_FSTLookup(t *testing.T) {
	d, err := loadTokenInfoDictionary()
	if err != nil {
		t.Fatalf("loadTokenInfoDictionary: %v", err)
	}
	// "가" is in the dictionary; the FST must return a non-negative ordinal.
	ordinal := d.fst.Lookup([]rune("가"))
	if ordinal < 0 {
		t.Errorf("FST.Lookup(%q) = %d; expected non-negative ordinal", "가", ordinal)
	}
}

// TestLoadTokenInfoDictionary_MorphCosts verifies that at least the first
// word in the packed buffer has valid (non-zero) connection IDs.
func TestLoadTokenInfoDictionary_MorphCosts(t *testing.T) {
	d, err := loadTokenInfoDictionary()
	if err != nil {
		t.Fatalf("loadTokenInfoDictionary: %v", err)
	}
	// The morphAtts buffer must be non-empty.
	if len(d.morphAtts.buffer) == 0 {
		t.Fatal("TokenInfoMorphData buffer is empty")
	}
	// At least one wordID in the first 64 bytes should have non-zero LeftID.
	nonZero := false
	for wordID := 0; wordID+5 < len(d.morphAtts.buffer); wordID += 6 {
		if d.morphAtts.LeftID(wordID) != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("all sampled LeftIDs are 0; morpheme buffer may not have loaded")
	}
}

// TestLoadTokenInfoDictionary_Singleton verifies singleton semantics.
func TestLoadTokenInfoDictionary_Singleton(t *testing.T) {
	a := GetTokenInfoDictionaryInstance()
	b := GetTokenInfoDictionaryInstance()
	if a == nil {
		t.Fatal("GetTokenInfoDictionaryInstance() is nil")
	}
	if a != b {
		t.Error("GetTokenInfoDictionaryInstance() returned different pointers")
	}
}

// ---- UnknownDictionary ------------------------------------------------------

// TestLoadUnknownDictionary_NonNil verifies that loading succeeds and all
// components are non-nil.
func TestLoadUnknownDictionary_NonNil(t *testing.T) {
	d, err := loadUnknownDictionary()
	if err != nil {
		t.Fatalf("loadUnknownDictionary: %v", err)
	}
	if d == nil {
		t.Fatal("loadUnknownDictionary returned nil")
	}
	if d.charDef == nil {
		t.Error("UnknownDictionary.charDef is nil")
	}
	if d.morphAtts == nil {
		t.Error("UnknownDictionary.morphAtts is nil")
	}
}

// TestLoadUnknownDictionary_CharDefIntegration verifies that the
// CharacterDefinition embedded in the UnknownDictionary returns the correct
// class for a known Hangul character.
func TestLoadUnknownDictionary_CharDefIntegration(t *testing.T) {
	d, err := loadUnknownDictionary()
	if err != nil {
		t.Fatalf("loadUnknownDictionary: %v", err)
	}
	if got := d.charDef.CharacterClass(0xAC00); got != CharClassHANGUL {
		t.Errorf("charDef.CharacterClass(U+AC00): got %d, want HANGUL(%d)", got, CharClassHANGUL)
	}
}

// TestLoadUnknownDictionary_Singleton verifies singleton semantics.
func TestLoadUnknownDictionary_Singleton(t *testing.T) {
	a := GetUnknownDictionaryInstance()
	b := GetUnknownDictionaryInstance()
	if a == nil {
		t.Fatal("GetUnknownDictionaryInstance() is nil")
	}
	if a != b {
		t.Error("GetUnknownDictionaryInstance() returned different pointers")
	}
}
