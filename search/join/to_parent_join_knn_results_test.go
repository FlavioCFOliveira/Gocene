// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestToParentJoinKnnResults.java
// (Apache Lucene 10.5.0).

func mustBitSetOf(t testing.TB, it util.DocIdSetIterator, maxDoc int) util.BitSet {
	t.Helper()
	bs, err := util.OfDocIdSetIterator(it, maxDoc)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

func mustNewDiversifyingCollector(t testing.TB, k, visitLimit int, parentBitSet util.BitSet) *DiversifyingNearestChildrenKnnCollector {
	t.Helper()
	c, err := NewDiversifyingNearestChildrenKnnCollector(k, visitLimit, parentBitSet)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustKnnCollect(t testing.TB, c *DiversifyingNearestChildrenKnnCollector, docID int, score float32) bool {
	t.Helper()
	ok, err := c.Collect(docID, score)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestToParentJoinKnnResultsNeighborsProduct(t *testing.T) {
	// make sure we have the sign correct
	parentBitSet := mustBitSetOf(t, newIntArrayDocIdSetIterator([]int{1, 3, 5}, 3), 6)
	nn := mustNewDiversifyingCollector(t, 2, math.MaxInt32, parentBitSet)
	if !mustKnnCollect(t, nn, 2, 0.5) {
		t.Fatal("collect(2, 0.5f)")
	}
	if !mustKnnCollect(t, nn, 0, 0.2) {
		t.Fatal("collect(0, 0.2f)")
	}
	if !mustKnnCollect(t, nn, 4, 1) {
		t.Fatal("collect(4, 1f)")
	}
	if got := nn.MinCompetitiveSimilarity(); got != 0.5 {
		t.Fatalf("minCompetitiveSimilarity: expected 0.5, got %v", got)
	}
	topDocs := nn.TopDocsSearch()
	if topDocs.ScoreDocs[0].Score != 1 {
		t.Fatalf("scoreDocs[0].score: expected 1, got %v", topDocs.ScoreDocs[0].Score)
	}
	if topDocs.ScoreDocs[1].Score != 0.5 {
		t.Fatalf("scoreDocs[1].score: expected 0.5, got %v", topDocs.ScoreDocs[1].Score)
	}
}

func TestToParentJoinKnnResultsInsertions(t *testing.T) {
	nodes := []int{4, 1, 5, 7, 8, 10, 2}
	scores := []float32{1, 0.5, 0.6, 2, 2, 1.2, 4}
	parentBitSet := mustBitSetOf(t, newIntArrayDocIdSetIterator([]int{3, 6, 9, 12}, 4), 13)
	results := mustNewDiversifyingCollector(t, 7, math.MaxInt32, parentBitSet)
	for i := range nodes {
		mustKnnCollect(t, results, nodes[i], scores[i])
	}
	topDocs := results.TopDocsSearch()
	assertNodesAndScores(t, topDocs, []int{2, 7, 10, 4}, []float32{4, 2, 1.2, 1}, len(topDocs.ScoreDocs))
}

// assertNodesAndScores renders the sortedNodes/sortedScores arrays and the two
// assertArrayEquals of the insertion tests; size is the length Java allocates.
func assertNodesAndScores(t testing.TB, topDocs *search.TopDocs, wantNodes []int, wantScores []float32, size int) {
	t.Helper()
	sortedNodes := make([]int, size)
	sortedScores := make([]float32, size)
	for i, sd := range topDocs.ScoreDocs {
		sortedNodes[i] = sd.Doc
		sortedScores[i] = sd.Score
	}
	if len(sortedNodes) != len(wantNodes) {
		t.Fatalf("nodes: expected %v, got %v", wantNodes, sortedNodes)
	}
	for i := range wantNodes {
		if sortedNodes[i] != wantNodes[i] || sortedScores[i] != wantScores[i] {
			t.Fatalf("expected %v / %v, got %v / %v", wantNodes, wantScores, sortedNodes, sortedScores)
		}
	}
}

func TestToParentJoinKnnResultsInsertionWithOverflow(t *testing.T) {
	nodes := []int{4, 1, 5, 7, 8, 10, 2, 12, 14}
	scores := []float32{1, 0.5, 0.6, 2, 2, 3, 4, 1, 0.2}
	parentBitSet := mustBitSetOf(t, newIntArrayDocIdSetIterator([]int{3, 6, 9, 11, 13, 15}, 6), 16)
	results := mustNewDiversifyingCollector(t, 5, math.MaxInt32, parentBitSet)
	for i := 0; i < len(nodes)-1; i++ {
		mustKnnCollect(t, results, nodes[i], scores[i])
	}
	if mustKnnCollect(t, results, nodes[len(nodes)-1], scores[len(nodes)-1]) {
		t.Fatal("the last collect must be rejected")
	}
	topDocs := results.TopDocsSearch()
	assertNodesAndScores(t, topDocs, []int{2, 10, 7, 4, 12}, []float32{4, 3, 2, 1, 1}, 5)
}

func TestToParentJoinKnnResultsRandomInsertionsWithOverflow(t *testing.T) {
	parents := make([]int, 100)
	children := make([]int, 0)
	childrenScores := make([]float32, 0)
	previousParent := -1
	nextParent := random().Intn(50) + 2
	for i := 0; i < 100; i++ {
		for j := previousParent + 1; j < nextParent; j++ {
			children = append(children, j)
			childrenScores = append(childrenScores, random().Float32())
		}
		parents[i] = nextParent
		previousParent = nextParent
		nextParent = random().Intn(50) + 2 + previousParent
	}
	// Collections.shuffle(children, random()): only the children are shuffled.
	r := random()
	for i := len(children); i > 1; i-- {
		j := r.Intn(i)
		children[i-1], children[j] = children[j], children[i-1]
	}
	parentBitSet := mustBitSetOf(t, newIntArrayDocIdSetIterator(parents, len(parents)), nextParent+1)
	results := mustNewDiversifyingCollector(t, 20, math.MaxInt32, parentBitSet)
	for i := range children {
		mustKnnCollect(t, results, children[i], childrenScores[i])
	}
}

// intArrayDocIdSetIterator renders the static class IntArrayDocIdSetIterator,
// which extends AbstractDocIdSetIterator.
type intArrayDocIdSetIterator struct {
	docs   []int
	length int
	i      int
	doc    int
}

func newIntArrayDocIdSetIterator(docs []int, length int) *intArrayDocIdSetIterator {
	return &intArrayDocIdSetIterator{docs: docs, length: length, doc: -1}
}

// DocID renders AbstractDocIdSetIterator.docID().
func (it *intArrayDocIdSetIterator) DocID() int { return it.doc }

func (it *intArrayDocIdSetIterator) NextDoc() (int, error) {
	if it.i >= it.length {
		return search.NO_MORE_DOCS, nil
	}
	it.doc = it.docs[it.i]
	it.i++
	return it.doc, nil
}

func (it *intArrayDocIdSetIterator) Advance(target int) (int, error) {
	bound := 1
	// given that we use this for small arrays only, this is very unlikely to overflow
	for it.i+bound < it.length && it.docs[it.i+bound] < target {
		bound *= 2
	}
	it.i = javaBinarySearch(it.docs, it.i+bound/2, min(it.i+bound+1, it.length), target)
	if it.i < 0 {
		it.i = -1 - it.i
	}
	it.doc = it.docs[it.i]
	it.i++
	return it.doc, nil
}

func (it *intArrayDocIdSetIterator) Cost() int64 { return int64(it.length) }

func (it *intArrayDocIdSetIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

func (it *intArrayDocIdSetIterator) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(it) }

// javaBinarySearch renders Arrays.binarySearch(int[], int fromIndex, int
// toIndex, int key).
func javaBinarySearch(a []int, fromIndex, toIndex, key int) int {
	i := sort.SearchInts(a[fromIndex:toIndex], key) + fromIndex
	if i < toIndex && a[i] == key {
		return i
	}
	return -(i + 1)
}
