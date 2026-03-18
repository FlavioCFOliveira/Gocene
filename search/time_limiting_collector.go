// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"time"
)

// TimeLimitingCollector terminates collection if it exceeds a time limit.
// This is the Go port of Lucene's org.apache.lucene.search.TimeLimitingCollector.
type TimeLimitingCollector struct {
	delegate Collector
	timeout  time.Duration
	baseline time.Time
}

// ErrTimeLimitExceeded is returned when the time limit is exceeded.
var ErrTimeLimitExceeded = errors.New("time limit exceeded")

// NewTimeLimitingCollector creates a new TimeLimitingCollector.
func NewTimeLimitingCollector(delegate Collector, timeout time.Duration) *TimeLimitingCollector {
	return &TimeLimitingCollector{
		delegate: delegate,
		timeout:  timeout,
		baseline: time.Now(),
	}
}

// Collect collects a document.
func (c *TimeLimitingCollector) Collect(doc int) error {
	if c.IsTimeout() {
		return ErrTimeLimitExceeded
	}
	return nil
}

// GetLeafCollector returns a LeafCollector for the given reader.
func (c *TimeLimitingCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	return c, nil
}

// ScoreMode returns the score mode for this collector.
func (c *TimeLimitingCollector) ScoreMode() ScoreMode {
	return c.delegate.ScoreMode()
}

// SetScorer sets the scorer for this collector.
func (c *TimeLimitingCollector) SetScorer(scorer Scorer) error {
	return nil
}

// SetBaseline sets the baseline time for timeout calculation.
func (c *TimeLimitingCollector) SetBaseline() {
	c.baseline = time.Now()
}

// IsTimeout returns true if the time limit has been exceeded.
func (c *TimeLimitingCollector) IsTimeout() bool {
	return time.Since(c.baseline) > c.timeout
}

// Ensure TimeLimitingCollector implements Collector and LeafCollector
var _ Collector = (*TimeLimitingCollector)(nil)
var _ LeafCollector = (*TimeLimitingCollector)(nil)
