// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// TotalHitCountCollector counts the total number of matching documents.
// This is the Go port of Lucene's org.apache.lucene.search.TotalHitCountCollector.
type TotalHitCountCollector struct {
	BaseCollector
	BaseLeafCollector
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

// GetLeafCollector returns a LeafCollector for the given context.
func (c *TotalHitCountCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	return c, nil
}

// ScoreMode returns the score mode for this collector.
func (c *TotalHitCountCollector) ScoreMode() ScoreMode {
	return COMPLETE_NO_SCORES
}

// SetScorer sets the scorer for this collector.
func (c *TotalHitCountCollector) SetScorer(scorer Scorable) error {
	return nil
}

// Ensure TotalHitCountCollector implements Collector and LeafCollector
var _ Collector = (*TotalHitCountCollector)(nil)
var _ LeafCollector = (*TotalHitCountCollector)(nil)

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (t *TotalHitCountCollector) CollectRange(min, max int) error {
	return DefaultCollectRange(t, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (t *TotalHitCountCollector) CollectStream(stream DocIdStream) error {
	return DefaultCollectStream(t, stream)
}
