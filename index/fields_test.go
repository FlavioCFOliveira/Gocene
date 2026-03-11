// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestEmptyFields(t *testing.T) {
	empty := &EmptyFields{}

	// Test Size
	if empty.Size() != 0 {
		t.Errorf("Expected Size=0, got %d", empty.Size())
	}

	// Test Iterator
	iter, err := empty.Iterator()
	if err != nil {
		t.Fatalf("Iterator error: %v", err)
	}
	if iter == nil {
		t.Fatal("Iterator should not be nil")
	}

	// Iterator should return empty immediately
	field, err := iter.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if field != "" {
		t.Errorf("Expected empty field, got '%s'", field)
	}

	// HasNext should be false
	if iter.HasNext() {
		t.Error("HasNext should be false for empty fields")
	}

	// Test Terms
	terms, err := empty.Terms("anyfield")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if terms != nil {
		t.Error("Terms should return nil for empty fields")
	}
}

func TestMemoryFields(t *testing.T) {
	mf := NewMemoryFields()

	// Initially empty
	if mf.Size() != 0 {
		t.Errorf("Expected Size=0, got %d", mf.Size())
	}

	// Add a field
	term := NewTerm("title", "test")
	terms := NewSingleTermTerms(term, 1, 1)
	mf.AddField("title", terms)

	if mf.Size() != 1 {
		t.Errorf("Expected Size=1, got %d", mf.Size())
	}

	// Check HasField
	if !mf.HasField("title") {
		t.Error("Should have field 'title'")
	}
	if mf.HasField("body") {
		t.Error("Should not have field 'body'")
	}

	// Get Terms
	gotTerms, err := mf.Terms("title")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if gotTerms == nil {
		t.Fatal("Terms should not be nil")
	}

	// Get non-existent Terms
	gotTerms2, err := mf.Terms("body")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if gotTerms2 != nil {
		t.Error("Terms should be nil for non-existent field")
	}

	// Add another field
	term2 := NewTerm("body", "content")
	terms2 := NewSingleTermTerms(term2, 1, 1)
	mf.AddField("body", terms2)

	if mf.Size() != 2 {
		t.Errorf("Expected Size=2, got %d", mf.Size())
	}

	// Get field names
	names := mf.GetFieldNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 field names, got %d", len(names))
	}
	// Should be sorted
	if names[0] != "body" || names[1] != "title" {
		t.Errorf("Expected ['body', 'title'], got %v", names)
	}

	// Remove a field
	mf.RemoveField("title")
	if mf.HasField("title") {
		t.Error("Should not have field 'title' after removal")
	}
	if mf.Size() != 1 {
		t.Errorf("Expected Size=1 after removal, got %d", mf.Size())
	}
}

func TestMemoryFields_Iterator(t *testing.T) {
	mf := NewMemoryFields()

	// Add fields (not in alphabetical order)
	mf.AddField("zebra", NewSingleTermTerms(NewTerm("zebra", "z"), 1, 1))
	mf.AddField("alpha", NewSingleTermTerms(NewTerm("alpha", "a"), 1, 1))
	mf.AddField("beta", NewSingleTermTerms(NewTerm("beta", "b"), 1, 1))

	iter, err := mf.Iterator()
	if err != nil {
		t.Fatalf("Iterator error: %v", err)
	}

	// Should iterate in sorted order
	expected := []string{"alpha", "beta", "zebra"}
	for i, exp := range expected {
		if !iter.HasNext() {
			t.Fatalf("HasNext should be true at index %d", i)
		}
		field, err := iter.Next()
		if err != nil {
			t.Fatalf("Next error at index %d: %v", i, err)
		}
		if field != exp {
			t.Errorf("Expected field '%s' at index %d, got '%s'", exp, i, field)
		}
	}

	// No more fields
	if iter.HasNext() {
		t.Error("HasNext should be false at end")
	}

	field, err := iter.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if field != "" {
		t.Errorf("Expected empty field at end, got '%s'", field)
	}
}

func TestSingleFieldFields(t *testing.T) {
	term := NewTerm("title", "test")
	terms := NewSingleTermTerms(term, 1, 1)
	sf := NewSingleFieldFields("title", terms)

	// Test Size
	if sf.Size() != 1 {
		t.Errorf("Expected Size=1, got %d", sf.Size())
	}

	// Test Terms
	gotTerms, err := sf.Terms("title")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if gotTerms == nil {
		t.Fatal("Terms should not be nil for matching field")
	}

	// Non-matching field
	gotTerms2, err := sf.Terms("body")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if gotTerms2 != nil {
		t.Error("Terms should be nil for non-matching field")
	}

	// Test Iterator
	iter, err := sf.Iterator()
	if err != nil {
		t.Fatalf("Iterator error: %v", err)
	}

	if !iter.HasNext() {
		t.Error("HasNext should be true initially")
	}

	field, err := iter.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if field != "title" {
		t.Errorf("Expected field 'title', got '%s'", field)
	}

	if iter.HasNext() {
		t.Error("HasNext should be false after first Next")
	}

	field2, err := iter.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if field2 != "" {
		t.Errorf("Expected empty field, got '%s'", field2)
	}
}

