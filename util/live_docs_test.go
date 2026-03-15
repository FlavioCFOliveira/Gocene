// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"testing"
)

// collectDocs collects all documents from an iterator into a slice
func collectDocs(it DocIdSetIterator) ([]int, error) {
	var docs []int
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return nil, err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// TestSparseLiveDocsBasic tests basic SparseLiveDocs operations
func TestSparseLiveDocsBasic(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	sparseSet.Set(10)
	sparseSet.Set(50)
	sparseSet.Set(100)

	// WHEN
	liveDocs := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()

	// THEN
	if liveDocs.DeletedCount() != 3 {
		t.Errorf("Expected deleted count 3, got %d", liveDocs.DeletedCount())
	}
	if liveDocs.Get(10) {
		t.Error("Doc 10 should be deleted")
	}
	if liveDocs.Get(50) {
		t.Error("Doc 50 should be deleted")
	}
	if liveDocs.Get(100) {
		t.Error("Doc 100 should be deleted")
	}
	if !liveDocs.Get(11) {
		t.Error("Doc 11 should be live")
	}
	if !liveDocs.Get(51) {
		t.Error("Doc 51 should be live")
	}
}

// TestDenseLiveDocsBasic tests basic DenseLiveDocs operations
func TestDenseLiveDocsBasic(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	fixedSet.Clear(10)
	fixedSet.Clear(50)
	fixedSet.Clear(100)

	// WHEN
	liveDocs := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// THEN
	if liveDocs.DeletedCount() != 3 {
		t.Errorf("Expected deleted count 3, got %d", liveDocs.DeletedCount())
	}
	if liveDocs.Get(10) {
		t.Error("Doc 10 should be deleted")
	}
	if liveDocs.Get(50) {
		t.Error("Doc 50 should be deleted")
	}
	if liveDocs.Get(100) {
		t.Error("Doc 100 should be deleted")
	}
	if !liveDocs.Get(11) {
		t.Error("Doc 11 should be live")
	}
	if !liveDocs.Get(51) {
		t.Error("Doc 51 should be live")
	}
}

// TestSparseLiveDocsIterator tests the deleted docs iterator for SparseLiveDocs
func TestSparseLiveDocsIterator(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	deletedDocs := []int{5, 50, 100, 150, 500, 999}
	for _, doc := range deletedDocs {
		sparseSet.Set(doc)
	}
	liveDocs := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()

	// WHEN
	it := liveDocs.DeletedDocsIterator()
	iteratedDocs, err := collectDocs(it)
	if err != nil {
		t.Fatalf("Failed to collect docs: %v", err)
	}

	// THEN
	if it.Cost() != int64(len(deletedDocs)) {
		t.Errorf("Iterator cost should match deleted count: expected %d, got %d", len(deletedDocs), it.Cost())
	}
	if len(iteratedDocs) != len(deletedDocs) {
		t.Errorf("Should iterate exact number of deleted docs: expected %d, got %d", len(deletedDocs), len(iteratedDocs))
	}
	for i, expected := range deletedDocs {
		if iteratedDocs[i] != expected {
			t.Errorf("Deleted doc mismatch at position %d: expected %d, got %d", i, expected, iteratedDocs[i])
		}
	}
}

// TestDenseLiveDocsIterator tests the deleted docs iterator for DenseLiveDocs
func TestDenseLiveDocsIterator(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	deletedDocs := []int{5, 50, 100, 150, 500, 999}
	for _, doc := range deletedDocs {
		fixedSet.Clear(doc)
	}
	liveDocs := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// WHEN
	it := liveDocs.DeletedDocsIterator()
	iteratedDocs, err := collectDocs(it)
	if err != nil {
		t.Fatalf("Failed to collect docs: %v", err)
	}

	// THEN
	if it.Cost() != int64(len(deletedDocs)) {
		t.Errorf("Iterator cost should match deleted count: expected %d, got %d", len(deletedDocs), it.Cost())
	}
	if len(iteratedDocs) != len(deletedDocs) {
		t.Errorf("Should iterate exact number of deleted docs: expected %d, got %d", len(deletedDocs), len(iteratedDocs))
	}
	for i, expected := range deletedDocs {
		if iteratedDocs[i] != expected {
			t.Errorf("Deleted doc mismatch at position %d: expected %d, got %d", i, expected, iteratedDocs[i])
		}
	}
}

// TestSparseDenseEquivalence tests that SparseLiveDocs and DenseLiveDocs behave identically
func TestSparseDenseEquivalence(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	deletedDocs := []int{1, 10, 50, 100, 200, 500, 750, 999}
	for _, doc := range deletedDocs {
		sparseSet.Set(doc)
		fixedSet.Clear(doc)
	}

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseIt := sparse.DeletedDocsIterator()
	denseIt := dense.DeletedDocsIterator()

	// THEN
	if sparse.DeletedCount() != dense.DeletedCount() {
		t.Errorf("Deleted counts should match: sparse=%d, dense=%d", sparse.DeletedCount(), dense.DeletedCount())
	}
	if sparse.Length() != dense.Length() {
		t.Errorf("Lengths should match: sparse=%d, dense=%d", sparse.Length(), dense.Length())
	}
	for i := 0; i < maxDoc; i++ {
		if sparse.Get(i) != dense.Get(i) {
			t.Errorf("Get(%d) should match: sparse=%v, dense=%v", i, sparse.Get(i), dense.Get(i))
		}
	}

	// Compare iterators
	for {
		sparseDoc, err := sparseIt.NextDoc()
		if err != nil {
			t.Fatalf("Failed to get next doc from sparse: %v", err)
		}
		denseDoc, err := denseIt.NextDoc()
		if err != nil {
			t.Fatalf("Failed to get next doc from dense: %v", err)
		}
		if sparseDoc != denseDoc {
			t.Errorf("Iterators should return same documents: sparse=%d, dense=%d", sparseDoc, denseDoc)
		}
		if sparseDoc == NO_MORE_DOCS {
			break
		}
	}
}

// TestEmptyIterator tests iterators when there are no deleted documents
func TestEmptyIterator(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// WHEN
	sparseIt := sparse.DeletedDocsIterator()
	denseIt := dense.DeletedDocsIterator()

	// THEN
	sparseDoc, err := sparseIt.NextDoc()
	if err != nil {
		t.Fatalf("Failed to get next doc from sparse: %v", err)
	}
	denseDoc, err := denseIt.NextDoc()
	if err != nil {
		t.Fatalf("Failed to get next doc from dense: %v", err)
	}
	if sparseDoc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS for sparse, got %d", sparseDoc)
	}
	if denseDoc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS for dense, got %d", denseDoc)
	}
}

