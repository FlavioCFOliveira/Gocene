// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"sort"
	"testing"
)

func collectRoaring(t *testing.T, set *RoaringDocIdSet) []int {
	t.Helper()
	it := set.Iterator()
	if it == nil {
		return nil
	}
	var got []int
	for {
		d, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			return got
		}
		got = append(got, d)
	}
}

func TestRoaringDocIdSet_Empty(t *testing.T) {
	b := NewRoaringDocIdSetBuilder(1024)
	set := b.Build()
	if set.Cardinality() != 0 {
		t.Fatalf("Cardinality()=%d want 0", set.Cardinality())
	}
	if it := set.Iterator(); it != nil {
		t.Fatalf("Iterator() on empty set must be nil, got %T", it)
	}
}

func TestRoaringDocIdSet_SparseBlock(t *testing.T) {
	docs := []int{0, 7, 42, 1000, 65535}
	b := NewRoaringDocIdSetBuilder(1 << 17)
	for _, d := range docs {
		if err := b.Add(d); err != nil {
			t.Fatalf("Add(%d): %v", d, err)
		}
	}
	set := b.Build()
	if got := set.Cardinality(); got != len(docs) {
		t.Fatalf("Cardinality()=%d want %d", got, len(docs))
	}
	if got := collectRoaring(t, set); !equalInts(got, docs) {
		t.Fatalf("iteration mismatch: got=%v want=%v", got, docs)
	}
}

func TestRoaringDocIdSet_MultipleBlocks(t *testing.T) {
	docs := []int{0, 5, 70000, 70001, 1 << 17, (1 << 17) + 5}
	b := NewRoaringDocIdSetBuilder(2 << 17)
	for _, d := range docs {
		if err := b.Add(d); err != nil {
			t.Fatalf("Add(%d): %v", d, err)
		}
	}
	set := b.Build()
	if got := collectRoaring(t, set); !equalInts(got, docs) {
		t.Fatalf("iteration mismatch: got=%v want=%v", got, docs)
	}
}

func TestRoaringDocIdSet_DenseBlock(t *testing.T) {
	// Add MAX_ARRAY_LENGTH + 100 docs in the first block to trigger
	// the fixed-bitset encoding.
	n := roaringMaxArrayLength + 100
	docs := make([]int, n)
	for i := range docs {
		docs[i] = i * 3 // < 65536 for n=4196 -> ok
	}
	b := NewRoaringDocIdSetBuilder(1 << 17)
	for _, d := range docs {
		if err := b.Add(d); err != nil {
			t.Fatalf("Add(%d): %v", d, err)
		}
	}
	set := b.Build()
	if set.Cardinality() != n {
		t.Fatalf("Cardinality()=%d want %d", set.Cardinality(), n)
	}
	if got := collectRoaring(t, set); !equalInts(got, docs) {
		t.Fatalf("iteration mismatch: len(got)=%d want=%d", len(got), len(docs))
	}
}

func TestRoaringDocIdSet_SuperDenseBlockInverse(t *testing.T) {
	// Fill almost the entire first block, leaving fewer than
	// MAX_ARRAY_LENGTH gaps. This triggers the inverse encoding path.
	missing := map[int]struct{}{17: {}, 99: {}, 12345: {}}
	docs := make([]int, 0, roaringBlockSize-len(missing))
	for i := 0; i < roaringBlockSize; i++ {
		if _, skip := missing[i]; skip {
			continue
		}
		docs = append(docs, i)
	}
	b := NewRoaringDocIdSetBuilder(roaringBlockSize)
	for _, d := range docs {
		if err := b.Add(d); err != nil {
			t.Fatalf("Add(%d): %v", d, err)
		}
	}
	set := b.Build()
	if set.Cardinality() != len(docs) {
		t.Fatalf("Cardinality()=%d want %d", set.Cardinality(), len(docs))
	}
	got := collectRoaring(t, set)
	if !equalInts(got, docs) {
		t.Fatalf("iteration mismatch")
	}
}

func TestRoaringDocIdSet_AdvanceAcrossBlocks(t *testing.T) {
	docs := []int{0, 5, 65535, 65536, 70000, 200000}
	b := NewRoaringDocIdSetBuilder(300000)
	for _, d := range docs {
		if err := b.Add(d); err != nil {
			t.Fatalf("Add(%d): %v", d, err)
		}
	}
	it := b.Build().Iterator()
	if d, _ := it.Advance(0); d != 0 {
		t.Fatalf("Advance(0)=%d want 0", d)
	}
	if d, _ := it.Advance(65536); d != 65536 {
		t.Fatalf("Advance(65536)=%d want 65536", d)
	}
	if d, _ := it.Advance(100000); d != 200000 {
		t.Fatalf("Advance(100000)=%d want 200000", d)
	}
	if d, _ := it.Advance(200001); d != NO_MORE_DOCS {
		t.Fatalf("Advance past end = %d want NO_MORE_DOCS", d)
	}
}

func TestRoaringDocIdSet_OutOfOrderRejected(t *testing.T) {
	b := NewRoaringDocIdSetBuilder(1024)
	_ = b.Add(10)
	if err := b.Add(10); err == nil {
		t.Fatalf("expected error on duplicate doc id")
	}
	if err := b.Add(5); err == nil {
		t.Fatalf("expected error on out-of-order doc id")
	}
}

func TestRoaringDocIdSet_RandomizedRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(0xc0ffee))
	const maxDoc = 400000
	want := make([]int, 0, 1000)
	seen := make(map[int]struct{}, 1000)
	for len(want) < 1000 {
		d := rng.Intn(maxDoc)
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		want = append(want, d)
	}
	sort.Ints(want)
	b := NewRoaringDocIdSetBuilder(maxDoc)
	for _, d := range want {
		if err := b.Add(d); err != nil {
			t.Fatalf("Add(%d): %v", d, err)
		}
	}
	set := b.Build()
	if got := collectRoaring(t, set); !equalInts(got, want) {
		t.Fatalf("round-trip mismatch: got len=%d want len=%d", len(got), len(want))
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
