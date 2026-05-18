package grouping

// LongRangeFactory is the int64 counterpart of DoubleRangeFactory. Mirrors
// org.apache.lucene.search.grouping.LongRangeFactory.
type LongRangeFactory struct {
	ranges []*LongRange
}

// NewLongRangeFactory builds a factory.
func NewLongRangeFactory(ranges ...*LongRange) *LongRangeFactory {
	clone := make([]*LongRange, len(ranges))
	copy(clone, ranges)
	return &LongRangeFactory{ranges: clone}
}

// Ranges returns a copy of the configured ranges.
func (f *LongRangeFactory) Ranges() []*LongRange {
	out := make([]*LongRange, len(f.ranges))
	copy(out, f.ranges)
	return out
}

// GetRange returns the first range that accepts v, or nil when none.
func (f *LongRangeFactory) GetRange(v int64) *LongRange {
	for _, r := range f.ranges {
		if r.Accepts(v) {
			return r
		}
	}
	return nil
}
