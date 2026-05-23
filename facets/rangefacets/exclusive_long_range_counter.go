// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package rangefacets

import (
	"math"
	"sort"
)

// ExclusiveLongRangeCounter is the fast-path counter used when the requested
// ranges do not overlap. It binary-searches directly into the requested ranges
// as values arrive, avoiding any rollup step. Mirrors
// org.apache.lucene.facet.range.ExclusiveLongRangeCounter.
type ExclusiveLongRangeCounter struct {
	base *longRangeCounter

	// boundaries holds the inclusive end of each elementary interval.
	boundaries []int64

	// rangeNums maps elementary interval index → requested range index,
	// or -1 for "gap" intervals.
	rangeNums []int

	missingCount int

	multiValuedDocMatchedRange bool
}

func newExclusiveLongRangeCounter(ranges []*LongRange, countBuffer []int) *ExclusiveLongRangeCounter {
	c := &ExclusiveLongRangeCounter{
		base: newLongRangeCounter(countBuffer),
	}

	// Build sorted (by incl. min) copy with original positions.
	sorted := make([]longRangeAndPos, len(ranges))
	for i, r := range ranges {
		sorted[i] = longRangeAndPos{r, i}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].r.InclusiveMin() < sorted[j].r.InclusiveMin()
	})

	intervals := buildExclusiveElementaryIntervals(sorted)

	c.boundaries = make([]int64, len(intervals))
	c.rangeNums = make([]int, len(intervals))
	for i := range c.rangeNums {
		c.rangeNums[i] = -1
	}

	currRange := 0
	for i, iv := range intervals {
		c.boundaries[i] = iv.end
		if currRange < len(sorted) {
			if iv.end == sorted[currRange].r.InclusiveMax() {
				c.rangeNums[i] = sorted[currRange].pos
				currRange++
			}
		}
	}

	// Wire the abstract methods.
	c.base.boundariesFn = func() []int64 { return c.boundaries }
	c.base.processSingleValuedFn = c.processSingleValuedHit
	c.base.processMultiValuedFn = c.processMultiValuedHit
	return c
}

func (c *ExclusiveLongRangeCounter) processSingleValuedHit(idx int) {
	rn := c.rangeNums[idx]
	if rn != -1 {
		c.base.increment(rn)
	} else {
		c.missingCount++
	}
}

func (c *ExclusiveLongRangeCounter) processMultiValuedHit(idx int) {
	rn := c.rangeNums[idx]
	if rn != -1 {
		c.base.increment(rn)
		c.multiValuedDocMatchedRange = true
	}
}

// StartMultiValuedDoc implements LongRangeCounterI.
func (c *ExclusiveLongRangeCounter) StartMultiValuedDoc() {
	c.base.startMultiValuedDoc()
	c.multiValuedDocMatchedRange = false
}

// EndMultiValuedDoc implements LongRangeCounterI.
func (c *ExclusiveLongRangeCounter) EndMultiValuedDoc() bool {
	return c.multiValuedDocMatchedRange
}

// AddSingleValued implements LongRangeCounterI.
func (c *ExclusiveLongRangeCounter) AddSingleValued(v int64) {
	if c.base.rangeCount() == 0 {
		c.missingCount++
		return
	}
	c.base.addSingleValued(v)
}

// AddMultiValued implements LongRangeCounterI.
func (c *ExclusiveLongRangeCounter) AddMultiValued(v int64) { c.base.addMultiValued(v) }

// Finish implements LongRangeCounterI.
func (c *ExclusiveLongRangeCounter) Finish() int { return c.missingCount }

// ensure math import is used
var _ = math.MaxInt64

var _ LongRangeCounterI = (*ExclusiveLongRangeCounter)(nil)