// TestIteratorAdvance tests the Advance method on iterators
func TestIteratorAdvance(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	for i := 10; i <= 50; i += 10 {
		fixedSet.Clear(i)
	}
	liveDocs := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// WHEN
	it := liveDocs.DeletedDocsIterator()

	// THEN
	doc, err := it.Advance(15)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if doc != 20 {
		t.Errorf("Expected advance(15) to return 20, got %d", doc)
	}

	doc, err = it.NextDoc()
	if err != nil {
		t.Fatalf("Failed to get next doc: %v", err)
	}
	if doc != 30 {
		t.Errorf("Expected nextDoc() to return 30, got %d", doc)
	}

	doc, err = it.Advance(45)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if doc != 50 {
		t.Errorf("Expected advance(45) to return 50, got %d", doc)
	}

	doc, err = it.Advance(1000)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected advance(1000) to return NO_MORE_DOCS, got %d", doc)
	}
}

// TestRandomDeletions tests random deletion patterns
func TestRandomDeletions(t *testing.T) {
	// GIVEN
	maxDoc := 5000
	deleteRatio := 0.25
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()

	var expectedDeleted []int
	r := rand.New(rand.NewSource(42))
	for i := 0; i < maxDoc; i++ {
		if r.Float64() < deleteRatio {
			sparseSet.Set(i)
			fixedSet.Clear(i)
			expectedDeleted = append(expectedDeleted, i)
		}
	}

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseDeleted, err := collectDocs(sparse.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse docs: %v", err)
	}
	denseDeleted, err := collectDocs(dense.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense docs: %v", err)
	}

	// THEN
	if sparse.DeletedCount() != len(expectedDeleted) {
		t.Errorf("Sparse deleted count should match expected: expected %d, got %d", len(expectedDeleted), sparse.DeletedCount())
	}
	if sparse.DeletedCount() != dense.DeletedCount() {
		t.Errorf("Sparse and dense counts should match: sparse=%d, dense=%d", sparse.DeletedCount(), dense.DeletedCount())
	}
	for i := 0; i < maxDoc; i++ {
		expectedLive := true
		for _, d := range expectedDeleted {
			if d == i {
				expectedLive = false
				break
			}
		}
		if sparse.Get(i) != expectedLive {
			t.Errorf("Sparse Get(%d) mismatch: expected %v, got %v", i, expectedLive, sparse.Get(i))
		}
		if dense.Get(i) != expectedLive {
			t.Errorf("Dense Get(%d) mismatch: expected %v, got %v", i, expectedLive, dense.Get(i))
		}
	}

	// Compare deleted lists
	if len(sparseDeleted) != len(expectedDeleted) {
		t.Errorf("Sparse iterator should return all deleted docs: expected %d, got %d", len(expectedDeleted), len(sparseDeleted))
	}
	if len(denseDeleted) != len(expectedDeleted) {
		t.Errorf("Dense iterator should return all deleted docs: expected %d, got %d", len(expectedDeleted), len(denseDeleted))
	}
	for i := range expectedDeleted {
		if i < len(sparseDeleted) && sparseDeleted[i] != expectedDeleted[i] {
			t.Errorf("Sparse deleted doc mismatch at %d: expected %d, got %d", i, expectedDeleted[i], sparseDeleted[i])
		}
		if i < len(denseDeleted) && denseDeleted[i] != expectedDeleted[i] {
			t.Errorf("Dense deleted doc mismatch at %d: expected %d, got %d", i, expectedDeleted[i], denseDeleted[i])
		}
	}
}

