// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import "math"

// DoubleRangeFactory groups double values into ranges.
//
// Mirrors org.apache.lucene.search.grouping.DoubleRangeFactory.
type DoubleRangeFactory struct {
	min   float64
	width float64
	max   float64
}

// NewDoubleRangeFactory creates a new DoubleRangeFactory.
//
// min is a minimum value; all doubles below this value are grouped into a
// single range. width is a standard width; all ranges between min and max are
// this wide, with the exception of the final range which may be up to this
// width. Ranges are inclusive at the lower end, and exclusive at the upper
// end. max is a maximum value; all doubles above this value are grouped into
// a single range.
//
// Mirrors DoubleRangeFactory(double min, double width, double max).
func NewDoubleRangeFactory(min, width, max float64) *DoubleRangeFactory {
	return &DoubleRangeFactory{min: min, width: width, max: max}
}

// GetRange finds the DoubleRange that a value should be grouped into, reusing
// the supplied DoubleRange when it is not nil.
//
// Mirrors DoubleRange getRange(double value, DoubleRange reuse).
func (f *DoubleRangeFactory) GetRange(value float64, reuse *DoubleRange) *DoubleRange {
	if reuse == nil {
		reuse = NewDoubleRange(javaDoubleMinValue, math.MaxFloat64)
	}
	if value < f.min {
		reuse.Max = f.min
		reuse.Min = javaDoubleMinValue
		return reuse
	}
	if value >= f.max {
		reuse.Min = f.max
		reuse.Max = math.MaxFloat64
		return reuse
	}
	bucket := math.Floor((value - f.min) / f.width)
	reuse.Min = f.min + (bucket * f.width)
	reuse.Max = reuse.Min + f.width
	return reuse
}

// javaDoubleMinValue mirrors java.lang.Double.MIN_VALUE, the smallest
// positive nonzero double, which is what DoubleRangeFactory uses as the open
// lower bound.
const javaDoubleMinValue = math.SmallestNonzeroFloat64
