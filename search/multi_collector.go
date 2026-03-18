// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MultiCollector combines multiple collectors.
// This is the Go port of Lucene's org.apache.lucene.search.MultiCollector.
type MultiCollector struct {
	collectors []Collector
}

// NewMultiCollector creates a new MultiCollector.
func NewMultiCollector(collectors ...Collector) *MultiCollector {
	return &MultiCollector{
		collectors: collectors,
	}
}

// Collect collects a document.
func (c *MultiCollector) Collect(doc int) error {
	for _, collector := range c.collectors {
		leafCollector, err := collector.GetLeafCollector(nil)
		if err != nil {
			return err
		}
		if err := leafCollector.Collect(doc); err != nil {
			return err
		}
	}
	return nil
}

// GetLeafCollector returns a LeafCollector for the given reader.
func (c *MultiCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	return c, nil
}

// ScoreMode returns the score mode for this collector.
func (c *MultiCollector) ScoreMode() ScoreMode {
	if len(c.collectors) > 0 {
		return c.collectors[0].ScoreMode()
	}
	return COMPLETE_NO_SCORES
}

// SetScorer sets the scorer for this collector.
func (c *MultiCollector) SetScorer(scorer Scorer) error {
	return nil
}

// GetCollectors returns the wrapped collectors.
func (c *MultiCollector) GetCollectors() []Collector {
	return c.collectors
}

// Ensure MultiCollector implements Collector and LeafCollector
var _ Collector = (*MultiCollector)(nil)
var _ LeafCollector = (*MultiCollector)(nil)
