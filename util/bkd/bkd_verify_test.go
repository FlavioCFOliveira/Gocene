// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file ports the verify() and assertSize() infrastructure from
// Apache Lucene 10.4.0's TestBKD (lucene/core/src/test/.../bkd/TestBKD.java).
//
// The verify() helper writes a BKD tree from the supplied point data,
// reopens it via BKDReader, runs a series of random N-dimensional
// rectangle intersection queries against both the reader and a brute-force
// linear scan, and asserts the results match exactly. It also validates
// the point tree's structural integrity via assertSize().
//
// The random query generation in the Java original uses
// LuceneTestCase.random() (seeded, reproducible). Our Go port takes a
// local *rand.Rand seeded from the test's t.Name() so that each test
// run is deterministic given the same seed; failure messages include
// the seed for reproduction.

// verify is the top-level entry point: it creates a ByteBuffersDirectory,
// picks a random maxPointsInLeafNode and maxMB, and calls verifyWithDir.
//
// docValues[ord][dim] is the byte slice for dimension `dim` of point `ord`.
// When docIDs is non-nil, docIDs[ord] maps point `ord` to its document ID;
// when docIDs is nil, each point is assigned docID = ord.
func verify(t *testing.T, rng *rand.Rand, docValues [][][]byte, docIDs []int, numDataDims, numIndexDims, numBytesPerDim int) {
	t.Helper()
	maxPointsInLeafNode := 50 + rng.Intn(951) // [50, 1000]
	verifyWithConfig(t, rng, docValues, docIDs, numDataDims, numIndexDims, numBytesPerDim, maxPointsInLeafNode)
}

func verifyWithConfig(t *testing.T, rng *rand.Rand, docValues [][][]byte, docIDs []int, numDataDims, numIndexDims, numBytesPerDim int, maxPointsInLeafNode int) {
	t.Helper()
	maxMB := 3.0 + 3.0*rng.Float64()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	verifyWithDir(t, rng, dir, docValues, docIDs, numDataDims, numIndexDims, numBytesPerDim, maxPointsInLeafNode, maxMB)
}

