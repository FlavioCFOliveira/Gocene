package grouping

import "fmt"

// DoubleRange represents a contiguous range of double values, with an inclusive minimum and exclusive maximum.
type DoubleRange struct {
	Min float64
	Max float64
}

// NewDoubleRange creates a new double range, running from min inclusive to max exclusive.
func NewDoubleRange(min, max float64) DoubleRange {
	return DoubleRange{
		Min: min,
		Max: max,
	}
}

func (dr DoubleRange) String() string {
	return fmt.Sprintf("DoubleRange(%f, %f)", dr.Min, dr.Max)
}
