// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestNewFieldInfos(t *testing.T) {
	fis := NewFieldInfos()

	if fis.Size() != 0 {
		t.Errorf("Expected Size=0, got %d", fis.Size())
	}
	if fis.IsFrozen() {
		t.Error("New FieldInfos should not be frozen")
	}
	if fis.GetNextFieldNumber() != 0 {
		t.Errorf("Expected next field number=0, got %d", fis.GetNextFieldNumber())
	}
}

func TestFieldInfos_Add(t *testing.T) {
	fis := NewFieldInfos()

	// Add first field
	fi1 := NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
		Stored:       true,
	})
	err := fis.Add(fi1)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}
	if fis.Size() != 1 {
		t.Errorf("Expected Size=1, got %d", fis.Size())
	}

	// Add second field
	fi2 := NewFieldInfo("body", 1, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqsAndPositions,
		Stored:       true,
	})
	err = fis.Add(fi2)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}
	if fis.Size() != 2 {
		t.Errorf("Expected Size=2, got %d", fis.Size())
	}

	// Verify GetByName
	got := fis.GetByName("title")
	if got == nil {
		t.Fatal("GetByName('title') should not be nil")
	}
	if got.Name() != "title" {
		t.Errorf("Expected name 'title', got '%s'", got.Name())
	}

	// Verify GetByNumber
	got2 := fis.GetByNumber(1)
	if got2 == nil {
		t.Fatal("GetByNumber(1) should not be nil")
	}
	if got2.Name() != "body" {
		t.Errorf("Expected name 'body', got '%s'", got2.Name())
	}

	// Get non-existent
	if fis.GetByName("nonexistent") != nil {
		t.Error("GetByName('nonexistent') should be nil")
	}
	if fis.GetByNumber(999) != nil {
		t.Error("GetByNumber(999) should be nil")
	}
}

func TestFieldInfos_Add_Duplicate(t *testing.T) {
	fis := NewFieldInfos()

	// Add first field
	fi1 := NewFieldInfo("title", 0, FieldInfoOptions{})
	fis.Add(fi1)

	// Add same field again (same name and number) - should succeed
	fi1Again := NewFieldInfo("title", 0, FieldInfoOptions{})
	err := fis.Add(fi1Again)
	if err != nil {
		t.Errorf("Adding same field again should succeed: %v", err)
	}
	if fis.Size() != 1 {
		t.Errorf("Size should still be 1, got %d", fis.Size())
	}

	// Add field with same name but different number - should fail
	fiDifferent := NewFieldInfo("title", 1, FieldInfoOptions{})
	err = fis.Add(fiDifferent)
	if err == nil {
		t.Error("Adding field with same name but different number should fail")
	}

	// Add field with different name but same number - should fail
	fis2 := NewFieldInfos()
	fis2.Add(NewFieldInfo("title", 0, FieldInfoOptions{}))
	fiConflict := NewFieldInfo("body", 0, FieldInfoOptions{})
	err = fis2.Add(fiConflict)
	if err == nil {
		t.Error("Adding field with same number but different name should fail")
	}
}

func TestFieldInfos_Add_WhenFrozen(t *testing.T) {
	fis := NewFieldInfos()
	fis.Add(NewFieldInfo("title", 0, FieldInfoOptions{}))
	fis.Freeze()

	if !fis.IsFrozen() {
		t.Error("FieldInfos should be frozen")
	}

	err := fis.Add(NewFieldInfo("body", 1, FieldInfoOptions{}))
	if err == nil {
		t.Error("Adding to frozen FieldInfos should fail")
	}
}

