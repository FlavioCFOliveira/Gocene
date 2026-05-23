package grouping

// DoubleRangeGroupSelector groups documents by the DoubleRange their value
// falls into. Mirrors org.apache.lucene.search.grouping.DoubleRangeGroupSelector.
//
// The Java original uses DoubleValuesSource + DoubleValues to retrieve per-document
// float64 values. In Gocene's current stage those search-layer abstractions are not
// yet ported, so callers supply a valuesFunc that maps doc IDs to (value, exists)
// pairs — semantically equivalent to DoubleValues.advanceExact().
type DoubleRangeGroupSelector struct {
	factory    *DoubleRangeFactory
	valuesFunc func(doc int) (float64, bool) // nil → no value for any doc

	// second-pass state (populated by SetGroups)
	inSecondPass map[doubleRangeKey]bool
	includeEmpty bool

	// per-document state
	positioned bool
	current    *DoubleRange
}

// doubleRangeKey is a comparable key for DoubleRange used in the second-pass set.
type doubleRangeKey struct{ min, max float64 }

// NewDoubleRangeGroupSelector builds a selector backed by factory and the
// supplied doc-value function.  Pass nil for valuesFunc when no documents have
// values (every doc maps to the empty group).
func NewDoubleRangeGroupSelector(factory *DoubleRangeFactory, valuesFunc func(doc int) (float64, bool)) *DoubleRangeGroupSelector {
	return &DoubleRangeGroupSelector{
		factory:    factory,
		valuesFunc: valuesFunc,
	}
}

// SetGroups configures the selector for the second pass.  Only groups whose
// *DoubleRange key appears in searchGroups will be accepted; if any group has
// a nil key the empty group (documents without a value) is also included.
// Mirrors DoubleRangeGroupSelector.setGroups.
func (s *DoubleRangeGroupSelector) SetGroups(searchGroups []*SearchGroup[*DoubleRange]) {
	s.inSecondPass = make(map[doubleRangeKey]bool, len(searchGroups))
	s.includeEmpty = false
	for _, g := range searchGroups {
		if g.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.inSecondPass[doubleRangeKey{g.GroupValue.Min, g.GroupValue.Max}] = true
		}
	}
}

// AdvanceTo positions the selector on document doc.  It returns whether the
// document should be included in the current pass.
// Mirrors DoubleRangeGroupSelector.advanceTo.
func (s *DoubleRangeGroupSelector) AdvanceTo(doc int) bool {
	if s.valuesFunc == nil {
		s.positioned = false
		s.current = nil
		return s.includeEmpty || s.inSecondPass == nil
	}
	v, ok := s.valuesFunc(doc)
	s.positioned = ok
	if !ok {
		s.current = nil
		return s.includeEmpty || s.inSecondPass == nil
	}
	s.current = s.factory.GetRange(v)
	if s.inSecondPass == nil {
		return true
	}
	if s.current == nil {
		return s.includeEmpty
	}
	return s.inSecondPass[doubleRangeKey{s.current.Min, s.current.Max}]
}

// Select implements GroupSelector.  It returns the *DoubleRange for the given
// doc, or nil when the doc has no value.
func (s *DoubleRangeGroupSelector) Select(doc int) interface{} {
	s.AdvanceTo(doc)
	r := s.CurrentValue()
	if r == nil {
		return nil
	}
	return r
}

// CurrentValue returns the DoubleRange resolved during the last AdvanceTo call,
// or nil when the document had no value.
// Mirrors DoubleRangeGroupSelector.currentValue.
func (s *DoubleRangeGroupSelector) CurrentValue() *DoubleRange {
	if !s.positioned {
		return nil
	}
	return s.current
}

// CopyValue returns a copy of the current DoubleRange (to be stored as a group
// key independent of any reuse buffer).
// Mirrors DoubleRangeGroupSelector.copyValue.
func (s *DoubleRangeGroupSelector) CopyValue() *DoubleRange {
	if !s.positioned || s.current == nil {
		return nil
	}
	cp := *s.current
	return &cp
}

// Ensure DoubleRangeGroupSelector implements GroupSelector.
var _ GroupSelector = (*DoubleRangeGroupSelector)(nil)
