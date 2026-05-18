package grouping

// DoubleRange is a single inclusive-min, exclusive-max float64 bucket used
// by the grouping range selectors. Mirrors
// org.apache.lucene.search.grouping.DoubleRange.
type DoubleRange struct {
	Label string
	Min   float64
	Max   float64
}

// NewDoubleRange builds a [Min, Max) range, swapping reversed bounds.
func NewDoubleRange(label string, min, max float64) *DoubleRange {
	if min > max {
		min, max = max, min
	}
	return &DoubleRange{Label: label, Min: min, Max: max}
}

// Accepts reports whether v lies in [Min, Max).
func (r *DoubleRange) Accepts(v float64) bool { return v >= r.Min && v < r.Max }