// TestMemoryUsage tests that sparse uses less memory than dense for sparse deletions
func TestMemoryUsage(t *testing.T) {
	// GIVEN
	maxDoc := 1000000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	for i := 0; i < maxDoc/1000; i++ {
		sparseSet.Set(i * 1000)
		fixedSet.Clear(i * 1000)
	}

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseBytes := sparse.RamBytesUsed()
	denseBytes := dense.RamBytesUsed()

	// THEN
	if sparseBytes >= denseBytes {
		t.Errorf("Sparse should use less memory than dense for 0.1%% deletions: sparse=%d, dense=%d", sparseBytes, denseBytes)
	}
	if sparseBytes >= denseBytes/2 {
		t.Errorf("Sparse should use significantly less memory (< 50%%) for very sparse deletions: sparse=%d, dense=%d, ratio=%.2f%%",
			sparseBytes, denseBytes, 100.0*float64(sparseBytes)/float64(denseBytes))
	}
}

// TestWrappingExistingBitSets tests wrapping existing bit sets
func TestWrappingExistingBitSets(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseDeleted, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	sparseDeleted.Set(10)
	sparseDeleted.Set(50)
	sparseDeleted.Set(100)

	liveSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	liveSet.SetAll()
	liveSet.Clear(10)
	liveSet.Clear(50)
	liveSet.Clear(100)

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseDeleted, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(liveSet, maxDoc).Build()

	// THEN
	if sparse.DeletedCount() != 3 {
		t.Errorf("Expected sparse deleted count 3, got %d", sparse.DeletedCount())
	}
	if sparse.Get(10) {
		t.Error("Sparse doc 10 should be deleted")
	}
	if sparse.Get(50) {
		t.Error("Sparse doc 50 should be deleted")
	}
	if !sparse.Get(11) {
		t.Error("Sparse doc 11 should be live")
	}

	if dense.DeletedCount() != 3 {
		t.Errorf("Expected dense deleted count 3, got %d", dense.DeletedCount())
	}
	if dense.Get(10) {
		t.Error("Dense doc 10 should be deleted")
	}
	if dense.Get(50) {
		t.Error("Dense doc 50 should be deleted")
	}
	if !dense.Get(11) {
		t.Error("Dense doc 11 should be live")
	}
}

