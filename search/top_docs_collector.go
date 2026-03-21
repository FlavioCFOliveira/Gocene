// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"container/heap"
	"math"
	"sync"
	"sync/atomic"
)

// TopDocsCollector collects top-N documents by score.
//
// This is the Go port of Lucene's org.apache.lucene.search.TopDocsCollector.
//
// This collector maintains a priority queue of the top scoring documents
// and returns them as TopDocs when the search is complete.
// Uses atomic operations for hot path counters to minimize lock contention.
type TopDocsCollector struct {
	*SimpleCollector

	// numHits is the maximum number of hits to collect
	numHits int

	// pq is the priority queue of scored documents
	pq *ScoreDocPriorityQueue

	// totalHits tracks the total number of hits (atomic for lock-free increments)
	totalHits atomic.Int32

	// maxScore tracks the maximum score seen as uint32 bits (atomic for lock-free updates)
	maxScore atomic.Uint32

	// mu protects priority queue operations (not for counters)
	mu sync.RWMutex
}

// NewTopDocsCollector creates a new TopDocsCollector.
func NewTopDocsCollector(numHits int) *TopDocsCollector {
	c := &TopDocsCollector{
		SimpleCollector: NewSimpleCollector(COMPLETE),
		numHits:         numHits,
		pq:              NewScoreDocPriorityQueue(numHits),
	}
	c.totalHits.Store(0)
	c.maxScore.Store(0) // 0 bits for float32 0.0
	return c
}

// GetLeafCollector returns a LeafCollector for the given context.
func (c *TopDocsCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	// For now, we don't have a way to get the docBase from the reader here
	// but the searcher will call a method to set it or we'll pass it.
	return NewTopDocsLeafCollector(c, 0), nil
}

// TopDocs returns the collected top documents.
func (c *TopDocsCollector) TopDocs() *TopDocs {
	c.mu.Lock()
	defer c.mu.Unlock()

	scoreDocs := make([]*ScoreDoc, c.pq.Len())
	for i := len(scoreDocs) - 1; i >= 0; i-- {
		scoreDocs[i] = heap.Pop(c.pq).(*ScoreDoc)
	}

	return &TopDocs{
		TotalHits: NewTotalHits(int64(c.totalHits.Load()), EQUAL_TO),
		ScoreDocs: scoreDocs,
		MaxScore:  math.Float32frombits(c.maxScore.Load()),
	}
}

// GetTotalHits returns the total number of hits collected.
func (c *TopDocsCollector) GetTotalHits() int {
	return int(c.totalHits.Load())
}

// GetMaxScore returns the maximum score seen.
func (c *TopDocsCollector) GetMaxScore() float32 {
	return math.Float32frombits(c.maxScore.Load())
}

// TopDocsLeafCollector collects documents for a single segment.
type TopDocsLeafCollector struct {
	*BaseLeafCollector
	collector *TopDocsCollector
	scorer    Scorer
	docBase   int
}

// NewTopDocsLeafCollector creates a new TopDocsLeafCollector.
func NewTopDocsLeafCollector(collector *TopDocsCollector, docBase int) *TopDocsLeafCollector {
	return &TopDocsLeafCollector{
		BaseLeafCollector: NewBaseLeafCollector(),
		collector:         collector,
		docBase:           docBase,
	}
}

// SetScorer sets the scorer.
func (c *TopDocsLeafCollector) SetScorer(scorer Scorer) error {
	c.scorer = scorer
	return nil
}

// SetDocBase sets the doc base.
func (c *TopDocsLeafCollector) SetDocBase(docBase int) {
	c.docBase = docBase
}

// Collect collects a document.
// Uses atomic operations for counters to minimize lock contention.
func (c *TopDocsLeafCollector) Collect(doc int) error {
	score := c.scorer.Score()

	// Atomic increment for totalHits (lock-free)
	c.collector.totalHits.Add(1)

	// Atomic update for maxScore using uint32 comparison (lock-free)
	scoreBits := math.Float32bits(score)
	for {
		oldMaxBits := c.collector.maxScore.Load()
		oldMax := math.Float32frombits(oldMaxBits)
		if score <= oldMax {
			break
		}
		if c.collector.maxScore.CompareAndSwap(oldMaxBits, scoreBits) {
			break
		}
	}

	// Create a ScoreDoc for this document
	docID := c.docBase + doc
	scoreDoc := NewScoreDoc(docID, score, 0)

	// Only lock for priority queue operations
	c.collector.mu.Lock()
	if c.collector.pq.Len() < c.collector.numHits {
		heap.Push(c.collector.pq, scoreDoc)
	} else if c.collector.pq.Len() > 0 {
		// Get the lowest scoring document in the queue
		bottom := c.collector.pq.Peek()
		if bottom != nil && score > bottom.Score {
			heap.Pop(c.collector.pq)
			heap.Push(c.collector.pq, scoreDoc)
		}
	}
	c.collector.mu.Unlock()

	return nil
}

// ScoreDocPriorityQueue implements a priority queue for ScoreDoc.
type ScoreDocPriorityQueue struct {
	items []*ScoreDoc
	mu    sync.RWMutex
}

// NewScoreDocPriorityQueue creates a new ScoreDocPriorityQueue.
func NewScoreDocPriorityQueue(capacity int) *ScoreDocPriorityQueue {
	return &ScoreDocPriorityQueue{
		items: make([]*ScoreDoc, 0, capacity),
	}
}

// Len returns the length of the queue.
func (pq *ScoreDocPriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.items)
}

// Less compares two items.
func (pq *ScoreDocPriorityQueue) Less(i, j int) bool {
	return pq.items[i].Score < pq.items[j].Score
}

// Swap swaps two items.
func (pq *ScoreDocPriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

// Push adds an item.
func (pq *ScoreDocPriorityQueue) Push(x interface{}) {
	item := x.(*ScoreDoc)
	pq.items = append(pq.items, item)
}

// Pop removes and returns the lowest scoring item.
func (pq *ScoreDocPriorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	pq.items = old[0 : n-1]
	return item
}

// Peek returns the lowest scoring item without removing it.
func (pq *ScoreDocPriorityQueue) Peek() *ScoreDoc {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	if len(pq.items) == 0 {
		return nil
	}
	return pq.items[0]
}

// Ensure TopDocsCollector implements Collector
var _ Collector = (*TopDocsCollector)(nil)
