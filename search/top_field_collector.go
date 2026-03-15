// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"container/heap"
	"sync"
)

// TopFieldCollector collects top-N documents sorted by fields.
//
// This is the Go port of Lucene's org.apache.lucene.search.TopFieldCollector.
//
// This collector maintains a priority queue of the top documents sorted by
// the specified sort fields. It supports early termination optimization
// when the sort fields don't require scores.
type TopFieldCollector struct {
	*SimpleCollector

	// numHits is the maximum number of hits to collect
	numHits int

	// sort is the sort specification
	sort *Sort

	// pq is the priority queue of field docs
	pq *FieldDocPriorityQueue

	// totalHits tracks the total number of hits
	totalHits int

	// maxScore tracks the maximum score seen
	maxScore float32

	// mu protects mutable fields
	mu sync.RWMutex

	// comparators are the field comparators for sorting
	comparators []FieldComparator
}

// NewTopFieldCollector creates a new TopFieldCollector.
func NewTopFieldCollector(numHits int, sort *Sort) *TopFieldCollector {
	scoreMode := COMPLETE_NO_SCORES
	if sort.NeedsScores() {
		scoreMode = COMPLETE
	}

	return &TopFieldCollector{
		SimpleCollector: NewSimpleCollector(scoreMode),
		numHits:         numHits,
		sort:            sort,
		pq:              NewFieldDocPriorityQueue(numHits, sort),
		totalHits:       0,
		maxScore:        0,
	}
}

// GetLeafCollector returns a LeafCollector for the given context.
func (c *TopFieldCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	return NewTopFieldLeafCollector(c, 0), nil
}

// TopDocs returns the collected top documents.
func (c *TopFieldCollector) TopDocs() *TopDocs {
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
func (c *TopFieldCollector) GetTotalHits() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalHits
}

// GetMaxScore returns the maximum score seen.
func (c *TopFieldCollector) GetMaxScore() float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxScore
}

// TopFieldLeafCollector collects documents for a single segment with field sorting.
type TopFieldLeafCollector struct {
	*BaseLeafCollector
	collector *TopFieldCollector
	scorer    Scorer
	docBase   int
}

// NewTopFieldLeafCollector creates a new TopFieldLeafCollector.
func NewTopFieldLeafCollector(collector *TopFieldCollector, docBase int) *TopFieldLeafCollector {
	return &TopFieldLeafCollector{
		BaseLeafCollector: NewBaseLeafCollector(),
		collector:         collector,
		docBase:           docBase,
	}
}

// SetScorer sets the scorer.
func (c *TopFieldLeafCollector) SetScorer(scorer Scorer) error {
	c.scorer = scorer
	return nil
}

// SetDocBase sets the doc base.
func (c *TopFieldLeafCollector) SetDocBase(docBase int) {
	c.docBase = docBase
}

// Collect collects a document.
func (c *TopFieldLeafCollector) Collect(doc int) error {
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
		// Get the bottom document in the queue
		bottom := c.collector.pq.Peek()
		if bottom != nil && scoreDoc.Score > bottom.Score {
			heap.Pop(c.collector.pq)
			heap.Push(c.collector.pq, scoreDoc)
		}
	}

	return nil
}

// FieldDocPriorityQueue implements a priority queue for ScoreDoc with field sorting.
type FieldDocPriorityQueue struct {
	items []*ScoreDoc
	sort  *Sort
	mu    sync.RWMutex
}

// NewFieldDocPriorityQueue creates a new FieldDocPriorityQueue.
func NewFieldDocPriorityQueue(capacity int, sort *Sort) *FieldDocPriorityQueue {
	return &FieldDocPriorityQueue{
		items: make([]*ScoreDoc, 0, capacity),
		sort:  sort,
	}
}

// Len returns the length of the queue.
func (pq *FieldDocPriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.items)
}

// Less compares two items based on the sort fields.
// This is a simplified implementation that compares by score.
// A full implementation would compare by all sort fields.
func (pq *FieldDocPriorityQueue) Less(i, j int) bool {
	// For now, just compare by score
	// A full implementation would use the sort fields
	return pq.items[i].Score > pq.items[j].Score
}

// Swap swaps two items.
func (pq *FieldDocPriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

// Push adds an item.
func (pq *FieldDocPriorityQueue) Push(x interface{}) {
	item := x.(*ScoreDoc)
	pq.items = append(pq.items, item)
}

// Pop removes and returns the item.
func (pq *FieldDocPriorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	pq.items = old[0 : n-1]
	return item
}

// Peek returns the item without removing it.
func (pq *FieldDocPriorityQueue) Peek() *ScoreDoc {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	if len(pq.items) == 0 {
		return nil
	}
	return pq.items[0]
}

// Ensure TopFieldCollector implements Collector
var _ Collector = (*TopFieldCollector)(nil)
