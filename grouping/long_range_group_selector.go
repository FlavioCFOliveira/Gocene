package grouping

// LongRangeGroupSelector is the int64 counterpart of DoubleRangeGroupSelector.
// Mirrors org.apache.lucene.search.grouping.LongRangeGroupSelector.
//
// The Java original uses LongValuesSource + LongValues to retrieve per-document
// int64 values. In Gocene's current stage those abstractions are not yet ported,
// so callers supply a valuesFunc that maps doc IDs to (value, exists) pairs —
// semantically equivalent to LongValues.advanceExact().
type LongRangeGroupSelector struct {
	factory    *LongRangeFactory
	valuesFunc func(doc int) (int64, bool) // nil → no value for any doc

	// second-pass state (populated by SetGroups)
	inSecondPass map[longRangeKey]bool
	includeEmpty bool

	// per-document state
	positioned bool
	current    *LongRange
}

// longRangeKey is a comparable key for LongRange used in the second-pass set.
type longRangeKey struct{ min, max int64 }

// NewLongRangeGroupSelector builds a selector backed by factory and the
// supplied doc-value function.  Pass nil for valuesFunc when no documents have
// values (every doc maps to the empty group).
func NewLongRangeGroupSelector(factory *LongRangeFactory, valuesFunc func(doc int) (int64, bool)) *LongRangeGroupSelector {
	return &LongRangeGroupSelector{
		factory:    factory,
		valuesFunc: valuesFunc,
	}
}

// SetGroups configures the selector for the second pass.  Only groups whose
// *LongRange key appears in searchGroups will be accepted; if any group has
// a nil key the empty group (documents without a value) is also included.
// Mirrors LongRangeGroupSelector.setGroups.
func (s *LongRangeGroupSelector) SetGroups(searchGroups []*SearchGroup[*LongRange]) {
	s.inSecondPass = make(map[longRangeKey]bool, len(searchGroups))
	s.includeEmpty = false
	for _, g := range searchGroups {
		if g.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.inSecondPass[longRangeKey{g.GroupValue.Min, g.GroupValue.Max}] = true
		}
	}
}

// AdvanceTo positions the selector on document doc.  It returns whether the
// document should be included in the current pass.
// Mirrors LongRangeGroupSelector.advanceTo.
func (s *LongRangeGroupSelector) AdvanceTo(doc int) bool {
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
	return s.inSecondPass[longRangeKey{s.current.Min, s.current.Max}]
}

// Select implements GroupSelector.  It returns the *LongRange for the given
// doc, or nil when the doc has no value.
func (s *LongRangeGroupSelector) Select(doc int) interface{} {
	s.AdvanceTo(doc)
	r := s.CurrentValue()
	if r == nil {
		return nil
	}
	return r
}

// CurrentValue returns the LongRange resolved during the last AdvanceTo call,
// or nil when the document had no value.
// Mirrors LongRangeGroupSelector.currentValue.
func (s *LongRangeGroupSelector) CurrentValue() *LongRange {
	if !s.positioned {
		return nil
	}
	return s.current
}

// CopyValue returns a copy of the current LongRange (to be stored as a group
// key independent of any reuse buffer).
// Mirrors LongRangeGroupSelector.copyValue.
func (s *LongRangeGroupSelector) CopyValue() *LongRange {
	if !s.positioned || s.current == nil {
		return nil
	}
	cp := *s.current
	return &cp
}

// Ensure LongRangeGroupSelector implements GroupSelector.
var _ GroupSelector = (*LongRangeGroupSelector)(nil)
