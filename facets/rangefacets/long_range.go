package rangefacets

import "fmt"

// LongRange is the int64 counterpart of DoubleRange. Mirrors
// org.apache.lucene.facet.range.LongRange.
type LongRange struct {
	Label        string
	Min          int64
	Max          int64
	MinInclusive bool
	MaxInclusive bool
}

// NewLongRange builds a LongRange.
func NewLongRange(label string, min int64, minInclusive bool, max int64, maxInclusive bool) *LongRange {
	return &LongRange{
		Label:        label,
		Min:          min,
		Max:          max,
		MinInclusive: minInclusive,
		MaxInclusive: maxInclusive,
	}
}

// Accept reports whether v lies inside the range.
func (r *LongRange) Accept(v int64) bool {
	if r.MinInclusive {
		if v < r.Min {
			return false
		}
	} else if v <= r.Min {
		return false
	}
	if r.MaxInclusive {
		if v > r.Max {
			return false
		}
	} else if v >= r.Max {
		return false
	}
	return true
}

// String returns a debug rendering of the range.
func (r *LongRange) String() string {
	lo, hi := '(', ')'
	if r.MinInclusive {
		lo = '['
	}
	if r.MaxInclusive {
		hi = ']'
	}
	return fmt.Sprintf("%s%c%d,%d%c", r.Label, lo, r.Min, r.Max, hi)
}
