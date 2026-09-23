// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/mutable"
)

// ValueSourceGroupSelector is a GroupSelector that groups via a ValueSource.
//
// Mirrors org.apache.lucene.search.grouping.ValueSourceGroupSelector (Apache
// Lucene 10.5.0), which extends GroupSelector<MutableValue>.
type ValueSourceGroupSelector struct {
	valueSource      function.ValueSource
	context          function.Context
	secondPassGroups *groupSet[mutable.MutableValue]
	includeEmpty     bool

	filler function.ValueFiller
}

// NewValueSourceGroupSelector creates a new ValueSourceGroupSelector.
//
// valueSource is the ValueSource to group by; context is a context map for
// the ValueSource (Java's Map<Object, Object>).
func NewValueSourceGroupSelector(valueSource function.ValueSource, context function.Context) *ValueSourceGroupSelector {
	return &ValueSourceGroupSelector{valueSource: valueSource, context: context}
}

// SetNextReader renders setNextReader(LeafReaderContext).
func (s *ValueSourceGroupSelector) SetNextReader(readerContext *index.LeafReaderContext) error {
	values, err := s.valueSource.GetValues(s.context, readerContext)
	if err != nil {
		return err
	}
	s.filler = values.GetValueFiller()
	return nil
}

// SetScorer renders setScorer(Scorable), which does nothing.
func (s *ValueSourceGroupSelector) SetScorer(scorer search.Scorable) error { return nil }

// AdvanceTo renders advanceTo(int).
func (s *ValueSourceGroupSelector) AdvanceTo(doc int) (GroupSelectorState, error) {
	if err := s.filler.FillValue(doc); err != nil {
		return GroupSelectorStateSkip, err
	}
	value := s.filler.GetValue()
	if value.Exists() == false {
		if s.includeEmpty {
			return GroupSelectorStateAccept, nil
		}
		return GroupSelectorStateSkip, nil
	}
	if s.secondPassGroups != nil {
		if s.secondPassGroups.contains(value) == false {
			return GroupSelectorStateSkip, nil
		}
	}
	return GroupSelectorStateAccept, nil
}

// CurrentValue renders currentValue().
func (s *ValueSourceGroupSelector) CurrentValue() (mutable.MutableValue, error) {
	return s.filler.GetValue(), nil
}

// CopyValue renders copyValue(): filler.getValue().duplicate().
func (s *ValueSourceGroupSelector) CopyValue() (mutable.MutableValue, error) {
	return s.filler.GetValue().Duplicate(), nil
}

// SetGroups renders setGroups(Collection<SearchGroup<MutableValue>>).
func (s *ValueSourceGroupSelector) SetGroups(searchGroups []*SearchGroup[mutable.MutableValue]) {
	s.secondPassGroups = newGroupSet[mutable.MutableValue]()
	for _, group := range searchGroups {
		if group.GroupValue.Exists() == false {
			s.includeEmpty = true
		} else {
			s.secondPassGroups.add(group.GroupValue)
		}
	}
}

var _ GroupSelector[mutable.MutableValue] = (*ValueSourceGroupSelector)(nil)
