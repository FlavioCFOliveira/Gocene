// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DiversifyingNearestChildrenKnnCollector collects the nearest child vectors
// while diversifying results so that at most one child per parent is returned.
//
// Mirrors org.apache.lucene.search.join.DiversifyingNearestChildrenKnnCollector.
//
// Gocene deviations:
//   - AbstractKnnCollector fields (k, visitedCount, visitLimit, earlyTerminated)
//     are embedded directly; the stub search.AbstractKnnCollector is not used.
//   - KnnSearchStrategy parameter is accepted but not yet applied (stub).
//   - IntIntHashMap is replaced with a plain Go map[int]int.
type DiversifyingNearestChildrenKnnCollector struct {
	k              int
	visitLimit     int64
	visitedCount   int64
	earlyTerminated bool
	parentBitSet   util.BitSet
	heap           *nodeIdCachingHeap
}

// NewDiversifyingNearestChildrenKnnCollector creates a collector.
//
//   - k: maximum number of parent results to collect
//   - visitLimit: maximum number of child vectors to visit
//   - parentBitSet: leaf-level bitset marking parent documents
func NewDiversifyingNearestChildrenKnnCollector(k, visitLimit int, parentBitSet util.BitSet) (*DiversifyingNearestChildrenKnnCollector, error) {
	if k < 1 {
		return nil, fmt.Errorf("k must be >= 1; got %d", k)
	}
	return &DiversifyingNearestChildrenKnnCollector{
		k:            k,
		visitLimit:   int64(visitLimit),
		parentBitSet: parentBitSet,
		heap:         newNodeIdCachingHeap(k),
	}, nil
}

// NewDiversifyingNearestChildrenKnnCollectorWithStrategy creates a collector with
// an explicit search strategy (strategy is accepted but not yet applied).
func NewDiversifyingNearestChildrenKnnCollectorWithStrategy(k, visitLimit int, strategy search.KnnSearchStrategy, parentBitSet util.BitSet) (*DiversifyingNearestChildrenKnnCollector, error) {
	c, err := NewDiversifyingNearestChildrenKnnCollector(k, visitLimit, parentBitSet)
	if err != nil {
		return nil, err
	}
	_ = strategy
	return c, nil
}

// Collect adds a candidate child doc with its similarity score.
// Returns true when the doc was accepted (score improved parent's best).
func (c *DiversifyingNearestChildrenKnnCollector) Collect(docID int, score float32) (bool, error) {
	c.visitedCount++
	if c.visitedCount > c.visitLimit {
		c.earlyTerminated = true
	}
	// Find the nearest parent by scanning forward from docID.
	parentNode := c.parentBitSet.NextSetBitBounded(docID)
	if parentNode == util.NO_MORE_DOCS {
		return false, nil
	}
	return c.heap.insertWithOverflow(docID, parentNode, score), nil
}

// MinCompetitiveSimilarity returns the score threshold for competitive results.
func (c *DiversifyingNearestChildrenKnnCollector) MinCompetitiveSimilarity() float32 {
	if c.heap.size >= c.k {
		return c.heap.topScore()
	}
	return float32(math.Inf(-1))
}

// K returns the configured top-K budget.
func (c *DiversifyingNearestChildrenKnnCollector) K() int { return c.k }

// VisitedCount returns the number of nodes visited.
func (c *DiversifyingNearestChildrenKnnCollector) VisitedCount() int64 { return c.visitedCount }

// EarlyTerminated reports whether the visit limit was exceeded.
func (c *DiversifyingNearestChildrenKnnCollector) EarlyTerminated() bool {
	return c.earlyTerminated
}

// NumCollected returns the number of results currently in the heap.
func (c *DiversifyingNearestChildrenKnnCollector) NumCollected() int { return c.heap.size }

// TopDocs drains the heap and returns the top scoring (child-doc, score) pairs
// in descending score order.
func (c *DiversifyingNearestChildrenKnnCollector) TopDocs() []search.ScoreDoc {
	// Drain excess entries beyond k.
	for c.heap.size > c.k {
		c.heap.popToDrain()
	}
	n := c.heap.size
	docs := make([]search.ScoreDoc, n)
	// Heap drains in ascending score order; we reverse to descending.
	for i := n; i >= 1; i-- {
		docs[i-1] = search.ScoreDoc{Doc: c.heap.topNode(), Score: c.heap.topScore()}
		c.heap.popToDrain()
	}
	return docs
}

// String implements fmt.Stringer.
func (c *DiversifyingNearestChildrenKnnCollector) String() string {
	return fmt.Sprintf("DiversifyingNearestChildrenKnnCollector[k=%d, size=%d]", c.k, c.heap.size)
}

// --- min-heap with parent-caching ---

// parentChildScore is a heap element holding a child doc, its parent doc, and its score.
type parentChildScore struct {
	child  int
	parent int
	score  float32
}