func TestFieldInfos_Names(t *testing.T) {
	fis := NewFieldInfos()

	// Add fields out of order
	fis.Add(NewFieldInfo("zebra", 2, FieldInfoOptions{}))
	fis.Add(NewFieldInfo("alpha", 0, FieldInfoOptions{}))
	fis.Add(NewFieldInfo("beta", 1, FieldInfoOptions{}))

	names := fis.Names()
	if len(names) != 3 {
		t.Fatalf("Expected 3 names, got %d", len(names))
	}

	// Should be sorted
	expected := []string{"alpha", "beta", "zebra"}
	for i, exp := range expected {
		if names[i] != exp {
			t.Errorf("Expected name '%s' at index %d, got '%s'", exp, i, names[i])
		}
	}

	// Modify returned slice should not affect original
	names[0] = "modified"
	names2 := fis.Names()
	if names2[0] != "alpha" {
		t.Error("Names should return a copy")
	}
}

func TestFieldInfos_Iterator(t *testing.T) {
	fis := NewFieldInfos()

	// Add fields with non-sequential numbers
	fis.Add(NewFieldInfo("title", 5, FieldInfoOptions{}))
	fis.Add(NewFieldInfo("body", 2, FieldInfoOptions{}))
	fis.Add(NewFieldInfo("author", 10, FieldInfoOptions{}))

	iter := fis.Iterator()

	// Should iterate by field number order
	expected := []int{2, 5, 10}
	for _, expNum := range expected {
		if !iter.HasNext() {
			t.Fatal("HasNext should be true")
		}
		fi := iter.Next()
		if fi == nil {
			t.Fatal("Next should not be nil")
		}
		if fi.Number() != expNum {
			t.Errorf("Expected number %d, got %d", expNum, fi.Number())
		}
	}

	if iter.HasNext() {
		t.Error("HasNext should be false at end")
	}

	if iter.Next() != nil {
		t.Error("Next at end should be nil")
	}
}

func TestFieldInfos_HasProx(t *testing.T) {
	// No positions
	fis1 := NewFieldInfos()
	fis1.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
	}))
	if fis1.HasProx() {
		t.Error("HasProx should be false when no field has positions")
	}

	// Has positions
	fis2 := NewFieldInfos()
	fis2.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqsAndPositions,
	}))
	if !fis2.HasProx() {
		t.Error("HasProx should be true when field has positions")
	}
}

func TestFieldInfos_HasFreq(t *testing.T) {
	// No freqs
	fis1 := NewFieldInfos()
	fis1.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocs,
	}))
	if fis1.HasFreq() {
		t.Error("HasFreq should be false when no field has freqs")
	}

	// Has freqs
	fis2 := NewFieldInfos()
	fis2.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
	}))
	if !fis2.HasFreq() {
		t.Error("HasFreq should be true when field has freqs")
	}
}

func TestFieldInfos_HasOffsets(t *testing.T) {
	// No offsets
	fis1 := NewFieldInfos()
	fis1.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqsAndPositions,
	}))
	if fis1.HasOffsets() {
		t.Error("HasOffsets should be false when no field has offsets")
	}

	// Has offsets
	fis2 := NewFieldInfos()
	fis2.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
	}))
	if !fis2.HasOffsets() {
		t.Error("HasOffsets should be true when field has offsets")
	}
}

func TestFieldInfos_HasDocValues(t *testing.T) {
	// No doc values
	fis1 := NewFieldInfos()
	fis1.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		DocValuesType: DocValuesTypeNone,
	}))
	if fis1.HasDocValues() {
		t.Error("HasDocValues should be false when no field has doc values")
	}

	// Has doc values
	fis2 := NewFieldInfos()
	fis2.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		DocValuesType: DocValuesTypeNumeric,
	}))
	if !fis2.HasDocValues() {
		t.Error("HasDocValues should be true when field has doc values")
	}
}

func TestFieldInfos_HasNorms(t *testing.T) {
	// No norms (omitNorms=true)
	fis1 := NewFieldInfos()
	fis1.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
		OmitNorms:    true,
	}))
	if fis1.HasNorms() {
		t.Error("HasNorms should be false when omitNorms=true")
	}

	// Has norms
	fis2 := NewFieldInfos()
	fis2.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
		OmitNorms:    false,
	}))
	if !fis2.HasNorms() {
		t.Error("HasNorms should be true when norms enabled")
	}
}

