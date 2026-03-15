// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"container/heap"
	"sync"
)

// TopScoreDocCollector collects top-N documents sorted by score.
//
// This is the Go port of Lucene's org.apache.lucene.search.TopScoreDocCollector.
//
// This is a specialized collector that optimizes for score-based sorting.
// It is more efficient than TopFieldCollector when only score sorting is needed.
type TopScoreDocCollector struct {
	*SimpleCollector

	// numHits is the maximum number of hits to collect
	numHits int

	// after is the ScoreDoc to start after (for pagination)
	after *ScoreDoc

	// pq is the priority queue of scored documents
	pq *ScoreDocPriorityQueue

	// totalHits tracks the total number of hits
	totalHits int

	// maxScore tracks the maximum score seen
	maxScore float32

	// mu protects mutable fields
	mu sync.RWMutex
}

// NewTopScoreDocCollector creates a new TopScoreDocCollector.
func NewTopScoreDocCollector(numHits int) *TopScoreDocCollector {
	return &TopScoreDocCollector{
		SimpleCollector: NewSimpleCollector(COMPLETE),
		numHits:         numHits,
		pq:              NewScoreDocPriorityQueue(numHits),
		totalHits:       0,
		maxScore:        0,
	}
}

// NewTopScoreDocCollectorWithAfter creates a new TopScoreDocCollector with pagination.
func NewTopScoreDocCollectorWithAfter(numHits int, after *ScoreDoc) *TopScoreDocCollector {
	return &TopScoreDocCollector{
		SimpleCollector: NewSimpleCollector(COMPLETE),
		numHits:         numHits,
		after:           after,
		pq:              NewScoreDocPriorityQueue(numHits),
		totalHits:       0,
		maxScore:        0,
	}
}

// GetLeafCollector returns a LeafCollector for the given context.
func (c *TopScoreDocCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	return NewTopScoreDocLeafCollector(c, 0), nil
}

// TopDocs returns the collected top documents.
func (c *TopScoreDocCollector) TopDocs() *TopDocs {
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
func (c *TopScoreDocCollector) GetTotalHits() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalHits
}

// GetMaxScore returns the maximum score seen.
func (c *TopScoreDocCollector) GetMaxScore() float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxScore
}

// TopScoreDocLeafCollector collects documents for a single segment with score sorting.
type TopScoreDocLeafCollector struct {
	*BaseLeafCollector
	collector *TopScoreDocCollector
	scorer    Scorer
	docBase   int
}

// NewTopScoreDocLeafCollector creates a new TopScoreDocLeafCollector.
func NewTopScoreDocLeafCollector(collector *TopScoreDocCollector, docBase int) *TopScoreDocLeafCollector {
	return &TopScoreDocLeafCollector{
		BaseLeafCollector: NewBaseLeafCollector(),
		collector:         collector,
		docBase:           docBase,
	}
}

// SetScorer sets the scorer.
func (c *TopScoreDocLeafCollector) SetScorer(scorer Scorer) error {
	c.scorer = scorer
	return nil
}

// SetDocBase sets the doc base.
func (c *TopScoreDocLeafCollector) SetDocBase(docBase int) {
	c.docBase = docBase
}

// Collect collects a document.
func (c *TopScoreDocLeafCollector) Collect(doc int) error {
	c.collector.mu.Lock()
	defer c.collector.mu.Unlock()

	c.collector.totalHits++
	score := c.scorer.Score()

	if score > c.collector.maxScore {
		c.collector.maxScore = score
	}

	// Create a ScoreDoc for this document
	docID := c.docBase + doc

	// Check pagination - skip documents before 'after'
	if c.collector.after != nil {
		if docID <= c.collector.after.Doc {
			return nil
		}
	}

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

// Ensure TopScoreDocCollector implements Collector
var _ Collector = (*TopScoreDocCollector)(nil)
