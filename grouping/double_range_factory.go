package grouping

// DoubleRangeFactory turns observed float64 values into the DoubleRange they
// belong to. Mirrors org.apache.lucene.search.grouping.DoubleRangeFactory.
type DoubleRangeFactory struct {
	ranges []*DoubleRange
}

// NewDoubleRangeFactory builds a factory over the supplied ranges. The order
// matters: a value picks the first matching range.
func NewDoubleRangeFactory(ranges ...*DoubleRange) *DoubleRangeFactory {
	clone := make([]*DoubleRange, len(ranges))
	copy(clone, ranges)
	return &DoubleRangeFactory{ranges: clone}
}

// Ranges returns the configured ranges.
func (f *DoubleRangeFactory) Ranges() []*DoubleRange {
	out := make([]*DoubleRange, len(f.ranges))
	copy(out, f.ranges)
	return out
}

// GetRange returns the first range that accepts v, or nil when none.
func (f *DoubleRangeFactory) GetRange(v float64) *DoubleRange {
	for _, r := range f.ranges {
		if r.Accepts(v) {
			return r
		}
	}
	return nil
}