// TestSparseLiveDocsLiveIterator tests the live docs iterator for SparseLiveDocs
func TestSparseLiveDocsLiveIterator(t *testing.T) {
	// GIVEN
	maxDoc := 100
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	deletedDocs := []int{5, 10, 50, 99}
	for _, doc := range deletedDocs {
		sparseSet.Set(doc)
	}
	liveDocs := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()

	// WHEN
	it := liveDocs.LiveDocsIterator()
	iteratedDocs, err := collectDocs(it)
	if err != nil {
		t.Fatalf("Failed to collect docs: %v", err)
	}

	// THEN
	expectedLiveCount := maxDoc - len(deletedDocs)
	if it.Cost() != int64(expectedLiveCount) {
		t.Errorf("Iterator cost should match live count: expected %d, got %d", expectedLiveCount, it.Cost())
	}
	if len(iteratedDocs) != expectedLiveCount {
		t.Errorf("Should iterate exact number of live docs: expected %d, got %d", expectedLiveCount, len(iteratedDocs))
	}
	for _, deletedDoc := range deletedDocs {
		for _, doc := range iteratedDocs {
			if doc == deletedDoc {
				t.Errorf("Deleted doc %d should not be in live iterator", deletedDoc)
			}
		}
	}
	for i := 1; i < len(iteratedDocs); i++ {
		if iteratedDocs[i] <= iteratedDocs[i-1] {
			t.Errorf("Docs should be in ascending order: got %d after %d", iteratedDocs[i], iteratedDocs[i-1])
		}
	}
}

// TestDenseLiveDocsLiveIterator tests the live docs iterator for DenseLiveDocs
func TestDenseLiveDocsLiveIterator(t *testing.T) {
	// GIVEN
	maxDoc := 100
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	deletedDocs := []int{5, 10, 50, 99}
	for _, doc := range deletedDocs {
		fixedSet.Clear(doc)
	}
	liveDocs := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// WHEN
	it := liveDocs.LiveDocsIterator()
	iteratedDocs, err := collectDocs(it)
	if err != nil {
		t.Fatalf("Failed to collect docs: %v", err)
	}

	// THEN
	expectedLiveCount := maxDoc - len(deletedDocs)
	if it.Cost() != int64(expectedLiveCount) {
		t.Errorf("Iterator cost should match live count: expected %d, got %d", expectedLiveCount, it.Cost())
	}
	if len(iteratedDocs) != expectedLiveCount {
		t.Errorf("Should iterate exact number of live docs: expected %d, got %d", expectedLiveCount, len(iteratedDocs))
	}
	for _, deletedDoc := range deletedDocs {
		for _, doc := range iteratedDocs {
			if doc == deletedDoc {
				t.Errorf("Deleted doc %d should not be in live iterator", deletedDoc)
			}
		}
	}
	for i := 1; i < len(iteratedDocs); i++ {
		if iteratedDocs[i] <= iteratedDocs[i-1] {
			t.Errorf("Docs should be in ascending order: got %d after %d", iteratedDocs[i], iteratedDocs[i-1])
		}
	}
}