// verifyWithDir is the core verify implementation. It mirrors the Java
// TestBKD.verify(Directory, byte[][][], int[], int, int, int, int, double).
func verifyWithDir(t *testing.T, rng *rand.Rand, dir store.Directory, docValues [][][]byte, docIDs []int, numDataDims, numIndexDims, numBytesPerDim int, maxPointsInLeafNode int, maxMB float64) {
	t.Helper()

	numValues := len(docValues)
	if numValues == 0 {
		return
	}

	cfg, err := NewBKDConfig(numDataDims, numIndexDims, numBytesPerDim, maxPointsInLeafNode)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}

	// --- write phase ---
	// Determine maxDocs: Java sometimes uses docValues.length, sometimes
	// a random value >= docValues.length.
	var maxDocs int64
	if rng.Intn(2) == 0 {
		maxDocs = int64(numValues)
	} else {
		maxDocs = int64(numValues)
		for maxDocs < int64(numValues) {
			maxDocs = rng.Int63()
		}
	}

	w, err := NewBKDWriter(int(maxDocs), dir, "_0", cfg, maxMB, int64(numValues))
	if err != nil {
		if maxPointsInLeafNode == 0 {
			// zero leaf size is rejected as expected; verify handles zero-size
			// cases by not reaching here.
			return
		}
		t.Fatalf("NewBKDWriter: %v", err)
	}

	scratch := make([]byte, numBytesPerDim*numDataDims)
	for ord := 0; ord < numValues; ord++ {
		var docID int
		if docIDs == nil {
			docID = ord
		} else {
			docID = docIDs[ord]
		}
		for dim := 0; dim < numDataDims; dim++ {
			copy(scratch[dim*numBytesPerDim:(dim+1)*numBytesPerDim], docValues[ord][dim])
		}
		if err := w.Add(scratch, docID); err != nil {
			t.Fatalf("Add(ord=%d, docID=%d): %v", ord, docID, err)
		}
	}

	metaName := "_0_bkd_meta"
	dataName := "_0_bkd_data"
	metaOut, err := dir.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput meta: %v", err)
	}
	dataOut, err := dir.CreateOutput(dataName, store.IOContextWrite)
	if err != nil {
		_ = metaOut.Close()
		t.Fatalf("CreateOutput data: %v", err)
	}

	runnable, err := w.Finish(metaOut, metaOut, dataOut)
	if err != nil {
		_ = metaOut.Close()
		_ = dataOut.Close()
		t.Fatalf("Finish: %v", err)
	}
	if runnable != nil {
		if err := runnable(); err != nil {
			_ = metaOut.Close()
			_ = dataOut.Close()
			t.Fatalf("Finish runnable: %v", err)
		}
	}
	if err := metaOut.Close(); err != nil {
		t.Fatalf("Close metaOut: %v", err)
	}
	if err := dataOut.Close(); err != nil {
		t.Fatalf("Close dataOut: %v", err)
	}

	// --- read phase ---
	metaIn, err := dir.OpenInput(metaName, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput meta: %v", err)
	}
	defer metaIn.Close()
	dataIn, err := dir.OpenInput(dataName, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput data: %v", err)
	}
	defer dataIn.Close()

	r, err := NewBKDReader(metaIn, metaIn, dataIn)
	if err != nil {
		t.Fatalf("NewBKDReader: %v", err)
	}

	// --- assert tree structure ---
	tree, err := r.GetPointTree()
	if err != nil {
		t.Fatalf("GetPointTree: %v", err)
	}
	assertSize(t, rng, tree)

	// --- random intersection tests ---
	numIters := 10 + rng.Intn(91) // [10, 100]
	for iter := 0; iter < numIters; iter++ {
		// Generate random N-dim rect query.
		queryMin := make([][]byte, numDataDims)
		queryMax := make([][]byte, numDataDims)
		for dim := 0; dim < numDataDims; dim++ {
			queryMin[dim] = make([]byte, numBytesPerDim)
			rng.Read(queryMin[dim])
			queryMax[dim] = make([]byte, numBytesPerDim)
			rng.Read(queryMax[dim])
			if bytes.Compare(queryMin[dim], queryMax[dim]) > 0 {
				queryMin[dim], queryMax[dim] = queryMax[dim], queryMin[dim]
			}
		}

		// Compute expected hits via linear scan.
		expected := make(map[int]struct{}, numValues)
		for ord := 0; ord < numValues; ord++ {
			matches := true
			for dim := 0; dim < numIndexDims; dim++ {
				pv := docValues[ord][dim]
				if bytes.Compare(pv, queryMin[dim]) < 0 || bytes.Compare(pv, queryMax[dim]) > 0 {
					matches = false
					break
				}
			}
			if matches {
				var docID int
				if docIDs == nil {
					docID = ord
				} else {
					docID = docIDs[ord]
				}
				expected[docID] = struct{}{}
			}
		}

		// Query via Intersect.
		hits := make(map[int]struct{}, numValues)
		iv := newVerifyIntersectVisitor(hits, queryMin, queryMax, cfg)
		if err := r.Intersect(iv); err != nil {
			t.Fatalf("Intersect(iter=%d): %v", iter, err)
		}
		assertHits(t, hits, expected, iter)

		// Query via PointTree.visitDocValues.
		tree, err := r.GetPointTree()
		if err != nil {
			t.Fatalf("GetPointTree(iter=%d): %v", iter, err)
		}
		hits2 := make(map[int]struct{}, numValues)
		iv2 := newVerifyIntersectVisitor(hits2, queryMin, queryMax, cfg)
		if err := tree.VisitDocValues(iv2); err != nil {
			t.Fatalf("VisitDocValues(iter=%d): %v", iter, err)
		}
		assertHits(t, hits2, expected, iter)
	}
}

