// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestEmptyTermsEnum(t *testing.T) {
	empty := &EmptyTermsEnum{}

	// Test Next returns nil
	term, err := empty.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if term != nil {
		t.Error("EmptyTermsEnum.Next should return nil")
	}

	// Test SeekCeil returns nil
	seekTerm := NewTerm("field", "value")
	gotTerm, err := empty.SeekCeil(seekTerm)
	if err != nil {
		t.Fatalf("SeekCeil error: %v", err)
	}
	if gotTerm != nil {
		t.Error("EmptyTermsEnum.SeekCeil should return nil")
	}

	// Test SeekExact returns false
	found, err := empty.SeekExact(seekTerm)
	if err != nil {
		t.Fatalf("SeekExact error: %v", err)
	}
	if found {
		t.Error("EmptyTermsEnum.SeekExact should return false")
	}

	// Test Term returns nil
	if empty.Term() != nil {
		t.Error("EmptyTermsEnum.Term should return nil")
	}

	// Test DocFreq returns 0
	docFreq, err := empty.DocFreq()
	if err != nil {
		t.Fatalf("DocFreq error: %v", err)
	}
	if docFreq != 0 {
		t.Errorf("Expected DocFreq=0, got %d", docFreq)
	}

	// Test TotalTermFreq returns 0
	totalFreq, err := empty.TotalTermFreq()
	if err != nil {
		t.Fatalf("TotalTermFreq error: %v", err)
	}
	if totalFreq != 0 {
		t.Errorf("Expected TotalTermFreq=0, got %d", totalFreq)
	}
}

func TestSingleTermsEnum(t *testing.T) {
	term := NewTerm("field", "test")
	st := NewSingleTermsEnum(term, 5, 10)

	// First Next should return the term
	gotTerm, err := st.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm == nil {
		t.Fatal("First Next should return the term")
	}
	if !gotTerm.Equals(term) {
		t.Errorf("Expected term %v, got %v", term, gotTerm)
	}

	// Check current term
	if st.Term() == nil || !st.Term().Equals(term) {
		t.Error("Term() should return the current term")
	}

	// Check DocFreq
	docFreq, err := st.DocFreq()
	if err != nil {
		t.Fatalf("DocFreq error: %v", err)
	}
	if docFreq != 5 {
		t.Errorf("Expected DocFreq=5, got %d", docFreq)
	}

	// Check TotalTermFreq
	totalFreq, err := st.TotalTermFreq()
	if err != nil {
		t.Fatalf("TotalTermFreq error: %v", err)
	}
	if totalFreq != 10 {
		t.Errorf("Expected TotalTermFreq=10, got %d", totalFreq)
	}

	// Second Next should return nil
	gotTerm2, err := st.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if gotTerm2 != nil {
		t.Error("Second Next should return nil")
	}
}

func TestSingleTermsEnum_SeekExact(t *testing.T) {
	term := NewTerm("field", "hello")
	st := NewSingleTermsEnum(term, 1, 1)

	// SeekExact with matching term
	found, err := st.SeekExact(term)
	if err != nil {
		t.Fatalf("SeekExact error: %v", err)
	}
	if !found {
		t.Error("SeekExact should find matching term")
	}
	if st.Term() == nil || !st.Term().Equals(term) {
		t.Error("Term should be positioned after SeekExact")
	}

	// SeekExact with non-matching term
	st2 := NewSingleTermsEnum(term, 1, 1)
	otherTerm := NewTerm("field", "world")
	found2, err := st2.SeekExact(otherTerm)
	if err != nil {
		t.Fatalf("SeekExact error: %v", err)
	}
	if found2 {
		t.Error("SeekExact should not find non-matching term")
	}
}

func TestSingleTermsEnum_SeekCeil(t *testing.T) {
	term := NewTerm("field", "hello")
	st := NewSingleTermsEnum(term, 1, 1)

	// SeekCeil with term before ours
	beforeTerm := NewTerm("field", "alpha")
	gotTerm, err := st.SeekCeil(beforeTerm)
	if err != nil {
		t.Fatalf("SeekCeil error: %v", err)
	}
	if gotTerm == nil || !gotTerm.Equals(term) {
		t.Error("SeekCeil should return term when seeking before it")
	}

	// SeekCeil with term after ours
	st2 := NewSingleTermsEnum(term, 1, 1)
	afterTerm := NewTerm("field", "zebra")
	gotTerm2, err := st2.SeekCeil(afterTerm)
	if err != nil {
		t.Fatalf("SeekCeil error: %v", err)
	}
	if gotTerm2 != nil {
		t.Error("SeekCeil should return nil when seeking after term")
	}

	// SeekCeil with exact term
	st3 := NewSingleTermsEnum(term, 1, 1)
	gotTerm3, err := st3.SeekCeil(term)
	if err != nil {
		t.Fatalf("SeekCeil error: %v", err)
	}
	if gotTerm3 == nil || !gotTerm3.Equals(term) {
		t.Error("SeekCeil should return term when seeking exact match")
	}
}

func TestSingleTermsEnum_Unpositioned(t *testing.T) {
	term := NewTerm("field", "test")
	st := NewSingleTermsEnum(term, 5, 10)

	// Before calling Next, DocFreq and TotalTermFreq should return 0
	docFreq, err := st.DocFreq()
	if err != nil {
		t.Fatalf("DocFreq error: %v", err)
	}
	if docFreq != 0 {
		t.Errorf("Expected DocFreq=0 when unpositioned, got %d", docFreq)
	}

	totalFreq, err := st.TotalTermFreq()
	if err != nil {
		t.Fatalf("TotalTermFreq error: %v", err)
	}
	if totalFreq != 0 {
		t.Errorf("Expected TotalTermFreq=0 when unpositioned, got %d", totalFreq)
	}
}
