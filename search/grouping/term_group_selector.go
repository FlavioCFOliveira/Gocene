package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TermGroupSelector implements GroupSelector for terms (binary values).
type TermGroupSelector struct {
	field     string
	values    []string
	ordsToIDs map[int]int

	docValues index.SortedDocValues
	groupId   int

	secondPass   bool
	includeEmpty bool
	scratch      []byte
}

func NewTermGroupSelector(field string) *TermGroupSelector {
	return &TermGroupSelector{
		field:     field,
		ordsToIDs: make(map[int]int),
	}
}

func (s *TermGroupSelector) SetNextReader(readerContext index.LeafReaderContext) error {
	var err error
	s.docValues, err = index.GetSortedDocValues(readerContext.Reader(), s.field)
	if err != nil {
		return err
	}

	s.ordsToIDs = make(map[int]int)
	for i, val := range s.values {
		ord, err := s.docValues.LookupTerm([]byte(val))
		if err == nil && ord >= 0 {
			s.ordsToIDs[ord] = i
		}
	}
	return nil
}

func (s *TermGroupSelector) SetScorer(scorer search.Scorable) error {
	return nil
}

func (s *TermGroupSelector) AdvanceTo(doc int) (State, error) {
	positioned, err := s.docValues.AdvanceExact(doc)
	if err != nil {
		return StateSkip, err
	}
	if !positioned {
		if s.includeEmpty {
			return StateAccept, nil
		}
		return StateSkip, nil
	}

	ord, err := s.docValues.OrdValue()
	if err != nil {
		return StateSkip, err
	}

	if id, ok := s.ordsToIDs[ord]; ok {
		s.groupId = id
		return StateAccept, nil
	}

	if s.secondPass {
		return StateSkip, nil
	}

	val, err := s.docValues.LookupOrd(ord)
	if err != nil {
		return StateSkip, err
	}

	valStr := string(val)
	s.groupId = len(s.values)
	s.values = append(s.values, valStr)
	s.ordsToIDs[ord] = s.groupId

	return StateAccept, nil
}

func (s *TermGroupSelector) CurrentValue() ([]byte, error) {
	if s.groupId == -1 {
		return nil, nil
	}
	return []byte(s.values[s.groupId]), nil
}

func (s *TermGroupSelector) CopyValue() ([]byte, error) {
	val, err := s.CurrentValue()
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, nil
	}
	cp := make([]byte, len(val))
	copy(cp, val)
	return cp, nil
}

func (s *TermGroupSelector) SetGroups(groups []SearchGroup[[]byte]) {
	s.values = make([]string, 0)
	s.ordsToIDs = make(map[int]int)
	for _, group := range groups {
		if group.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.values = append(s.values, string(*group.GroupValue))
		}
	}
	s.secondPass = true
}
