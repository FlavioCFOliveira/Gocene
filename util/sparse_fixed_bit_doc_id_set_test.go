// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"testing"
)

// This file ports org.apache.lucene.util.TestSparseFixedBitDocIdSet from
// Apache Lucene 10.4.0 (core/src/test/...). The Java test extends
// BaseDocIdSetTestCase<BitDocIdSet> and only overrides copyOf (to build a
// SparseFixedBitSet-backed BitDocIdSet with randomised insertion order)
// and assertEquals (to additionally verify the underlying bits / cardinality).
// All other behaviour is inherited from BaseDocIdSetTestCase, which Gocene
// already mirrors in fixed_bit_doc_id_set_test.go for the FixedBitSet variant.
//
// Helpers reused from that file (same package): assertBitSetEquals,
// nextExpectedDoc, nextExpectedDocAtOrAfter, maxInt.
//
// Source: core/src/test/org/apache/lucene/util/TestSparseFixedBitDocIdSet.java
// Source: core/src/test/org/apache/lucene/tests/util/BaseDocIdSetTestCase.java

// copyOfSparse mirrors TestSparseFixedBitDocIdSet.copyOf(BitSet, int): build
// a SparseFixedBitSet with the same bits as expected, but shuffle the
// insertion order (the data structure is sensitive to insertion order).
func copyOfSparse(t *testing.T, r *rand.Rand, expected []int, length int) *BitDocIdSet {
	t.Helper()
	set, err := NewSparseFixedBitSet(length)
	if err != nil {
		t.Fatalf("NewSparseFixedBitSet(%d): %v", length, err)
	}
	// Drain in chunks of 100_000, shuffling each chunk, to mirror the Java
	// implementation that buffers + shuffles to stress insertion order.
	buf := make([]int, 0, 1024)
	flush := func() {
		r.Shuffle(len(buf), func(i, j int) { buf[i], buf[j] = buf[j], buf[i] })
		for _, d := range buf {
			set.Set(d)
		}
		buf = buf[:0]
	}
	for _, d := range expected {
		buf = append(buf, d)
		if len(buf) >= 100000 {
			flush()
		}
	}
	flush()
	bds, err := NewBitDocIdSetWithBitSet(set, int64(set.ApproximateCardinality()))
	if err != nil {
		t.Fatalf("NewBitDocIdSetWithBitSet: %v", err)
	}
	return bds
}

// assertEqualsSparse mirrors TestSparseFixedBitDocIdSet.assertEquals():
// in addition to the iterator equality checks performed by
// assertBitSetEquals, verify each bit and the cardinality directly on the
// underlying SparseFixedBitSet.
func assertEqualsSparse(t *testing.T, numBits int, expected []int, bds *BitDocIdSet) {
	t.Helper()
	bs, ok := bds.BitSet().(*SparseFixedBitSet)
	if !ok {
		t.Fatalf("expected underlying SparseFixedBitSet, got %T", bds.BitSet())
	}
	// Build an expected lookup table.
	want := make(map[int]struct{}, len(expected))
	for _, d := range expected {
		want[d] = struct{}{}
	}
	for i := 0; i < numBits; i++ {
		_, expectSet := want[i]
		if got := bs.Get(i); got != expectSet {
			t.Fatalf("bit %d: got=%v want=%v", i, got, expectSet)
		}
	}
	if got, w := bs.Cardinality(), len(expected); got != w {
		t.Fatalf("cardinality: got=%d want=%d", got, w)
	}
	// super.assertEquals(...) — exercise the DocIdSet iterator contract.
	assertBitSetEquals(t, numBits, expected, bds)
}

// TestSparseFixedBitDocIdSet_NoBit mirrors BaseDocIdSetTestCase.testNoBit()
// applied to a SparseFixedBitSet-backed BitDocIdSet.
func TestSparseFixedBitDocIdSet_NoBit(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	bds := copyOfSparse(t, r, nil, 1)
	assertEqualsSparse(t, 1, nil, bds)
}

// TestSparseFixedBitDocIdSet_OneBit mirrors BaseDocIdSetTestCase.test1Bit().
func TestSparseFixedBitDocIdSet_OneBit(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	bds := copyOfSparse(t, r, []int{0}, 1)
	assertEqualsSparse(t, 1, []int{0}, bds)

	bds2 := copyOfSparse(t, r, nil, 1)
	assertEqualsSparse(t, 1, nil, bds2)
}

