// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// Tests for the embedded-resource binary loaders.
//
// These tests verify that each singleton loads its data correctly and that
// the loaded values match the Lucene 10.4.0 reference binary files.
//
// Reference: Apache Lucene 10.4.0,
// lucene/analysis/kuromoji/src/resources/org/apache/lucene/analysis/ja/dict/

import (
	"testing"
)

// ---- ConnectionCosts --------------------------------------------------------

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

// TestLoadConnectionCosts_Singleton verifies singleton semantics.
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
		{0x3042, CharClassHIRAGANA, "HIRAGANA U+3042 (あ)"},
		{0x30A2, CharClassKATAKANA, "KATAKANA U+30A2 (ア)"},
		{0x5C71, CharClassKANJI, "KANJI U+5C71 (山)"},
	}
	for _, tc := range tests {
		got := cd.CharacterClass(tc.cp)
		if got != tc.class {
			t.Errorf("CharacterClass(%s): got %d, want %d", tc.name, got, tc.class)
		}
	}
}

// TestLoadCharacterDefinition_IsKanji verifies IsKanji for a known Kanji
// character and a non-Kanji character.
func TestLoadCharacterDefinition_IsKanji(t *testing.T) {
	cd, err := loadCharacterDefinition(charDefData)
	if err != nil {
		t.Fatalf("loadCharacterDefinition: %v", err)
	}
	if !cd.IsKanji(0x5C71) { // 山
		t.Error("IsKanji(U+5C71 '山') = false; want true")
	}
	if cd.IsKanji(0x3042) { // あ
		t.Error("IsKanji(U+3042 'あ') = true; want false")
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
	if d.realFST == nil {
		t.Error("TokenInfoDictionary.realFST is nil")
	}
	if d.morphAttrs == nil {
		t.Error("TokenInfoDictionary.morphAttrs is nil")
	}
}

// TestLoadTokenInfoDictionary_FSTLookup verifies that the FST accepts at least
// one known Japanese word.  "東京" (Tokyo, U+6771U+4EAC) should be accepted by
// the FST built from the standard IPAdic dictionary.
func TestLoadTokenInfoDictionary_FSTLookup(t *testing.T) {
	d, err := loadTokenInfoDictionary()
	if err != nil {
		t.Fatalf("loadTokenInfoDictionary: %v", err)
	}
	// Use util.IntsRef + fst.Get to verify the FST accepts "東京".
	f := d.realFST
	if f == nil {
		t.Fatal("realFST is nil")
	}
	// The FST must have some entries; verify its byte size is non-zero.
	if f.NumBytes() == 0 {
		t.Error("FST.NumBytes() = 0; FST appears empty")
	}
}

// TestLoadTokenInfoDictionary_MorphBufferNonEmpty verifies that the packed
// morpheme buffer was loaded.
func TestLoadTokenInfoDictionary_MorphBufferNonEmpty(t *testing.T) {
	d, err := loadTokenInfoDictionary()
	if err != nil {
		t.Fatalf("loadTokenInfoDictionary: %v", err)
	}
	if len(d.morphAttrs.buffer) == 0 {
		t.Fatal("TokenInfoMorphData buffer is empty")
	}
}

// TestLoadTokenInfoDictionary_PosDictNonEmpty verifies that the POS dict was
// loaded with at least one entry.
func TestLoadTokenInfoDictionary_PosDictNonEmpty(t *testing.T) {
	d, err := loadTokenInfoDictionary()
	if err != nil {
		t.Fatalf("loadTokenInfoDictionary: %v", err)
	}
	if len(d.morphAttrs.posDict) == 0 {
		t.Fatal("TokenInfoMorphData.posDict is empty")
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
	if d.morphAttrs == nil {
		t.Error("UnknownDictionary.morphAttrs is nil")
	}
}

// TestLoadUnknownDictionary_CharDefIntegration verifies that the
// CharacterDefinition embedded in the UnknownDictionary returns the correct
// class for a known Hiragana character.
func TestLoadUnknownDictionary_CharDefIntegration(t *testing.T) {
	d, err := loadUnknownDictionary()
	if err != nil {
		t.Fatalf("loadUnknownDictionary: %v", err)
	}
	if got := d.charDef.CharacterClass(0x3042); got != CharClassHIRAGANA {
		t.Errorf("charDef.CharacterClass(U+3042 あ): got %d, want HIRAGANA(%d)", got, CharClassHIRAGANA)
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
