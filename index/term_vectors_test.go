// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
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

	if err := writer.StartField("content", true, false); err != nil {
		t.Fatalf("StartField failed: %v", err)
	}

	if err := writer.AddTerm([]byte("hello"), 2, []int{0, 5}, nil, nil); err != nil {
		t.Fatalf("AddTerm failed: %v", err)
	}

	if err := writer.AddTerm([]byte("world"), 1, []int{10}, nil, nil); err != nil {
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

	if err := writer.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMemoryTermVectorsReader(t *testing.T) {
	writer := NewMemoryTermVectorsWriter()

	// Write document
	writer.StartDocument(0)
	writer.StartField("content", true, false)
	writer.AddTerm([]byte("hello"), 1, nil, nil, nil)
	writer.FinishField()
	writer.FinishDocument()

	// Read
	reader := NewMemoryTermVectorsReader(writer)

	vectors, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(vectors) != 1 {
		t.Errorf("Expected 1 field, got %d", len(vectors))
	}

	// Get specific field
	vector, err := reader.GetField(0, "content")
	if err != nil {
		t.Fatalf("GetField failed: %v", err)
	}
	if len(vector.Terms) != 1 || vector.Terms[0] != "hello" {
		t.Errorf("Expected term 'hello', got %v", vector.Terms)
	}

	// Get non-existent document
	_, err = reader.Get(1)
	if err == nil {
		t.Error("Expected error for non-existent document")
	}

	// Get non-existent field
	_, err = reader.GetField(0, "missing")
	if err == nil {
		t.Error("Expected error for non-existent field")
	}

	if err := reader.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMemoryTermVectorsWriter_AddTermNoField(t *testing.T) {
	writer := NewMemoryTermVectorsWriter()
	writer.StartDocument(0)

	// Try to add term without starting field
	err := writer.AddTerm([]byte("hello"), 1, nil, nil, nil)
	if err == nil {
		t.Error("Expected error when adding term without starting field")
	}
}

func TestMemoryTermVectorsWriter_FinishFieldNoField(t *testing.T) {
	writer := NewMemoryTermVectorsWriter()
	writer.StartDocument(0)

	// Try to finish field without starting one
	err := writer.FinishField()
	if err == nil {
		t.Error("Expected error when finishing field without starting one")
	}
}

func TestNewTermFreqVector(t *testing.T) {
	tfv := NewTermFreqVector("content")
	if tfv.Field != "content" {
		t.Errorf("Expected field 'content', got %s", tfv.Field)
	}
}

func TestTermFreqVector_Add(t *testing.T) {
	tfv := NewTermFreqVector("content")
	tfv.Add("hello", 2)
	tfv.Add("world", 1)

	if len(tfv.Terms) != 2 {
		t.Errorf("Expected 2 terms, got %d", len(tfv.Terms))
	}
	if tfv.Freqs[0] != 2 {
		t.Errorf("Expected freq 2, got %d", tfv.Freqs[0])
	}
}

func TestTermFreqVector_IndexOf(t *testing.T) {
	tfv := NewTermFreqVector("content")
	tfv.Add("hello", 1)
	tfv.Add("world", 2)

	if idx := tfv.IndexOf("hello"); idx != 0 {
		t.Errorf("Expected index 0 for 'hello', got %d", idx)
	}
	if idx := tfv.IndexOf("world"); idx != 1 {
		t.Errorf("Expected index 1 for 'world', got %d", idx)
	}
	if idx := tfv.IndexOf("missing"); idx != -1 {
		t.Errorf("Expected index -1 for missing term, got %d", idx)
	}
}

func TestTermFreqVector_String(t *testing.T) {
	tfv := NewTermFreqVector("content")
	tfv.Add("hello", 1)

	s := tfv.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestBytesToTermVector(t *testing.T) {
	tv := NewTermVector("content")
	tv.AddTerm("hello", 2, nil, nil, nil)
	tv.AddTerm("world", 1, nil, nil, nil)

	encoded := TermVectorToBytes(tv)
	decoded, err := BytesToTermVector(encoded)
	if err != nil {
		t.Fatalf("BytesToTermVector failed: %v", err)
	}

	if decoded.Field != "content" {
		t.Errorf("Expected field 'content', got %s", decoded.Field)
	}
	if len(decoded.Terms) != 2 {
		t.Errorf("Expected 2 terms, got %d", len(decoded.Terms))
	}
}

func TestBytesToTermVector_Invalid(t *testing.T) {
	_, err := BytesToTermVector([]byte{})
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

func TestNewTermVectorsFormat(t *testing.T) {
	format := NewTermVectorsFormat()
	if format.Version != 1 {
		t.Errorf("Expected version 1, got %d", format.Version)
	}
}

func TestTermVectorsMetadata(t *testing.T) {
	meta := TermVectorsMetadata{
		NumDocuments: 10,
		NumFields:    3,
		HasPositions: true,
		HasOffsets:   false,
	}

	if meta.NumDocuments != 10 {
		t.Errorf("Expected 10 documents, got %d", meta.NumDocuments)
	}
	if meta.NumFields != 3 {
		t.Errorf("Expected 3 fields, got %d", meta.NumFields)
	}
	if !meta.HasPositions {
		t.Error("Expected HasPositions to be true")
	}
	if meta.HasOffsets {
		t.Error("Expected HasOffsets to be false")
	}
}
