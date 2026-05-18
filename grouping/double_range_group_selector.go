package grouping

// DoubleRangeGroupSelector groups documents by the DoubleRange their value
// falls into. Mirrors org.apache.lucene.search.grouping.DoubleRangeGroupSelector.
type DoubleRangeGroupSelector struct {
	factory *DoubleRangeFactory
	value   float64
	group   *DoubleRange
}

// NewDoubleRangeGroupSelector builds a selector for the supplied factory.
func NewDoubleRangeGroupSelector(factory *DoubleRangeFactory) *DoubleRangeGroupSelector {
	return &DoubleRangeGroupSelector{factory: factory}
}

// AdvanceTo positions the selector on a new value; the group is resolved
// lazily via GetCurrentGroup.
func (s *DoubleRangeGroupSelector) AdvanceTo(v float64) {
	s.value = v
	s.group = s.factory.GetRange(v)
}

// GetCurrentGroup returns the DoubleRange the most recent value belongs to.
func (s *DoubleRangeGroupSelector) GetCurrentGroup() *DoubleRange { return s.group }

// CurrentValue returns the most recently observed value.
func (s *DoubleRangeGroupSelector) CurrentValue() float64 { return s.value }
