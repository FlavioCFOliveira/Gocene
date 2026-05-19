// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newSortedNumericTestWriter wires a SortedNumericDocValuesWriter with a
// fresh field info and a dedicated counter. SortedNumericDocValuesWriter
// does not need a ByteBlockPool (no BytesRefHash).
func newSortedNumericTestWriter(t *testing.T, name string) (*SortedNumericDocValuesWriter, *util.Counter) {
	t.Helper()
	fi := NewFieldInfo(name, 0, FieldInfoOptions{
		DocValuesType: DocValuesTypeSortedNumeric,
	})
	counter := util.NewCounter()
	return NewSortedNumericDocValuesWriter(fi, counter), counter
}

// drainSortedNumeric walks dv to completion and returns the per-doc values
// in the order they surface.
func drainSortedNumeric(t *testing.T, dv SortedNumericDocValues) map[int][]int64 {
	t.Helper()
	out := map[int][]int64{}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			return out
		}
		vals, err := dv.Get(d)
		if err != nil {
			t.Fatalf("Get(%d): %v", d, err)
		}
		out[d] = vals
	}
}

func equalInt64Slices(a, b []int64) bool {
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

// TestSortedNumericDocValuesWriter_SingleValuedPath asserts the singleton
// fast path: when every doc has exactly one value the writer must not
// allocate the per-doc counts stream.
func TestSortedNumericDocValuesWriter_SingleValuedPath(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	inputs := []struct {
		doc int
		val int64
	}{
		{0, 10}, {1, 20}, {2, 30}, {3, 40},
	}
	for _, in := range inputs {
		if err := w.AddValue(in.doc, in.val); err != nil {
			t.Fatalf("AddValue(%d,%d): %v", in.doc, in.val, err)
		}
	}
	if w.pendingCounts != nil {
		t.Fatalf("pendingCounts must remain nil while every doc has 1 value")
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	// Singleton wrapper exposes the SortedNumericDocValues surface with
	// a one-element slice per doc.
	got := drainSortedNumeric(t, dv)
	want := map[int][]int64{0: {10}, 1: {20}, 2: {30}, 3: {40}}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for d, v := range want {
		if !equalInt64Slices(got[d], v) {
			t.Errorf("doc %d: %v, want %v", d, got[d], v)
		}
	}
}

// TestSortedNumericDocValuesWriter_MultiValuedSortsWithinDoc asserts that
// multiple values added for the same doc surface ascending and that
// duplicates are preserved (Lucene's contract).
func TestSortedNumericDocValuesWriter_MultiValuedSortsWithinDoc(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	// Doc 0: 3,1,2,1 -> sorted 1,1,2,3 (dup preserved).
	for _, v := range []int64{3, 1, 2, 1} {
		if err := w.AddValue(0, v); err != nil {
			t.Fatalf("AddValue: %v", err)
		}
	}
	// Doc 1: 5,5 -> sorted 5,5.
	for _, v := range []int64{5, 5} {
		if err := w.AddValue(1, v); err != nil {
			t.Fatalf("AddValue: %v", err)
		}
	}
	// Doc 2: single value, exercising the back-fill path for prior
	// multi-valued docs is not needed here since pendingCounts was
	// already created at doc 0.
	if err := w.AddValue(2, 7); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	got := drainSortedNumeric(t, dv)
	want := map[int][]int64{0: {1, 1, 2, 3}, 1: {5, 5}, 2: {7}}
	for d, v := range want {
		if !equalInt64Slices(got[d], v) {
			t.Errorf("doc %d: %v, want %v", d, got[d], v)
		}
	}
}

// TestSortedNumericDocValuesWriter_PromoteCountsBackfill exercises the
// path where the first N docs are single-valued and the (N+1)th becomes
// multi-valued, forcing pendingCounts to be allocated and back-filled
// with 1s for every prior single-valued doc.
func TestSortedNumericDocValuesWriter_PromoteCountsBackfill(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	for i := 0; i < 5; i++ {
		if err := w.AddValue(i, int64(100+i)); err != nil {
			t.Fatalf("AddValue: %v", err)
		}
	}
	// Doc 5 has two values -> promotes pendingCounts on the next doc
	// boundary (Java collapses promotion in finishCurrentDoc, which only
	// runs when the current doc changes or on GetDocValues/Flush).
	if err := w.AddValue(5, 200); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	if err := w.AddValue(5, 100); err != nil {
		t.Fatalf("AddValue: %v", err)
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	if w.pendingCounts == nil {
		t.Fatalf("pendingCounts must be allocated after promotion")
	}
	got := drainSortedNumeric(t, dv)
	want := map[int][]int64{
		0: {100}, 1: {101}, 2: {102}, 3: {103}, 4: {104}, 5: {100, 200},
	}
	for d, v := range want {
		if !equalInt64Slices(got[d], v) {
			t.Errorf("doc %d: %v, want %v", d, got[d], v)
		}
	}
}

// TestSortedNumericDocValuesWriter_RejectOutOfOrder covers the docID
// monotonicity error branch.
func TestSortedNumericDocValuesWriter_RejectOutOfOrder(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	if err := w.AddValue(5, 1); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(3, 1); err == nil {
		t.Fatal("expected error for out-of-order docID")
	}
}

// TestSortedNumericDocValuesWriter_RejectNilConsumer covers the Flush
// guard.
func TestSortedNumericDocValuesWriter_RejectNilConsumer(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	if err := w.AddValue(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(1, nil, nil); err == nil {
		t.Fatal("expected error for nil consumer")
	}
}

// TestSortedNumericDocValuesWriter_FlushUnsorted routes the no-sortMap
// branch through Flush and verifies the consumer sees the same data as
// GetDocValues for a multi-valued field.
func TestSortedNumericDocValuesWriter_FlushUnsorted(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	if err := w.AddValue(0, 3); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(1, 7); err != nil {
		t.Fatal(err)
	}
	var got SortedNumericDocValues
	err := w.Flush(2, nil, func(fi *FieldInfo, v SortedNumericDocValues) error {
		if fi.Name() != "f" {
			t.Errorf("consumer field=%q, want %q", fi.Name(), "f")
		}
		got = v
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got == nil {
		t.Fatal("Flush did not invoke consumer")
	}
	drained := drainSortedNumeric(t, got)
	want := map[int][]int64{0: {1, 3}, 1: {7}}
	for d, v := range want {
		if !equalInt64Slices(drained[d], v) {
			t.Errorf("doc %d: %v, want %v", d, drained[d], v)
		}
	}
}

// TestSortedNumericDocValuesWriter_FlushSortedMulti exercises the
// sort-aware multi-valued branch with a reverse sortMap. Values must
// surface in the reversed new-doc order with original within-doc ordering
// preserved.
func TestSortedNumericDocValuesWriter_FlushSortedMulti(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	// Doc 0: {3,1} -> sorted {1,3}; doc 1: {5}; doc 2: {9,9}.
	if err := w.AddValue(0, 3); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(1, 5); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(2, 9); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(2, 9); err != nil {
		t.Fatal(err)
	}
	const maxDoc = 3
	sortMap := &reverseSortMap{n: maxDoc}

	var got map[int][]int64
	err := w.Flush(maxDoc, sortMap, func(_ *FieldInfo, v SortedNumericDocValues) error {
		got = drainSortedNumeric(t, v)
		return nil
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// Reverse: old 0 ({1,3}) -> new 2; old 1 ({5}) -> new 1; old 2 ({9,9}) -> new 0.
	want := map[int][]int64{
		0: {9, 9},
		1: {5},
		2: {1, 3},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for d, v := range want {
		if !equalInt64Slices(got[d], v) {
			t.Errorf("doc %d: %v, want %v", d, got[d], v)
		}
	}
}

// TestSortedNumericDocValuesWriter_SparseDocs exercises the sparse
// DocsWithFieldSet path (non-contiguous docIDs) and checks the docs
// surface in addition order.
func TestSortedNumericDocValuesWriter_SparseDocs(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	pairs := []struct {
		doc int
		val int64
	}{
		{1, 11},
		{4, 44},
		{9, 99},
		{17, 1717},
	}
	for _, p := range pairs {
		if err := w.AddValue(p.doc, p.val); err != nil {
			t.Fatalf("AddValue(%d,%d): %v", p.doc, p.val, err)
		}
	}
	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	got := drainSortedNumeric(t, dv)
	want := map[int][]int64{1: {11}, 4: {44}, 9: {99}, 17: {1717}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for d, v := range want {
		if !equalInt64Slices(got[d], v) {
			t.Errorf("doc %d: %v, want %v", d, got[d], v)
		}
	}
}

// TestSortedNumericDocValuesWriter_BytesUsedReported asserts that adding
// values causes iwBytesUsed to grow monotonically. Exact accounting is
// not contractual.
func TestSortedNumericDocValuesWriter_BytesUsedReported(t *testing.T) {
	w, counter := newSortedNumericTestWriter(t, "f")
	start := counter.Get()
	for i := 0; i < 64; i++ {
		if err := w.AddValue(i, int64(i*7)); err != nil {
			t.Fatal(err)
		}
	}
	end := counter.Get()
	if end <= start {
		t.Errorf("counter did not grow: start=%d end=%d", start, end)
	}
}

// TestSortedNumericDocValuesWriter_GetDocValuesIdempotent asserts that
// repeated GetDocValues calls do not re-finalise the underlying builders
// (Java's finalValues/finalValuesCount sentinel).
func TestSortedNumericDocValuesWriter_GetDocValuesIdempotent(t *testing.T) {
	w, _ := newSortedNumericTestWriter(t, "f")
	if err := w.AddValue(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := w.AddValue(0, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := w.GetDocValues(); err != nil {
		t.Fatalf("GetDocValues #1: %v", err)
	}
	dv2, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues #2: %v", err)
	}
	got := drainSortedNumeric(t, dv2)
	if !equalInt64Slices(got[0], []int64{1, 2}) {
		t.Errorf("doc 0: %v, want [1 2]", got[0])
	}
}