// TestLiveIteratorEquivalence tests that live iterators return the same documents
func TestLiveIteratorEquivalence(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	deletedDocs := []int{1, 10, 50, 100, 200, 500, 750, 999}
	for _, doc := range deletedDocs {
		sparseSet.Set(doc)
		fixedSet.Clear(doc)
	}
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// WHEN
	sparseIt := sparse.LiveDocsIterator()
	denseIt := dense.LiveDocsIterator()

	// THEN
	for {
		sparseDoc, err := sparseIt.NextDoc()
		if err != nil {
			t.Fatalf("Failed to get next doc from sparse: %v", err)
		}
		denseDoc, err := denseIt.NextDoc()
		if err != nil {
			t.Fatalf("Failed to get next doc from dense: %v", err)
		}
		if sparseDoc != denseDoc {
			t.Errorf("Live iterators should return same documents: sparse=%d, dense=%d", sparseDoc, denseDoc)
		}
		if sparseDoc == NO_MORE_DOCS {
			break
		}
	}
}

// TestLiveIteratorFullIteration tests live iterator with no deletions
func TestLiveIteratorFullIteration(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// WHEN
	sparseDocs, err := collectDocs(sparse.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse docs: %v", err)
	}
	denseDocs, err := collectDocs(dense.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense docs: %v", err)
	}

	// THEN
	if len(sparseDocs) != maxDoc {
		t.Errorf("Sparse iterator should return all docs: expected %d, got %d", maxDoc, len(sparseDocs))
	}
	if len(denseDocs) != maxDoc {
		t.Errorf("Dense iterator should return all docs: expected %d, got %d", maxDoc, len(denseDocs))
	}
	for i := 0; i < maxDoc; i++ {
		if sparseDocs[i] != i {
			t.Errorf("Sparse doc at position %d: expected %d, got %d", i, i, sparseDocs[i])
		}
		if denseDocs[i] != i {
			t.Errorf("Dense doc at position %d: expected %d, got %d", i, i, denseDocs[i])
		}
	}
}

// TestLiveIteratorAdvance tests the Advance method on live iterators
func TestLiveIteratorAdvance(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	for i := 10; i <= 50; i += 10 {
		fixedSet.Clear(i)
	}
	liveDocs := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	// WHEN
	it := liveDocs.LiveDocsIterator()

	// THEN
	doc, err := it.Advance(10)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if doc != 11 {
		t.Errorf("Expected advance(10) to return 11, got %d", doc)
	}

	doc, err = it.NextDoc()
	if err != nil {
		t.Fatalf("Failed to get next doc: %v", err)
	}
	if doc != 12 {
		t.Errorf("Expected nextDoc() to return 12, got %d", doc)
	}

	doc, err = it.Advance(20)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if doc != 21 {
		t.Errorf("Expected advance(20) to return 21, got %d", doc)
	}

	doc, err = it.Advance(500)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if doc != 500 {
		t.Errorf("Expected advance(500) to return 500, got %d", doc)
	}

	doc, err = it.Advance(1000)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected advance(1000) to return NO_MORE_DOCS, got %d", doc)
	}
}

// TestRandomDeletionsLiveIterator tests live iterator with random deletions
func TestRandomDeletionsLiveIterator(t *testing.T) {
	// GIVEN
	maxDoc := 5000
	deleteRatio := 0.25
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()

	var expectedLive []int
	r := rand.New(rand.NewSource(42))
	for i := 0; i < maxDoc; i++ {
		if r.Float64() < deleteRatio {
			sparseSet.Set(i)
			fixedSet.Clear(i)
		} else {
			expectedLive = append(expectedLive, i)
		}
	}

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseLive, err := collectDocs(sparse.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse docs: %v", err)
	}
	denseLive, err := collectDocs(dense.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense docs: %v", err)
	}

	// THEN
	if len(sparseLive) != len(expectedLive) {
		t.Errorf("Sparse live iterator should return all live docs: expected %d, got %d", len(expectedLive), len(sparseLive))
	}
	if len(denseLive) != len(expectedLive) {
		t.Errorf("Dense live iterator should return all live docs: expected %d, got %d", len(expectedLive), len(denseLive))
	}
	for i := range expectedLive {
		if i < len(sparseLive) && sparseLive[i] != expectedLive[i] {
			t.Errorf("Sparse live doc mismatch at %d: expected %d, got %d", i, expectedLive[i], sparseLive[i])
		}
		if i < len(denseLive) && denseLive[i] != expectedLive[i] {
			t.Errorf("Dense live doc mismatch at %d: expected %d, got %d", i, expectedLive[i], denseLive[i])
		}
	}
}

