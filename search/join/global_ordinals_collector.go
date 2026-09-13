// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GlobalOrdinalsCollector collects all ordinals from the specified field
// matching the query.
//
// Mirrors org.apache.lucene.search.join.GlobalOrdinalsCollector of Apache
// Lucene 10.5.0.
type GlobalOrdinalsCollector struct {
	// field mirrors `final String field`.
	field string
	// collectedOrds mirrors `final LongBitSet collectedOrds`.
	collectedOrds *util.LongBitSet
	// ordinalMap mirrors `final OrdinalMap ordinalMap`.
	ordinalMap *index.OrdinalMap
}

// NewGlobalOrdinalsCollector mirrors the package-private constructor
// GlobalOrdinalsCollector(String field, OrdinalMap ordinalMap, long valueCount).
//
// Java's `new LongBitSet(valueCount)` throws on a negative count; the Gocene
// util.NewLongBitSet reports that same rejection as an error instead.
func NewGlobalOrdinalsCollector(field string, ordinalMap *index.OrdinalMap, valueCount int64) (*GlobalOrdinalsCollector, error) {
	collectedOrds, err := util.NewLongBitSet(valueCount)
	if err != nil {
		return nil, err
	}
	return &GlobalOrdinalsCollector{
		field:         field,
		ordinalMap:    ordinalMap,
		collectedOrds: collectedOrds,
	}, nil
}

// GetCollectedOrds returns the ordinals collected so far.
//
// Mirrors `public LongBitSet getCollectorOrdinals()`; the accessor carries the
// name already used across the Gocene join package (see
// GlobalOrdinalsWithScoreCollector).
func (c *GlobalOrdinalsCollector) GetCollectedOrds() *util.LongBitSet {
	return c.collectedOrds
}

// ScoreMode mirrors `public ScoreMode scoreMode()`, which returns
// org.apache.lucene.search.ScoreMode.COMPLETE_NO_SCORES.
func (c *GlobalOrdinalsCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// SetWeight mirrors the default body of Collector.setWeight(Weight), which is
// empty.
func (c *GlobalOrdinalsCollector) SetWeight(_ search.Weight) {}

// GetLeafCollector mirrors
// `public LeafCollector getLeafCollector(LeafReaderContext context)`.
//
// Java obtains the iterator with DocValues.getSorted(context.reader(), field),
// which substitutes an empty instance when the field carries no doc values.
// The Gocene helper getSortedDocValues reports that same "nothing to read"
// case as a nil iterator, which the leaf collectors below treat as empty.
func (c *GlobalOrdinalsCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	docTermOrds, err := getSortedDocValues(context, c.field)
	if err != nil {
		return nil, err
	}
	if c.ordinalMap != nil {
		var segmentOrdToGlobalOrdLookup []int64
		if context != nil {
			segmentOrdToGlobalOrdLookup = c.ordinalMap.GetGlobalOrds(context.Ord)
		}
		return &ordinalMapCollector{
			docTermOrds:                 docTermOrds,
			segmentOrdToGlobalOrdLookup: segmentOrdToGlobalOrdLookup,
			parent:                      c,
		}, nil
	}
	return &segmentOrdinalCollector{
		docTermOrds: docTermOrds,
		parent:      c,
	}, nil
}

// ordinalMapCollector mirrors the inner class
// GlobalOrdinalsCollector.OrdinalMapCollector.
type ordinalMapCollector struct {
	search.BaseLeafCollector

	docTermOrds                 index.SortedDocValues
	segmentOrdToGlobalOrdLookup []int64
	parent                      *GlobalOrdinalsCollector
}

// SetScorer mirrors `public void setScorer(Scorable scorer)`, whose body is empty.
func (c *ordinalMapCollector) SetScorer(_ search.Scorable) error { return nil }

// Collect mirrors `public void collect(int doc)`.
func (c *ordinalMapCollector) Collect(doc int) error {
	if c.docTermOrds == nil {
		return nil
	}
	ok, err := c.docTermOrds.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	segmentOrd, err := c.docTermOrds.OrdValue()
	if err != nil {
		return err
	}
	if segmentOrd < 0 || segmentOrd >= len(c.segmentOrdToGlobalOrdLookup) {
		return nil
	}
	globalOrd := c.segmentOrdToGlobalOrdLookup[segmentOrd]
	c.parent.collectedOrds.Set(globalOrd)
	return nil
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *ordinalMapCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *ordinalMapCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// segmentOrdinalCollector mirrors the inner class
// GlobalOrdinalsCollector.SegmentOrdinalCollector.
type segmentOrdinalCollector struct {
	search.BaseLeafCollector

	docTermOrds index.SortedDocValues
	parent      *GlobalOrdinalsCollector
}

// SetScorer mirrors `public void setScorer(Scorable scorer)`, whose body is empty.
func (c *segmentOrdinalCollector) SetScorer(_ search.Scorable) error { return nil }

// Collect mirrors `public void collect(int doc)`.
func (c *segmentOrdinalCollector) Collect(doc int) error {
	if c.docTermOrds == nil {
		return nil
	}
	ok, err := c.docTermOrds.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	ord, err := c.docTermOrds.OrdValue()
	if err != nil {
		return err
	}
	if ord < 0 {
		return nil
	}
	c.parent.collectedOrds.Set(int64(ord))
	return nil
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *segmentOrdinalCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *segmentOrdinalCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

var (
	_ search.Collector     = (*GlobalOrdinalsCollector)(nil)
	_ search.LeafCollector = (*ordinalMapCollector)(nil)
	_ search.LeafCollector = (*segmentOrdinalCollector)(nil)
)
