package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LongRangeGroupSelector implements GroupSelector for long values.
type LongRangeGroupSelector struct {
	source       search.LongValuesSource
	rangeFactory *LongRangeFactory
	inSecondPass map[LongRange]struct{}
	includeEmpty bool
	positioned   bool
	current      *LongRange
	context      index.LeafReaderContext
	values       search.LongValues
}

func NewLongRangeGroupSelector(source search.LongValuesSource, rangeFactory *LongRangeFactory) *LongRangeGroupSelector {
	return &LongRangeGroupSelector{
		source:       source,
		rangeFactory: rangeFactory,
	}
}

func (s *LongRangeGroupSelector) SetNextReader(readerContext *index.LeafReaderContext) error {
	s.context = *readerContext
	return nil
}

func (s *LongRangeGroupSelector) SetScorer(scorer search.Scorable) error {
	var err error
	s.values, err = s.source.GetValues(s.context, search.DoubleValuesSourceFromScorer(scorer))
	return err
}

func (s *LongRangeGroupSelector) AdvanceTo(doc int) (State, error) {
	positioned, err := s.values.AdvanceExact(doc)
	if err != nil {
		return StateSkip, err
	}
	s.positioned = positioned
	if !positioned {
		if s.includeEmpty {
			return StateAccept, nil
		}
		return StateSkip, nil
	}
	s.current = s.rangeFactory.GetRange(s.values.LongValue(), s.current)
	if s.inSecondPass == nil {
		return StateAccept, nil
	}
	if _, ok := s.inSecondPass[*s.current]; ok {
		return StateAccept, nil
	}
	return StateSkip, nil
}

func (s *LongRangeGroupSelector) CurrentValue() (LongRange, error) {
	if !s.positioned || s.current == nil {
		return LongRange{}, fmt.Errorf("selector not positioned on a value")
	}
	return *s.current, nil
}

func (s *LongRangeGroupSelector) CopyValue() (LongRange, error) {
	if !s.positioned || s.current == nil {
		return LongRange{}, fmt.Errorf("selector not positioned on a value")
	}
	return LongRange{Min: s.current.Min, Max: s.current.Max}, nil
}

func (s *LongRangeGroupSelector) SetGroups(groups []SearchGroup[LongRange]) {
	s.inSecondPass = make(map[LongRange]struct{})
	for _, group := range groups {
		if group.GroupValue == nil {

			s.includeEmpty = true
		} else {
			s.inSecondPass[*group.GroupValue] = struct{}{}
		}
	}
}
