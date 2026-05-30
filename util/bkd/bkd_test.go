// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"sort"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file is the Go port of org.apache.lucene.util.bkd.TestBKD
// (Apache Lucene 10.4.0, core/src/test/.../bkd/TestBKD.java, 1750 LOC).
//
// Porting strategy (Sprint 56, task GOC-4308):
//
//   - Java TestBKD extends LuceneTestCase and relies on a broad surface
//     that is not yet ported into Gocene: random Directory wrappers
//     (MockDirectoryWrapper, CorruptingIndexOutput, FilterDirectory with
//     ExtrasFS), LuceneTestCase utilities (random(), atLeast, VERBOSE,
//     expectThrows), the codecs MutablePointTree contract, and
//     index.MergeState.
//   - The bulk of TestBKD's randomised behaviour is therefore beyond
//     the scope of this single task. Sprint 55/56 option (c) (port-or-skip
//     where direct gaps exist) is applied: every Java @Test gets a Go
//     counterpart in this file, but tests that require ungated
//     dependencies call t.Skip with the exact missing infrastructure.
//   - Tests that map directly onto already-ported Gocene infrastructure
//     (BKDConfig + BKDWriter + BKDReader + util.IntToSortableBytes +
//     store.ByteBuffersDirectory + the visitor helpers in this package)
//     are implemented end-to-end and exercise the same behaviour as
//     the Java original.
//
// Behavioural coverage already provided by the surrounding test files
// in this package is not re-implemented here; this file's value is
// the explicit 1:1 method map against the Java reference, so that the
// Sprint 55+ follow-ups can flip each Skip into a real implementation
// as the missing dependencies land.

// TestBKD_BasicInts1D mirrors testBasicInts1D: write 100 sorted 1D ints
// (docID == value), then range-query [42, 87] and assert the hit set.
// This is the simplest end-to-end Java test and ports cleanly onto the
// existing BKDWriter + BKDReader pair.
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

// TestBKD_RandomIntsNDims mirrors testRandomIntsNDims: randomised N-dim
// int points with a random sub-range query verified against ground truth.
//
// Skipped until: a Gocene equivalent of LuceneTestCase's random() seeding
// harness is available so the test is reproducible on failure.
func TestBKD_RandomIntsNDims(t *testing.T) {
	t.Fatal("requires LuceneTestCase random() seeding harness; deferred to Sprint 56+")
}

// TestBKD_BigIntNDims mirrors testBigIntNDims: same as the N-dim random
// test but with java.math.BigInteger packed values (variable byte width).
//
// Skipped until: util.BigIntToSortableBytes round-trip helpers are
// wrapped with the Gocene test fixtures and a reproducible random
// seed harness is in place.
func TestBKD_BigIntNDims(t *testing.T) {
	t.Fatal("requires reproducible random seeding and BigInt fixture harness; deferred")
}

// TestBKD_WithExceptions mirrors testWithExceptions: drives the writer
// against a Directory that injects IOExceptions at random points and
// asserts the writer's failure-recovery contract.
//
// Skipped until: a Gocene port of MockDirectoryWrapper +
// CorruptingIndexOutput exists. Neither has a counterpart in
// store/ at the time of writing.
func TestBKD_WithExceptions(t *testing.T) {
	t.Fatal("requires MockDirectoryWrapper + CorruptingIndexOutput ports; not in store/ yet")
}

// TestBKD_RandomBinaryTiny mirrors testRandomBinaryTiny: doTestRandomBinary(10).
//
// Skipped: doTestRandomBinary() depends on the verify() helper, which in
// turn depends on the MutablePointTree-based reopen path and on
// LuceneTestCase utilities (atLeast, random(), TestUtil.nextInt).
func TestBKD_RandomBinaryTiny(t *testing.T) {
	t.Fatal("requires verify() helper + MutablePointTree-based reopen; not yet ported")
}

// TestBKD_RandomBinaryMedium mirrors testRandomBinaryMedium:
// doTestRandomBinary(10000).
func TestBKD_RandomBinaryMedium(t *testing.T) {
	t.Fatal("requires verify() helper; see TestBKD_RandomBinaryTiny")
}

// TestBKD_RandomBinaryBig mirrors testRandomBinaryBig (@Nightly):
// doTestRandomBinary(200000).
func TestBKD_RandomBinaryBig(t *testing.T) {
	t.Fatal("requires verify() helper and @Nightly gating; deferred")
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
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("NewBKDWriter error: got %q, want it to contain %q", err.Error(), want)
	}
}

// TestBKD_AllEqual mirrors testAllEqual: every doc has the same packed
// value across every dim; the writer must still produce a valid index.
//
// Skipped until: verify() helper port lands; this test relies on the
// shared randomised verification scaffolding.
func TestBKD_AllEqual(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
}

