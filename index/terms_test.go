// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestEmptyTerms(t *testing.T) {
	empty := &EmptyTerms{}

	// Test GetIterator
	te, err := empty.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator error: %v", err)
	}
	if te == nil {
		t.Fatal("GetIterator should return non-nil TermsEnum")
	}

	// Test iterator returns nil immediately
	term, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if term != nil {
		t.Error("Empty Terms should return nil on first Next()")
	}

	// Test GetIteratorWithSeek
	seekTerm := NewTerm("field", "value")
	te2, err := empty.GetIteratorWithSeek(seekTerm)
	if err != nil {
		t.Fatalf("GetIteratorWithSeek error: %v", err)
	}
	if te2 == nil {
		t.Fatal("GetIteratorWithSeek should return non-nil TermsEnum")
	}

	// Test Size
	if empty.Size() != 0 {
		t.Errorf("Expected Size=0, got %d", empty.Size())
	}

	// Test GetDocCount
	docCount, err := empty.GetDocCount()
	if err != nil {
		t.Fatalf("GetDocCount error: %v", err)
	}
	if docCount != 0 {
		t.Errorf("Expected GetDocCount=0, got %d", docCount)
	}

	// Test GetMin
	min, err := empty.GetMin()
	if err != nil {
		t.Fatalf("GetMin error: %v", err)
	}
	if min != nil {
		t.Error("GetMin should return nil for empty terms")
	}

	// Test GetMax
	max, err := empty.GetMax()
	if err != nil {
		t.Fatalf("GetMax error: %v", err)
	}
	if max != nil {
		t.Error("GetMax should return nil for empty terms")
	}
}

func TestSingleTermTerms(t *testing.T) {
	term := NewTerm("field", "test")
	st := NewSingleTermTerms(term, 5, 10)

	// Test Size
	if st.Size() != 1 {
		t.Errorf("Expected Size=1, got %d", st.Size())
	}

	// Test GetIterator
	te, err := st.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator error: %v", err)
	}

	// First Next should return the term
	gotTerm, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm == nil {
		t.Fatal("Expected term, got nil")
	}
	if !gotTerm.Equals(term) {
		t.Errorf("Expected term %v, got %v", term, gotTerm)
	}

	// Second Next should return nil
	gotTerm2, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm2 != nil {
		t.Error("Second Next should return nil")
	}

	// Test GetDocCount
	docCount, err := st.GetDocCount()
	if err != nil {
		t.Fatalf("GetDocCount error: %v", err)
	}
	if docCount != 1 {
		t.Errorf("Expected GetDocCount=1, got %d", docCount)
	}

	// Test GetSumDocFreq
	sumDocFreq, err := st.GetSumDocFreq()
	if err != nil {
		t.Fatalf("GetSumDocFreq error: %v", err)
	}
	if sumDocFreq != 5 {
		t.Errorf("Expected GetSumDocFreq=5, got %d", sumDocFreq)
	}

	// Test GetSumTotalTermFreq
	sumTotalFreq, err := st.GetSumTotalTermFreq()
	if err != nil {
		t.Fatalf("GetSumTotalTermFreq error: %v", err)
	}
	if sumTotalFreq != 10 {
		t.Errorf("Expected GetSumTotalTermFreq=10, got %d", sumTotalFreq)
	}

	// Test GetMin
	min, err := st.GetMin()
	if err != nil {
		t.Fatalf("GetMin error: %v", err)
	}
	if min == nil {
		t.Fatal("GetMin should return the term")
	}
	if !min.Equals(term) {
		t.Error("GetMin should return the single term")
	}

	// Test GetMax
	max, err := st.GetMax()
	if err != nil {
		t.Fatalf("GetMax error: %v", err)
	}
	if max == nil {
		t.Fatal("GetMax should return the term")
	}
	if !max.Equals(term) {
		t.Error("GetMax should return the single term")
	}
}

func TestSingleTermTerms_SeekCeil(t *testing.T) {
	term := NewTerm("field", "hello")
	st := NewSingleTermTerms(term, 1, 1)

	// Seek to a term before ours - should return our term
	beforeTerm := NewTerm("field", "alpha")
	te, err := st.GetIteratorWithSeek(beforeTerm)
	if err != nil {
		t.Fatalf("GetIteratorWithSeek error: %v", err)
	}
	gotTerm, err := te.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm == nil || !gotTerm.Equals(term) {
		t.Error("SeekCeil before term should return our term")
	}

	// Seek to a term after ours - should return nil
	st2 := NewSingleTermTerms(term, 1, 1)
	afterTerm := NewTerm("field", "zebra")
	te2, err := st2.GetIteratorWithSeek(afterTerm)
	if err != nil {
		t.Fatalf("GetIteratorWithSeek error: %v", err)
	}
	gotTerm2, err := te2.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm2 != nil {
		t.Error("SeekCeil after term should return nil")
	}
}

func TestTermsBase_Defaults(t *testing.T) {
	base := &TermsBase{}

	if base.Size() != -1 {
		t.Errorf("Expected Size=-1 (unknown), got %d", base.Size())
	}

	docCount, _ := base.GetDocCount()
	if docCount != 0 {
		t.Errorf("Expected GetDocCount=0, got %d", docCount)
	}

	sumDocFreq, _ := base.GetSumDocFreq()
	if sumDocFreq != -1 {
		t.Errorf("Expected GetSumDocFreq=-1, got %d", sumDocFreq)
	}

	sumTotalFreq, _ := base.GetSumTotalTermFreq()
	if sumTotalFreq != -1 {
		t.Errorf("Expected GetSumTotalTermFreq=-1, got %d", sumTotalFreq)
	}

	if base.HasFreqs() {
		t.Error("Expected HasFreqs=false by default")
	}
	if base.HasOffsets() {
		t.Error("Expected HasOffsets=false by default")
	}
	if base.HasPositions() {
		t.Error("Expected HasPositions=false by default")
	}
	if base.HasPayloads() {
		t.Error("Expected HasPayloads=false by default")
	}
}
