package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortedSetSelectorType picks one value from a multi-valued set to use as the representative value.
type SortedSetSelectorType int

const (
	// SortedSetSelectorMin selects the minimum value in the set.
	SortedSetSelectorMin SortedSetSelectorType = iota
	// SortedSetSelectorMax selects the maximum value in the set.
	SortedSetSelectorMax
	// SortedSetSelectorMiddleMin selects the middle value in the set.
	// If the set has an even number of values, the lower of the middle two is chosen.
	SortedSetSelectorMiddleMin
	// SortedSetSelectorMiddleMax selects the middle value in the set.
	// If the set has an even number of values, the higher of the middle two is chosen.
	SortedSetSelectorMiddleMax
)

func (t SortedSetSelectorType) String() string {
	switch t {
	case SortedSetSelectorMin:
		return "MIN"
	case SortedSetSelectorMax:
		return "MAX"
	case SortedSetSelectorMiddleMin:
		return "MIDDLE_MIN"
	case SortedSetSelectorMiddleMax:
		return "MIDDLE_MAX"
	default:
		return fmt.Sprintf("SortedSetSelectorType(%d)", t)
	}
}

// SortedSetSelector wraps a multi-valued SortedSetDocValues as a single-valued view, using the specified selector.
type SortedSetSelector struct{}

func (s *SortedSetSelector) Wrap(sortedSet index.SortedSetDocValues, selector SortedSetSelectorType) index.SortedDocValues {
	if sortedSet.GetValueCount() >= 2147483647 { // Integer.MAX_VALUE
		panic("fields containing more than 2147483646 unique terms are unsupported")
	}

	singleton := index.UnwrapSingletonSortedSet(sortedSet)
	if singleton != nil {
		return singleton
	}

	switch selector {
	case SortedSetSelectorMin:
		return &minValue{in: sortedSet}
	case SortedSetSelectorMax:
		return &maxValue{in: sortedSet}
	case SortedSetSelectorMiddleMin:
		return &middleMinValue{in: sortedSet}
	case SortedSetSelectorMiddleMax:
		return &middleMaxValue{in: sortedSet}
	default:
		panic("invalid selector type")
	}
}

// wrap is the internal function to avoid creating a selector instance if not needed.
func WrapSortedSet(sortedSet index.SortedSetDocValues, selector SortedSetSelectorType) index.SortedDocValues {
	s := &SortedSetSelector{}
	return s.Wrap(sortedSet, selector)
}

type minValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *minValue) DocID() int {
	return m.in.DocID()
}

func (m *minValue) NextDoc() int {
	m.in.NextDoc()
	m.setOrd()
	return m.DocID()
}

func (m *minValue) Advance(target int) int {
	m.in.Advance(target)
	m.setOrd()
	return m.DocID()
}

func (m *minValue) AdvanceExact(target int) bool {
	if m.in.AdvanceExact(target) {
		m.setOrd()
		return true
	}
	return false
}

func (m *minValue) Cost() int64 {
	return m.in.Cost()
}

func (m *minValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) {
	m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *minValue) DocIDRunEnd() int {
	return m.in.DocIDRunEnd()
}

func (m *minValue) OrdValue() int {
	return m.ord
}

func (m *minValue) LookupOrd(ord int) (*util.BytesRef, error) {
	return m.in.LookupOrd(ord)
}

func (m *minValue) GetValueCount() int {
	return int(m.in.GetValueCount())
}

func (m *minValue) LookupTerm(key *util.BytesRef) int {
	return int(m.in.LookupTerm(key))
}

func (m *minValue) setOrd() {
	if m.DocID() != index.NoMoreDocs {
		m.ord = int(m.in.NextOrd())
	}
}

type maxValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *maxValue) DocID() int {
	return m.in.DocID()
}

func (m *maxValue) NextDoc() int {
	m.in.NextDoc()
	m.setOrd()
	return m.DocID()
}

func (m *maxValue) Advance(target int) int {
	m.in.Advance(target)
	m.setOrd()
	return m.DocID()
}

