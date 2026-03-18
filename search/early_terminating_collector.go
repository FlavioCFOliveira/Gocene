// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// EarlyTerminatingCollector terminates collection after a specified number of documents.
// This is the Go port of Lucene's org.apache.lucene.search.EarlyTerminatingCollector.
type EarlyTerminatingCollector struct {
	delegate  Collector
	maxDocs   int
	collected int
}

// NewEarlyTerminatingCollector creates a new EarlyTerminatingCollector.
func NewEarlyTerminatingCollector(delegate Collector, maxDocs int) *EarlyTerminatingCollector {
	return &EarlyTerminatingCollector{
		delegate:  delegate,
		maxDocs:   maxDocs,
		collected: 0,
	}
}

// Collect collects a document.
func (c *EarlyTerminatingCollector) Collect(doc int) error {
	if c.collected >= c.maxDocs {
		return nil // Early termination
	}
	c.collected++
	return nil
}

// GetCollected returns the number of collected documents.
func (c *EarlyTerminatingCollector) GetCollected() int {
	return c.collected
}

// GetLeafCollector returns a LeafCollector for the given reader.
func (c *EarlyTerminatingCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	return c, nil
}

// ScoreMode returns the score mode for this collector.
func (c *EarlyTerminatingCollector) ScoreMode() ScoreMode {
	return c.delegate.ScoreMode()
}

// SetScorer sets the scorer for this collector.
func (c *EarlyTerminatingCollector) SetScorer(scorer Scorer) error {
	return nil
}

// Ensure EarlyTerminatingCollector implements Collector and LeafCollector
var _ Collector = (*EarlyTerminatingCollector)(nil)
var _ LeafCollector = (*EarlyTerminatingCollector)(nil)
