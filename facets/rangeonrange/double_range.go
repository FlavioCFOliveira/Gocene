// Package rangeonrange implements org.apache.lucene.facet.rangeonrange: a
// counterpart of facet/range that operates on per-document [min, max]
// intervals rather than scalar values, with configurable interval overlap
// semantics (within / contains / overlaps / before / after).
package rangeonrange

// DoubleRange is the inclusive [Min, Max] interval used by
// DoubleRangeOnRangeFacetCounts. Mirrors
// org.apache.lucene.facet.rangeonrange.DoubleRange.
type DoubleRange struct {
	Label string
	Min   float64
	Max   float64
}

// NewDoubleRange builds an inclusive interval.
func NewDoubleRange(label string, min, max float64) *DoubleRange {
	if min > max {
		min, max = max, min
	}
	return &DoubleRange{Label: label, Min: min, Max: max}
}

// Contains reports whether [docMin, docMax] is fully contained in this range.
func (r *DoubleRange) Contains(docMin, docMax float64) bool {
	return docMin >= r.Min && docMax <= r.Max
}

// Within reports whether this range is fully contained in [docMin, docMax].
func (r *DoubleRange) Within(docMin, docMax float64) bool {
	return docMin <= r.Min && docMax >= r.Max
}

// Overlaps reports whether the two intervals overlap (inclusive bounds).
func (r *DoubleRange) Overlaps(docMin, docMax float64) bool {
	return docMin <= r.Max && docMax >= r.Min
}