// TestBKD_IndexDimEqualDataDimDifferent mirrors
// testIndexDimEqualDataDimDifferent: index dims share a single value;
// data dims vary.
func TestBKD_IndexDimEqualDataDimDifferent(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
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

// TestBKD_OneDimLowCard mirrors testOneDimLowCard: one dim takes one of
// two values, forcing many splits on that dim.
func TestBKD_OneDimLowCard(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
}

// TestBKD_OneDimTwoValues mirrors testOneDimTwoValues: one dim takes one
// of two values; should trigger run-length compression with run lengths
// greater than 255.
func TestBKD_OneDimTwoValues(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
}

// TestBKD_RandomFewDifferentValues mirrors testRandomFewDifferentValues:
// few cardinalities across many docs, exercising the low-cardinality
// leaf path.
func TestBKD_RandomFewDifferentValues(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
}

// TestBKD_MultiValued mirrors testMultiValued (~500 LOC in Java): a single
// doc carries multiple packed values; checks the BKD writer/reader path
// against a multi-valued points scenario.
//
// Skipped until: the verify() helper accepts (docID -> []packed) and the
// MutablePointTree-based reopen path is wired.
func TestBKD_MultiValued(t *testing.T) {
	t.Fatal("requires multi-valued verify() helper + MutablePointTree reopen; deferred")
}

// TestBKD_BitFlippedOnPartition1 mirrors testBitFlippedOnPartition1: a
// single random bit in the index file is flipped; the reader must
// surface a CorruptIndexException.
//
// Skipped until: a Gocene FilterDirectory + checksum-bypassing
// IndexInput corruption helper exists.
func TestBKD_BitFlippedOnPartition1(t *testing.T) {
	t.Fatal("requires FilterDirectory + IndexInput bit-corruption helper; not yet ported")
}

// TestBKD_BitFlippedOnPartition2 mirrors testBitFlippedOnPartition2:
// same as BitFlippedOnPartition1 but at a different file offset.
func TestBKD_BitFlippedOnPartition2(t *testing.T) {
	t.Fatal("requires bit-corruption helper; see TestBKD_BitFlippedOnPartition1")
}

// TestBKD_TieBreakOrder mirrors testTieBreakOrder: when all points share
// the same value on the split dim, the writer must break ties by docID
// so the output is deterministic across runs.
func TestBKD_TieBreakOrder(t *testing.T) {
	t.Fatal("requires byte-exact comparison against a Java-produced fixture; deferred")
}

// TestBKD_CheckDataDimOptimalOrder mirrors testCheckDataDimOptimalOrder:
// assertion that the writer reorders data dims to minimise leaf-block
// size when index dims < data dims.
func TestBKD_CheckDataDimOptimalOrder(t *testing.T) {
	t.Fatal("requires data-dim reordering inspection hook; not exposed by Gocene BKDWriter yet")
}

// TestBKD_2DLongOrdsOffline mirrors test2DLongOrdsOffline: 2D, 8-byte
// dims, offline (disk-backed) writer path.
func TestBKD_2DLongOrdsOffline(t *testing.T) {
	t.Fatal("requires verify() helper exercising the offline path; deferred")
}

// TestBKD_WastedLeadingBytes mirrors testWastedLeadingBytes: every doc
// has the same leading bytes on every dim, exercising the common-prefix
// compression of the leaf block format.
func TestBKD_WastedLeadingBytes(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
}

// TestBKD_EstimatePointCount mirrors testEstimatePointCount: the reader's
// EstimatePointCount must agree with a manual count for a variety of
// query shapes.
//
// Gocene already has a focused EstimatePointCount test in
// bkd_reader_test.go (TestBKDReader_EstimatePointCount). This Java
// counterpart drives the same code path but over a randomised input;
// it remains skipped until the verify()/random harness lands.
func TestBKD_EstimatePointCount(t *testing.T) {
	t.Fatal("randomised counterpart; see TestBKDReader_EstimatePointCount for the focused port")
}

// TestBKD_TotalPointCountValidation mirrors testTotalPointCountValidation:
// the writer must reject Add() once the declared totalPointCount is
// reached.
func TestBKD_TotalPointCountValidation(t *testing.T) {
	t.Fatal("requires verify() helper and assertion of writer's totalPointCount guard; deferred")
}

// TestBKD_TooManyPoints mirrors testTooManyPoints: Add() must fail once
// totalPointCount is exceeded (multi-dim variant).
func TestBKD_TooManyPoints(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
}

// TestBKD_TooManyPoints1D mirrors testTooManyPoints1D: same as
// TestBKD_TooManyPoints but for the 1D specialised writer path.
func TestBKD_TooManyPoints1D(t *testing.T) {
	t.Fatal("requires verify() helper; deferred")
}

// --- helpers ---------------------------------------------------------

// sortableIntRangeVisitor is the Compare-driven range visitor used by
// TestBKD_BasicInts1D. It mirrors the Java getIntersectVisitor that
// TestBKD builds locally: CELL_INSIDE when the cell is wholly inside
// [queryMin, queryMax] across every dim, CELL_OUTSIDE when wholly
// outside, and CELL_CROSSES otherwise.
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
	// CELL_OUTSIDE: max < queryMin OR min > queryMax
	if compareUnsigned(maxPackedValue, v.queryMin) < 0 {
		return codecs.RelationCellOutsideQuery
	}
	if compareUnsigned(minPackedValue, v.queryMax) > 0 {
		return codecs.RelationCellOutsideQuery
	}
	// CELL_INSIDE: min >= queryMin AND max <= queryMax
	if compareUnsigned(minPackedValue, v.queryMin) >= 0 &&
		compareUnsigned(maxPackedValue, v.queryMax) <= 0 {
		return codecs.RelationCellInsideQuery
	}
	return codecs.RelationCellCrossesQuery
}

func (v *sortableIntRangeVisitor) Grow(count int) {}

// compareUnsigned is the unsigned-byte lexicographic comparison used by
// the sortable-int encoding (Lucene NumericUtils.intToSortableBytes
// flips the sign bit so unsigned byte order == signed int order).
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
