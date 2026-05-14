// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

// TestBitDocIdSet_NewWithBitSetSparse verifies the BitSet-typed
// constructor accepts a *SparseFixedBitSet, mirroring Lucene's
// BitDocIdSet(BitSet, long).
func TestBitDocIdSet_NewWithBitSetSparse(t *testing.T) {
	sparse, err := NewSparseFixedBitSet(1024)
	if err != nil {
		t.Fatalf("NewSparseFixedBitSet: %v", err)
	}
	sparse.Set(10)
	sparse.Set(100)
	sparse.Set(1023)

	set, err := NewBitDocIdSetWithBitSet(sparse, 3)
	if err != nil {
		t.Fatalf("NewBitDocIdSetWithBitSet: %v", err)
	}
	if set.Cost() != 3 {
		t.Errorf("Cost = %d, want 3", set.Cost())
	}
	if set.BitSet() != sparse {
		t.Error("BitSet() did not return the wrapped sparse set")
	}
	if set.Bits() != nil {
		t.Error("Bits() should return nil for non-FixedBitSet wrappers")
	}
}

// TestBitDocIdSet_NewWithBitSetNegativeCostErrors verifies the
// IllegalArgumentException analogue.
func TestBitDocIdSet_NewWithBitSetNegativeCostErrors(t *testing.T) {
	fs, err := NewFixedBitSet(10)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	if _, err := NewBitDocIdSetWithBitSet(fs, -1); err == nil {
		t.Error("NewBitDocIdSetWithBitSet must reject negative cost")
	}
}

// TestBitDocIdSet_StringMirrorsLucene verifies the toString output
// follows the Lucene reference: "BitDocIdSet(set=<set>,cost=<cost>)".
func TestBitDocIdSet_StringMirrorsLucene(t *testing.T) {
	fs, err := NewFixedBitSet(4)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	fs.Set(1)
	set, err := NewBitDocIdSetWithBitSet(fs, 1)
	if err != nil {
		t.Fatalf("NewBitDocIdSetWithBitSet: %v", err)
	}
	got := set.String()
	if !strings.HasPrefix(got, "BitDocIdSet(set=") {
		t.Errorf("String() = %q, want prefix BitDocIdSet(set=", got)
	}
	if !strings.HasSuffix(got, ",cost=1)") {
		t.Errorf("String() = %q, want suffix ,cost=1)", got)
	}
}
