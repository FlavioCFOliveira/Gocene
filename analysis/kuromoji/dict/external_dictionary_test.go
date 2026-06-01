// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict_test

// TestExternalDictionary is the Go port of
// org.apache.lucene.analysis.ja.dict.TestExternalDictionary from Apache
// Lucene 10.4.0.
//
// The Java test builds a minimal IPADIC dictionary from CSV/DEF source files
// using DictionaryBuilder, then loads the resulting binary files into
// TokenInfoDictionary, UnknownDictionary, and ConnectionCosts, asserting
// that the FST and character definition are non-nil and that a specific
// connection cost is correct.
//
// Deviation: DictionaryBuilder.Build (full CSV-to-binary compilation) and
// binary file loading constructors for TokenInfoDictionary/UnknownDictionary/
// ConnectionCosts are deferred to the codec sprint.  All sub-tests that
// exercise those paths are skipped until the codec sprint lands.  The tests
// below validate the data structures that are already available.

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
)

// TestExternalDictionary_ConnectionCostsBuilder verifies that
// ConnectionCostsBuilder can parse a minimal matrix.def and returns a
// matrix with the expected cost.
func TestExternalDictionary_ConnectionCostsBuilder(t *testing.T) {
	// Minimal matrix.def matching the Java test fixture:
	//   3 3
	//   0 1 1
	//   0 2 -1630
	matrixDef := "3 3\n0 1 1\n0 2 -1630\n"
	cc, err := dict.ConnectionCostsBuilder{}.Build(strings.NewReader(matrixDef))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if cc == nil {
		t.Fatal("ConnectionCosts is nil")
	}
	// Java test asserts cc.get(0, 1) == 1
	got := cc.Get(0, 1)
	if got != 1 {
		t.Errorf("cc.Get(0,1) = %d, want 1", got)
	}
}

// TestExternalDictionary_TokenInfoDictionaryEntryWriter verifies that
// TokenInfoDictionaryEntryWriter can encode a minimal IPADIC CSV entry and
// reports the expected byte offset.
func TestExternalDictionary_TokenInfoDictionaryEntryWriter(t *testing.T) {
	// Minimal entry: 白昼夢 (noun, leftID=1285, rightID=1285, cost=5622)
	entry := []string{
		"白昼夢",        // 0 surface
		"1285",       // 1 leftID
		"1285",       // 2 rightID
		"5622",       // 3 word cost
		"名詞",         // 4 pos1
		"一般",         // 5 pos2
		"*",          // 6 pos3
		"*",          // 7 pos4
		"*",          // 8 inflType
		"*",          // 9 inflForm
		"白昼夢",        // 10 baseForm (same as surface → no HasBaseform flag)
		"ハクチュウム",     // 11 reading
		"ハクチューム",     // 12 pronunciation (differs from reading → HasPronunciation)
	}
	w := dict.NewTokenInfoDictionaryEntryWriter(4096)
	wordID, err := w.PutEntry(entry)
	if err != nil {
		t.Fatalf("PutEntry: %v", err)
	}
	if wordID != 0 {
		t.Errorf("first wordID = %d, want 0", wordID)
	}
	if w.CurrentPosition() == 0 {
		t.Error("buffer is empty after PutEntry")
	}
}

// TestExternalDictionary_UserDictionaryOpen verifies that UserDictionary
// can parse a minimal CSV user dictionary.
func TestExternalDictionary_UserDictionaryOpen(t *testing.T) {
	csv := "日本経済新聞,日本 経済 新聞,ニホン ケイザイ シンブン,カスタム名詞\n"
	ud, err := dict.Open(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if ud == nil {
		t.Fatal("UserDictionary is nil")
	}
}

// TestExternalDictionary_LoadTokenInfoDictionary verifies that
// GetTokenInfoDictionaryInstance loads a non-nil dictionary from the embedded
// binary resources and that the FST and morphological attributes are populated.
func TestExternalDictionary_LoadTokenInfoDictionary(t *testing.T) {
	d := dict.GetTokenInfoDictionaryInstance()
	if d == nil {
		t.Fatal("GetTokenInfoDictionaryInstance() returned nil")
	}
	if d.GetRealFST() == nil {
		t.Fatal("TokenInfoDictionary.GetRealFST() is nil; FST was not loaded")
	}
	if d.GetMorphAttributes() == nil {
		t.Fatal("TokenInfoDictionary.GetMorphAttributes() is nil")
	}
}

// TestExternalDictionary_LoadUnknownDictionary verifies that
// GetUnknownDictionaryInstance loads a non-nil dictionary from the embedded
// binary resources.
func TestExternalDictionary_LoadUnknownDictionary(t *testing.T) {
	d := dict.GetUnknownDictionaryInstance()
	if d == nil {
		t.Fatal("GetUnknownDictionaryInstance() returned nil")
	}
	if d.GetCharacterDefinition() == nil {
		t.Fatal("UnknownDictionary.GetCharacterDefinition() is nil")
	}
}

// TestExternalDictionary_LoadConnectionCostsFromFile verifies that
// GetConnectionCostsInstance loads a non-zero ConnectionCosts from the embedded
// binary resource and that at least one entry in the cost matrix is non-zero
// (a matrix of all zeros would indicate the data was not read).
func TestExternalDictionary_LoadConnectionCostsFromFile(t *testing.T) {
	cc := dict.GetConnectionCostsInstance()
	if cc == nil {
		t.Fatal("GetConnectionCostsInstance() returned nil")
	}
	// Probe a broad diagonal to verify at least one non-zero cost.
	nonZero := false
	for i := 0; i < 100; i++ {
		if cc.Get(i, i) != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("all sampled ConnectionCosts entries are zero; binary data may not have loaded")
	}
}