func (a parentChildScore) less(b parentChildScore) bool {
	if a.score != b.score {
		return a.score < b.score
	}
	// Tie-break: lower child id is preferred (matches Java: lower id wins).
	return a.child > b.child
}

// nodeIdCachingHeap is a 1-indexed min-heap that tracks parent → heap position
// so that a parent's score can be updated in O(log n) instead of O(n).
type nodeIdCachingHeap struct {
	maxSize        int
	nodes          []parentChildScore
	size           int
	parentToIndex  map[int]int // parentID → 1-based heap index
	closed         bool
}

func newNodeIdCachingHeap(maxSize int) *nodeIdCachingHeap {
	if maxSize < 1 {
		panic(fmt.Sprintf("nodeIdCachingHeap: maxSize must be >= 1; got %d", maxSize))
	}
	// +1: heap is 1-indexed; nodes[0] is unused.
	h := &nodeIdCachingHeap{
		maxSize:       maxSize,
		nodes:         make([]parentChildScore, maxSize+1),
		parentToIndex: make(map[int]int, maxSize),
	}
	return h
}

func (h *nodeIdCachingHeap) topNode() int   { return h.nodes[1].child }
func (h *nodeIdCachingHeap) topScore() float32 { return h.nodes[1].score }

func (h *nodeIdCachingHeap) pushIn(child, parent int, score float32) {
	h.size++
	if h.size >= len(h.nodes) {
		grown := make([]parentChildScore, h.size*3/2+1)
		copy(grown, h.nodes)
		h.nodes = grown
	}
	h.nodes[h.size] = parentChildScore{child, parent, score}
	h.upHeap(h.size)
}

func (h *nodeIdCachingHeap) updateElement(idx, child, parent int, score float32) {
	oldScore := h.nodes[idx].score
	h.nodes[idx] = parentChildScore{child, parent, score}
	if score < oldScore {
		h.upHeap(idx)
	} else {
		h.downHeap(idx)
	}
}

// insertWithOverflow either inserts a new (child, parent, score) or updates an
// existing parent entry if the new score is higher. Returns true when the heap
// changed (accepted or updated).
func (h *nodeIdCachingHeap) insertWithOverflow(child, parent int, score float32) bool {
	if h.closed {
		panic("nodeIdCachingHeap: insertWithOverflow called after drain")
	}
	if idx, ok := h.parentToIndex[parent]; ok {
		if h.nodes[idx].score < score {
			h.updateElement(idx, child, parent, score)
			return true
		}
		return false
	}
	if h.size >= h.maxSize {
		top := h.nodes[1]
		if score < top.score || (score == top.score && child > top.child) {
			return false
		}
		h.updateTop(child, parent, score)
		return true
	}
	h.pushIn(child, parent, score)
	return true
}

func (h *nodeIdCachingHeap) popToDrain() {
	h.closed = true
	if h.size == 0 {
		panic("nodeIdCachingHeap: popToDrain on empty heap")
	}
	h.nodes[1] = h.nodes[h.size]
	h.size--
	h.downHeapWithoutCacheUpdate(1)
}

func (h *nodeIdCachingHeap) updateTop(child, parent int, score float32) {
	delete(h.parentToIndex, h.nodes[1].parent)
	h.nodes[1] = parentChildScore{child, parent, score}
	h.downHeap(1)
}

func (h *nodeIdCachingHeap) upHeap(origPos int) {
	i := origPos
	bottom := h.nodes[i]
	j := i >> 1
	for j > 0 && bottom.less(h.nodes[j]) {
		h.nodes[i] = h.nodes[j]
		h.parentToIndex[h.nodes[i].parent] = i
		i = j
		j = j >> 1
	}
	h.parentToIndex[bottom.parent] = i
	h.nodes[i] = bottom
}

func (h *nodeIdCachingHeap) downHeap(i int) {
	node := h.nodes[i]
	j := i << 1
	k := j + 1
	if k <= h.size && h.nodes[k].less(h.nodes[j]) {
		j = k
	}
	for j <= h.size && h.nodes[j].less(node) {
		h.nodes[i] = h.nodes[j]
		h.parentToIndex[h.nodes[i].parent] = i
		i = j
		j = i << 1
		k = j + 1
		if k <= h.size && h.nodes[k].less(h.nodes[j]) {
			j = k
		}
	}
	h.parentToIndex[node.parent] = i
	h.nodes[i] = node
}

func (h *nodeIdCachingHeap) downHeapWithoutCacheUpdate(i int) {
	node := h.nodes[i]
	j := i << 1
	k := j + 1
	if k <= h.size && h.nodes[k].less(h.nodes[j]) {
		j = k
	}
	for j <= h.size && h.nodes[j].less(node) {
		h.nodes[i] = h.nodes[j]
		i = j
		j = i << 1
		k = j + 1
		if k <= h.size && h.nodes[k].less(h.nodes[j]) {
			j = k
		}
	}
	h.nodes[i] = node
}
