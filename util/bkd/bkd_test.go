// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file contains the core BKD tests that do not require the
// randomised verify() infrastructure. The randomised tests that
// depend on verify() live in bkd_random_test.go; corruption and
// validation-edge tests live in bkd_corruption_test.go.

// ----- Deterministic tests (no verify() dependency) -----

// TestBKD_BasicInts1D mirrors testBasicInts1D: write 100 sorted 1D ints
// (docID == value), then range-query [42, 87] and assert the hit set.
func TestBKD_BasicInts1D(t *testing.T) {
	cfg := mustConfig(t, 1, 1, 4, 2)

	points := make([]selectorPoint, 100)
	scratch := make([]byte, 4)
	for docID := 0; docID < 100; docID++ {
		util.IntToSortableBytes(int32(docID), scratch, 0)
		packed := append([]byte(nil), scratch...)
		points[docID] = selectorPoint{packed: packed, docID: docID}
	}

	f := buildReader(t, cfg, points, 100)

	// Range query: [42, 87] inclusive in sortable-int space.
	queryMin := make([]byte, 4)
	queryMax := make([]byte, 4)
	util.IntToSortableBytes(42, queryMin, 0)
	util.IntToSortableBytes(87, queryMax, 0)

	vis := newSortableIntRangeVisitor(queryMin, queryMax)
	if err := f.r.Intersect(vis); err != nil {
		t.Fatalf("Intersect: %v", err)
	}

	hits := make(map[int]struct{}, len(vis.visitedIDs))
	for _, id := range vis.visitedIDs {
		hits[id] = struct{}{}
	}

	for docID := 0; docID < 100; docID++ {
		want := docID >= 42 && docID <= 87
		_, got := hits[docID]
		if want != got {
			t.Fatalf("docID=%d: want=%v got=%v", docID, want, got)
		}
	}
}

// TestBKD_OneDimEqual mirrors testOneDimEqual: exactly one dim is held
// constant across all docs, all other dims are random; small leaf size
// forces many splits.
//
// We materialise the deterministic shape (no randomness) end-to-end:
// 16 docs in 2 dims, 4 bytes per dim, dim 0 equal across all docs, dim 1
// monotonically increasing. The query is a full-range scan, so all 16
// docs must come back.
func TestBKD_OneDimEqual(t *testing.T) {
	cfg := mustConfig(t, 2, 2, 4, 4)
	const numDocs = 16

	constDim := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	points := make([]selectorPoint, numDocs)
	for i := 0; i < numDocs; i++ {
		packed := make([]byte, 8)
		copy(packed[:4], constDim)
		copy(packed[4:], be4(uint32(i+1)))
		points[i] = selectorPoint{packed: packed, docID: i}
	}
	f := buildReader(t, cfg, points, numDocs)

	vis := &readerCaptureVisitor{relation: codecs.RelationCellInsideQuery}
	if err := f.r.Intersect(vis); err != nil {
		t.Fatalf("Intersect: %v", err)
	}

	sort.Ints(vis.visitedIDs)
	if len(vis.visitedIDs) != numDocs {
		t.Fatalf("visited count: got %d, want %d", len(vis.visitedIDs), numDocs)
	}
	for i, id := range vis.visitedIDs {
		if id != i {
			t.Fatalf("visited[%d]: got docID=%d, want %d", i, id, i)
		}
	}
}

// TestBKD_TooLittleHeap mirrors testTooLittleHeap: NewBKDWriter must
// reject a (maxPointsInLeafNode, maxMBSortInHeap) pair where the heap
// cannot hold even a single full leaf, with the canonical error message.
func TestBKD_TooLittleHeap(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	cfg, err := NewBKDConfig(1, 1, 16, 1_000_000)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}

	_, err = NewBKDWriter(1, dir, "bkd", cfg, 0.001, 0)
	if err == nil {
		t.Fatalf("NewBKDWriter: expected error, got nil")
	}
	const want = "either increase maxMBSortInHeap or decrease "
	if !contains(err.Error(), want) {
		t.Fatalf("NewBKDWriter error: got %q, want it to contain %q", err.Error(), want)
	}
}

// ----- Tests that use error-injecting infrastructure -----

// TestBKD_WithExceptions mirrors testWithExceptions: drives the writer
// against a Directory that injects IOExceptions at random points and
// asserts the writer's failure-recovery contract.
//
// Ported using a test-local nthOutputCorruptingDir (bkd_corruption_test.go)
// that wraps the second temp output with a corruptingIndexOutput.
func TestBKD_WithExceptions(t *testing.T) {
	rng := verifyRNG(t)
	numDocs := 1000 + rng.Intn(9001) // ~1000-10000
	numBytesPerDim := 2 + rng.Intn(9) // [2, 10]
	numDataDims := 1 + rng.Intn(MaxDims)
	numIndexDims := 1 + rng.Intn(numDataDims)
	if numIndexDims > MaxIndexDims {
		numIndexDims = MaxIndexDims
	}

	docValues := make([][][]byte, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		values := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			buf := make([]byte, numBytesPerDim)
			rng.Read(buf)
			values[dim] = buf
		}
		docValues[docID] = values
	}

	baseDir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = baseDir.Close() })

	dir := &nthOutputCorruptingDir{
		ByteBuffersDirectory: baseDir,
		corruptAt:           2,
		byteToCorrupt:       12,
	}

	err := captureVerifyError(t, rng, dir, docValues, nil, numDataDims, numIndexDims, numBytesPerDim)
	if err == nil {
		t.Fatal("expected error from corruption, got nil")
	}
	// Any error is acceptable; the Java test accepts Exception.class.
}

// ----- Helpers -----------------------------------------------------------

// sortableIntRangeVisitor is the Compare-driven range visitor used by
// TestBKD_BasicInts1D.
type sortableIntRangeVisitor struct {
	queryMin   []byte
	queryMax   []byte
	visitedIDs []int
}

func newSortableIntRangeVisitor(min, max []byte) *sortableIntRangeVisitor {
	return &sortableIntRangeVisitor{
		queryMin: append([]byte(nil), min...),
		queryMax: append([]byte(nil), max...),
	}
}

func (v *sortableIntRangeVisitor) Visit(docID int) error {
	v.visitedIDs = append(v.visitedIDs, docID)
	return nil
}

func (v *sortableIntRangeVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if compareUnsigned(packedValue, v.queryMin) < 0 {
		return nil
	}
	if compareUnsigned(packedValue, v.queryMax) > 0 {
		return nil
	}
	v.visitedIDs = append(v.visitedIDs, docID)
	return nil
}

func (v *sortableIntRangeVisitor) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	if compareUnsigned(maxPackedValue, v.queryMin) < 0 {
		return codecs.RelationCellOutsideQuery
	}
	if compareUnsigned(minPackedValue, v.queryMax) > 0 {
		return codecs.RelationCellOutsideQuery
	}
	if compareUnsigned(minPackedValue, v.queryMin) >= 0 &&
		compareUnsigned(maxPackedValue, v.queryMax) <= 0 {
		return codecs.RelationCellInsideQuery
	}
	return codecs.RelationCellCrossesQuery
}

func (v *sortableIntRangeVisitor) Grow(count int) {}

// compareUnsigned is the unsigned-byte lexicographic comparison.
func compareUnsigned(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

// contains reports whether substr is in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
