// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"testing"
)

func TestNewTermVector(t *testing.T) {
	tv := NewTermVector("content")
	if tv.Field != "content" {
		t.Errorf("Expected field 'content', got %s", tv.Field)
	}
	if len(tv.Terms) != 0 {
		t.Errorf("Expected 0 terms, got %d", len(tv.Terms))
	}
}

func TestTermVector_AddTerm(t *testing.T) {
	tv := NewTermVector("content")
	tv.AddTerm("hello", 2, []int{0, 5}, []int{0, 10}, []int{5, 15})

	if len(tv.Terms) != 1 {
		t.Errorf("Expected 1 term, got %d", len(tv.Terms))
	}
	if tv.Terms[0] != "hello" {
		t.Errorf("Expected term 'hello', got %s", tv.Terms[0])
	}
	if tv.TermFreqs[0] != 2 {
		t.Errorf("Expected freq 2, got %d", tv.TermFreqs[0])
	}
	if len(tv.Positions[0]) != 2 {
		t.Errorf("Expected 2 positions, got %d", len(tv.Positions[0]))
	}
}

func TestTermVector_HasPositions(t *testing.T) {
	tv := NewTermVector("content")
	if tv.HasPositions() {
		t.Error("Expected no positions initially")
	}

	tv.AddTerm("hello", 1, []int{0}, nil, nil)
	if !tv.HasPositions() {
		t.Error("Expected positions after adding term with positions")
	}
}

func TestTermVector_HasOffsets(t *testing.T) {
	tv := NewTermVector("content")
	if tv.HasOffsets() {
		t.Error("Expected no offsets initially")
	}

	tv.AddTerm("hello", 1, nil, []int{0}, []int{5})
	if !tv.HasOffsets() {
		t.Error("Expected offsets after adding term with offsets")
	}
}

func TestTermVector_GetTermFreq(t *testing.T) {
	tv := NewTermVector("content")
	tv.AddTerm("hello", 3, nil, nil, nil)
	tv.AddTerm("world", 2, nil, nil, nil)

	if freq := tv.GetTermFreq("hello"); freq != 3 {
		t.Errorf("Expected freq 3 for 'hello', got %d", freq)
	}
	if freq := tv.GetTermFreq("world"); freq != 2 {
		t.Errorf("Expected freq 2 for 'world', got %d", freq)
	}
	if freq := tv.GetTermFreq("missing"); freq != 0 {
		t.Errorf("Expected freq 0 for missing term, got %d", freq)
	}
}

