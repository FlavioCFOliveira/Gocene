// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/LongRangeGroupSelector.java

// LongRangeGroupSelector is a GroupSelector implementation that groups
// documents by long values.
//
// Mirrors org.apache.lucene.search.grouping.LongRangeGroupSelector, which
// extends GroupSelector<LongRange>. Java's currentValue and copyValue return
// null when the selector is not positioned, so the Go type parameter is
// *LongRange rather than LongRange: only a pointer can carry that null.
type LongRangeGroupSelector struct {
	source       search.LongValuesSource
	rangeFactory *LongRangeFactory

	inSecondPass map[LongRange]struct{}
	includeEmpty bool
	positioned   bool
	current      *LongRange

	context *index.LeafReaderContext
	values  search.LongValues
}

// NewLongRangeGroupSelector creates a new LongRangeGroupSelector. source
// retrieves long values per document; rangeFactory defines how to group those
// long values into range buckets.
//
// Mirrors LongRangeGroupSelector(LongValuesSource, LongRangeFactory).
func NewLongRangeGroupSelector(source search.LongValuesSource, rangeFactory *LongRangeFactory) *LongRangeGroupSelector {
	return &LongRangeGroupSelector{
		source:       source,
		rangeFactory: rangeFactory,
	}
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
	value, err := s.values.LongValue()
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

// CurrentValue mirrors currentValue(), which returns null when the selector is
// not positioned on a value.
func (s *LongRangeGroupSelector) CurrentValue() (*LongRange, error) {
	if !s.positioned {
		return nil, nil
	}
	return s.current, nil
}

// CopyValue mirrors copyValue(), which returns null when the selector is not
// positioned on a value.
func (s *LongRangeGroupSelector) CopyValue() (*LongRange, error) {
	if !s.positioned {
		return nil, nil
	}
	return NewLongRange(s.current.Min, s.current.Max), nil
}

// SetGroups mirrors setGroups(Collection<SearchGroup<LongRange>>). Java holds
// the restriction in a Set<LongRange>, whose membership test is
// LongRange.equals — a value comparison; the Go map is therefore keyed by the
// dereferenced range, not by the pointer.
func (s *LongRangeGroupSelector) SetGroups(groups []SearchGroup[*LongRange]) {
	s.inSecondPass = make(map[LongRange]struct{})
	for _, group := range groups {
		if group.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.inSecondPass[*group.GroupValue] = struct{}{}
		}
	}
}

var _ GroupSelector[*LongRange] = (*LongRangeGroupSelector)(nil)
