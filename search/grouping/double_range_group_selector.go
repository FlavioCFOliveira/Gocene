package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DoubleRangeGroupSelector is a GroupSelector implementation that groups
// documents by double values.
//
// Port of org.apache.lucene.search.grouping.DoubleRangeGroupSelector (Apache
// Lucene 10.5.0).
type DoubleRangeGroupSelector struct {
	source       search.DoubleValuesSource
	rangeFactory *DoubleRangeFactory

	inSecondPass map[DoubleRange]struct{}
	includeEmpty bool
	positioned   bool
	current      *DoubleRange

	context *index.LeafReaderContext
	values  search.DoubleValues
}

// NewDoubleRangeGroupSelector creates a new DoubleRangeGroupSelector.
//
//   - source: a DoubleValuesSource to retrieve double values per document
//   - rangeFactory: a DoubleRangeFactory that defines how to group the double
//     values into range buckets
func NewDoubleRangeGroupSelector(source search.DoubleValuesSource, rangeFactory *DoubleRangeFactory) *DoubleRangeGroupSelector {
	return &DoubleRangeGroupSelector{
		source:       source,
		rangeFactory: rangeFactory,
	}
}

// SetNextReader sets the LeafReaderContext.
func (s *DoubleRangeGroupSelector) SetNextReader(readerContext *index.LeafReaderContext) error {
	s.context = readerContext
	return nil
}

// SetScorer sets the current Scorer.
func (s *DoubleRangeGroupSelector) SetScorer(scorer search.Scorable) error {
	values, err := s.source.GetValues(s.context, search.DoubleValuesSourceFromScorer(scorer))
	if err != nil {
		return err
	}
	s.values = values
	return nil
}

// AdvanceTo advances this selector's iterator to the given document.
func (s *DoubleRangeGroupSelector) AdvanceTo(doc int) (State, error) {
	positioned, err := s.values.AdvanceExact(doc)
	if err != nil {
		return StateSkip, err
	}
	s.positioned = positioned
	if !s.positioned {
		if s.includeEmpty {
			return StateAccept, nil
		}
		return StateSkip, nil
	}
	value, err := s.values.DoubleValue()
	if err != nil {
		return StateSkip, err
	}
	s.current = s.rangeFactory.GetRange(value, s.current)
	if s.inSecondPass == nil {
		return StateAccept, nil
	}
	if _, ok := s.inSecondPass[*s.current]; ok {
		return StateAccept, nil
	}
	return StateSkip, nil
}

// CurrentValue returns the group value of the current document, or nil when the
// selector is not positioned.
func (s *DoubleRangeGroupSelector) CurrentValue() (*DoubleRange, error) {
	if s.positioned {
		return s.current, nil
	}
	return nil, nil
}

// CopyValue returns a copy of the group value of the current document, or nil
// when the selector is not positioned.
func (s *DoubleRangeGroupSelector) CopyValue() (*DoubleRange, error) {
	if s.positioned {
		r := NewDoubleRange(s.current.Min, s.current.Max)
		return &r, nil
	}
	return nil, nil
}

// SetGroups sets a restriction on the group values returned by this selector.
func (s *DoubleRangeGroupSelector) SetGroups(groups []SearchGroup[*DoubleRange]) {
	s.inSecondPass = make(map[DoubleRange]struct{}, len(groups))
	s.includeEmpty = false
	for _, group := range groups {
		if group.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.inSecondPass[*group.GroupValue] = struct{}{}
		}
	}
}

var _ GroupSelector[*DoubleRange] = (*DoubleRangeGroupSelector)(nil)
