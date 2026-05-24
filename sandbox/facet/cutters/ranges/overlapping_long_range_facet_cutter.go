// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.cutters.ranges.OverlappingLongRangeFacetCutter.
package ranges

import (
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/rangefacets"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// OverlappingLongRangeFacetCutter is the LongRangeFacetCutter for ranges of
// long values that may overlap. It uses a segment-tree optimisation to find
// all matching ranges for a given value efficiently.
//
// Mirrors
// org.apache.lucene.sandbox.facet.cutters.ranges.OverlappingLongRangeFacetCutter.
type OverlappingLongRangeFacetCutter struct {
	*BaseLongRangeFacetCutter
	root *LongRangeNode
}

// NewOverlappingLongRangeFacetCutter creates a cutter for overlapping
// ranges.
func NewOverlappingLongRangeFacetCutter(
	valuesSource facets.MultiLongValuesSource,
	singleValues search.LongValues,
	longRanges []*rangefacets.LongRange,
) *OverlappingLongRangeFacetCutter {
	c := &OverlappingLongRangeFacetCutter{}
	c.BaseLongRangeFacetCutter = initBase(valuesSource, singleValues, longRanges, overlappingBuilder{})
	c.root = splitNode(0, len(c.elementaryIntervals), c.elementaryIntervals)
	for _, r := range c.sortedRanges {
		c.root.AddOutputs(r)
	}
	return c
}

// overlappingBuilder implements ElementaryIntervalBuilder for overlapping
// ranges.
type overlappingBuilder struct{}

// BuildElementaryIntervals produces elementary intervals by collecting all
// unique endpoints and computing a 1-D Venn diagram.
//
// Mirrors OverlappingLongRangeFacetCutter.buildElementaryIntervals.
func (overlappingBuilder) BuildElementaryIntervals(sortedRanges []LongRangeAndPos) []InclusiveRange {
	// Map all endpoints to flags: 1=start, 2=end, 3=both.
	endsMap := map[int64]int{math.MinInt64: 1, math.MaxInt64: 2}

	for _, r := range sortedRanges {
		endsMap[r.Range.Min] |= 1
		endsMap[r.Range.Max] |= 2
	}

	endsList := make([]int64, 0, len(endsMap))
	for k := range endsMap {
		endsList = append(endsList, k)
	}
	sort.Slice(endsList, func(i, j int) bool { return endsList[i] < endsList[j] })

	var intervals []InclusiveRange
	v := endsList[0]
	var prev int64
	if endsMap[v] == 3 {
		intervals = append(intervals, InclusiveRange{Start: v, End: v})
		prev = v + 1
	} else {
		prev = v
	}

	for upto := 1; upto < len(endsList); upto++ {
		v = endsList[upto]
		flags := endsMap[v]
		switch flags {
		case 3:
			if v > prev {
				intervals = append(intervals, InclusiveRange{Start: prev, End: v - 1})
			}
			intervals = append(intervals, InclusiveRange{Start: v, End: v})
			prev = v + 1
		case 1:
			if v > prev {
				intervals = append(intervals, InclusiveRange{Start: prev, End: v - 1})
			}
			prev = v
		default: // 2
			intervals = append(intervals, InclusiveRange{Start: prev, End: v})
			prev = v + 1
		}
	}

	return intervals
}

// splitNode builds the binary segment tree over [start, end).
//
// Mirrors OverlappingLongRangeFacetCutter.split.
func splitNode(start, end int, elementaryIntervals []InclusiveRange) *LongRangeNode {
	if start == end-1 {
		iv := elementaryIntervals[start]
		return NewLongRangeNode(iv.Start, iv.End, nil, nil)
	}
	mid := (start + end) >> 1
	left := splitNode(start, mid, elementaryIntervals)
	right := splitNode(mid, end, elementaryIntervals)
	return NewLongRangeNode(left.Start, right.End, left, right)
}

// CreateLeafCutter returns a per-segment leaf cutter.
func (c *OverlappingLongRangeFacetCutter) CreateLeafCutter(values facets.MultiLongValues, singleValues search.LongValues) OverlappingLeafCutter {
	if singleValues != nil {
		return &overlappingSingleValueLeafCutter{
			singleValues:           singleValues,
			boundaries:             c.boundaries,
			pos:                    c.pos,
			elementaryIntervalRoot: c.root,
			requestedTracker:       NewMultiIntervalTracker(c.requestedRangeCount),
		}
	}
	return &overlappingMultiValueLeafCutter{
		values:                 values,
		boundaries:             c.boundaries,
		pos:                    c.pos,
		elementaryIntervalRoot: c.root,
		elementaryTracker:      NewMultiIntervalTracker(len(c.boundaries)),
		requestedTracker:       NewMultiIntervalTracker(c.requestedRangeCount),
	}
}

// OverlappingLeafCutter is the per-segment leaf interface for overlapping
// ranges.
type OverlappingLeafCutter interface {
	AdvanceExact(doc int) (bool, error)
	NextOrd() int
}

// overlappingMultiValueLeafCutter handles multi-valued docs.
//
// Mirrors OverlappingLongRangeFacetCutter.OverlappingMultivaluedRangeLeafFacetCutter.
type overlappingMultiValueLeafCutter struct {
	values                 facets.MultiLongValues
	boundaries             []int64
	pos                    []int
	elementaryIntervalRoot *LongRangeNode
	elementaryTracker      *MultiIntervalTracker
	requestedTracker       *MultiIntervalTracker
	elementaryIntervalUpto int
}

func (c *overlappingMultiValueLeafCutter) maybeRollUp(_ IntervalTracker) {
	c.elementaryIntervalUpto = 0
	c.rollupMultiValued(c.elementaryIntervalRoot)
}

func (c *overlappingMultiValueLeafCutter) rollupMultiValued(node *LongRangeNode) bool {
	var containedHit bool
	if node.Left != nil {
		containedHit = c.rollupMultiValued(node.Left)
		containedHit = c.rollupMultiValued(node.Right) || containedHit
	} else {
		containedHit = c.elementaryTracker.Get(c.elementaryIntervalUpto)
		c.elementaryIntervalUpto++
	}
	if containedHit && len(node.Outputs) > 0 {
		for _, idx := range node.Outputs {
			c.requestedTracker.Set(int(idx))
		}
	}
	return containedHit
}

func (c *overlappingMultiValueLeafCutter) AdvanceExact(doc int) (bool, error) {
	return advanceExactMulti(
		doc, c.values, c.boundaries, c.elementaryTracker, c.requestedTracker,
		func(t IntervalTracker) { c.maybeRollUp(t) },
	)
}

func (c *overlappingMultiValueLeafCutter) NextOrd() int {
	return c.requestedTracker.NextOrd()
}

// overlappingSingleValueLeafCutter handles single-valued docs.
//
// Mirrors OverlappingLongRangeFacetCutter.OverlappingSingleValuedRangeLeafFacetCutter.
type overlappingSingleValueLeafCutter struct {
	singleValues           search.LongValues
	boundaries             []int64
	pos                    []int
	elementaryIntervalRoot *LongRangeNode
	requestedTracker       *MultiIntervalTracker
	elementaryIntervalOrd  int
	elementaryIntervalUpto int
}

func (c *overlappingSingleValueLeafCutter) maybeRollUp(_ IntervalTracker) {
	c.elementaryIntervalUpto = 0
	c.rollupSingleValued(c.elementaryIntervalRoot)
}

func (c *overlappingSingleValueLeafCutter) rollupSingleValued(node *LongRangeNode) bool {
	var containedHit bool
	if node.Left != nil {
		containedHit = c.rollupSingleValued(node.Left)
		containedHit = c.rollupSingleValued(node.Right) || containedHit
	} else {
		containedHit = c.elementaryIntervalUpto == c.elementaryIntervalOrd
		c.elementaryIntervalUpto++
	}
	if containedHit && len(node.Outputs) > 0 {
		for _, idx := range node.Outputs {
			c.requestedTracker.Set(int(idx))
		}
	}
	return containedHit
}

func (c *overlappingSingleValueLeafCutter) AdvanceExact(doc int) (bool, error) {
	ok, err := c.singleValues.AdvanceExact(doc)
	if err != nil || !ok {
		return ok, err
	}
	c.requestedTracker.Clear()
	v, err := c.singleValues.LongValue()
	if err != nil {
		return false, err
	}
	c.elementaryIntervalOrd = processValue(v, c.boundaries, 0)
	c.maybeRollUp(c.requestedTracker)
	c.requestedTracker.Freeze()
	return true, nil
}

func (c *overlappingSingleValueLeafCutter) NextOrd() int {
	return c.requestedTracker.NextOrd()
}
