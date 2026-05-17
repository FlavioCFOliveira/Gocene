// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// TopKnnCollector is the concrete KnnCollector that keeps the top-K best
// similarity scores seen during an HNSW search. It uses a min-heap so the
// lowest score is always available as the displacement threshold.
//
// Mirrors org.apache.lucene.search.TopKnnCollector. The collector exposes its
// state through the minimal contract needed by callers in this package; the
// canonical KnnCollector interface lives in util/hnsw (Sprint 25).
type TopKnnCollector struct {
	k          int
	visitLimit int
	visited    int
	terminated bool

	heap []float32
	docs []int
}

// NewTopKnnCollector builds a collector with the given k and visit limit.
func NewTopKnnCollector(k, visitLimit int) *TopKnnCollector {
	if k <= 0 {
		k = 1
	}
	if visitLimit <= 0 {
		visitLimit = math.MaxInt32
	}
	return &TopKnnCollector{
		k:          k,
		visitLimit: visitLimit,
		heap:       make([]float32, 0, k),
		docs:       make([]int, 0, k),
	}
}

// Collect records (docID, similarity). It returns true if the document was
// kept in the top-K set.
func (c *TopKnnCollector) Collect(docID int, sim float32) bool {
	c.visited++
	if c.visited > c.visitLimit {
		c.terminated = true
		return false
	}
	if len(c.heap) < c.k {
		c.heap = append(c.heap, sim)
		c.docs = append(c.docs, docID)
		c.siftUp(len(c.heap) - 1)
		return true
	}
	if sim <= c.heap[0] {
		return false
	}
	c.heap[0] = sim
	c.docs[0] = docID
	c.siftDown(0)
	return true
}

// MinCompetitiveSimilarity returns the lowest score currently in the heap, or
// -Inf if the heap is not full yet.
func (c *TopKnnCollector) MinCompetitiveSimilarity() float32 {
	if len(c.heap) < c.k {
		return float32(math.Inf(-1))
	}
	return c.heap[0]
}

// NumCollected returns the current heap size.
func (c *TopKnnCollector) NumCollected() int { return len(c.heap) }

// EarlyTerminated reports whether the visit limit has been reached.
func (c *TopKnnCollector) EarlyTerminated() bool { return c.terminated }

// TopDocs returns the collected hits as a TopDocs, sorted by score descending.
func (c *TopKnnCollector) TopDocs() *TopDocs {
	n := len(c.heap)
	out := make([]*ScoreDoc, n)
	for i := 0; i < n; i++ {
		out[i] = &ScoreDoc{Doc: c.docs[i], Score: c.heap[i]}
	}
	// Sort by score descending using a simple in-place insertion sort which is
	// fine for the small heap sizes used by k-NN.
	for i := 1; i < n; i++ {
		j := i
		for j > 0 && out[j-1].Score < out[j].Score {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}
	relation := EQUAL_TO
	if c.terminated {
		relation = GREATER_THAN_OR_EQUAL_TO
	}
	return NewTopDocs(NewTotalHits(int64(n), relation), out)
}

func (c *TopKnnCollector) siftUp(i int) {
	for i > 0 {
		p := (i - 1) >> 1
		if c.heap[i] >= c.heap[p] {
			return
		}
		c.heap[p], c.heap[i] = c.heap[i], c.heap[p]
		c.docs[p], c.docs[i] = c.docs[i], c.docs[p]
		i = p
	}
}

func (c *TopKnnCollector) siftDown(i int) {
	n := len(c.heap)
	for {
		l := i*2 + 1
		if l >= n {
			return
		}
		j := l
		if r := l + 1; r < n && c.heap[r] < c.heap[l] {
			j = r
		}
		if c.heap[j] >= c.heap[i] {
			return
		}
		c.heap[j], c.heap[i] = c.heap[i], c.heap[j]
		c.docs[j], c.docs[i] = c.docs[i], c.docs[j]
		i = j
	}
}
