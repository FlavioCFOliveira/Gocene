// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestBitSetIterator_NegativeCostPanics mirrors the Java
// IllegalArgumentException.
func TestBitSetIterator_NegativeCostPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewBitSetIterator should panic on negative cost")
		}
	}()
	fbs, _ := NewFixedBitSet(8)
	_ = NewBitSetIterator(fbs, -1)
}

// TestBitSetIterator_NextDocOverFixedBitSet exercises NextDoc on a
// FixedBitSet-backed iterator.
func TestBitSetIterator_NextDocOverFixedBitSet(t *testing.T) {
	fbs, _ := NewFixedBitSet(64)
	for _, d := range []int{0, 5, 7, 32, 63} {
		fbs.Set(d)
	}
	it := NewBitSetIterator(fbs, int64(fbs.Cardinality()))

	want := []int{0, 5, 7, 32, 63, NO_MORE_DOCS}
	for i, exp := range want {
		got, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if got != exp {
			t.Errorf("NextDoc[%d] = %d, want %d", i, got, exp)
		}
	}
}

// TestBitSetIterator_NextDocOverSparseFixedBitSet exercises NextDoc on
// a SparseFixedBitSet-backed iterator.
func TestBitSetIterator_NextDocOverSparseFixedBitSet(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(4096)
	for _, d := range []int{0, 100, 1000, 4095} {
		sfs.Set(d)
	}
	it := NewBitSetIterator(sfs, int64(sfs.Cardinality()))

	want := []int{0, 100, 1000, 4095, NO_MORE_DOCS}
	for i, exp := range want {
		got, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if got != exp {
			t.Errorf("NextDoc[%d] = %d, want %d", i, got, exp)
		}
	}
}

// TestBitSetIterator_AdvanceLandsAtTargetOrNext verifies Advance lands
// on the first set bit at or after target.
func TestBitSetIterator_AdvanceLandsAtTargetOrNext(t *testing.T) {
	fbs, _ := NewFixedBitSet(64)
	for _, d := range []int{5, 10, 20} {
		fbs.Set(d)
	}
	it := NewBitSetIterator(fbs, int64(fbs.Cardinality()))

	got, _ := it.Advance(7)
	if got != 10 {
		t.Errorf("Advance(7) = %d, want 10", got)
	}
	got, _ = it.Advance(20)
	if got != 20 {
		t.Errorf("Advance(20) = %d, want 20", got)
	}
	got, _ = it.Advance(64)
	if got != NO_MORE_DOCS {
		t.Errorf("Advance past length = %d, want NO_MORE_DOCS", got)
	}
}

// TestBitSetIterator_SetDocIdMutatesPosition verifies SetDocId allows
// callers to forcibly reposition.
func TestBitSetIterator_SetDocIdMutatesPosition(t *testing.T) {
	fbs, _ := NewFixedBitSet(16)
	fbs.Set(3)
	fbs.Set(10)
	it := NewBitSetIterator(fbs, 2)

	it.SetDocId(2)
	got, _ := it.NextDoc()
	if got != 3 {
		t.Errorf("after SetDocId(2), NextDoc = %d, want 3", got)
	}
}

// TestBitSetIterator_GetFixedBitSetOrNull mirrors the Java static
// helper: returns the wrapped FixedBitSet, or nil for other shapes.
func TestBitSetIterator_GetFixedBitSetOrNull(t *testing.T) {
	fbs, _ := NewFixedBitSet(8)
	it := NewBitSetIterator(fbs, 0)
	if got := GetFixedBitSetOrNull(it); got != fbs {
		t.Errorf("GetFixedBitSetOrNull = %v, want %v", got, fbs)
	}
	sfs, _ := NewSparseFixedBitSet(128)
	itSparse := NewBitSetIterator(sfs, 0)
	if got := GetFixedBitSetOrNull(itSparse); got != nil {
		t.Errorf("GetFixedBitSetOrNull on sparse should be nil, got %v", got)
	}
	// Not a BitSetIterator at all.
	if got := GetFixedBitSetOrNull(EmptyDocIdSetIterator()); got != nil {
		t.Errorf("GetFixedBitSetOrNull on unrelated iterator should be nil, got %v", got)
	}
}

// TestBitSetIterator_GetSparseFixedBitSetOrNull mirrors the Java
// static helper.
func TestBitSetIterator_GetSparseFixedBitSetOrNull(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(128)
	it := NewBitSetIterator(sfs, 0)
	if got := GetSparseFixedBitSetOrNull(it); got != sfs {
		t.Errorf("GetSparseFixedBitSetOrNull = %v, want %v", got, sfs)
	}
	fbs, _ := NewFixedBitSet(8)
	itFixed := NewBitSetIterator(fbs, 0)
	if got := GetSparseFixedBitSetOrNull(itFixed); got != nil {
		t.Errorf("GetSparseFixedBitSetOrNull on fixed should be nil, got %v", got)
	}
}

// TestBitSetIterator_DocIDRunEnd verifies the run-length helper.
func TestBitSetIterator_DocIDRunEnd(t *testing.T) {
	fbs, _ := NewFixedBitSet(16)
	for i := 3; i < 7; i++ {
		fbs.Set(i)
	}
	it := NewBitSetIterator(fbs, 4)
	it.NextDoc() // at 3
	if got := it.DocIDRunEnd(); got != 7 {
		t.Errorf("DocIDRunEnd = %d, want 7", got)
	}
	if it.DocID() != 6 {
		t.Errorf("after DocIDRunEnd, DocID = %d, want 6", it.DocID())
	}
}
