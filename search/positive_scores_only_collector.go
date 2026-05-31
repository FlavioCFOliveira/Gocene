// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// PositiveScoresOnlyCollector wraps a Collector so only documents whose score
// is strictly greater than 0 are forwarded to the inner collector.
//
// Mirrors org.apache.lucene.search.PositiveScoresOnlyCollector.
type PositiveScoresOnlyCollector struct {
	inner Collector
}

// NewPositiveScoresOnlyCollector wraps inner so only positive-score docs are
// collected.
func NewPositiveScoresOnlyCollector(inner Collector) *PositiveScoresOnlyCollector {
	return &PositiveScoresOnlyCollector{inner: inner}
}

// GetLeafCollector returns a LeafCollector that filters by score > 0.
func (c *PositiveScoresOnlyCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	leaf, err := c.inner.GetLeafCollector(context)
	if err != nil {
		return nil, err
	}
	return &positiveScoresOnlyLeafCollector{leaf: leaf}, nil
}

// ScoreMode delegates to the wrapped collector.
func (c *PositiveScoresOnlyCollector) ScoreMode() ScoreMode { return c.inner.ScoreMode() }

type positiveScoresOnlyLeafCollector struct {
	leaf   LeafCollector
	scorer Scorer
}

func (l *positiveScoresOnlyLeafCollector) SetScorer(scorer Scorer) error {
	wrapped := WrapScoreCachingScorer(scorer)
	l.scorer = wrapped
	return l.leaf.SetScorer(wrapped)
}

func (l *positiveScoresOnlyLeafCollector) Collect(doc int) error {
	if l.scorer == nil {
		return l.leaf.Collect(doc)
	}
	if l.scorer.Score() > 0 {
		return l.leaf.Collect(doc)
	}
	return nil
}

// Compile-time guard
var (
	_ Collector     = (*PositiveScoresOnlyCollector)(nil)
	_ LeafCollector = (*positiveScoresOnlyLeafCollector)(nil)
)
