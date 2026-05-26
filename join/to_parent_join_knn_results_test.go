// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestToParentJoinKnnResults.
package join

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// buildParentBitSet creates a FixedBitSet with the given parent doc IDs set.
func buildParentBitSet(t *testing.T, parentDocIDs []int, numBits int) util.BitSet {
	t.Helper()
	bs, err := util.NewFixedBitSet(numBits)
	if err != nil {
		t.Fatalf("NewFixedBitSet(%d): %v", numBits, err)
	}
	for _, id := range parentDocIDs {
		bs.Set(id)
	}
	return bs
}

// mustCollect asserts that Collect does not error and returns the expected bool.
func mustCollect(t *testing.T, c *DiversifyingNearestChildrenKnnCollector, docID int, score float32, wantAccepted bool) {
	t.Helper()
	got, err := c.Collect(docID, score)
	if err != nil {
		t.Fatalf("Collect(%d, %v): %v", docID, score, err)
	}
	if got != wantAccepted {
		t.Errorf("Collect(%d, %v) = %v, want %v", docID, score, got, wantAccepted)
	}
}

// TestToParentJoinKnnResults_NeighborsProduct mirrors
// TestToParentJoinKnnResults.testNeighborsProduct: verifies sign correctness
// and that results are returned in descending score order.
func TestToParentJoinKnnResults_NeighborsProduct(t *testing.T) {
	// parents at positions 1, 3, 5 → numBits=6
	parents := buildParentBitSet(t, []int{1, 3, 5}, 6)
	nn, err := NewDiversifyingNearestChildrenKnnCollector(2, int(^uint(0)>>1), parents)
	if err != nil {
		t.Fatalf("NewDiversifyingNearestChildrenKnnCollector: %v", err)
	}

	mustCollect(t, nn, 2, 0.5, true) // child 2, parent 3 → accepted
	mustCollect(t, nn, 0, 0.2, true) // child 0, parent 1 → accepted
	mustCollect(t, nn, 4, 1.0, true) // child 4, parent 5 → accepted; evicts score 0.2

	want := float32(0.5)
	if got := nn.MinCompetitiveSimilarity(); got != want {
		t.Errorf("MinCompetitiveSimilarity() = %v, want %v", got, want)
	}

	docs := nn.TopDocs()
	if len(docs) != 2 {
		t.Fatalf("TopDocs() len = %d, want 2", len(docs))
	}
	if docs[0].Score != 1.0 {
		t.Errorf("docs[0].Score = %v, want 1.0", docs[0].Score)
	}
	if docs[1].Score != 0.5 {
		t.Errorf("docs[1].Score = %v, want 0.5", docs[1].Score)
	}
}

// TestToParentJoinKnnResults_Insertions mirrors
// TestToParentJoinKnnResults.testInsertions: verifies that inserting 7 entries
// into a k=7 collector with 4 parents yields the best-per-parent set.
func TestToParentJoinKnnResults_Insertions(t *testing.T) {
	nodes := []int{4, 1, 5, 7, 8, 10, 2}
	scores := []float32{1.0, 0.5, 0.6, 2.0, 2.0, 1.2, 4.0}
	// parents at 3, 6, 9, 12 → numBits=13
	parents := buildParentBitSet(t, []int{3, 6, 9, 12}, 13)
	results, err := NewDiversifyingNearestChildrenKnnCollector(7, int(^uint(0)>>1), parents)
	if err != nil {
		t.Fatalf("NewDiversifyingNearestChildrenKnnCollector: %v", err)
	}

	for i := range nodes {
		if _, err := results.Collect(nodes[i], scores[i]); err != nil {
			t.Fatalf("Collect: %v", err)
		}
	}

	docs := results.TopDocs()

	gotNodes := make([]int, len(docs))
	gotScores := make([]float32, len(docs))
	for i, sd := range docs {
		gotNodes[i] = sd.Doc
		gotScores[i] = sd.Score
	}

	// Java expected: nodes={2,7,10,4}, scores={4,2,1.2,1}
	wantNodes := []int{2, 7, 10, 4}
	wantScores := []float32{4, 2, 1.2, 1}

	if len(gotNodes) != len(wantNodes) {
		t.Fatalf("TopDocs len = %d, want %d", len(gotNodes), len(wantNodes))
	}
	for i := range wantNodes {
		if gotNodes[i] != wantNodes[i] {
			t.Errorf("TopDocs[%d].Doc = %d, want %d", i, gotNodes[i], wantNodes[i])
		}
		if gotScores[i] != wantScores[i] {
			t.Errorf("TopDocs[%d].Score = %v, want %v", i, gotScores[i], wantScores[i])
		}
	}
}

