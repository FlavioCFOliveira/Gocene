// Package rangefacets implements the org.apache.lucene.facet.range package:
// numeric range definitions and the matching DoubleRangeFacetCounts /
// LongRangeFacetCounts aggregators.
package rangefacets

import "fmt"

// DoubleRange describes a single numeric range over float64 values with
// configurable inclusivity on both ends. Mirrors
// org.apache.lucene.facet.range.DoubleRange.
type DoubleRange struct {
	Label        string
	Min          float64
	Max          float64
	MinInclusive bool
	MaxInclusive bool
}

// NewDoubleRange builds a DoubleRange.
func NewDoubleRange(label string, min float64, minInclusive bool, max float64, maxInclusive bool) *DoubleRange {
	return &DoubleRange{
		Label:        label,
		Min:          min,
		Max:          max,
		MinInclusive: minInclusive,
		MaxInclusive: maxInclusive,
	}
}

// Accept reports whether v lies inside the range with the configured
// inclusivity.
func (r *DoubleRange) Accept(v float64) bool {
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
func (r *DoubleRange) String() string {
	lo, hi := '(', ')'
	if r.MinInclusive {
		lo = '['
	}
	if r.MaxInclusive {
		hi = ']'
	}
	return fmt.Sprintf("%s%c%v,%v%c", r.Label, lo, r.Min, r.Max, hi)
}
