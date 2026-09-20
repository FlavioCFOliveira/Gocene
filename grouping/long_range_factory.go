// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import "math"

// LongRangeFactory groups double values into ranges.
//
// Mirrors org.apache.lucene.search.grouping.LongRangeFactory.
type LongRangeFactory struct {
	min   int64
	width int64
	max   int64
}

// NewLongRangeFactory creates a new LongRangeFactory.
//
// min is a minimum value; all longs below this value are grouped into a
// single range. width is a standard width; all ranges between min and max are
// this wide, with the exception of the final range which may be up to this
// width. Ranges are inclusive at the lower end, and exclusive at the upper
// end. max is a maximum value; all longs above this value are grouped into a
// single range.
//
// Mirrors LongRangeFactory(long min, long width, long max).
func NewLongRangeFactory(min, width, max int64) *LongRangeFactory {
	return &LongRangeFactory{min: min, width: width, max: max}
}

// GetRange finds the LongRange that a value should be grouped into, reusing
// the supplied LongRange when it is not nil.
//
// Mirrors LongRange getRange(long value, LongRange reuse).
func (f *LongRangeFactory) GetRange(value int64, reuse *LongRange) *LongRange {
	if reuse == nil {
		reuse = NewLongRange(math.MinInt64, math.MaxInt64)
	}
	if value < f.min {
		reuse.Max = f.min
		reuse.Min = math.MinInt64
		return reuse
	}
	if value >= f.max {
		reuse.Min = f.max
		reuse.Max = math.MaxInt64
		return reuse
	}
	bucket := (value - f.min) / f.width
	reuse.Min = f.min + (bucket * f.width)
	reuse.Max = reuse.Min + f.width
	return reuse
}