// TestSingleDocumentSegment tests with a single document segment
func TestSingleDocumentSegment(t *testing.T) {
	// GIVEN
	maxDoc := 1
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	sparseSet.Set(0)
	fixedSet.Clear(0)

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseDeleted, err := collectDocs(sparse.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse deleted: %v", err)
	}
	denseDeleted, err := collectDocs(dense.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense deleted: %v", err)
	}
	sparseLive, err := collectDocs(sparse.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse live: %v", err)
	}
	denseLive, err := collectDocs(dense.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense live: %v", err)
	}

	// THEN
	if sparse.Get(0) {
		t.Error("Sparse doc 0 should be deleted")
	}
	if dense.Get(0) {
		t.Error("Dense doc 0 should be deleted")
	}
	if sparse.DeletedCount() != 1 {
		t.Errorf("Expected sparse deleted count 1, got %d", sparse.DeletedCount())
	}
	if dense.DeletedCount() != 1 {
		t.Errorf("Expected dense deleted count 1, got %d", dense.DeletedCount())
	}
	if len(sparseDeleted) != 1 || sparseDeleted[0] != 0 {
		t.Errorf("Expected sparse deleted [0], got %v", sparseDeleted)
	}
	if len(denseDeleted) != 1 || denseDeleted[0] != 0 {
		t.Errorf("Expected dense deleted [0], got %v", denseDeleted)
	}
	if len(sparseLive) != 0 {
		t.Errorf("Expected sparse live count 0, got %d", len(sparseLive))
	}
	if len(denseLive) != 0 {
		t.Errorf("Expected dense live count 0, got %d", len(denseLive))
	}
}

// TestAllDocumentsDeleted tests when all documents are deleted
func TestAllDocumentsDeleted(t *testing.T) {
	// GIVEN
	maxDoc := 100
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	for i := 0; i < maxDoc; i++ {
		sparseSet.Set(i)
		fixedSet.Clear(i)
	}

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseDeleted, err := collectDocs(sparse.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse deleted: %v", err)
	}
	denseDeleted, err := collectDocs(dense.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense deleted: %v", err)
	}
	sparseLive, err := collectDocs(sparse.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse live: %v", err)
	}
	denseLive, err := collectDocs(dense.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense live: %v", err)
	}

	// THEN
	if sparse.DeletedCount() != maxDoc {
		t.Errorf("Expected sparse deleted count %d, got %d", maxDoc, sparse.DeletedCount())
	}
	if dense.DeletedCount() != maxDoc {
		t.Errorf("Expected dense deleted count %d, got %d", maxDoc, dense.DeletedCount())
	}
	for i := 0; i < maxDoc; i++ {
		if sparse.Get(i) {
			t.Errorf("Sparse doc %d should be deleted", i)
		}
		if dense.Get(i) {
			t.Errorf("Dense doc %d should be deleted", i)
		}
	}
	if len(sparseDeleted) != maxDoc {
		t.Errorf("Expected sparse deleted size %d, got %d", maxDoc, len(sparseDeleted))
	}
	if len(denseDeleted) != maxDoc {
		t.Errorf("Expected dense deleted size %d, got %d", maxDoc, len(denseDeleted))
	}
	for i := 0; i < maxDoc; i++ {
		if sparseDeleted[i] != i {
			t.Errorf("Sparse deleted doc at %d: expected %d, got %d", i, i, sparseDeleted[i])
		}
		if denseDeleted[i] != i {
			t.Errorf("Dense deleted doc at %d: expected %d, got %d", i, i, denseDeleted[i])
		}
	}
	if len(sparseLive) != 0 {
		t.Errorf("Expected sparse live size 0, got %d", len(sparseLive))
	}
	if len(denseLive) != 0 {
		t.Errorf("Expected dense live size 0, got %d", len(denseLive))
	}
}