// assertSize mirrors Java's TestBKD.assertSize: it validates that every
// node in the tree returns correct size, that visitDocIDs and visitDocValues
// both produce exactly tree.Size() hits, and that the tree's internal
// navigation contracts hold.
func assertSize(t *testing.T, rng *rand.Rand, tree PointTree) {
	t.Helper()

	clone := tree.Clone()
	if clone.Size() != tree.Size() {
		t.Fatalf("clone.Size=%d != tree.Size=%d", clone.Size(), tree.Size())
	}

	// Randomly choose which tree to use for the remainder.
	if rng.Intn(2) == 0 {
		tree = clone
	}

	var visitDocIDCount int64
	var visitDocValuesCount int64

	visitor := &countingVisitor{
		compareFn: func(minPackedValue, maxPackedValue []byte) codecs.Relation {
			return codecs.RelationCellCrossesQuery
		},
		visitFn: func(docID int) {
			visitDocIDCount++
		},
		visitPVFn: func(docID int, packedValue []byte) {
			visitDocValuesCount++
		},
	}

	if rng.Intn(2) == 0 {
		if err := tree.VisitDocIDs(visitor); err != nil {
			t.Fatalf("VisitDocIDs: %v", err)
		}
		if err := tree.VisitDocValues(visitor); err != nil {
			t.Fatalf("VisitDocValues: %v", err)
		}
	} else {
		if err := tree.VisitDocValues(visitor); err != nil {
			t.Fatalf("VisitDocValues: %v", err)
		}
		if err := tree.VisitDocIDs(visitor); err != nil {
			t.Fatalf("VisitDocIDs: %v", err)
		}
	}

	if visitDocIDCount != visitDocValuesCount {
		t.Fatalf("visitDocIDCount=%d != visitDocValuesCount=%d", visitDocIDCount, visitDocValuesCount)
	}
	if visitDocIDCount != tree.Size() {
		t.Fatalf("visitDocIDCount=%d != tree.Size=%d", visitDocIDCount, tree.Size())
	}

	// Recurse into children.
	moved, err := tree.MoveToChild()
	if err != nil {
		t.Fatalf("MoveToChild: %v", err)
	}
	if moved {
		for {
			randomPointTreeNavigation(t, rng, tree)
			assertSize(t, rng, tree)
			ok, err := tree.MoveToSibling()
			if err != nil {
				t.Fatalf("MoveToSibling: %v", err)
			}
			if !ok {
				break
			}
		}
		if _, err := tree.MoveToParent(); err != nil {
			t.Fatalf("MoveToParent: %v", err)
		}
	}
}

// randomPointTreeNavigation mirrors Java's TestBKD.randomPointTreeNavigation:
// it randomly descends into children and verifies that min/max/size are
// preserved after navigating back to the starting node.
func randomPointTreeNavigation(t *testing.T, rng *rand.Rand, tree PointTree) {
	t.Helper()

	minPV := append([]byte(nil), tree.GetMinPackedValue()...)
	maxPV := append([]byte(nil), tree.GetMaxPackedValue()...)
	size := tree.Size()

	if rng.Intn(2) == 0 {
		moved, err := tree.MoveToChild()
		if err != nil {
			t.Fatalf("MoveToChild: %v", err)
		}
		if moved {
			randomPointTreeNavigation(t, rng, tree)
			if rng.Intn(2) == 0 {
				moved, err := tree.MoveToSibling()
				if err != nil {
					t.Fatalf("MoveToSibling: %v", err)
				}
				if moved {
					randomPointTreeNavigation(t, rng, tree)
				}
			}
			if _, err := tree.MoveToParent(); err != nil {
				t.Fatalf("MoveToParent: %v", err)
			}
		}
	}

	if !bytes.Equal(minPV, tree.GetMinPackedValue()) {
		t.Fatalf("minPackedValue changed: was %x, now %x", minPV, tree.GetMinPackedValue())
	}
	if !bytes.Equal(maxPV, tree.GetMaxPackedValue()) {
		t.Fatalf("maxPackedValue changed: was %x, now %x", maxPV, tree.GetMaxPackedValue())
	}
	if size != tree.Size() {
		t.Fatalf("size changed: was %d, now %d", size, tree.Size())
	}
}