func TestFieldInfos_HasTermVectors(t *testing.T) {
	// No term vectors
	fis1 := NewFieldInfos()
	fis1.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		StoreTermVectors: false,
	}))
	if fis1.HasTermVectors() {
		t.Error("HasTermVectors should be false when no field has term vectors")
	}

	// Has term vectors
	fis2 := NewFieldInfos()
	fis2.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		StoreTermVectors: true,
	}))
	if !fis2.HasTermVectors() {
		t.Error("HasTermVectors should be true when field has term vectors")
	}
}

func TestFieldInfos_Clear(t *testing.T) {
	fis := NewFieldInfos()
	fis.Add(NewFieldInfo("title", 0, FieldInfoOptions{}))
	fis.Add(NewFieldInfo("body", 1, FieldInfoOptions{}))

	if fis.Size() != 2 {
		t.Error("Expected Size=2 before clear")
	}

	fis.Clear()

	if fis.Size() != 0 {
		t.Errorf("Expected Size=0 after clear, got %d", fis.Size())
	}

	// Clear frozen FieldInfos should not work
	fis.Add(NewFieldInfo("title", 0, FieldInfoOptions{}))
	fis.Freeze()
	fis.Clear()
	if fis.Size() != 1 {
		t.Error("Clear on frozen FieldInfos should not work")
	}
}

func TestFieldInfos_String(t *testing.T) {
	fis := NewFieldInfos()
	fis.Add(NewFieldInfo("title", 0, FieldInfoOptions{}))

	str := fis.String()
	if str != "FieldInfos(size=1)" {
		t.Errorf("Expected 'FieldInfos(size=1)', got '%s'", str)
	}
}

func TestEmptyFieldInfos(t *testing.T) {
	// Test that EmptyFieldInfos is frozen and empty
	if !EmptyFieldInfos.IsFrozen() {
		t.Error("EmptyFieldInfos should be frozen")
	}
	if EmptyFieldInfos.Size() != 0 {
		t.Error("EmptyFieldInfos should have size 0")
	}

	err := EmptyFieldInfos.Add(NewFieldInfo("title", 0, FieldInfoOptions{}))
	if err == nil {
		t.Error("Adding to EmptyFieldInfos should fail")
	}
}

func TestFieldInfosBuilder(t *testing.T) {
	builder := NewFieldInfosBuilder()

	builder.Add(NewFieldInfo("title", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
	}))

	builder.AddFromOptions("body", FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqsAndPositions,
		Stored:       true,
	})

	fis := builder.Build()

	if !fis.IsFrozen() {
		t.Error("Built FieldInfos should be frozen")
	}
	if fis.Size() != 2 {
		t.Errorf("Expected Size=2, got %d", fis.Size())
	}

	// Check auto-assigned number for second field
	body := fis.GetByName("body")
	if body == nil {
		t.Fatal("body should exist")
	}
	if body.Number() != 1 {
		t.Errorf("Expected body number=1, got %d", body.Number())
	}
}

func TestFieldInfos_ConcurrentAccess(t *testing.T) {
	fis := NewFieldInfos()
	fis.Add(NewFieldInfo("title", 0, FieldInfoOptions{}))

	// Concurrent reads
	done := make(chan bool, 3)

	// Reader 1
	go func() {
		for i := 0; i < 100; i++ {
			fis.GetByName("title")
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for i := 0; i < 100; i++ {
			fis.GetByNumber(0)
		}
		done <- true
	}()

	// Reader 3
	go func() {
		for i := 0; i < 100; i++ {
			fis.HasProx()
		}
		done <- true
	}()

	// Wait for all readers
	for i := 0; i < 3; i++ {
		<-done
	}
}
