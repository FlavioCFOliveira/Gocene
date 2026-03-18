// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// TotalHitCountCollector counts the total number of matching documents.
// This is the Go port of Lucene's org.apache.lucene.search.TotalHitCountCollector.
type TotalHitCountCollector struct {
	totalHits int
}

// NewTotalHitCountCollector creates a new TotalHitCountCollector.
func NewTotalHitCountCollector() *TotalHitCountCollector {
	return &TotalHitCountCollector{}
}

// Collect collects a document.
func (c *TotalHitCountCollector) Collect(doc int) error {
	c.totalHits++
	return nil
}

// GetTotalHits returns the total number of hits.
func (c *TotalHitCountCollector) GetTotalHits() int {
	return c.totalHits
}

// GetLeafCollector returns a LeafCollector for the given reader.
func (c *TotalHitCountCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	return c, nil
}

// ScoreMode returns the score mode for this collector.
func (c *TotalHitCountCollector) ScoreMode() ScoreMode {
	return COMPLETE_NO_SCORES
}

// SetScorer sets the scorer for this collector.
func (c *TotalHitCountCollector) SetScorer(scorer Scorer) error {
	return nil
}

// Ensure TotalHitCountCollector implements Collector and LeafCollector
var _ Collector = (*TotalHitCountCollector)(nil)
var _ LeafCollector = (*TotalHitCountCollector)(nil)