func TestTermVector_String(t *testing.T) {
	tv := NewTermVector("content")
	tv.AddTerm("hello", 1, nil, nil, nil)

	s := tv.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestMemoryTermVectorsWriter(t *testing.T) {
	writer := NewMemoryTermVectorsWriter()

	// Write document 0
	if err := writer.StartDocument(0); err != nil {
		t.Fatalf("StartDocument failed: %v", err)
	}

	if err := writer.StartField("content", true, true); err != nil {
		t.Fatalf("StartField failed: %v", err)
	}

	if err := writer.AddTerm([]byte("hello"), 2, []int{0, 5}, []int{0, 10}, []int{5, 15}); err != nil {
		t.Fatalf("AddTerm failed: %v", err)
	}

	if err := writer.AddTerm([]byte("world"), 1, []int{10}, []int{20}, []int{25}); err != nil {
		t.Fatalf("AddTerm failed: %v", err)
	}

	if err := writer.FinishField(); err != nil {
		t.Fatalf("FinishField failed: %v", err)
	}

	if err := writer.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument failed: %v", err)
	}

	// Verify
	vectors, ok := writer.GetDocument(0)
	if !ok {
		t.Fatal("Document 0 not found")
	}

	contentVector, ok := vectors["content"]
	if !ok {
		t.Fatal("content field not found")
	}

	if len(contentVector.Terms) != 2 {
		t.Errorf("Expected 2 terms, got %d", len(contentVector.Terms))
	}

	if !contentVector.HasPositions() {
		t.Error("Expected positions to be preserved")
	}

	if !contentVector.HasOffsets() {
		t.Error("Expected offsets to be preserved")
	}

	if err := writer.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMemoryTermVectorsReader(t *testing.T) {
	writer := NewMemoryTermVectorsWriter()

	// Write document
	writer.StartDocument(0)
	writer.StartField("content", true, false)
	writer.AddTerm([]byte("hello"), 1, []int{0}, nil, nil)
	writer.FinishField()
	writer.FinishDocument()

	// Read
	reader := NewMemoryTermVectorsReader(writer)

	vectors, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Check that we can iterate over fields
	iter, err := vectors.Iterator()
	if err != nil {
		t.Fatalf("Iterator failed: %v", err)
	}

	count := 0
	for {
		field, err := iter.Next()
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		if field == "" {
			break
		}
		count++
	}
	if count != 1 {
		t.Errorf("Expected 1 field, got %d", count)
	}

	// Get specific field
	terms, err := reader.GetField(0, "content")
	if err != nil {
		t.Fatalf("GetField failed: %v", err)
	}
	if terms == nil {
		t.Fatal("Expected Terms, got nil")
	}

	// Verify we can iterate over terms
	termsEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator failed: %v", err)
	}

	term, err := termsEnum.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if term == nil {
		t.Fatal("Expected term, got nil")
	}
	if term.Text() != "hello" {
		t.Errorf("Expected term 'hello', got %s", term.Text())
	}

	// Get non-existent document
	_, err = reader.Get(1)
	if err == nil {
		t.Error("Expected error for non-existent document")
	}

	// Get non-existent field
	terms, err = reader.GetField(0, "missing")
	if err != nil {
		t.Error("GetField should not return error for missing field, just nil")
	}
	if terms != nil {
		t.Error("Expected nil for missing field")
	}

	if err := reader.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestTermVectorMapper(t *testing.T) {
	tv := NewTermVector("content")
	tv.AddTerm("apple", 1, nil, nil, nil)
	tv.AddTerm("banana", 2, nil, nil, nil)
	tv.AddTerm("cherry", 3, nil, nil, nil)

	// Mapper that only includes terms starting with 'a' or 'c'
	mapper := &testMapper{
		filter: func(term string) bool {
			return term[0] == 'a' || term[0] == 'c'
		},
	}

	filtered := FilterTermVector(tv, mapper)

	if len(filtered.Terms) != 2 {
		t.Errorf("Expected 2 terms after filtering, got %d", len(filtered.Terms))
	}

	if filtered.GetTermFreq("banana") != 0 {
		t.Error("Expected 'banana' to be filtered out")
	}

	if filtered.GetTermFreq("apple") != 1 || filtered.GetTermFreq("cherry") != 3 {
		t.Error("Expected 'apple' and 'cherry' to be preserved")
	}
}

type testMapper struct {
	filter func(string) bool
}

func (m *testMapper) Map(term string, freq int, positions, startOffsets, endOffsets []int) bool {
	return m.filter(term)
}

func TestTermVector_LargeTermCount(t *testing.T) {
	tv := NewTermVector("large")
	count := 1000
	for i := 0; i < count; i++ {
		tv.AddTerm(fmt.Sprintf("term-%d", i), 1, nil, nil, nil)
	}

	if len(tv.Terms) != count {
		t.Errorf("Expected %d terms, got %d", count, len(tv.Terms))
	}

	if tv.GetTermFreq("term-500") != 1 {
		t.Error("Expected to find term-500")
	}
}

func TestTermVector_MultipleFields(t *testing.T) {
	writer := NewMemoryTermVectorsWriter()
	writer.StartDocument(0)

	// Field 1: positions
	writer.StartField("title", true, false)
	writer.AddTerm([]byte("gocene"), 1, []int{0}, nil, nil)
	writer.FinishField()

	// Field 2: offsets
	writer.StartField("body", false, true)
	writer.AddTerm([]byte("hello"), 1, nil, []int{0}, []int{5})
	writer.FinishField()

	writer.FinishDocument()

	reader := NewMemoryTermVectorsReader(writer)
	vectors, _ := reader.Get(0)

	// Check field count
	size := vectors.Size()
	if size != 2 {
		t.Errorf("Expected 2 fields, got %d", size)
	}

	// Check title field has positions but not offsets
	titleTerms, _ := vectors.Terms("title")
	if titleTerms == nil {
		t.Fatal("title field not found")
	}
	if !titleTerms.HasPositions() {
		t.Error("Title should have positions")
	}
	if titleTerms.HasOffsets() {
		t.Error("Title should not have offsets")
	}

	// Check body field has offsets but not positions
	bodyTerms, _ := vectors.Terms("body")
	if bodyTerms == nil {
		t.Fatal("body field not found")
	}
	if bodyTerms.HasPositions() {
		t.Error("Body should not have positions")
	}
	if !bodyTerms.HasOffsets() {
		t.Error("Body should have offsets")
	}
}