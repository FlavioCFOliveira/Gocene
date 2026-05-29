// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// sliceDISI is a minimal slice-backed DocIdSetIterator over an ascending set of
// leaf-local doc IDs, used to exercise diversifyingExactSearch without building
// a full segment.
type sliceDISI struct {
	docs []int
	pos  int // -1 before first NextDoc
}

func newSliceDISI(docs []int) *sliceDISI { return &sliceDISI{docs: docs, pos: -1} }

func (it *sliceDISI) DocID() int {
	if it.pos < 0 {
		return -1
	}
	if it.pos >= len(it.docs) {
		return search.NO_MORE_DOCS
	}
	return it.docs[it.pos]
}

func (it *sliceDISI) NextDoc() (int, error) {
	it.pos++
	return it.DocID(), nil
}

func (it *sliceDISI) Advance(target int) (int, error) {
	for {
		d, _ := it.NextDoc()
		if d == search.NO_MORE_DOCS || d >= target {
			return d, nil
		}
	}
}

func (it *sliceDISI) Cost() int64      { return int64(len(it.docs)) }
func (it *sliceDISI) DocIDRunEnd() int { return it.DocID() + 1 }

// TestDiversifyingExactSearch_BestPerParent verifies the core diversifying scan:
// children are grouped into parent blocks by the parent bitset, exactly one
// (the best-scoring) child per parent survives, and the global top-K is
// returned in descending score order.
//
// Layout (leaf-local doc IDs):
//
//	block A: children 0,1   parent bit 2
//	block B: children 3,4   parent bit 5
//	block C: child 6        parent bit 7
func TestDiversifyingExactSearch_BestPerParent(t *testing.T) {
	// Parent bitset over 8 docs with parents at 2, 5, 7.
	parents := NewFixedBitSet(8)
	parents.Set(2)
	parents.Set(5)
	parents.Set(7)

	// Per-child scores: in block A, child 1 beats child 0; in block B, child 3
	// beats child 4; block C has only child 6.
	scores := map[int]float32{
		0: 0.10, 1: 0.90, // block A -> best child 1 (0.90)
		3: 0.70, 4: 0.20, // block B -> best child 3 (0.70)
		6: 0.50, // block C -> child 6 (0.50)
	}
	score := func(docID int) (float32, bool, error) {
		s, ok := scores[docID]
		return s, ok, nil
	}

	// Accept all six child docs (parents are not in the accept iterator, as in
	// the no-filter codec iterator that only positions on docs with a vector).
	it := newSliceDISI([]int{0, 1, 3, 4, 6})

	td, err := diversifyingExactSearch(it, parents, 3, nil, score)
	if err != nil {
		t.Fatalf("diversifyingExactSearch: %v", err)
	}
	if len(td.ScoreDocs) != 3 {
		t.Fatalf("scoreDocs = %d, want 3", len(td.ScoreDocs))
	}

	wantDoc := []int{1, 3, 6}
	wantScore := []float32{0.90, 0.70, 0.50}
	for i, sd := range td.ScoreDocs {
		if sd.Doc != wantDoc[i] {
			t.Errorf("scoreDocs[%d].Doc = %d, want %d", i, sd.Doc, wantDoc[i])
		}
		if diff := sd.Score - wantScore[i]; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("scoreDocs[%d].Score = %v, want %v", i, sd.Score, wantScore[i])
		}
	}
}

// TestDiversifyingExactSearch_TopKTruncates verifies that k caps the number of
// surviving parents while still selecting the highest-scoring blocks.
func TestDiversifyingExactSearch_TopKTruncates(t *testing.T) {
	parents := NewFixedBitSet(8)
	parents.Set(2)
	parents.Set(5)
	parents.Set(7)

	scores := map[int]float32{
		0: 0.10, 1: 0.90, // block A best 0.90
		3: 0.70, 4: 0.20, // block B best 0.70
		6: 0.50, // block C 0.50
	}
	score := func(docID int) (float32, bool, error) {
		s, ok := scores[docID]
		return s, ok, nil
	}
	it := newSliceDISI([]int{0, 1, 3, 4, 6})

	// k = 2 keeps only the two best parents (children 1 and 3).
	td, err := diversifyingExactSearch(it, parents, 2, nil, score)
	if err != nil {
		t.Fatalf("diversifyingExactSearch: %v", err)
	}
	if len(td.ScoreDocs) != 2 {
		t.Fatalf("scoreDocs = %d, want 2", len(td.ScoreDocs))
	}
	if td.ScoreDocs[0].Doc != 1 || td.ScoreDocs[1].Doc != 3 {
		t.Errorf("docs = [%d %d], want [1 3]", td.ScoreDocs[0].Doc, td.ScoreDocs[1].Doc)
	}
}