// TestSparseFixedBitDocIdSet_TwoBits mirrors BaseDocIdSetTestCase.test2Bits().
// SparseFixedBitSet requires length >= 1, but two bits implies length >= 2.
func TestSparseFixedBitDocIdSet_TwoBits(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	cases := []struct {
		set0, set1 bool
		expected   []int
	}{
		{false, false, nil},
		{true, false, []int{0}},
		{false, true, []int{1}},
		{true, true, []int{0, 1}},
	}
	for _, tc := range cases {
		bds := copyOfSparse(t, r, tc.expected, 2)
		assertEqualsSparse(t, 2, tc.expected, bds)
		_ = tc.set0
		_ = tc.set1
	}
}

// TestSparseFixedBitDocIdSet_AgainstBitSet mirrors
// BaseDocIdSetTestCase.testAgainstBitSet(): a battery over sizes and
// densities (including the SingleDoc and RegularIncrements scenarios that
// the Java base test exercises).
func TestSparseFixedBitDocIdSet_AgainstBitSet(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	sizes := []int{100, 1000, 10000}
	densities := []float64{0.0, 0.01, 0.1, 0.5, 0.9, 1.0}
	for _, size := range sizes {
		for _, density := range densities {
			expected := make([]int, 0, int(float64(size)*density)+1)
			for i := 0; i < size; i++ {
				if r.Float64() < density {
					expected = append(expected, i)
				}
			}
			bds := copyOfSparse(t, r, expected, size)
			assertEqualsSparse(t, size, expected, bds)
		}
	}
}

// TestSparseFixedBitDocIdSet_SingleDoc mirrors the "one doc" branch of
// BaseDocIdSetTestCase.testAgainstBitSet().
func TestSparseFixedBitDocIdSet_SingleDoc(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	const n = 1000
	for _, d := range []int{0, n / 2, n - 1} {
		bds := copyOfSparse(t, r, []int{d}, n)
		assertEqualsSparse(t, n, []int{d}, bds)
	}
}

// TestSparseFixedBitDocIdSet_RegularIncrements mirrors the "regular
// increments" branch of BaseDocIdSetTestCase.testAgainstBitSet(). The
// Java base test sweeps a few increments; we replicate the spirit.
func TestSparseFixedBitDocIdSet_RegularIncrements(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	const size = 1000
	for inc := 2; inc < 100; inc += 7 {
		expected := make([]int, 0, size/inc+1)
		for d := 0; d < size; d += inc {
			expected = append(expected, d)
		}
		bds := copyOfSparse(t, r, expected, size)
		assertEqualsSparse(t, size, expected, bds)
	}
}

// TestSparseFixedBitDocIdSet_LargeBitSet exercises bits spanning multiple
// 4096-bit blocks (SparseFixedBitSet's internal block size). This is the
// SparseFixedBitSet analogue of TestBitDocIdSet_LargeBitSet.
func TestSparseFixedBitDocIdSet_LargeBitSet(t *testing.T) {
	r := rand.New(rand.NewSource(13))
	expected := []int{0, 63, 64, 127, 128, 4095, 4096, 5000, 9999}
	bds := copyOfSparse(t, r, expected, 10000)
	assertEqualsSparse(t, 10000, expected, bds)
}

// TestSparseFixedBitDocIdSet_ImplementsDocIdSet verifies the interface
// satisfaction at compile time. Equivalent of the same check in
// fixed_bit_doc_id_set_test.go.
func TestSparseFixedBitDocIdSet_ImplementsDocIdSet(t *testing.T) {
	r := rand.New(rand.NewSource(17))
	bds := copyOfSparse(t, r, []int{1, 2, 3}, 100)
	var _ DocIdSet = bds
}

