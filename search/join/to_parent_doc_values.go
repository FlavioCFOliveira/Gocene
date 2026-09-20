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
	parents                  util.BitSet
	childWithValues          util.DocIdSetIterator
	collector                accumulator
	docID                    int
	hasChildWithMissingValue bool
	seen                     bool
}

func (t *toParentDocValues) DocID() int {
	return t.docID
}

func (t *toParentDocValues) NextDoc() (int, error) {
	if t.docID == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS, nil
	}

	if t.childWithValues.DocID() < t.docID || t.docID == -1 {
		if _, err := t.childWithValues.NextDoc(); err != nil {
			return 0, err
		}
	}
	if t.childWithValues.DocID() == search.NO_MORE_DOCS {
		t.docID = search.NO_MORE_DOCS
		return t.docID, nil
	}

	// assert t.parents.Get(t.childWithValues.DocID()) == false

	nextParentDocID := t.parents.NextSetBitBounded(t.childWithValues.DocID())
	if err := t.collector.Reset(); err != nil {
		return 0, err
	}
	t.seen = true

	for {
		childDocID, err := t.childWithValues.NextDoc()
		if err != nil {
			return 0, err
		}
		if childDocID > nextParentDocID || childDocID == search.NO_MORE_DOCS {
			break
		}
		if err := t.collector.Increment(); err != nil {
			return 0, err
		}
	}

	t.docID = nextParentDocID
	prevParentDocID := t.parents.PrevSetBit(t.docID - 1)
	totalChildren := t.docID - prevParentDocID - 1
	t.hasChildWithMissingValue = t.collector.GetChildrenWithValuesCount() < totalChildren

	return t.docID, nil
}

func (t *toParentDocValues) Advance(target int) (int, error) {
	if target >= t.parents.Length() {
		t.docID = search.NO_MORE_DOCS
		return t.docID, nil
	}
	if target == 0 {
		if t.DocID() == -1 {
			return t.NextDoc()
		}
		return t.DocID(), nil
	}
	prevParentDocID := t.parents.PrevSetBit(target - 1)
	if t.childWithValues.DocID() <= prevParentDocID {
		if _, err := t.childWithValues.Advance(prevParentDocID + 1); err != nil {
			return 0, err
		}
	}
	return t.NextDoc()
}

func (t *toParentDocValues) AdvanceExact(targetParentDocID int) (bool, error) {
	if targetParentDocID < t.docID {
		return false, fmt.Errorf("target must be after the current document: current=%d target=%d", t.docID, targetParentDocID)
	}
	previousDocId := t.docID
	t.docID = targetParentDocID
	if targetParentDocID == previousDocId {
		return t.seen, nil
	}
	t.docID = targetParentDocID
	t.seen = false
	t.hasChildWithMissingValue = false

	if !t.parents.Get(targetParentDocID) {
		return false, nil
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
		var err error
		childDoc, err = t.childWithValues.Advance(prevParentDocId + 1)
		if err != nil {
			return false, err
		}
	}
	if childDoc >= t.docID || childDoc == search.NO_MORE_DOCS {
		return false, nil
	}

	if t.childWithValues.DocID() < t.docID {
		if err := t.collector.Reset(); err != nil {
			return false, err
		}
		t.seen = true
		if _, err := t.childWithValues.NextDoc(); err != nil {
			return false, err
		}
	}

	if !t.seen {
		return false, nil
	}

	for doc := t.childWithValues.DocID(); doc < t.docID && doc != search.NO_MORE_DOCS; {
		if err := t.collector.Increment(); err != nil {
			return false, err
		}
		next, err := t.childWithValues.NextDoc()
		if err != nil {
			return false, err
		}
		doc = next
	}
	t.hasChildWithMissingValue = t.collector.GetChildrenWithValuesCount() < totalChildren
	return true, nil
}

func (t *toParentDocValues) Cost() int64 {
	return 0
}

func (t *toParentDocValues) HasChildWithMissingValue() bool {
	return t.hasChildWithMissingValue
}

