// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.cutters.ranges.NonOverlappingLongRangeFacetCutter.
package ranges

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/rangefacets"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// NonOverlappingLongRangeFacetCutter is the LongRangeFacetCutter for ranges
// of long values that do not overlap.
//
// Mirrors
// org.apache.lucene.sandbox.facet.cutters.ranges.NonOverlappingLongRangeFacetCutter.
type NonOverlappingLongRangeFacetCutter struct {
	*BaseLongRangeFacetCutter
}

// NewNonOverlappingLongRangeFacetCutter creates a cutter for non-overlapping
// ranges.
func NewNonOverlappingLongRangeFacetCutter(
	valuesSource facets.MultiLongValuesSource,
	singleValues search.LongValues,
	longRanges []*rangefacets.LongRange,
) *NonOverlappingLongRangeFacetCutter {
	return &NonOverlappingLongRangeFacetCutter{
		BaseLongRangeFacetCutter: initBase(valuesSource, singleValues, longRanges, nonOverlappingBuilder{}),
	}
}

// nonOverlappingBuilder implements ElementaryIntervalBuilder for exclusive
// ranges.
type nonOverlappingBuilder struct{}

// BuildElementaryIntervals produces the elementary interval list for non-
// overlapping ranges by inserting "gap" intervals between requested ranges.
//
// Mirrors NonOverlappingLongRangeFacetCutter.buildElementaryIntervals.
func (nonOverlappingBuilder) BuildElementaryIntervals(sortedRanges []LongRangeAndPos) []InclusiveRange {
	var intervals []InclusiveRange
	prev := int64(math.MinInt64)
	for _, r := range sortedRanges {
		if r.Range.Min > prev {
			intervals = append(intervals, InclusiveRange{Start: prev, End: r.Range.Min - 1})
		}
		intervals = append(intervals, InclusiveRange{Start: r.Range.Min, End: r.Range.Max})
		prev = r.Range.Max + 1
	}
	if len(intervals) > 0 {
		last := intervals[len(intervals)-1]
		if last.End < math.MaxInt64 {
			intervals = append(intervals, InclusiveRange{Start: last.End + 1, End: math.MaxInt64})
		}
	} else {
		intervals = append(intervals, InclusiveRange{Start: math.MinInt64, End: math.MaxInt64})
	}
	return intervals
}

// CreateLeafCutter returns a per-segment leaf cutter.
func (c *NonOverlappingLongRangeFacetCutter) CreateLeafCutter(values facets.MultiLongValues, singleValues search.LongValues) NonOverlappingLeafCutter {
	if singleValues != nil {
		return &nonOverlappingSingleValueLeafCutter{
			singleValues: singleValues,
			boundaries:   c.boundaries,
			pos:          c.pos,
		}
	}
	return &nonOverlappingMultiValueLeafCutter{
		values:     values,
		boundaries: c.boundaries,
		pos:        c.pos,
		tracker:    NewMultiIntervalTracker(len(c.boundaries)),
	}
}

// NonOverlappingLeafCutter is the per-segment leaf interface for non-
// overlapping ranges.
type NonOverlappingLeafCutter interface {
	AdvanceExact(doc int) (bool, error)
	NextOrd() int
}

// nonOverlappingMultiValueLeafCutter handles multi-valued docs.
//
// Mirrors NonOverlappingLongRangeFacetCutter.NonOverlappingLongRangeMultiValueLeafFacetCutter.
type nonOverlappingMultiValueLeafCutter struct {
	values     facets.MultiLongValues
	boundaries []int64
	pos        []int
	tracker    *MultiIntervalTracker
}

func (c *nonOverlappingMultiValueLeafCutter) AdvanceExact(doc int) (bool, error) {
	return advanceExactMulti(doc, c.values, c.boundaries, c.tracker, nil, nil)
}

func (c *nonOverlappingMultiValueLeafCutter) NextOrd() int {
	for {
		ord := c.tracker.NextOrd()
		if ord == NoMoreOrds {
			return NoMoreOrds
		}
		if result := c.pos[ord]; result != skipIntervalPosition {
			return result
		}
	}
}

// nonOverlappingSingleValueLeafCutter handles single-valued docs.
//
// Mirrors NonOverlappingLongRangeFacetCutter.NonOverlappingLongRangeSingleValueLeafFacetCutter.
type nonOverlappingSingleValueLeafCutter struct {
	singleValues       search.LongValues
	boundaries         []int64
	pos                []int
	elementaryInterval int
}

func (c *nonOverlappingSingleValueLeafCutter) AdvanceExact(doc int) (bool, error) {
	ok, err := c.singleValues.AdvanceExact(doc)
	if err != nil || !ok {
		c.elementaryInterval = NoMoreOrds
		return ok, err
	}
	v, err := c.singleValues.LongValue()
	if err != nil {
		c.elementaryInterval = NoMoreOrds
		return false, err
	}
	c.elementaryInterval = processValue(v, c.boundaries, 0)
	return true, nil
}

func (c *nonOverlappingSingleValueLeafCutter) NextOrd() int {
	if c.elementaryInterval == NoMoreOrds {
		return NoMoreOrds
	}
	result := c.pos[c.elementaryInterval]
	c.elementaryInterval = NoMoreOrds
	if result != skipIntervalPosition {
		return result
	}
	return NoMoreOrds
}
