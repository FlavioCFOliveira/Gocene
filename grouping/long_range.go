package grouping

// LongRange is the int64 counterpart of grouping.DoubleRange. Mirrors
// org.apache.lucene.search.grouping.LongRange.
type LongRange struct {
	Label string
	Min   int64
	Max   int64
}

// NewLongRange builds a [Min, Max) int64 range.
func NewLongRange(label string, min, max int64) *LongRange {
	if min > max {
		min, max = max, min
	}
	return &LongRange{Label: label, Min: min, Max: max}
}

// Accepts reports whether v lies in [Min, Max).
func (r *LongRange) Accepts(v int64) bool { return v >= r.Min && v < r.Max }
