// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LongRangeGroupSelector is a GroupSelector implementation that groups
// documents by long values.
//
// Mirrors org.apache.lucene.search.grouping.LongRangeGroupSelector, which
// extends GroupSelector<LongRange>.
type LongRangeGroupSelector struct {
	source       search.LongValuesSource
	rangeFactory *LongRangeFactory

	inSecondPass *groupSet[*LongRange]
	includeEmpty bool
	positioned   bool
	current      *LongRange

	context *index.LeafReaderContext
	values  search.LongValues
}

// NewLongRangeGroupSelector creates a new LongRangeGroupSelector.
//
// source is a LongValuesSource to retrieve long values per document, and
// rangeFactory is a LongRangeFactory that defines how to group the long
// values into range buckets.
//
// Mirrors LongRangeGroupSelector(LongValuesSource, LongRangeFactory).
func NewLongRangeGroupSelector(source search.LongValuesSource, rangeFactory *LongRangeFactory) *LongRangeGroupSelector {
	return &LongRangeGroupSelector{source: source, rangeFactory: rangeFactory}
}

// SetNextReader mirrors setNextReader(LeafReaderContext).
func (s *LongRangeGroupSelector) SetNextReader(readerContext *index.LeafReaderContext) error {
	s.context = readerContext
	return nil
}

// SetScorer mirrors setScorer(Scorable).
func (s *LongRangeGroupSelector) SetScorer(scorer search.Scorable) error {
	values, err := s.source.GetValues(s.context, search.DoubleValuesSourceFromScorer(scorer))
	if err != nil {
		return err
	}
	s.values = values
	return nil
}

// AdvanceTo mirrors advanceTo(int).
func (s *LongRangeGroupSelector) AdvanceTo(doc int) (GroupSelectorState, error) {
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
	value, err := s.values.LongValue()
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
func (s *LongRangeGroupSelector) CurrentValue() (*LongRange, error) {
	if s.positioned {
		return s.current, nil
	}
	return nil, nil
}

// CopyValue mirrors copyValue().
func (s *LongRangeGroupSelector) CopyValue() (*LongRange, error) {
	if s.positioned {
		return NewLongRange(s.current.Min, s.current.Max), nil
	}
	return nil, nil
}

// SetGroups mirrors setGroups(Collection<SearchGroup<LongRange>>).
func (s *LongRangeGroupSelector) SetGroups(searchGroups []*SearchGroup[*LongRange]) {
	s.inSecondPass = newGroupSet[*LongRange]()
	for _, group := range searchGroups {
		if group.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.inSecondPass.add(group.GroupValue)
		}
	}
}

// Ensure LongRangeGroupSelector implements GroupSelector[*LongRange].
var _ GroupSelector[*LongRange] = (*LongRangeGroupSelector)(nil)