// TestLargeSegment tests with a large segment
func TestLargeSegment(t *testing.T) {
	// GIVEN - use smaller size for faster test
	maxDoc := 100000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	for i := 0; i < maxDoc; i += 1000 {
		sparseSet.Set(i)
		fixedSet.Clear(i)
	}

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseIt := sparse.DeletedDocsIterator()
	sparseBytes := sparse.RamBytesUsed()
	denseBytes := dense.RamBytesUsed()

	// THEN
	expectedDeleted := maxDoc / 1000
	if sparse.DeletedCount() != expectedDeleted {
		t.Errorf("Expected sparse deleted count %d, got %d", expectedDeleted, sparse.DeletedCount())
	}
	if dense.DeletedCount() != expectedDeleted {
		t.Errorf("Expected dense deleted count %d, got %d", expectedDeleted, dense.DeletedCount())
	}

	doc, err := sparseIt.NextDoc()
	if err != nil {
		t.Fatalf("Failed to get next doc: %v", err)
	}
	if doc != 0 {
		t.Errorf("Expected first deleted doc 0, got %d", doc)
	}

	_, err = sparseIt.Advance(maxDoc - 1000)
	if err != nil {
		t.Fatalf("Failed to advance: %v", err)
	}
	if sparseIt.DocID() != maxDoc-1000 {
		t.Errorf("Expected doc ID %d after advance, got %d", maxDoc-1000, sparseIt.DocID())
	}

	if sparseBytes >= denseBytes {
		t.Errorf("Sparse should use less memory than dense for 0.1%% deletions: sparse=%d, dense=%d", sparseBytes, denseBytes)
	}
	if sparseBytes >= denseBytes/2 {
		t.Errorf("Sparse should use significantly less memory (< 50%%) for very sparse deletions: sparse=%d, dense=%d, ratio=%.2f%%",
			sparseBytes, denseBytes, 100.0*float64(sparseBytes)/float64(denseBytes))
	}
}

// TestFirstAndLastDocDeletion tests deleting first and last documents
func TestFirstAndLastDocDeletion(t *testing.T) {
	// GIVEN
	maxDoc := 1000
	sparseSet, err := NewSparseFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	fixedSet, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	fixedSet.SetAll()
	sparseSet.Set(0)
	sparseSet.Set(maxDoc - 1)
	fixedSet.Clear(0)
	fixedSet.Clear(maxDoc - 1)

	// WHEN
	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	sparseDeleted, err := collectDocs(sparse.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse deleted: %v", err)
	}
	denseDeleted, err := collectDocs(dense.DeletedDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense deleted: %v", err)
	}
	sparseLive, err := collectDocs(sparse.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect sparse live: %v", err)
	}
	denseLive, err := collectDocs(dense.LiveDocsIterator())
	if err != nil {
		t.Fatalf("Failed to collect dense live: %v", err)
	}

	// THEN
	if sparse.Get(0) {
		t.Error("Sparse first doc should be deleted")
	}
	if sparse.Get(maxDoc-1) {
		t.Error("Sparse last doc should be deleted")
	}
	if dense.Get(0) {
		t.Error("Dense first doc should be deleted")
	}
	if dense.Get(maxDoc-1) {
		t.Error("Dense last doc should be deleted")
	}
	if len(sparseDeleted) != 2 {
		t.Errorf("Expected sparse deleted count 2, got %d", len(sparseDeleted))
	}
	if len(denseDeleted) != 2 {
		t.Errorf("Expected dense deleted count 2, got %d", len(denseDeleted))
	}
	if sparseDeleted[0] != 0 {
		t.Errorf("Expected sparse first deleted 0, got %d", sparseDeleted[0])
	}
	if sparseDeleted[1] != maxDoc-1 {
		t.Errorf("Expected sparse last deleted %d, got %d", maxDoc-1, sparseDeleted[1])
	}
	if len(sparseLive) != maxDoc-2 {
		t.Errorf("Expected sparse live count %d, got %d", maxDoc-2, len(sparseLive))
	}
	if len(denseLive) != maxDoc-2 {
		t.Errorf("Expected dense live count %d, got %d", maxDoc-2, len(denseLive))
	}
	if sparseLive[0] != 1 {
		t.Errorf("Expected sparse first live 1, got %d", sparseLive[0])
	}
	if sparseLive[len(sparseLive)-1] != maxDoc-2 {
		t.Errorf("Expected sparse last live %d, got %d", maxDoc-2, sparseLive[len(sparseLive)-1])
	}
}

