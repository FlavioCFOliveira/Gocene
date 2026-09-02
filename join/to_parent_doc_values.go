package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type accumulator interface {
	Reset() error
	Increment() error
	GetChildrenWithValuesCount() int
}

type toParentDocValues struct {
	parents                *util.FixedBitSet
	childWithValues        util.DocIdSetIterator
	collector              accumulator
	docID                  int
	hasChildWithMissingValue bool
	seen                   bool
}

func (t *toParentDocValues) DocID() int {
	return t.docID
}

func (t *toParentDocValues) NextDoc() int {
	if t.docID == search.NoMoreDocs {
		return search.NoMoreDocs
	}

	if t.childWithValues.DocID() < t.docID || t.docID == -1 {
		t.childWithValues.NextDoc()
	}
	if t.childWithValues.DocID() == search.NoMoreDocs {
		t.docID = search.NoMoreDocs
		return t.docID
	}

	// assert t.parents.Get(t.childWithValues.DocID()) == false

	nextParentDocID := t.parents.NextSetBit(t.childWithValues.DocID())
	if err := t.collector.Reset(); err != nil {
		panic(err)
	}
	t.seen = true

	for {
		childDocID := t.childWithValues.NextDoc()
		if childDocID > nextParentDocID || childDocID == search.NoMoreDocs {
			break
		}
		if err := t.collector.Increment(); err != nil {
			panic(err)
		}
	}

	t.docID = nextParentDocID
	prevParentDocID := t.parents.PrevSetBit(t.docID - 1)
	totalChildren := t.docID - prevParentDocID - 1
	t.hasChildWithMissingValue = t.collector.GetChildrenWithValuesCount() < totalChildren

	return t.docID
}

func (t *toParentDocValues) Advance(target int) int {
	if target >= t.parents.Length() {
		t.docID = search.NoMoreDocs
		return t.docID
	}
	if target == 0 {
		if t.DocID() == -1 {
			return t.NextDoc()
		}
		return t.DocID()
	}
	prevParentDocID := t.parents.PrevSetBit(target - 1)
	if t.childWithValues.DocID() <= prevParentDocID {
		t.childWithValues.Advance(prevParentDocID + 1)
	}
	return t.NextDoc()
}

func (t *toParentDocValues) AdvanceExact(targetParentDocID int) bool {
	if targetParentDocID < t.docID {
		panic(fmt.Sprintf("target must be after the current document: current=%d target=%d", t.docID, targetParentDocID))
	}
	previousDocId := t.docID
	t.docID = targetParentDocID
	if targetParentDocID == previousDocId {
		return t.seen
	}
	t.docID = targetParentDocID
	t.seen = false
	t.hasChildWithMissingValue = false

	if !t.parents.Get(targetParentDocID) {
		return false
	}
	prevParentDocId := 0
	if t.docID != 0 {
		prevParentDocId = t.parents.PrevSetBit(t.docID - 1)
	} else {
		prevParentDocId = -1
	}
	totalChildren := t.docID - prevParentDocId - 1
	childDoc := t.childWithValues.DocID()
	if childDoc <= prevParentDocId {
		childDoc = t.childWithValues.Advance(prevParentDocId + 1)
	}
	if childDoc >= t.docID || childDoc == search.NoMoreDocs {
		return false
	}

	if t.childWithValues.DocID() < t.docID {
		if err := t.collector.Reset(); err != nil {
			panic(err)
		}
		t.seen = true
		t.childWithValues.NextDoc()
	}

	if !t.seen {
		return false
	}

	for doc := t.childWithValues.DocID(); doc < t.docID && doc != search.NoMoreDocs; doc = t.childWithValues.NextDoc() {
		if err := t.collector.Increment(); err != nil {
			panic(err)
		}
	}
	t.hasChildWithMissingValue = t.collector.GetChildrenWithValuesCount() < totalChildren
	return true
}

func (t *toParentDocValues) Cost() int64 {
	return 0
}

func (t *toParentDocValues) HasChildWithMissingValue() bool {
	return t.hasChildWithMissingValue
}

type sortedDVs struct {
	values          index.SortedDocValues
	selection       BlockJoinSelectorType
	missingOrd      int
	ord             int
	childrenCount   int
	iter            *toParentDocValues
}

func (s *sortedDVs) DocID() int {
	return s.iter.DocID()
}

func (s *sortedDVs) Reset() error {
	s.ord = s.values.OrdValue()
	s.childrenCount = 1
	return nil
}

func (s *sortedDVs) Increment() error {
	switch s.selection {
	case BlockJoinSelectorMin:
		if s.values.OrdValue() < s.ord {
			s.ord = s.values.OrdValue()
		}
	case BlockJoinSelectorMax:
		if s.values.OrdValue() > s.ord {
			s.ord = s.values.OrdValue()
		}
	}
	s.childrenCount++
	return nil
}

func (s *sortedDVs) GetChildrenWithValuesCount() int {
	return s.childrenCount
}

func (s *sortedDVs) NextDoc() int {
	return s.iter.NextDoc()
}

func (s *sortedDVs) Advance(target int) int {
	return s.iter.Advance(target)
}

