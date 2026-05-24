// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.cutters.ranges.LongRangeFacetCutter.
package ranges

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/rangefacets"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// skipIntervalPosition is the sentinel stored in pos[] when an elementary
// interval is a "gap" and should be skipped during ordinal emission.
//
// Mirrors LongRangeFacetCutter.SKIP_INTERVAL_POSITION.
const skipIntervalPosition = -1

// LongRangeAndPos pairs a LongRange with its 0-based position in the
// original caller-supplied range slice.
//
// Mirrors LongRangeFacetCutter.LongRangeAndPos.
type LongRangeAndPos struct {
	Range *rangefacets.LongRange
	Pos   int
}

// InclusiveRange is a closed interval [Start, End].
//
// Mirrors LongRangeFacetCutter.InclusiveRange.
type InclusiveRange struct {
	Start int64
	End   int64
}

// ElementaryIntervalBuilder is the strategy interface that distinguishes
// overlapping from non-overlapping cutters.
type ElementaryIntervalBuilder interface {
	BuildElementaryIntervals(sortedRanges []LongRangeAndPos) []InclusiveRange
}

// BaseLongRangeFacetCutter holds the shared state for both
// OverlappingLongRangeFacetCutter and NonOverlappingLongRangeFacetCutter.
//
// Mirrors the package-private abstract LongRangeFacetCutter base class.
type BaseLongRangeFacetCutter struct {
	valuesSource        facets.MultiLongValuesSource
	singleValues        search.LongValues
	sortedRanges        []LongRangeAndPos
	requestedRangeCount int
	elementaryIntervals []InclusiveRange
	// boundaries holds the end values of each elementary interval (for
	// binary search).
	boundaries []int64
	// pos maps elementary interval index → requested range position, or
	// skipIntervalPosition when the interval is a gap.
	pos []int
}

// initBase initialises the shared fields. It must be called from each
// concrete constructor after the ElementaryIntervalBuilder has produced the
// elementary intervals.
func initBase(
	valuesSource facets.MultiLongValuesSource,
	singleValues search.LongValues,
	longRanges []*rangefacets.LongRange,
	builder ElementaryIntervalBuilder,
) *BaseLongRangeFacetCutter {
	b := &BaseLongRangeFacetCutter{
		valuesSource:        valuesSource,
		singleValues:        singleValues,
		requestedRangeCount: len(longRanges),
	}

	// Build sortedRanges.
	b.sortedRanges = make([]LongRangeAndPos, len(longRanges))
	for i, r := range longRanges {
		b.sortedRanges[i] = LongRangeAndPos{Range: r, Pos: i}
	}
	sort.Slice(b.sortedRanges, func(i, j int) bool {
		return b.sortedRanges[i].Range.Min < b.sortedRanges[j].Range.Min
	})

	b.elementaryIntervals = builder.BuildElementaryIntervals(b.sortedRanges)

	// Build boundaries and pos arrays.
	b.boundaries = make([]int64, len(b.elementaryIntervals))
	b.pos = make([]int, len(b.elementaryIntervals))
	for i := range b.pos {
		b.pos[i] = skipIntervalPosition
	}

	currRange := 0
	for i, iv := range b.elementaryIntervals {
		b.boundaries[i] = iv.End
		if currRange < len(b.sortedRanges) {
			curr := b.sortedRanges[currRange]
			if b.boundaries[i] == curr.Range.Max {
				b.pos[i] = curr.Pos
				currRange++
			}
		}
	}

	return b
}

// areOverlappingRanges reports whether any two ranges in the slice overlap.
// Ranges are considered overlapping when the next range's min is ≤ the
// previous max (closed intervals).
//
// Mirrors LongRangeFacetCutter.areOverlappingRanges.
func areOverlappingRanges(ranges []*rangefacets.LongRange) bool {
	if len(ranges) == 0 {
		return false
	}
	sorted := make([]*rangefacets.LongRange, len(ranges))
	copy(sorted, ranges)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Min < sorted[j].Min })

	prevMax := sorted[0].Max
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Min <= prevMax {
			return true
		}
		prevMax = sorted[i].Max
	}
	return false
}

// processValue returns the elementary interval index for value v, starting
// the binary search from lowerBound.
//
// Mirrors LongRangeFacetCutter's private processValue.
func processValue(v int64, boundaries []int64, lowerBound int) int {
	lo, hi := lowerBound, len(boundaries)-1
	for {
		mid := (lo + hi) >> 1
		if v <= boundaries[mid] {
			if mid == lowerBound {
				return mid
			}
			hi = mid - 1
		} else if v > boundaries[mid+1] {
			lo = mid + 1
		} else {
			return mid + 1
		}
	}
}

// processValueMulti is the entry point for multi-valued processing.
// It returns the elementary interval index for v, honouring lastIntervalSeen
// to avoid regressing in the multi-valued case.
//
// Mirrors LongRangeFacetCutter.LongRangeMultivaluedLeafFacetCutter.processValue.
func processValueMulti(v int64, boundaries []int64, lastIntervalSeen int) int {
	lo := 0
	if lastIntervalSeen != -1 {
		if v <= boundaries[lastIntervalSeen] {
			return lastIntervalSeen
		}
		lo = lastIntervalSeen + 1
		if lo == len(boundaries) {
			return lastIntervalSeen
		}
	}
	return processValue(v, boundaries, lo)
}

// AdvanceExactMulti processes a multi-valued document by scanning all values
// and tracking which elementary intervals were hit.
//
// Shared by both leaf cutter implementations.
func advanceExactMulti(
	doc int,
	values facets.MultiLongValues,
	boundaries []int64,
	elementaryTracker *MultiIntervalTracker,
	requestedTracker IntervalTracker,
	maybeRollUp func(IntervalTracker),
) (bool, error) {
	ok, err := values.AdvanceExact(doc)
	if err != nil || !ok {
		return ok, err
	}

	elementaryTracker.Clear()
	if requestedTracker != nil {
		requestedTracker.Clear()
	}

	numValues := values.DocValueCount()
	lastIntervalSeen := -1

	for i := 0; i < numValues; i++ {
		v, err := values.NextValue()
		if err != nil {
			return false, err
		}
		lastIntervalSeen = processValueMulti(v, boundaries, lastIntervalSeen)
		elementaryTracker.Set(lastIntervalSeen)
		if lastIntervalSeen == len(boundaries)-1 {
			break
		}
	}

	if maybeRollUp != nil {
		maybeRollUp(requestedTracker)
	}

	elementaryTracker.Freeze()
	if requestedTracker != nil {
		requestedTracker.Freeze()
	}

	return true, nil
}