// DocIDRunEnd carries the default body of
// DocIdSetIterator.docIDRunEnd() in Apache Lucene 10.5.0, which
// ToParentDocValues does not override.
func (t *toParentDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(t)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which ToParentDocValues does not override.
func (t *toParentDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(t, upTo, bitSet, offset)
}

type sortedDVs struct {
	values        index.SortedDocValues
	selection     BlockJoinSelectorType
	missingOrd    int
	ord           int
	childrenCount int
	iter          *toParentDocValues
}

func (s *sortedDVs) DocID() int {
	return s.iter.DocID()
}

func (s *sortedDVs) Reset() error {
	ord, err := s.values.OrdValue()
	if err != nil {
		return err
	}
	s.ord = ord
	s.childrenCount = 1
	return nil
}

func (s *sortedDVs) Increment() error {
	ord, err := s.values.OrdValue()
	if err != nil {
		return err
	}
	switch s.selection {
	case BlockJoinSelectorMin:
		if ord < s.ord {
			s.ord = ord
		}
	case BlockJoinSelectorMax:
		if ord > s.ord {
			s.ord = ord
		}
	}
	s.childrenCount++
	return nil
}

func (s *sortedDVs) GetChildrenWithValuesCount() int {
	return s.childrenCount
}

func (s *sortedDVs) NextDoc() (int, error) {
	return s.iter.NextDoc()
}

func (s *sortedDVs) Advance(target int) (int, error) {
	return s.iter.Advance(target)
}

func (s *sortedDVs) AdvanceExact(target int) (bool, error) {
	return s.iter.AdvanceExact(target)
}

func (s *sortedDVs) OrdValue() (int, error) {
	if s.iter.HasChildWithMissingValue() {
		switch s.selection {
		case BlockJoinSelectorMin:
			if s.missingOrd < s.ord {
				return s.missingOrd, nil
			}
			return s.ord, nil
		case BlockJoinSelectorMax:
			if s.missingOrd > s.ord {
				return s.missingOrd, nil
			}
			return s.ord, nil
		}
	}
	return s.ord, nil
}

// LongValue satisfies the NumericDocValues half of Gocene's
// spi.SortedDocValues, which — unlike Java, where SortedDocValues extends
// BinaryDocValues — embeds NumericDocValues. It reports the current
// ordinal, the same convention the other Gocene SortedDocValues carry
// (see index.AssertingSortedDocValues, index.mergedSortedDocValues).
func (s *sortedDVs) LongValue() (int64, error) {
	ord, err := s.OrdValue()
	return int64(ord), err
}

func (s *sortedDVs) LookupOrd(ord int) ([]byte, error) {
	return s.values.LookupOrd(ord)
}

func (s *sortedDVs) GetValueCount() int {
	return s.values.GetValueCount()
}

func (s *sortedDVs) Cost() int64 {
	return s.values.Cost()
}

type numDV struct {
	values        index.NumericDocValues
	selection     BlockJoinSelectorType
	missingValue  *int64
	value         int64
	childrenCount int
	iter          *toParentDocValues
}

func (n *numDV) Reset() error {
	value, err := n.values.LongValue()
	if err != nil {
		return err
	}
	n.value = value
	n.childrenCount = 1
	return nil
}

func (n *numDV) Increment() error {
	value, err := n.values.LongValue()
	if err != nil {
		return err
	}
	switch n.selection {
	case BlockJoinSelectorMin:
		if value < n.value {
			n.value = value
		}
	case BlockJoinSelectorMax:
		if value > n.value {
			n.value = value
		}
	}
	n.childrenCount++
	return nil
}

func (n *numDV) GetChildrenWithValuesCount() int {
	return n.childrenCount
}

func (n *numDV) NextDoc() (int, error) {
	return n.iter.NextDoc()
}

func (n *numDV) Advance(target int) (int, error) {
	return n.iter.Advance(target)
}

func (n *numDV) AdvanceExact(target int) (bool, error) {
	return n.iter.AdvanceExact(target)
}

func (n *numDV) LongValue() (int64, error) {
	if n.missingValue != nil && n.iter.HasChildWithMissingValue() {
		switch n.selection {
		case BlockJoinSelectorMin:
			if *n.missingValue < n.value {
				return *n.missingValue, nil
			}
			return n.value, nil
		case BlockJoinSelectorMax:
			if *n.missingValue > n.value {
				return *n.missingValue, nil
			}
			return n.value, nil
		}
	}
	return n.value, nil
}

func (n *numDV) DocID() int {
	return n.iter.DocID()
}

func (n *numDV) Cost() int64 {
	return n.values.Cost()
}

func wrapSorted(values index.SortedDocValues, selection BlockJoinSelectorType, parents util.BitSet, children util.DocIdSetIterator, sortMissingLast bool) index.SortedDocValues {
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
		childWithValues: search.IntersectIterators([]search.DocIdSetIterator{children, values}),
		collector:       s,
	}
	return s
}

func wrapNumeric(values index.NumericDocValues, selection BlockJoinSelectorType, parents util.BitSet, children util.DocIdSetIterator, missingValue *int64) index.NumericDocValues {
	n := &numDV{
		values:       values,
		selection:    selection,
		missingValue: missingValue,
	}
	n.iter = &toParentDocValues{
		parents:         parents,
		childWithValues: search.IntersectIterators([]search.DocIdSetIterator{children, values}),
		collector:       n,
	}
	return n
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd()
// in Apache Lucene 10.5.0, which SortedDVs does not override.
func (s *sortedDVs) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(s)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which SortedDVs does not override.
func (s *sortedDVs) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd()
// in Apache Lucene 10.5.0, which NumDV does not override.
func (n *numDV) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(n)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which NumDV does not override.
func (n *numDV) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(n, upTo, bitSet, offset)
}
