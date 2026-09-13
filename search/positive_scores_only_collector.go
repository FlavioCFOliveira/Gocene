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
	BaseCollector
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
	return WrapScoreCachingLeafCollector(&positiveScoresOnlyLeafCollector{leaf: leaf}), nil
}

// ScoreMode delegates to the wrapped collector.
func (c *PositiveScoresOnlyCollector) ScoreMode() ScoreMode { return c.inner.ScoreMode() }

// positiveScoresOnlyLeafCollector is the anonymous FilterLeafCollector declared
// inside PositiveScoresOnlyCollector.getLeafCollector; the score caching that
// Java applies around it is supplied by WrapScoreCachingLeafCollector, exactly
// as in ScoreCachingWrappingScorer.wrap(...) at the call site above.
type positiveScoresOnlyLeafCollector struct {
	BaseLeafCollector
	leaf   LeafCollector
	scorer Scorable
}

func (l *positiveScoresOnlyLeafCollector) SetScorer(scorer Scorable) error {
	l.scorer = scorer
	return l.leaf.SetScorer(scorer)
}

func (l *positiveScoresOnlyLeafCollector) Collect(doc int) error {
	if l.scorer == nil {
		return l.leaf.Collect(doc)
	}
	score, err := l.scorer.Score()
	if err != nil {
		return err
	}
	if score > 0 {
		return l.leaf.Collect(doc)
	}
	return nil
}

// Compile-time guard
var (
	_ Collector     = (*PositiveScoresOnlyCollector)(nil)
	_ LeafCollector = (*positiveScoresOnlyLeafCollector)(nil)
)

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (p *positiveScoresOnlyLeafCollector) CollectRange(min, max int) error {
	return DefaultCollectRange(p, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (p *positiveScoresOnlyLeafCollector) CollectStream(stream DocIdStream) error {
	return DefaultCollectStream(p, stream)
}