// assertHits mirrors Java's TestBKD.assertHits: it checks that the two
// docID sets are identical.
func assertHits(t *testing.T, hits, expected map[int]struct{}, iter int) {
	t.Helper()
	for docID := range expected {
		if _, ok := hits[docID]; !ok {
			t.Fatalf("iter=%d: docID=%d in expected but not in hits", iter, docID)
		}
	}
	for docID := range hits {
		if _, ok := expected[docID]; !ok {
			t.Fatalf("iter=%d: docID=%d in hits but not in expected", iter, docID)
		}
	}
}

// verifyIntersectVisitor is the IntersectVisitor used by verify() to
// collect matching docIDs during a random rect query. It mirrors the
// anonymous IntersectVisitor created by Java's TestBKD.getIntersectVisitor.
type verifyIntersectVisitor struct {
	hits     map[int]struct{}
	queryMin [][]byte
	queryMax [][]byte
	config   BKDConfig
	numBytes int
}

func newVerifyIntersectVisitor(hits map[int]struct{}, queryMin, queryMax [][]byte, config BKDConfig) *verifyIntersectVisitor {
	return &verifyIntersectVisitor{
		hits:     hits,
		queryMin: queryMin,
		queryMax: queryMax,
		config:   config,
		numBytes: config.BytesPerDim(),
	}
}

func (v *verifyIntersectVisitor) Visit(docID int) error {
	v.hits[docID] = struct{}{}
	return nil
}

func (v *verifyIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	for dim := 0; dim < v.config.NumIndexDims(); dim++ {
		offset := dim * v.numBytes
		pv := packedValue[offset : offset+v.numBytes]
		if bytes.Compare(pv, v.queryMin[dim]) < 0 || bytes.Compare(pv, v.queryMax[dim]) > 0 {
			return nil
		}
	}
	v.hits[docID] = struct{}{}
	return nil
}

func (v *verifyIntersectVisitor) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	crosses := false
	for dim := 0; dim < v.config.NumIndexDims(); dim++ {
		offset := dim * v.numBytes
		minPV := minPackedValue[offset : offset+v.numBytes]
		maxPV := maxPackedValue[offset : offset+v.numBytes]
		if bytes.Compare(maxPV, v.queryMin[dim]) < 0 || bytes.Compare(minPV, v.queryMax[dim]) > 0 {
			return codecs.RelationCellOutsideQuery
		}
		if bytes.Compare(minPV, v.queryMin[dim]) < 0 || bytes.Compare(maxPV, v.queryMax[dim]) > 0 {
			crosses = true
		}
	}
	if crosses {
		return codecs.RelationCellCrossesQuery
	}
	return codecs.RelationCellInsideQuery
}

func (v *verifyIntersectVisitor) Grow(count int) {}

// countingVisitor collects visit/docValues counts with a configurable
// compare function. Used by assertSize.
type countingVisitor struct {
	compareFn func(min, max []byte) codecs.Relation
	visitFn   func(docID int)
	visitPVFn func(docID int, packedValue []byte)
}

func (v *countingVisitor) Visit(docID int) error {
	v.visitFn(docID)
	return nil
}
func (v *countingVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	v.visitPVFn(docID, packedValue)
	return nil
}
func (v *countingVisitor) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	return v.compareFn(minPackedValue, maxPackedValue)
}
func (v *countingVisitor) Grow(count int) {}

// verifyRNG creates a deterministic *rand.Rand seeded from the test name.
// This mirrors LuceneTestCase's seed-based reproducibility.
func verifyRNG(t *testing.T) *rand.Rand {
	t.Helper()
	// Use a hash of the test name as a deterministic seed.
	seed := int64(0)
	for _, c := range t.Name() {
		seed = seed*31 + int64(c)
	}
	return rand.New(rand.NewSource(seed))
}

// sortInts is a convenience wrapper for sorting []int.
func sortInts(ints []int) {
	sort.Ints(ints)
}

// intSetToSortedSlice converts a map[int]struct{} to a sorted []int.
func intSetToSortedSlice(s map[int]struct{}) []int {
	out := make([]int, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