// TestLiveDocsLength tests the Length method
func TestLiveDocsLength(t *testing.T) {
	maxDoc := 500
	sparseSet, _ := NewSparseFixedBitSet(maxDoc)
	fixedSet, _ := NewFixedBitSet(maxDoc)
	fixedSet.SetAll()

	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	if sparse.Length() != maxDoc {
		t.Errorf("Expected sparse length %d, got %d", maxDoc, sparse.Length())
	}
	if dense.Length() != maxDoc {
		t.Errorf("Expected dense length %d, got %d", maxDoc, dense.Length())
	}
}

// TestLiveDocsLiveCount tests the LiveCount method
func TestLiveDocsLiveCount(t *testing.T) {
	maxDoc := 100
	sparseSet, _ := NewSparseFixedBitSet(maxDoc)
	fixedSet, _ := NewFixedBitSet(maxDoc)
	fixedSet.SetAll()

	// Delete 5 documents
	for i := 0; i < 5; i++ {
		sparseSet.Set(i * 10)
		fixedSet.Clear(i * 10)
	}

	sparse := NewSparseLiveDocsBuilder(sparseSet, maxDoc).Build()
	dense := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()

	expectedLive := maxDoc - 5
	if sparse.LiveCount() != expectedLive {
		t.Errorf("Expected sparse live count %d, got %d", expectedLive, sparse.LiveCount())
	}
	if dense.LiveCount() != expectedLive {
		t.Errorf("Expected dense live count %d, got %d", expectedLive, dense.LiveCount())
	}
}

// TestDocIDMethod tests the DocID method on iterators
func TestDocIDMethod(t *testing.T) {
	maxDoc := 100
	fixedSet, _ := NewFixedBitSet(maxDoc)
	fixedSet.SetAll()
	fixedSet.Clear(50)

	liveDocs := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	it := liveDocs.DeletedDocsIterator()

	// Initially should be -1
	if it.DocID() != -1 {
		t.Errorf("Expected initial DocID -1, got %d", it.DocID())
	}

	// After nextDoc
	doc, _ := it.NextDoc()
	if doc != 50 {
		t.Fatalf("Expected doc 50, got %d", doc)
	}
	if it.DocID() != 50 {
		t.Errorf("Expected DocID 50, got %d", it.DocID())
	}
}

// TestAdvanceToCurrentDoc tests advancing to the current document
func TestAdvanceToCurrentDoc(t *testing.T) {
	maxDoc := 100
	fixedSet, _ := NewFixedBitSet(maxDoc)
	fixedSet.SetAll()
	fixedSet.Clear(50)
	fixedSet.Clear(60)

	liveDocs := NewDenseLiveDocsBuilder(fixedSet, maxDoc).Build()
	it := liveDocs.DeletedDocsIterator()

	// Move to first deleted doc
	doc, _ := it.NextDoc()
	if doc != 50 {
		t.Fatalf("Expected doc 50, got %d", doc)
	}

	// Advance to same position should return same doc
	doc, _ = it.Advance(50)
	if doc != 50 {
		t.Errorf("Expected advance(50) to return 50, got %d", doc)
	}

	// Advance to next deleted
	doc, _ = it.Advance(51)
	if doc != 60 {
		t.Errorf("Expected advance(51) to return 60, got %d", doc)
	}
}
