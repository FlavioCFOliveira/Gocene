// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FixedBitSetCollector is the Go port of
// org.apache.lucene.tests.search.FixedBitSetCollector (Apache Lucene 10.5.0):
// a collector that accumulates matching docs in a [util.FixedBitSet].
type FixedBitSetCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	bitSet  *util.FixedBitSet
	docBase int
}

// newFixedBitSetCollector renders the package-private
// FixedBitSetCollector(int maxDoc).
func newFixedBitSetCollector(maxDoc int) (*FixedBitSetCollector, error) {
	bitSet, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	c := &FixedBitSetCollector{bitSet: bitSet}
	c.Outer = c
	return c, nil
}

// GetLeafCollector renders SimpleCollector.getLeafCollector, which calls the
// overridden doSetNextReader and returns this.
func (c *FixedBitSetCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

// DoSetNextReader renders doSetNextReader(LeafReaderContext).
func (c *FixedBitSetCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	c.docBase = context.DocBase
	return nil
}

// Collect renders collect(int).
func (c *FixedBitSetCollector) Collect(doc int) error {
	c.bitSet.Set(c.docBase + doc)
	return nil
}

// ScoreMode renders scoreMode(): COMPLETE_NO_SCORES.
func (c *FixedBitSetCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// CollectRange renders the default LeafCollector.collectRange(int, int).
func (c *FixedBitSetCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream renders the default LeafCollector.collect(DocIdStream).
func (c *FixedBitSetCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// CompetitiveIterator renders the default LeafCollector.competitiveIterator().
func (c *FixedBitSetCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return c.BaseLeafCollector.CompetitiveIterator()
}

// Finish renders the default LeafCollector.finish().
func (c *FixedBitSetCollector) Finish() error {
	return c.BaseLeafCollector.Finish()
}

// fixedBitSetCollectorManager renders the anonymous CollectorManager of
// createManager(int).
type fixedBitSetCollectorManager struct {
	maxDoc int
}

// NewCollector renders newCollector().
func (m fixedBitSetCollectorManager) NewCollector() (*FixedBitSetCollector, error) {
	return newFixedBitSetCollector(m.maxDoc)
}

// Reduce renders reduce(Collection): the union of the collectors' bit sets.
func (m fixedBitSetCollectorManager) Reduce(collectors []*FixedBitSetCollector) (*util.FixedBitSet, error) {
	reduced, err := util.NewFixedBitSet(m.maxDoc)
	if err != nil {
		return nil, err
	}
	for _, collector := range collectors {
		if err := reduced.Or(collector.bitSet); err != nil {
			return nil, err
		}
	}
	return reduced, nil
}

// FixedBitSetCollectorCreateManager renders the static
// FixedBitSetCollector.createManager(int maxDoc): creates a CollectorManager
// that can concurrently collect matching docs in a FixedBitSet.
func FixedBitSetCollectorCreateManager(maxDoc int) search.CollectorManager[*FixedBitSetCollector, *util.FixedBitSet] {
	return fixedBitSetCollectorManager{maxDoc: maxDoc}
}
