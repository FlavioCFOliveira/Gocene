// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"strings"
	"testing"
)

// TestExternalDictionary is the Go port of
// org.apache.lucene.analysis.ko.dict.TestExternalDictionary from Apache Lucene
// 10.4.0.
//
// Deviation: the Java test builds full binary dictionaries via
// DictionaryBuilder.build() and loads them from codec-formatted binary files.
// Binary serialisation is deferred to the nori codec sprint, so this Go port
// exercises the same structural contracts using the in-memory builder API and
// validates the observable Go contract (zero-value singletons, builder
// construction, ConnectionCostsBuilder round-trip).
func TestExternalDictionary_ConnectionCostsBuilder(t *testing.T) {
	// matrix.def content as used in the Java setUp.
	matrixDef := "3 3\n1 1 0\n1 2 -100\n"
	costs, err := ConnectionCostsBuilder{}.Build(strings.NewReader(matrixDef))
	if err != nil {
		t.Fatalf("ConnectionCostsBuilder.Build: %v", err)
	}
	if got := costs.Get(1, 1); got != 0 {
		t.Errorf("Get(1,1) = %d; want 0", got)
	}
	if got := costs.Get(1, 2); got != -100 {
		t.Errorf("Get(1,2) = %d; want -100", got)
	}
}

func TestExternalDictionary_TokenInfoDictionarySingleton(t *testing.T) {
	// Verify the singleton does not panic and returns a non-nil instance.
	d := GetTokenInfoDictionaryInstance()
	if d == nil {
		t.Fatal("GetTokenInfoDictionaryInstance() returned nil")
	}
}

func TestExternalDictionary_UnknownDictionarySingleton(t *testing.T) {
	d := GetUnknownDictionaryInstance()
	if d == nil {
		t.Fatal("GetUnknownDictionaryInstance() returned nil")
	}
}

func TestExternalDictionary_ConnectionCostsSingleton(t *testing.T) {
	cc := GetConnectionCostsInstance()
	if cc == nil {
		t.Fatal("GetConnectionCostsInstance() returned nil")
	}
}

func TestExternalDictionary_DictionaryBuilder(t *testing.T) {
	b := NewDictionaryBuilder("utf-8", true)
	if b.Encoding() != "utf-8" {
		t.Errorf("Encoding() = %q; want %q", b.Encoding(), "utf-8")
	}
	if !b.NormalizeEntry() {
		t.Error("NormalizeEntry() = false; want true")
	}
}

func TestExternalDictionary_TokenInfoDictionaryEntryWriter(t *testing.T) {
	// Port of the Java setUp CSV lines for the noun.csv entries.
	// mecab-ko-dic layout: surface,leftID,rightID,wordCost,pos,semanticClass,coda,reading,posType,leftPOS,rightPOS,expression
	w := NewTokenInfoDictionaryEntryWriter(1024)

	entry := []string{"명사", "1", "1", "2", "NNG", "*", "*", "*", "*", "*", "*", "*"}
	wordID, err := w.PutEntry(entry)
	if err != nil {
		t.Fatalf("PutEntry(명사): %v", err)
	}
	if wordID < 0 {
		t.Errorf("wordID = %d; want ≥ 0", wordID)
	}

	entry2 := []string{"일반", "5000", "5000", "3", "NNG", "*", "*", "*", "*", "*", "*", "*"}
	wordID2, err := w.PutEntry(entry2)
	if err != nil {
		t.Fatalf("PutEntry(일반): %v", err)
	}
	if wordID2 <= wordID {
		t.Errorf("wordID2 (%d) should be > wordID (%d)", wordID2, wordID)
	}

	// Verify POSTag table is built correctly.
	posTable := w.BuildPOSTagTable()
	if len(posTable) == 0 {
		t.Error("BuildPOSTagTable() returned empty table")
	}
	// leftID=1 should map to NNG.
	if len(posTable) > 1 && posTable[1] != POSTagNNG {
		t.Errorf("posTable[1] = %v; want NNG", posTable[1])
	}
}

func TestExternalDictionary_UserDictionary(t *testing.T) {
	// Valid single-entry user dictionary.
	const input = "서울역 서울 역\n"
	ud, err := Open(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if ud == nil {
		t.Fatal("Open returned nil UserDictionary")
	}
	surface := []rune("서울역")
	ids := ud.Lookup(surface, 0, len(surface))
	if len(ids) == 0 {
		t.Error("Lookup(서울역) returned no IDs; want ≥1")
	}
}
