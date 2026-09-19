// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DoubleRangeGroupSelector is a GroupSelector implementation that groups
// documents by double values.
//
// Mirrors org.apache.lucene.search.grouping.DoubleRangeGroupSelector, which
// extends GroupSelector<DoubleRange>.
type DoubleRangeGroupSelector struct {
	source       search.DoubleValuesSource
	rangeFactory *DoubleRangeFactory

	inSecondPass *groupSet[*DoubleRange]
	includeEmpty bool
	positioned   bool
	current      *DoubleRange

	context *index.LeafReaderContext
	values  search.DoubleValues
}

// NewDoubleRangeGroupSelector creates a new DoubleRangeGroupSelector.
//
// source is a DoubleValuesSource to retrieve double values per document, and
// rangeFactory is a DoubleRangeFactory that defines how to group the double
// values into range buckets.
//
// Mirrors DoubleRangeGroupSelector(DoubleValuesSource, DoubleRangeFactory).
func NewDoubleRangeGroupSelector(source search.DoubleValuesSource, rangeFactory *DoubleRangeFactory) *DoubleRangeGroupSelector {
	return &DoubleRangeGroupSelector{source: source, rangeFactory: rangeFactory}
}

// SetNextReader mirrors setNextReader(LeafReaderContext).
func (s *DoubleRangeGroupSelector) SetNextReader(readerContext *index.LeafReaderContext) error {
	s.context = readerContext
	return nil
}

// SetScorer mirrors setScorer(Scorable).
func (s *DoubleRangeGroupSelector) SetScorer(scorer search.Scorable) error {
	values, err := s.source.GetValues(s.context, search.DoubleValuesSourceFromScorer(scorer))
	if err != nil {
		return err
	}
	s.values = values
	return nil
}

// AdvanceTo mirrors advanceTo(int).
func (s *DoubleRangeGroupSelector) AdvanceTo(doc int) (GroupSelectorState, error) {
	positioned, err := s.values.AdvanceExact(doc)
	if err != nil {
		return GroupSelectorStateSkip, err
	}
	s.positioned = positioned
	if !s.positioned {
		if s.includeEmpty {
			return GroupSelectorStateAccept, nil
		}
		return GroupSelectorStateSkip, nil
	}
	value, err := s.values.DoubleValue()
	if err != nil {
		return GroupSelectorStateSkip, err
	}
	s.current = s.rangeFactory.GetRange(value, s.current)
	if s.inSecondPass == nil {
		return GroupSelectorStateAccept, nil
	}
	if s.inSecondPass.contains(s.current) {
		return GroupSelectorStateAccept, nil
	}
	return GroupSelectorStateSkip, nil
}

// CurrentValue mirrors currentValue().
func (s *DoubleRangeGroupSelector) CurrentValue() (*DoubleRange, error) {
	if s.positioned {
		return s.current, nil
	}
	return nil, nil
}

// CopyValue mirrors copyValue().
func (s *DoubleRangeGroupSelector) CopyValue() (*DoubleRange, error) {
	if s.positioned {
		return NewDoubleRange(s.current.Min, s.current.Max), nil
	}
	return nil, nil
}

// SetGroups mirrors setGroups(Collection<SearchGroup<DoubleRange>>).
func (s *DoubleRangeGroupSelector) SetGroups(searchGroups []*SearchGroup[*DoubleRange]) {
	s.inSecondPass = newGroupSet[*DoubleRange]()
	for _, group := range searchGroups {
		if group.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.inSecondPass.add(group.GroupValue)
		}
	}
}

// Ensure DoubleRangeGroupSelector implements GroupSelector[*DoubleRange].
var _ GroupSelector[*DoubleRange] = (*DoubleRangeGroupSelector)(nil)
