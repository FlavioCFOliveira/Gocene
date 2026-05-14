// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"testing"
)

// TestBytesRefArray_SortStable_PreservesInsertionOrder mirrors
// TestBytesRefArray.testStableSort's contract: equal-keyed entries
// must keep their relative insertion order in the sort state.
func TestBytesRefArray_SortStable_PreservesInsertionOrder(t *testing.T) {
	bra := NewBytesRefArray(64)
	// Append the same key 5 times so any stable sort must preserve the
	// 0..4 ordinals in increasing order.
	for i := 0; i < 5; i++ {
		bra.AppendBytes([]byte("dup"))
	}

	state := bra.SortStable(func(a, b *BytesRef) bool {
		return bytes.Compare(a.ValidBytes(), b.ValidBytes()) < 0
	})

	var spare BytesRef
	lastOrd := -1
	for i := 0; i < 5; i++ {
		if !state.Next(&spare) {
			t.Fatalf("Next returned false at iteration %d", i)
		}
		ord := state.Ord()
		if ord <= lastOrd {
			t.Errorf("stable sort violated at i=%d: ord=%d, lastOrd=%d", i, ord, lastOrd)
		}
		lastOrd = ord
	}
	if state.Next(&spare) {
		t.Error("Next should return false after the last element")
	}
}

// TestBytesRefArray_SortStable_OrdMatchesInsertionIndex confirms that
// the Ord helper returns the original Append index.
func TestBytesRefArray_SortStable_OrdMatchesInsertionIndex(t *testing.T) {
	bra := NewBytesRefArray(64)
	idxC := bra.AppendBytes([]byte("c"))
	idxA := bra.AppendBytes([]byte("a"))
	idxB := bra.AppendBytes([]byte("b"))

	state := bra.SortStable(func(a, b *BytesRef) bool {
		return bytes.Compare(a.ValidBytes(), b.ValidBytes()) < 0
	})

	var spare BytesRef
	expectedOrds := []int{idxA, idxB, idxC}
	for i, wantOrd := range expectedOrds {
		if !state.Next(&spare) {
			t.Fatalf("Next returned false at i=%d", i)
		}
		if got := state.Ord(); got != wantOrd {
			t.Errorf("Ord at i=%d = %d, want %d", i, got, wantOrd)
		}
	}
}
