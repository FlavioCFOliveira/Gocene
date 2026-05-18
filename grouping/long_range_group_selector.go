package grouping

// LongRangeGroupSelector is the int64 counterpart of
// DoubleRangeGroupSelector. Mirrors
// org.apache.lucene.search.grouping.LongRangeGroupSelector.
type LongRangeGroupSelector struct {
	factory *LongRangeFactory
	value   int64
	group   *LongRange
}

// NewLongRangeGroupSelector builds a selector.
func NewLongRangeGroupSelector(factory *LongRangeFactory) *LongRangeGroupSelector {
	return &LongRangeGroupSelector{factory: factory}
}

// AdvanceTo positions the selector on a new value.
func (s *LongRangeGroupSelector) AdvanceTo(v int64) {
	s.value = v
	s.group = s.factory.GetRange(v)
}

// GetCurrentGroup returns the matching LongRange.
func (s *LongRangeGroupSelector) GetCurrentGroup() *LongRange { return s.group }

// CurrentValue returns the most recently observed value.
func (s *LongRangeGroupSelector) CurrentValue() int64 { return s.value }