func (s *sortedDVs) AdvanceExact(target int) bool {
	return s.iter.AdvanceExact(target)
}

func (s *sortedDVs) OrdValue() int {
	if s.iter.HasChildWithMissingValue() {
		switch s.selection {
		case BlockJoinSelectorMin:
			if s.missingOrd < s.ord {
				return s.missingOrd
			}
			return s.ord
		case BlockJoinSelectorMax:
			if s.missingOrd > s.ord {
				return s.missingOrd
			}
			return s.ord
		}
	}
	return s.ord
}

func (s *sortedDVs) LookupOrd(ord int) (*util.BytesRef, error) {
	return s.values.LookupOrd(ord)
}

func (s *sortedDVs) GetValueCount() int {
	return s.values.GetValueCount()
}

func (s *sortedDVs) Cost() int64 {
	return s.values.Cost()
}

type numDV struct {
	values          index.NumericDocValues
	selection       BlockJoinSelectorType
	missingValue    *int64
	value           int64
	childrenCount   int
	iter            *toParentDocValues
}

func (n *numDV) Reset() error {
	n.value = n.values.LongValue()
	n.childrenCount = 1
	return nil
}

func (n *numDV) Increment() error {
	switch n.selection {
	case BlockJoinSelectorMin:
		if n.values.LongValue() < n.value {
			n.value = n.values.LongValue()
		}
	case BlockJoinSelectorMax:
		if n.values.LongValue() > n.value {
			n.value = n.values.LongValue()
		}
	}
	n.childrenCount++
	return nil
}

func (n *numDV) GetChildrenWithValuesCount() int {
	return n.childrenCount
}

func (n *numDV) NextDoc() int {
	return n.iter.NextDoc()
}

func (n *numDV) Advance(target int) int {
	return n.iter.Advance(target)
}

func (n *numDV) AdvanceExact(target int) bool {
	return n.iter.AdvanceExact(target)
}

func (n *numDV) LongValue() int64 {
	if n.missingValue != nil && n.iter.HasChildWithMissingValue() {
		switch n.selection {
		case BlockJoinSelectorMin:
			if *n.missingValue < n.value {
				return *n.missingValue
			}
			return n.value
		case BlockJoinSelectorMax:
			if *n.missingValue > n.value {
				return *n.missingValue
			}
			return n.value
		}
	}
	return n.value
}

func (n *numDV) DocID() int {
	return n.iter.DocID()
}

func (n *numDV) Cost() int64 {
	return n.values.Cost()
}

func wrapSorted(values index.SortedDocValues, selection BlockJoinSelectorType, parents *util.FixedBitSet, children util.DocIdSetIterator, sortMissingLast bool) index.SortedDocValues {
	missingOrd := -1
	if sortMissingLast {
		missingOrd = 2147483647
	}
	s := &sortedDVs{
		values:     values,
		selection:  selection,
		missingOrd: missingOrd,
	}
	s.iter = &toParentDocValues{
		parents:         parents,
		childWithValues: intersectTwoIterators(children, values),
		collector:       s,
	}
	return s
}

func wrapNumeric(values index.NumericDocValues, selection BlockJoinSelectorType, parents *util.FixedBitSet, children util.DocIdSetIterator, missingValue *int64) index.NumericDocValues {
	n := &numDV{
		values:       values,
		selection:    selection,
		missingValue: missingValue,
	}
	n.iter = &toParentDocValues{
		parents:         parents,
		childWithValues: intersectTwoIterators(children, values),
		collector:       n,
	}
	return n
}

type intersectTwoIterators struct {
	it1, it2 util.DocIdSetIterator
}

func intersectTwoIterators(it1, it2 util.DocIdSetIterator) util.DocIdSetIterator {
	return &intersectTwoIterators{it1: it1, it2: it2}
}

func (i *intersectTwoIterators) DocID() int {
	return i.it1.DocID()
}

func (i *intersectTwoIterators) NextDoc() int {
	for {
		d1 := i.it1.NextDoc()
		d2 := i.it2.NextDoc()
		if d1 == search.NoMoreDocs || d2 == search.NoMoreDocs {
			return search.NoMoreDocs
		}
		if d1 == d2 {
			return d1
		}
		if d1 < d2 {
			i.it2.Advance(d1)
		} else {
			i.it1.Advance(d2)
		}
	}
}

func (i *intersectTwoIterators) Advance(target int) int {
	i.it1.Advance(target)
	i.it2.Advance(target)
	for {
		d1 := i.it1.DocID()
		d2 := i.it2.DocID()
		if d1 == search.NoMoreDocs || d2 == search.NoMoreDocs {
			return search.NoMoreDocs
		}
		if d1 == d2 {
			return d1
		}
		if d1 < d2 {
			i.it1.NextDoc()
		} else {
			i.it2.NextDoc()
		}
	}
}

func (i *intersectTwoIterators) AdvanceExact(target int) bool {
	if i.it1.AdvanceExact(target) && i.it2.AdvanceExact(target) {
		return true
	}
	return false
}

func (i *intersectTwoIterators) Cost() int64 {
	return i.it1.Cost() + i.it2.Cost()
}
