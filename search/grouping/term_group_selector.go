// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/internal/hppc"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermGroupSelector is a GroupSelector implementation that groups via
// SortedDocValues.
//
// Mirrors org.apache.lucene.search.grouping.TermGroupSelector, which extends
// GroupSelector<BytesRef>.
type TermGroupSelector struct {
	field          string
	values         *util.BytesRefHash
	ordsToGroupIds *hppc.IntIntHashMap

	docValues index.SortedDocValues
	groupID   int

	secondPass   bool
	includeEmpty bool

	scratch *util.BytesRef
}

// NewTermGroupSelector creates a new TermGroupSelector for the given
// SortedDocValues field to use for grouping.
//
// Mirrors TermGroupSelector(String field).
func NewTermGroupSelector(field string) *TermGroupSelector {
	return &TermGroupSelector{
		field:          field,
		values:         util.NewBytesRefHash(),
		ordsToGroupIds: hppc.NewIntIntHashMap(),
		scratch:        util.NewBytesRefEmpty(),
	}
}

// SetNextReader mirrors setNextReader(LeafReaderContext).
func (s *TermGroupSelector) SetNextReader(readerContext *index.LeafReaderContext) error {
	docValues, err := index.GetSorted(readerContext.LeafReader(), s.field)
	if err != nil {
		return err
	}
	s.docValues = docValues
	s.ordsToGroupIds.Clear()
	scratch := util.NewBytesRefEmpty()
	for i := 0; i < s.values.Size(); i++ {
		s.values.Get(i, scratch)
		ord, err := index.SortedDocValuesLookupTerm(s.docValues, scratch.Bytes[scratch.Offset:scratch.Offset+scratch.Length])
		if err != nil {
			return err
		}
		if ord >= 0 {
			s.ordsToGroupIds.Put(ord, i)
		}
	}
	return nil
}

// SetScorer mirrors setScorer(Scorable), whose body is empty.
func (s *TermGroupSelector) SetScorer(scorer search.Scorable) error {
	return nil
}

// AdvanceTo mirrors advanceTo(int).
func (s *TermGroupSelector) AdvanceTo(doc int) (GroupSelectorState, error) {
	exact, err := s.docValues.AdvanceExact(doc)
	if err != nil {
		return GroupSelectorStateSkip, err
	}
	if !exact {
		s.groupID = -1
		if s.includeEmpty {
			return GroupSelectorStateAccept, nil
		}
		return GroupSelectorStateSkip, nil
	}
	ord, err := s.docValues.OrdValue()
	if err != nil {
		return GroupSelectorStateSkip, err
	}
	if s.ordsToGroupIds.ContainsKey(ord) {
		s.groupID = s.ordsToGroupIds.Get(ord)
		return GroupSelectorStateAccept, nil
	}
	if s.secondPass {
		return GroupSelectorStateSkip, nil
	}
	term, err := s.docValues.LookupOrd(ord)
	if err != nil {
		return GroupSelectorStateSkip, err
	}
	groupID, err := s.values.Add(util.NewBytesRef(term))
	if err != nil {
		return GroupSelectorStateSkip, err
	}
	s.groupID = groupID
	s.ordsToGroupIds.Put(ord, s.groupID)
	return GroupSelectorStateAccept, nil
}

// CurrentValue mirrors currentValue().
func (s *TermGroupSelector) CurrentValue() (*util.BytesRef, error) {
	if s.groupID == -1 {
		return nil, nil
	}
	s.values.Get(s.groupID, s.scratch)
	return s.scratch, nil
}

// CopyValue mirrors copyValue().
func (s *TermGroupSelector) CopyValue() (*util.BytesRef, error) {
	if s.groupID == -1 {
		return nil, nil
	}
	current, err := s.CurrentValue()
	if err != nil {
		return nil, err
	}
	return util.BytesRefDeepCopyOf(current), nil
}

// SetGroups mirrors setGroups(Collection<SearchGroup<BytesRef>>). Java's
// BytesRefHash.add signals an over-long term with the unchecked
// MaxBytesLengthExceededException, which setGroups does not declare; the Go
// rendering keeps that failure unchecked too.
func (s *TermGroupSelector) SetGroups(searchGroups []*SearchGroup[*util.BytesRef]) {
	s.values.ClearWithPoolReset()
	s.values.Reinit()
	for _, sg := range searchGroups {
		if sg.GroupValue == nil {
			s.includeEmpty = true
		} else {
			if _, err := s.values.Add(sg.GroupValue); err != nil {
				panic(err)
			}
		}
	}
	s.secondPass = true
}

// Ensure TermGroupSelector implements GroupSelector[*util.BytesRef].
var _ GroupSelector[*util.BytesRef] = (*TermGroupSelector)(nil)