// TestToParentJoinKnnResults_InsertionWithOverflow mirrors
// TestToParentJoinKnnResults.testInsertionWithOverflow: verifies that
// inserting a below-threshold entry returns false.
func TestToParentJoinKnnResults_InsertionWithOverflow(t *testing.T) {
	nodes := []int{4, 1, 5, 7, 8, 10, 2, 12, 14}
	scores := []float32{1, 0.5, 0.6, 2, 2, 3, 4, 1, 0.2}
	// parents at 3,6,9,11,13,15 → numBits=16
	parents := buildParentBitSet(t, []int{3, 6, 9, 11, 13, 15}, 16)
	results, err := NewDiversifyingNearestChildrenKnnCollector(5, int(^uint(0)>>1), parents)
	if err != nil {
		t.Fatalf("NewDiversifyingNearestChildrenKnnCollector: %v", err)
	}

	// Collect all but the last, which should be below threshold.
	for i := 0; i < len(nodes)-1; i++ {
		if _, err := results.Collect(nodes[i], scores[i]); err != nil {
			t.Fatalf("Collect: %v", err)
		}
	}
	// Last entry (docID=14, score=0.2) should be rejected.
	mustCollect(t, results, nodes[len(nodes)-1], scores[len(nodes)-1], false)

	docs := results.TopDocs()
	if len(docs) != 5 {
		t.Fatalf("TopDocs len = %d, want 5", len(docs))
	}
	gotNodes := make([]int, 5)
	gotScores := make([]float32, 5)
	for i, sd := range docs {
		gotNodes[i] = sd.Doc
		gotScores[i] = sd.Score
	}

	// Java expected: nodes={2,10,7,4,12}, scores={4,3,2,1,1}
	wantNodes := []int{2, 10, 7, 4, 12}
	wantScores := []float32{4, 3, 2, 1, 1}

	for i := range wantNodes {
		if gotNodes[i] != wantNodes[i] {
			t.Errorf("TopDocs[%d].Doc = %d, want %d", i, gotNodes[i], wantNodes[i])
		}
		if gotScores[i] != wantScores[i] {
			t.Errorf("TopDocs[%d].Score = %v, want %v", i, gotScores[i], wantScores[i])
		}
	}
}

// TestToParentJoinKnnResults_RandomInsertionsWithOverflow mirrors
// TestToParentJoinKnnResults.testRandomInsertionsWithOverflow: sanity-checks
// that random insertions into a k=20 collector do not panic.
func TestToParentJoinKnnResults_RandomInsertionsWithOverflow(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	parents := make([]int, 100)
	var children []int
	var childScores []float32

	prevParent := -1
	nextParent := rng.Intn(50) + 2
	for i := 0; i < 100; i++ {
		for j := prevParent + 1; j < nextParent; j++ {
			children = append(children, j)
			childScores = append(childScores, rng.Float32())
		}
		parents[i] = nextParent
		prevParent = nextParent
		nextParent = rng.Intn(50) + 2 + prevParent
	}

	// Shuffle children (keep scores aligned).
	order := rng.Perm(len(children))
	shuffledChildren := make([]int, len(children))
	shuffledScores := make([]float32, len(children))
	for i, idx := range order {
		shuffledChildren[i] = children[idx]
		shuffledScores[i] = childScores[idx]
	}

	numBits := nextParent + 1
	parentBS := buildParentBitSet(t, parents[:], numBits)
	collector, err := NewDiversifyingNearestChildrenKnnCollector(20, int(^uint(0)>>1), parentBS)
	if err != nil {
		t.Fatalf("NewDiversifyingNearestChildrenKnnCollector: %v", err)
	}
	for i := range shuffledChildren {
		if _, err := collector.Collect(shuffledChildren[i], shuffledScores[i]); err != nil {
			t.Fatalf("Collect: %v", err)
		}
	}

	docs := collector.TopDocs()
	// Results must be sorted descending by score.
	for i := 1; i < len(docs); i++ {
		if docs[i].Score > docs[i-1].Score {
			t.Errorf("TopDocs not sorted descending at [%d]: %v > %v",
				i, docs[i].Score, docs[i-1].Score)
		}
	}
	// At most one child per parent.
	seenParents := map[int]bool{}
	for _, sd := range docs {
		p := parentBS.NextSetBitBounded(sd.Doc)
		if seenParents[p] {
			t.Errorf("duplicate parent %d in results", p)
		}
		seenParents[p] = true
	}
	_ = sort.Search // keep import used
}
