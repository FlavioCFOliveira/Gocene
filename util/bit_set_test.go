// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

// TestBitSet_FixedBitSetSatisfiesInterface confirms FixedBitSet
// satisfies the [BitSet] interface (and hence the Lucene abstract
// base class contract). The compile-time assertion lives in
// util/bit_set.go; this test ensures runtime invocations work.
func TestBitSet_FixedBitSetSatisfiesInterface(t *testing.T) {
	fbs, err := NewFixedBitSet(16)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	var bs BitSet = fbs
	bs.Set(3)
	if !bs.Get(3) {
		t.Error("Get(3) after Set(3) returned false")
	}
	if prev := bs.GetAndSet(5); prev {
		t.Error("GetAndSet on unset bit should return false")
	}
	if prev := bs.GetAndSet(5); !prev {
		t.Error("GetAndSet on now-set bit should return true")
	}
	if got := bs.Cardinality(); got != 2 {
		t.Errorf("Cardinality = %d, want 2", got)
	}
	if got := bs.ApproximateCardinality(); got != 2 {
		t.Errorf("ApproximateCardinality = %d, want 2", got)
	}

	bs.ClearRange(3, 6)
	if bs.Get(3) || bs.Get(5) {
		t.Error("ClearRange did not clear bits 3 and 5")
	}

	if got := bs.NextSetBitBounded(0); got != NO_MORE_DOCS {
		t.Errorf("NextSetBitBounded after ClearRange = %d, want NO_MORE_DOCS", got)
	}
	bs.Set(10)
	if got := bs.NextSetBitInRange(0, 16); got != 10 {
		t.Errorf("NextSetBitInRange = %d, want 10", got)
	}
	if got := bs.NextSetBitInRange(0, 5); got != NO_MORE_DOCS {
		t.Errorf("NextSetBitInRange bounded should not see bit 10, got %d", got)
	}
	if got := bs.PrevSetBit(15); got != 10 {
		t.Errorf("PrevSetBit(15) = %d, want 10", got)
	}
	if got := bs.RamBytesUsed(); got <= 0 {
		t.Errorf("RamBytesUsed should be > 0, got %d", got)
	}
}

// TestBitSet_SparseFixedBitSetSatisfiesInterface mirrors the same
// contract for the sparse implementation.
func TestBitSet_SparseFixedBitSetSatisfiesInterface(t *testing.T) {
	sfs, err := NewSparseFixedBitSet(128)
	if err != nil {
		t.Fatalf("NewSparseFixedBitSet: %v", err)
	}
	var bs BitSet = sfs
	bs.Set(7)
	bs.Set(64)
	if !bs.Get(7) || !bs.Get(64) {
		t.Error("Set/Get round-trip failed on sparse BitSet")
	}
	if got := bs.NextSetBitBounded(0); got != 7 {
		t.Errorf("NextSetBitBounded(0) = %d, want 7", got)
	}
	if got := bs.NextSetBitInRange(10, 128); got != 64 {
		t.Errorf("NextSetBitInRange(10,128) = %d, want 64", got)
	}
}

// rangeIterator is a minimal DocIdSetIterator that emits a fixed set
// of doc IDs, used to exercise OfDocIdSetIterator.
type rangeIterator struct {
	docs []int
	pos  int
	cost int64
}

func (r *rangeIterator) DocID() int {
	if r.pos == 0 {
		return -1
	}
	if r.pos > len(r.docs) {
		return NO_MORE_DOCS
	}
	return r.docs[r.pos-1]
}
func (r *rangeIterator) NextDoc() (int, error) {
	if r.pos >= len(r.docs) {
		r.pos++
		return NO_MORE_DOCS, nil
	}
	doc := r.docs[r.pos]
	r.pos++
	return doc, nil
}
func (r *rangeIterator) Advance(target int) (int, error) {
	for r.pos < len(r.docs) && r.docs[r.pos] < target {
		r.pos++
	}
	if r.pos >= len(r.docs) {
		return NO_MORE_DOCS, nil
	}
	doc := r.docs[r.pos]
	r.pos++
	return doc, nil
}
func (r *rangeIterator) Cost() int64      { return r.cost }
func (r *rangeIterator) DocIDRunEnd() int { return r.DocID() + 1 }

// TestBitSet_OfDocIdSetIterator_FixedBranch confirms the dense branch
// is taken when cost >= maxDoc/128 and returns a *FixedBitSet.
func TestBitSet_OfDocIdSetIterator_FixedBranch(t *testing.T) {
	docs := []int{1, 2, 3, 10, 20, 30, 40, 50, 60, 62}
	iter := &rangeIterator{docs: docs, cost: 10}
	bs, err := OfDocIdSetIterator(iter, 64) // 64/128 = 0; cost=10 >= 0
	if err != nil {
		t.Fatalf("OfDocIdSetIterator: %v", err)
	}
	if _, ok := bs.(*FixedBitSet); !ok {
		t.Errorf("Expected dense branch (*FixedBitSet), got %T", bs)
	}
	for _, d := range docs {
		if !bs.Get(d) {
			t.Errorf("bit %d not set", d)
		}
	}
}

// TestBitSet_OfDocIdSetIterator_SparseBranch confirms the sparse
// branch is taken when cost is well below maxDoc/128.
func TestBitSet_OfDocIdSetIterator_SparseBranch(t *testing.T) {
	docs := []int{5}
	iter := &rangeIterator{docs: docs, cost: 1}
	maxDoc := 4096 // threshold = 32, cost=1 < 32 -> sparse branch
	bs, err := OfDocIdSetIterator(iter, maxDoc)
	if err != nil {
		t.Fatalf("OfDocIdSetIterator: %v", err)
	}
	if _, ok := bs.(*SparseFixedBitSet); !ok {
		t.Errorf("Expected sparse branch (*SparseFixedBitSet), got %T", bs)
	}
	if !bs.Get(5) {
		t.Error("bit 5 not set")
	}
}
