package rangeonrange

// LongRange is the int64 counterpart of DoubleRange. Mirrors
// org.apache.lucene.facet.rangeonrange.LongRange.
type LongRange struct {
	Label string
	Min   int64
	Max   int64
}

// NewLongRange builds an inclusive int64 interval.
func NewLongRange(label string, min, max int64) *LongRange {
	if min > max {
		min, max = max, min
	}
	return &LongRange{Label: label, Min: min, Max: max}
}

// Contains reports whether [docMin, docMax] is fully contained.
func (r *LongRange) Contains(docMin, docMax int64) bool {
	return docMin >= r.Min && docMax <= r.Max
}

// Within reports whether this range is fully contained in [docMin, docMax].
func (r *LongRange) Within(docMin, docMax int64) bool {
	return docMin <= r.Min && docMax >= r.Max
}

// Overlaps reports whether the two intervals overlap (inclusive bounds).
func (r *LongRange) Overlaps(docMin, docMax int64) bool {
	return docMin <= r.Max && docMax >= r.Min
}
