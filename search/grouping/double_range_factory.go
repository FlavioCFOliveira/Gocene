package grouping

import "math"

// DoubleRangeFactory groups double values into ranges
type DoubleRangeFactory struct {
	min   float64
	width float64
	max   float64
}

// NewDoubleRangeFactory creates a new DoubleRangeFactory
//
// min: a minimum value; all doubles below this value are grouped into a single range
// width: a standard width; all ranges between min and max are this wide,
// with the exception of the final range which may be up to this width. Ranges are inclusive
// at the lower end, and exclusive at the upper end.
// max: a maximum value; all doubles above this value are grouped into a single range
func NewDoubleRangeFactory(min, width, max float64) *DoubleRangeFactory {
	return &DoubleRangeFactory{
		min:   min,
		width: width,
		max:   max,
	}
}

// GetRange finds the DoubleRange that a value should be grouped into
//
// value: the value to group
// reuse: an existing DoubleRange object to reuse
func (f *DoubleRangeFactory) GetRange(value float64, reuse *DoubleRange) *DoubleRange {
	if reuse == nil {
		reuse = &DoubleRange{Min: math.SmallestNonzeroFloat64, Max: math.MaxFloat64}
	}
	if value < f.min {
		reuse.Max = f.min
		reuse.Min = math.SmallestNonzeroFloat64
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
