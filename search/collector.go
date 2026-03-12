// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// LeafCollector collects matching documents in a segment.
//
// This is the Go port of Lucene's org.apache.lucene.search.LeafCollector.
type LeafCollector interface {
	// SetScorer sets the scorer for this collector.
	SetScorer(scorer Scorer) error

	// Collect collects the given document.
	Collect(doc int) error
}

// Collector collects matching documents during search.
//
// This is the Go port of Lucene's org.apache.lucene.search.Collector.
//
// Collector is the base class for all collectors. Collectors receive
// matching documents and typically store them in some data structure.
// Common collectors include TopDocsCollector (for top-N results) and
// TotalHitCountCollector (for counting total hits).
type Collector interface {
	// GetLeafCollector returns a LeafCollector for the given context.
	GetLeafCollector(reader IndexReader) (LeafCollector, error)

	// ScoreMode returns the ScoreMode indicating how scores are needed.
	ScoreMode() ScoreMode
}

// ScoreMode indicates how scores are needed by the collector.
type ScoreMode int

const (
	// COMPLETE - scores are needed and must be complete.
	COMPLETE ScoreMode = iota
	// COMPLETE_NO_SCORES - scores are not needed.
	COMPLETE_NO_SCORES
	// TOP_SCORES - only top scores are needed.
	TOP_SCORES
	// TOP_DOCS - only top docs are needed (no scores).
	TOP_DOCS
)

// SimpleCollector provides a base implementation for collectors.
type SimpleCollector struct {
	scoreMode ScoreMode
}

// NewSimpleCollector creates a new SimpleCollector.
func NewSimpleCollector(scoreMode ScoreMode) *SimpleCollector {
	return &SimpleCollector{scoreMode: scoreMode}
}

// ScoreMode returns the score mode.
func (c *SimpleCollector) ScoreMode() ScoreMode {
	return c.scoreMode
}

// BaseLeafCollector provides a base implementation for LeafCollector.
type BaseLeafCollector struct {
	scorer Scorer
}

// NewBaseLeafCollector creates a new BaseLeafCollector.
func NewBaseLeafCollector() *BaseLeafCollector {
	return &BaseLeafCollector{}
}

// SetScorer sets the scorer.
func (c *BaseLeafCollector) SetScorer(scorer Scorer) error {
	c.scorer = scorer
	return nil
}

// Collect collects a document.
func (c *BaseLeafCollector) Collect(doc int) error {
	return nil
}
