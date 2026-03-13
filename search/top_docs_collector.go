// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"container/heap"
	"sync"
)

// TopDocsCollector collects top-N documents by score.
//
// This is the Go port of Lucene's org.apache.lucene.search.TopDocsCollector.
//
// This collector maintains a priority queue of the top scoring documents
// and returns them as TopDocs when the search is complete.
type TopDocsCollector struct {
	*SimpleCollector

	// numHits is the maximum number of hits to collect
	numHits int

	// pq is the priority queue of scored documents
	pq *ScoreDocPriorityQueue

	// totalHits tracks the total number of hits
	totalHits int

	// maxScore tracks the maximum score seen
	maxScore float32

	// mu protects mutable fields
	mu sync.RWMutex
}

// NewTopDocsCollector creates a new TopDocsCollector.
func NewTopDocsCollector(numHits int) *TopDocsCollector {
	return &TopDocsCollector{
		SimpleCollector: NewSimpleCollector(COMPLETE),
		numHits:         numHits,
		pq:              NewScoreDocPriorityQueue(numHits),
		totalHits:       0,
		maxScore:        0,
	}
}

// GetLeafCollector returns a LeafCollector for the given context.
func (c *TopDocsCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	// For now, we don't have a way to get the docBase from the reader here
	// but the searcher will call a method to set it or we'll pass it.
	return NewTopDocsLeafCollector(c, 0), nil
}

// TopDocs returns the collected top documents.
func (c *TopDocsCollector) TopDocs() *TopDocs {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scoreDocs := make([]*ScoreDoc, c.pq.Len())
	for i := len(scoreDocs) - 1; i >= 0; i-- {
		scoreDocs[i] = heap.Pop(c.pq).(*ScoreDoc)
	}

	return &TopDocs{
		TotalHits: NewTotalHits(int64(c.totalHits), EQUAL_TO),
		ScoreDocs: scoreDocs,
		MaxScore:  c.maxScore,
	}
}

// GetTotalHits returns the total number of hits collected.
func (c *TopDocsCollector) GetTotalHits() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalHits
}

// GetMaxScore returns the maximum score seen.
func (c *TopDocsCollector) GetMaxScore() float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxScore
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
func (c *TopDocsLeafCollector) Collect(doc int) error {
	c.collector.mu.Lock()
	defer c.collector.mu.Unlock()

	c.collector.totalHits++
	score := c.scorer.Score()

	if score > c.collector.maxScore {
		c.collector.maxScore = score
	}

	// Create a ScoreDoc for this document
	docID := c.docBase + doc
	scoreDoc := NewScoreDoc(docID, score, 0)

	// Add to priority queue
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