// TestSparseFixedBitDocIdSet_RamBytesUsed mirrors
// BaseDocIdSetTestCase.testRamBytesUsed(). The Java test asserts that
// RamBytesUsed() on the DocIdSet matches the per-instance size measured
// via RamUsageTester. Gocene's BitDocIdSet does not yet expose
// RamBytesUsed (only the underlying SparseFixedBitSet does, via
// SparseFixedBitSet.RamBytesUsed in util/sparse_fixed_bit_set.go).
// Build the wiring verbatim so the test "just works" once
// BitDocIdSet.RamBytesUsed is added in util/bit_doc_id_set.go around
// line 91 (next to Cost()). See the matching skip in
// fixed_bit_doc_id_set_test.go.
func TestSparseFixedBitDocIdSet_RamBytesUsed(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	expected := make([]int, 0, 128)
	for i := 0; i < 1024; i++ {
		if r.Float64() < 0.1 {
			expected = append(expected, i)
		}
	}
	bds := copyOfSparse(t, r, expected, 1024)
	// Underlying SparseFixedBitSet does report RamBytesUsed; the gap is on
	// BitDocIdSet itself. Reference the value so the wiring is exercised
	// once the gap closes.
	bs, ok := bds.BitSet().(*SparseFixedBitSet)
	if !ok {
		t.Fatalf("expected SparseFixedBitSet, got %T", bds.BitSet())
	}
	// BitDocIdSet.RamBytesUsed must delegate to the underlying BitSet.
	bdsRam := bds.RamBytesUsed()
	sfsRam := bs.RamBytesUsed()
	if bdsRam < 0 {
		t.Fatalf("BitDocIdSet.RamBytesUsed() = %d (negative)", bdsRam)
	}
	if bdsRam != sfsRam {
		t.Fatalf("BitDocIdSet.RamBytesUsed() = %d, want SparseFixedBitSet.RamBytesUsed() = %d", bdsRam, sfsRam)
	}
}

// TestSparseFixedBitDocIdSet_IntoBitSet mirrors
// BaseDocIdSetTestCase.testIntoBitSet(). The Java test exercises
// DocIdSetIterator.intoBitSet(upTo, dest, offset).
func TestSparseFixedBitDocIdSet_IntoBitSet(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	expected := make([]int, 0, 128)
	for i := 0; i < 1024; i++ {
		if r.Float64() < 0.1 {
			expected = append(expected, i)
		}
	}
	bds := copyOfSparse(t, r, expected, 1024)
	it := bds.Iterator()
	if it == nil {
		t.Fatal("Iterator returned nil")
	}

	dest, err := NewFixedBitSet(1024)
	if err != nil {
		t.Fatalf("NewFixedBitSet dest: %v", err)
	}
	if err := IntoBitSet(it, 1024, dest, 0); err != nil {
		t.Fatalf("IntoBitSet: %v", err)
	}

	var got []int
	for i := 0; i < dest.Length(); i++ {
		if dest.Get(i) {
			got = append(got, i)
		}
	}
	if len(got) != len(expected) {
		t.Fatalf("IntoBitSet wrote %d docs, want %d", len(got), len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("doc[%d] = %d, want %d", i, got[i], expected[i])
		}
	}
}

// TestSparseFixedBitDocIdSet_IntoBitSetBoundChecks mirrors
// BaseDocIdSetTestCase.testIntoBitSetBoundChecks(). Validates
// IntoBitSet bounds and offset handling.
func TestSparseFixedBitDocIdSet_IntoBitSetBoundChecks(t *testing.T) {
	r := rand.New(rand.NewSource(8))
	bds := copyOfSparse(t, r, []int{20, 42}, 256)

	small, err := NewFixedBitSet(30)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	if err := IntoBitSet(bds.Iterator(), 40, small, 0); err == nil {
		t.Fatal("expected error when destination FixedBitSet is too small")
	}

	it := bds.Iterator()
	if _, err := it.Advance(25); err != nil {
		t.Fatalf("Advance(25): %v", err)
	}
	dest, err := NewFixedBitSet(256)
	if err != nil {
		t.Fatalf("NewFixedBitSet dest: %v", err)
	}
	if err := IntoBitSet(it, 30, dest, 0); err == nil {
		t.Fatal("expected error when iterator is already beyond upTo")
	}

	it2 := bds.Iterator()
	dest2, err := NewFixedBitSet(356)
	if err != nil {
		t.Fatalf("NewFixedBitSet dest2: %v", err)
	}
	if err := IntoBitSet(it2, 256, dest2, 100); err != nil {
		t.Fatalf("IntoBitSet with offset: %v", err)
	}
	if !dest2.Get(120) {
		t.Fatalf("expected bit 120 to be set (20 + offset 100)")
	}
	if !dest2.Get(142) {
		t.Fatalf("expected bit 142 to be set (42 + offset 100)")
	}
	if dest2.Get(20) || dest2.Get(42) {
		t.Fatalf("source bits should not be set without offset")
	}
}
