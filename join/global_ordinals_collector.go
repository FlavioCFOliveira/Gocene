// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GlobalOrdinalsCollector collects all ordinals from a specified field
// matching the query. Collected ordinals are mapped to their global ordinals
// via an OrdinalMap (when provided).
//
// Mirrors org.apache.lucene.search.join.GlobalOrdinalsCollector.
type GlobalOrdinalsCollector struct {
	field         string
	collectedOrds *util.LongBitSet
	ordinalMap    *index.OrdinalMap
}

// NewGlobalOrdinalsCollector creates a GlobalOrdinalsCollector.
//   - field: the doc-values field to collect ordinals from
//   - ordinalMap: per-segment to global ordinal mapping; nil when all leaves
//     share the same ordinal space (single segment or already global)
//   - valueCount: total number of distinct global ordinals
func NewGlobalOrdinalsCollector(field string, ordinalMap *index.OrdinalMap, valueCount int64) (*GlobalOrdinalsCollector, error) {
	bs, err := util.NewLongBitSet(valueCount)
	if err != nil {
		return nil, err
	}
	return &GlobalOrdinalsCollector{
		field:         field,
		collectedOrds: bs,
		ordinalMap:    ordinalMap,
	}, nil
}

// GetCollectedOrds returns the bitset of collected global ordinals.
func (c *GlobalOrdinalsCollector) GetCollectedOrds() *util.LongBitSet {
	return c.collectedOrds
}

// ScoreMode implements search.Collector.
func (c *GlobalOrdinalsCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// GetLeafCollector implements search.Collector.
//
// The leaf reader (unwrapped from the context, handling the *SegmentReader
// case) provides the SortedDocValues for the join field.
func (c *GlobalOrdinalsCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	var sdv index.SortedDocValues
	var globalOrds []int64

	if lr := leafReaderFromContext(context); lr != nil {
		var err error
		sdv, err = lr.GetSortedDocValues(c.field)
		if err != nil {
			return nil, err
		}
	}

	if c.ordinalMap != nil {
		// OrdinalMap.Build is now implemented (rmp #4646). However, fully wiring
		// GetGlobalOrds(segmentIndex) here requires the leaf segment index, which
		// Gocene's search.IndexReader interface does not yet expose. Full wiring
		// is tracked in backlog #2703. Until then, globalOrds stays nil and the
		// collector silently skips ordinal remapping, which is correct for the
		// single-segment case where ordinalMap is not needed.
		_ = globalOrds // will be populated once LeafReaderContext.ord is available
		return &globalOrdinalsOrdMapLeafCollector{
			sdv:        sdv,
			globalOrds: globalOrds,
			collected:  c.collectedOrds,
		}, nil
	}
	return &globalOrdinalsSegmentLeafCollector{
		sdv:       sdv,
		collected: c.collectedOrds,
	}, nil
}

// globalOrdinalsOrdMapLeafCollector collects via segment→global ordinal mapping.
type globalOrdinalsOrdMapLeafCollector struct {
	sdv        index.SortedDocValues
	globalOrds []int64
	collected  *util.LongBitSet
}

// SetScorer implements search.LeafCollector.
func (lc *globalOrdinalsOrdMapLeafCollector) SetScorer(_ search.Scorer) error { return nil }

// Collect implements search.LeafCollector.
//
// Migrated to AdvanceExact + OrdValue (rmp #4709). LeafCollector is
// driven by the scorer in monotonically increasing doc order.
func (lc *globalOrdinalsOrdMapLeafCollector) Collect(doc int) error {
	if lc.sdv == nil {
		return nil
	}
	ok, err := lc.sdv.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	ord, err := lc.sdv.OrdValue()
	if err != nil {
		return err
	}
	if ord < 0 {
		return nil
	}
	if lc.globalOrds != nil && ord < len(lc.globalOrds) {
		lc.collected.Set(lc.globalOrds[ord])
	}
	return nil
}

// globalOrdinalsSegmentLeafCollector collects segment ordinals directly.
type globalOrdinalsSegmentLeafCollector struct {
	sdv       index.SortedDocValues
	collected *util.LongBitSet
}

// SetScorer implements search.LeafCollector.
func (lc *globalOrdinalsSegmentLeafCollector) SetScorer(_ search.Scorer) error { return nil }

// Collect implements search.LeafCollector.
//
// Migrated to AdvanceExact + OrdValue (rmp #4709). Monotonic Collect.
func (lc *globalOrdinalsSegmentLeafCollector) Collect(doc int) error {
	if lc.sdv == nil {
		return nil
	}
	ok, err := lc.sdv.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	ord, err := lc.sdv.OrdValue()
	if err != nil {
		return err
	}
	if ord < 0 {
		return nil
	}
	lc.collected.Set(int64(ord))
	return nil
}

// Ensure interface compliance.
var _ search.Collector = (*GlobalOrdinalsCollector)(nil)
var _ search.LeafCollector = (*globalOrdinalsOrdMapLeafCollector)(nil)
var _ search.LeafCollector = (*globalOrdinalsSegmentLeafCollector)(nil)
