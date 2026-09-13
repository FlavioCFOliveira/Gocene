package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ValueSourceGroupSelector implements GroupSelector using a ValueSource.
type ValueSourceGroupSelector struct {
	valueSource function.ValueSource
	context     map[any]any

	secondPassGroups map[*function.MutableValueFloat]struct{}
	includeEmpty     bool
	filler           function.ValueFiller
}

func NewValueSourceGroupSelector(valueSource function.ValueSource, context map[any]any) *ValueSourceGroupSelector {
	return &ValueSourceGroupSelector{
		valueSource: valueSource,
		context:     context,
	}
}

func (s *ValueSourceGroupSelector) SetNextReader(readerContext *index.LeafReaderContext) error {
	values, err := s.valueSource.GetValues(s.context, readerContext)
	if err != nil {
		return err
	}
	s.filler = values.GetValueFiller()
	return nil
}

func (s *ValueSourceGroupSelector) SetScorer(scorer search.Scorable) error {
	return nil
}

func (s *ValueSourceGroupSelector) AdvanceTo(doc int) (State, error) {
	if err := s.filler.FillValue(doc); err != nil {
		return StateSkip, err
	}
	value := s.filler.GetValue()
	if !value.Exists {
		if s.includeEmpty {
			return StateAccept, nil
		}
		return StateSkip, nil
	}
	if s.secondPassGroups != nil {
		if _, ok := s.secondPassGroups[value]; !ok {
			return StateSkip, nil
		}
	}
	return StateAccept, nil
}

func (s *ValueSourceGroupSelector) CurrentValue() (*function.MutableValueFloat, error) {
	return s.filler.GetValue(), nil
}

func (s *ValueSourceGroupSelector) CopyValue() (*function.MutableValueFloat, error) {
	val := s.filler.GetValue()
	return &function.MutableValueFloat{
		Value:  val.Value,
		Exists: val.Exists,
	}, nil
}

func (s *ValueSourceGroupSelector) SetGroups(groups []SearchGroup[*function.MutableValueFloat]) {
	s.secondPassGroups = make(map[*function.MutableValueFloat]struct{})
	for _, group := range groups {
		if group.GroupValue == nil || !group.GroupValue.Exists {
			s.includeEmpty = true
		} else {
			s.secondPassGroups[group.GroupValue] = struct{}{}
		}
	}
}