func (m *maxValue) AdvanceExact(target int) bool {
	if m.in.AdvanceExact(target) {
		m.setOrd()
		return true
	}
	return false
}

func (m *maxValue) Cost() int64 {
	return m.in.Cost()
}

func (m *maxValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) {
	m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *maxValue) DocIDRunEnd() int {
	return m.in.DocIDRunEnd()
}

func (m *maxValue) OrdValue() int {
	return m.ord
}

func (m *maxValue) LookupOrd(ord int) (*util.BytesRef, error) {
	return m.in.LookupOrd(ord)
}

func (m *maxValue) GetValueCount() int {
	return int(m.in.GetValueCount())
}

func (m *maxValue) LookupTerm(key *util.BytesRef) int {
	return int(m.in.LookupTerm(key))
}

func (m *maxValue) setOrd() {
	if m.DocID() != index.NoMoreDocs {
		docValueCount := m.in.DocValueCount()
		for i := 0; i < docValueCount-1; i++ {
			m.in.NextOrd()
		}
		m.ord = int(m.in.NextOrd())
	}
}

type middleMinValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *middleMinValue) DocID() int {
	return m.in.DocID()
}

func (m *middleMinValue) NextDoc() int {
	m.in.NextDoc()
	m.setOrd()
	return m.DocID()
}

func (m *middleMinValue) Advance(target int) int {
	m.in.Advance(target)
	m.setOrd()
	return m.DocID()
}

func (m *middleMinValue) AdvanceExact(target int) bool {
	if m.in.AdvanceExact(target) {
		m.setOrd()
		return true
	}
	return false
}

func (m *middleMinValue) Cost() int64 {
	return m.in.Cost()
}

func (m *middleMinValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) {
	m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *middleMinValue) DocIDRunEnd() int {
	return m.in.DocIDRunEnd()
}

func (m *middleMinValue) OrdValue() int {
	return m.ord
}

func (m *middleMinValue) LookupOrd(ord int) (*util.BytesRef, error) {
	return m.in.LookupOrd(ord)
}

func (m *middleMinValue) GetValueCount() int {
	return int(m.in.GetValueCount())
}

func (m *middleMinValue) LookupTerm(key *util.BytesRef) int {
	return int(m.in.LookupTerm(key))
}

func (m *middleMinValue) setOrd() {
	if m.DocID() != index.NoMoreDocs {
		docValueCount := m.in.DocValueCount()
		targetIdx := (docValueCount - 1) >> 1
		for i := 0; i < targetIdx; i++ {
			m.in.NextOrd()
		}
		m.ord = int(m.in.NextOrd())
	}
}

type middleMaxValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *middleMaxValue) DocID() int {
	return m.in.DocID()
}

func (m *middleMaxValue) NextDoc() int {
	m.in.NextDoc()
	m.setOrd()
	return m.DocID()
}

func (m *middleMaxValue) Advance(target int) int {
	m.in.Advance(target)
	m.setOrd()
	return m.DocID()
}

func (m *middleMaxValue) AdvanceExact(target int) bool {
	if m.in.AdvanceExact(target) {
		m.setOrd()
		return true
	}
	return false
}

func (m *middleMaxValue) Cost() int64 {
	return m.in.Cost()
}

func (m *middleMaxValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) {
	m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *middleMaxValue) DocIDRunEnd() int {
	return m.in.DocIDRunEnd()
}

func (m *middleMaxValue) OrdValue() int {
	return m.ord
}

func (m *middleMaxValue) LookupOrd(ord int) (*util.BytesRef, error) {
	return m.in.LookupOrd(ord)
}

func (m *middleMaxValue) GetValueCount() int {
	return int(m.in.GetValueCount())
}

func (m *middleMaxValue) LookupTerm(key *util.BytesRef) int {
	return int(m.in.LookupTerm(key))
}

func (m *middleMaxValue) setOrd() {
	if m.DocID() != index.NoMoreDocs {
		docValueCount := m.in.DocValueCount()
		targetIdx := docValueCount >> 1
		for i := 0; i < targetIdx; i++ {
			m.in.NextOrd()
		}
		m.ord = int(m.in.NextOrd())
	}
}
