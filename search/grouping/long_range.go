package grouping

import "fmt"

// LongRange represents a contiguous range of long values, with an inclusive minimum and exclusive maximum
type LongRange struct {
	// Min is the inclusive minimum value of this range
	Min int64
	// Max is the exclusive maximum value of this range
	Max int64
}

// NewLongRange creates a new long range, running from min inclusive to max exclusive
func NewLongRange(min, max int64) *LongRange {
	return &LongRange{
		Min: min,
		Max: max,
	}
}

func (lr *LongRange) String() string {
	return fmt.Sprintf("LongRange(%d, %d)", lr.Min, lr.Max)
}