func TestMultiFields(t *testing.T) {
	// Create two MemoryFields
	mf1 := NewMemoryFields()
	mf1.AddField("title", NewSingleTermTerms(NewTerm("title", "t1"), 1, 1))
	mf1.AddField("body", NewSingleTermTerms(NewTerm("body", "b1"), 1, 1))

	mf2 := NewMemoryFields()
	mf2.AddField("author", NewSingleTermTerms(NewTerm("author", "a1"), 1, 1))
	mf2.AddField("title", NewSingleTermTerms(NewTerm("title", "t2"), 1, 1)) // Duplicate field

	// Combine them
	multi := NewMultiFields(mf1, mf2)

	// Test Size (should count unique fields)
	size := multi.Size()
	if size != 3 {
		t.Errorf("Expected Size=3 (unique fields), got %d", size)
	}

	// Test Iterator (should have all unique fields)
	iter, err := multi.Iterator()
	if err != nil {
		t.Fatalf("Iterator error: %v", err)
	}

	expected := []string{"author", "body", "title"}
	for _, exp := range expected {
		if !iter.HasNext() {
			t.Fatal("HasNext should be true")
		}
		field, err := iter.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if field != exp {
			t.Errorf("Expected field '%s', got '%s'", exp, field)
		}
	}

	// Test Terms (should return first non-nil)
	terms, err := multi.Terms("title")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms should not be nil")
	}

	// Test non-existent field
	terms2, err := multi.Terms("nonexistent")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if terms2 != nil {
		t.Error("Terms should be nil for non-existent field")
	}
}

func TestMultiFields_Empty(t *testing.T) {
	// Empty MultiFields
	multi := NewMultiFields()

	if multi.Size() != 0 {
		t.Errorf("Expected Size=0, got %d", multi.Size())
	}

	iter, err := multi.Iterator()
	if err != nil {
		t.Fatalf("Iterator error: %v", err)
	}

	field, err := iter.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if field != "" {
		t.Errorf("Expected empty field, got '%s'", field)
	}

	terms, err := multi.Terms("any")
	if err != nil {
		t.Fatalf("Terms error: %v", err)
	}
	if terms != nil {
		t.Error("Terms should be nil")
	}
}

func TestFieldsBase_Defaults(t *testing.T) {
	base := &FieldsBase{}

	if base.Size() != -1 {
		t.Errorf("Expected Size=-1 (unknown), got %d", base.Size())
	}
}

func TestEmptyFieldIterator(t *testing.T) {
	iter := &EmptyFieldIterator{}

	if iter.HasNext() {
		t.Error("HasNext should be false for empty iterator")
	}

	field, err := iter.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if field != "" {
		t.Errorf("Expected empty field, got '%s'", field)
	}
}

func TestMemoryFieldIterator(t *testing.T) {
	iter := &MemoryFieldIterator{
		fields: []string{"a", "b", "c"},
		index:  -1,
	}

	// HasNext before any Next
	if !iter.HasNext() {
		t.Error("HasNext should be true initially")
	}

	// Iterate through all fields
	expected := []string{"a", "b", "c"}
	for _, exp := range expected {
		field, err := iter.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if field != exp {
			t.Errorf("Expected '%s', got '%s'", exp, field)
		}
	}

	// No more fields
	if iter.HasNext() {
		t.Error("HasNext should be false at end")
	}

	field, err := iter.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if field != "" {
		t.Errorf("Expected empty field, got '%s'", field)
	}
}

func TestFieldsStats(t *testing.T) {
	stats := FieldsStats{
		NumFields: 5,
		NumTerms:  1000,
		NumDocs:   100,
	}

	str := stats.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	expected := "FieldsStats(numFields=5, numTerms=1000, numDocs=100)"
	if str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestMemoryFields_ConcurrentAccess(t *testing.T) {
	mf := NewMemoryFields()
	mf.AddField("title", NewSingleTermTerms(NewTerm("title", "t"), 1, 1))

	// Concurrent reads
	done := make(chan bool, 3)

	// Reader 1
	go func() {
		for i := 0; i < 100; i++ {
			mf.HasField("title")
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for i := 0; i < 100; i++ {
			mf.Terms("title")
		}
		done <- true
	}()

	// Reader 3
	go func() {
		for i := 0; i < 100; i++ {
			mf.GetFieldNames()
		}
		done <- true
	}()

	// Wait for all readers
	for i := 0; i < 3; i++ {
		<-done
	}
}
