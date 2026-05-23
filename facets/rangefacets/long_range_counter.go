// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package rangefacets

import "math"

// InclusiveRange holds the inclusive [start, end] of an elementary interval.
type InclusiveRange struct {
	start int64
	end   int64
}

// longRangeCounter is the abstract base for range counting. Concrete
// implementations are ExclusiveLongRangeCounter (non-overlapping) and
// OverlappingLongRangeCounter (overlapping). Mirrors
// org.apache.lucene.facet.range.LongRangeCounter.
type longRangeCounter struct {
	countBuffer []int

	// multiValuedDocLastSeenElementaryInterval tracks the last elementary
	// interval seen during multi-valued doc processing (values are sorted).
	multiValuedDocLastSeenElementaryInterval int

	// hooks called by the base binary-search routines.
	boundariesFn          func() []int64
	processSingleValuedFn func(elementaryIntervalNum int)
	processMultiValuedFn  func(elementaryIntervalNum int)
	endMultiValuedDocFn   func() bool
	finishFn              func() int
}

// newLongRangeCounter allocates the base struct.
func newLongRangeCounter(countBuffer []int) *longRangeCounter {
	return &longRangeCounter{countBuffer: countBuffer}
}

// rangeCount returns the number of user-requested ranges.
func (c *longRangeCounter) rangeCount() int { return len(c.countBuffer) }

// increment adds 1 to countBuffer[rangeNum].
func (c *longRangeCounter) increment(rangeNum int) { c.countBuffer[rangeNum]++ }

// incrementBy adds delta to countBuffer[rangeNum].
func (c *longRangeCounter) incrementBy(rangeNum, delta int) { c.countBuffer[rangeNum] += delta }

// startMultiValuedDoc resets the per-doc state.
func (c *longRangeCounter) startMultiValuedDoc() {
	c.multiValuedDocLastSeenElementaryInterval = -1
}

// addSingleValued finds the elementary interval for v via binary search and
// calls processSingleValuedFn. Mirrors LongRangeCounter.addSingleValued.
func (c *longRangeCounter) addSingleValued(v int64) {
	boundaries := c.boundariesFn()
	lo, hi := 0, len(boundaries)-1
	for {
		mid := (lo + hi) >> 1
		if v <= boundaries[mid] {
			if mid == 0 {
				c.processSingleValuedFn(mid)
				return
			}
			hi = mid - 1
		} else if v > boundaries[mid+1] {
			lo = mid + 1
		} else {
			c.processSingleValuedFn(mid + 1)
			return
		}
	}
}

// addMultiValued finds the elementary interval for v and calls
// processMultiValuedFn. Mirrors LongRangeCounter.addMultiValued.
func (c *longRangeCounter) addMultiValued(v int64) {
	if c.rangeCount() == 0 {
		return
	}
	boundaries := c.boundariesFn()
	last := c.multiValuedDocLastSeenElementaryInterval
	if last != -1 && v <= boundaries[last] {
		return
	}
	next := last + 1
	if next == len(boundaries) {
		return
	}
	lo, hi := next, len(boundaries)-1
	for {
		mid := (lo + hi) >> 1
		if v <= boundaries[mid] {
			if mid == next {
				c.processMultiValuedFn(mid)
				c.multiValuedDocLastSeenElementaryInterval = mid
				return
			}
			hi = mid - 1
		} else if v > boundaries[mid+1] {
			lo = mid + 1
		} else {
			idx := mid + 1
			c.processMultiValuedFn(idx)
			c.multiValuedDocLastSeenElementaryInterval = idx
			return
		}
	}
}

// hasOverlappingRanges checks whether any two ranges in the slice overlap.
// Mirrors LongRangeCounter.hasOverlappingRanges.
func hasOverlappingRanges(ranges []*LongRange) bool {
	if len(ranges) == 0 {
		return false
	}
	sorted := make([]*LongRange, len(ranges))
	copy(sorted, ranges)
	// sort by inclusive min
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].InclusiveMin() < sorted[j-1].InclusiveMin(); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	prevMax := sorted[0].InclusiveMax()
	for i := 1; i < len(sorted); i++ {
		if sorted[i].InclusiveMin() <= prevMax {
			return true
		}
		prevMax = sorted[i].InclusiveMax()
	}
	return false
}

// NewLongRangeCounter creates the appropriate counter for the supplied ranges:
// ExclusiveLongRangeCounter when ranges do not overlap, otherwise
// OverlappingLongRangeCounter. Mirrors LongRangeCounter.create.
func NewLongRangeCounter(ranges []*LongRange, countBuffer []int) LongRangeCounterI {
	if hasOverlappingRanges(ranges) {
		return newOverlappingLongRangeCounter(ranges, countBuffer)
	}
	return newExclusiveLongRangeCounter(ranges, countBuffer)
}

// LongRangeCounterI is the interface exposed to callers of the counter.
// Mirrors the public-ish API of the Java abstract class.
type LongRangeCounterI interface {
	// StartMultiValuedDoc begins processing a new multi-valued document.
	StartMultiValuedDoc()
	// EndMultiValuedDoc finishes the current multi-valued document and reports
	// whether it matched at least one range.
	EndMultiValuedDoc() bool
	// AddSingleValued counts a single document value.
	AddSingleValued(v int64)
	// AddMultiValued counts one value of a multi-valued document.
	AddMultiValued(v int64)
	// Finish completes processing and returns the number of missing docs (docs
	// that did not match any range and were not already reported by
	// EndMultiValuedDoc).
	Finish() int
}

// buildExclusiveElementaryIntervals creates the elementary intervals for a
// set of non-overlapping sorted ranges. Mirrors
// ExclusiveLongRangeCounter.buildElementaryIntervals.
func buildExclusiveElementaryIntervals(sortedRanges []longRangeAndPos) []InclusiveRange {
	var intervals []InclusiveRange
	prev := int64(math.MinInt64)
	for _, rp := range sortedRanges {
		lo := rp.r.InclusiveMin()
		hi := rp.r.InclusiveMax()
		if lo > prev {
			intervals = append(intervals, InclusiveRange{prev, lo - 1})
		}
		intervals = append(intervals, InclusiveRange{lo, hi})
		prev = hi + 1
	}
	if len(intervals) > 0 {
		last := intervals[len(intervals)-1].end
		if last < math.MaxInt64 {
			intervals = append(intervals, InclusiveRange{last + 1, math.MaxInt64})
		}
	} else {
		intervals = append(intervals, InclusiveRange{math.MinInt64, math.MaxInt64})
	}
	return intervals
}

type longRangeAndPos struct {
	r   *LongRange
	pos int
}
