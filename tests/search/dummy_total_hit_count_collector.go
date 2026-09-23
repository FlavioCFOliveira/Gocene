// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DummyTotalHitCountCollector is the Go port of
// org.apache.lucene.tests.search.DummyTotalHitCountCollector (Apache Lucene
// 10.5.0): a dummy version of [search.TotalHitCountCollector] that doesn't
// shortcut using Weight.count.
type DummyTotalHitCountCollector struct {
	search.BaseCollector
	totalHits int
}

// NewDummyTotalHitCountCollector renders the constructor.
func NewDummyTotalHitCountCollector() *DummyTotalHitCountCollector {
	return &DummyTotalHitCountCollector{}
}

// GetTotalHits gets the number of hits.
func (c *DummyTotalHitCountCollector) GetTotalHits() int {
	return c.totalHits
}

// ScoreMode renders scoreMode(): COMPLETE_NO_SCORES.
func (c *DummyTotalHitCountCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// GetLeafCollector renders getLeafCollector(LeafReaderContext): the anonymous
// LeafCollector counts every collected document.
func (c *DummyTotalHitCountCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	return &dummyTotalHitCountLeafCollector{outer: c}, nil
}

// dummyTotalHitCountLeafCollector renders the anonymous LeafCollector.
type dummyTotalHitCountLeafCollector struct {
	search.BaseLeafCollector
	outer *DummyTotalHitCountCollector
}

// SetScorer renders setScorer(Scorable), which is empty.
func (l *dummyTotalHitCountLeafCollector) SetScorer(scorer search.Scorable) error {
	return nil
}

// Collect renders collect(int): totalHits++.
func (l *dummyTotalHitCountLeafCollector) Collect(doc int) error {
	l.outer.totalHits++
	return nil
}

// CollectRange renders the default LeafCollector.collectRange(int, int).
func (l *dummyTotalHitCountLeafCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(l, min, max)
}

// CollectStream renders the default LeafCollector.collect(DocIdStream).
func (l *dummyTotalHitCountLeafCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(l, stream)
}

// dummyTotalHitCountCollectorManager renders the anonymous CollectorManager
// of createManager().
type dummyTotalHitCountCollectorManager struct{}

// NewCollector renders newCollector().
func (dummyTotalHitCountCollectorManager) NewCollector() (*DummyTotalHitCountCollector, error) {
	return NewDummyTotalHitCountCollector(), nil
}

// Reduce renders reduce(Collection): the sum of the collectors' hit counts.
func (dummyTotalHitCountCollectorManager) Reduce(collectors []*DummyTotalHitCountCollector) (int, error) {
	sum := 0
	for _, coll := range collectors {
		sum += coll.totalHits
	}
	return sum, nil
}

// DummyTotalHitCountCollectorCreateManager renders the static
// DummyTotalHitCountCollector.createManager(): create a collector manager.
func DummyTotalHitCountCollectorCreateManager() search.CollectorManager[*DummyTotalHitCountCollector, int] {
	return dummyTotalHitCountCollectorManager{}
}
