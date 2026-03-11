// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestEmptyPostingsEnum(t *testing.T) {
	empty := &EmptyPostingsEnum{}

	// Test NextDoc returns NO_MORE_DOCS
	docID, err := empty.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if docID != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS (%d), got %d", NO_MORE_DOCS, docID)
	}

	// Test Advance returns NO_MORE_DOCS
	docID2, err := empty.Advance(5)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if docID2 != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS (%d), got %d", NO_MORE_DOCS, docID2)
	}

	// Test DocID
	if empty.DocID() != NO_MORE_DOCS {
		t.Errorf("Expected DocID=NO_MORE_DOCS, got %d", empty.DocID())
	}

	// Test Freq returns 0
	freq, err := empty.Freq()
	if err != nil {
		t.Fatalf("Freq error: %v", err)
	}
	if freq != 0 {
		t.Errorf("Expected Freq=0, got %d", freq)
	}

	// Test NextPosition returns NO_MORE_POSITIONS
	pos, err := empty.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition error: %v", err)
	}
	if pos != NO_MORE_POSITIONS {
		t.Errorf("Expected NO_MORE_POSITIONS (%d), got %d", NO_MORE_POSITIONS, pos)
	}

	// Test StartOffset returns -1
	offset, err := empty.StartOffset()
	if err != nil {
		t.Fatalf("StartOffset error: %v", err)
	}
	if offset != -1 {
		t.Errorf("Expected StartOffset=-1, got %d", offset)
	}

	// Test EndOffset returns -1
	endOffset, err := empty.EndOffset()
	if err != nil {
		t.Fatalf("EndOffset error: %v", err)
	}
	if endOffset != -1 {
		t.Errorf("Expected EndOffset=-1, got %d", endOffset)
	}

	// Test GetPayload returns nil
	payload, err := empty.GetPayload()
	if err != nil {
		t.Fatalf("GetPayload error: %v", err)
	}
	if payload != nil {
		t.Error("Expected GetPayload=nil")
	}

	// Test Cost returns 0
	if empty.Cost() != 0 {
		t.Errorf("Expected Cost=0, got %d", empty.Cost())
	}
}

func TestSingleDocPostingsEnum(t *testing.T) {
	pe := NewSingleDocPostingsEnum(5, 3)

	// Test initial state
	if pe.DocID() != -1 {
		t.Errorf("Expected initial DocID=-1, got %d", pe.DocID())
	}

	// First NextDoc should return doc 5
	docID, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if docID != 5 {
		t.Errorf("Expected DocID=5, got %d", docID)
	}

	// Check DocID
	if pe.DocID() != 5 {
		t.Errorf("Expected DocID=5, got %d", pe.DocID())
	}

	// Check Freq
	freq, err := pe.Freq()
	if err != nil {
		t.Fatalf("Freq error: %v", err)
	}
	if freq != 3 {
		t.Errorf("Expected Freq=3, got %d", freq)
	}

	// Second NextDoc should return NO_MORE_DOCS
	docID2, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if docID2 != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS, got %d", docID2)
	}
}

func TestSingleDocPostingsEnum_Advance(t *testing.T) {
	pe := NewSingleDocPostingsEnum(10, 2)

	// Advance to a doc before ours
	docID, err := pe.Advance(5)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if docID != 10 {
		t.Errorf("Expected DocID=10, got %d", docID)
	}

	// Create new instance for next test
	pe2 := NewSingleDocPostingsEnum(10, 2)

	// Advance to exact doc
	docID2, err := pe2.Advance(10)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if docID2 != 10 {
		t.Errorf("Expected DocID=10, got %d", docID2)
	}

	// Create new instance for next test
	pe3 := NewSingleDocPostingsEnum(10, 2)

	// Advance to a doc after ours
	docID3, err := pe3.Advance(20)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if docID3 != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS, got %d", docID3)
	}
}

func TestSingleDocPostingsEnum_Cost(t *testing.T) {
	pe := NewSingleDocPostingsEnum(5, 3)
	if pe.Cost() != 1 {
		t.Errorf("Expected Cost=1, got %d", pe.Cost())
	}
}

func TestSingleDocPostingsEnum_Positions(t *testing.T) {
	pe := NewSingleDocPostingsEnum(5, 3)

	// Move to document
	pe.NextDoc()

	// Positions should return NO_MORE_POSITIONS (no positions indexed)
	pos, err := pe.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition error: %v", err)
	}
	if pos != NO_MORE_POSITIONS {
		t.Errorf("Expected NO_MORE_POSITIONS, got %d", pos)
	}

	// Offsets should return -1
	startOffset, err := pe.StartOffset()
	if err != nil {
		t.Fatalf("StartOffset error: %v", err)
	}
	if startOffset != -1 {
		t.Errorf("Expected StartOffset=-1, got %d", startOffset)
	}

	endOffset, err := pe.EndOffset()
	if err != nil {
		t.Fatalf("EndOffset error: %v", err)
	}
	if endOffset != -1 {
		t.Errorf("Expected EndOffset=-1, got %d", endOffset)
	}

	// Payload should return nil
	payload, err := pe.GetPayload()
	if err != nil {
		t.Fatalf("GetPayload error: %v", err)
	}
	if payload != nil {
		t.Error("Expected GetPayload=nil")
	}
}

func TestSingleDocPostingsEnum_Unpositioned(t *testing.T) {
	pe := NewSingleDocPostingsEnum(5, 3)

	// Before calling NextDoc, Freq should return 0
	freq, err := pe.Freq()
	if err != nil {
		t.Fatalf("Freq error: %v", err)
	}
	if freq != 0 {
		t.Errorf("Expected Freq=0 when unpositioned, got %d", freq)
	}
}
