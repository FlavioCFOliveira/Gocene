package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/SortedSetSelector.java

// SortedSetSelectorType picks one value from the document's set to use as the
// representative value.
//
// Mirrors the nested enum SortedSetSelector.Type.
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

// SortedSetSelector selects a value from the document's set to use as the
// representative value.
type SortedSetSelector struct{}

// Wrap wraps a multi-valued SortedSetDocValues as a single-valued view, using
// the specified selector.
//
// Mirrors the static SortedSetSelector.wrap(SortedSetDocValues, Type).
func (s *SortedSetSelector) Wrap(sortedSet index.SortedSetDocValues, selector SortedSetSelectorType) index.SortedDocValues {
	if sortedSet.GetValueCount() >= 2147483647 { // Integer.MAX_VALUE
		panic("fields containing more than 2147483646 unique terms are unsupported")
	}

	singleton := index.UnwrapSingletonSortedSet(sortedSet)
	if singleton != nil {
		// it's actually single-valued in practice, but indexed as multi-valued,
		// so just sort on the underlying single-valued dv directly.
		// regardless of selector type, this optimization is safe!
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

// WrapSortedSet is the package-level entry point for the static
// SortedSetSelector.wrap(SortedSetDocValues, Type).
func WrapSortedSet(sortedSet index.SortedSetDocValues, selector SortedSetSelectorType) index.SortedDocValues {
	s := &SortedSetSelector{}
	return s.Wrap(sortedSet, selector)
}

// minValue wraps a SortedSetDocValues and returns the first ordinal (min).
//
// Mirrors the static nested class SortedSetSelector.MinValue.
type minValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *minValue) DocID() int {
	return m.in.DocID()
}

func (m *minValue) NextDoc() (int, error) {
	if _, err := m.in.NextDoc(); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *minValue) Advance(target int) (int, error) {
	if _, err := m.in.Advance(target); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *minValue) AdvanceExact(target int) (bool, error) {
	ok, err := m.in.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if ok {
		if err := m.setOrd(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (m *minValue) Cost() int64 {
	return m.in.Cost()
}

func (m *minValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *minValue) DocIDRunEnd() (int, error) {
	return m.in.DocIDRunEnd()
}

func (m *minValue) OrdValue() (int, error) {
	return m.ord, nil
}

func (m *minValue) LongValue() (int64, error) {
	return int64(m.ord), nil
}

func (m *minValue) LookupOrd(ord int) ([]byte, error) {
	return m.in.LookupOrd(ord)
}

func (m *minValue) GetValueCount() int {
	return m.in.GetValueCount()
}

// LookupTerm mirrors SortedSetSelector.MinValue#lookupTerm in Apache
// Lucene 10.5.0, which forwards to the wrapped SortedSetDocValues. Lucene
// renders SortedSetDocValues#lookupTerm as the free function
// index.LookupSetTerm, whose body is the binary search of the Java base class.
func (m *minValue) LookupTerm(key *util.BytesRef) (int, error) {
	return index.LookupSetTerm(m.in, key)
}

func (m *minValue) setOrd() error {
	if m.DocID() != NO_MORE_DOCS {
		ord, err := m.in.NextOrd()
		if err != nil {
			return err
		}
		m.ord = ord
	}
	return nil
}

// maxValue wraps a SortedSetDocValues and returns the last ordinal (max).
//
// Mirrors the static nested class SortedSetSelector.MaxValue.
type maxValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *maxValue) DocID() int {
	return m.in.DocID()
}

func (m *maxValue) NextDoc() (int, error) {
	if _, err := m.in.NextDoc(); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *maxValue) Advance(target int) (int, error) {
	if _, err := m.in.Advance(target); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *maxValue) AdvanceExact(target int) (bool, error) {
	ok, err := m.in.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if ok {
		if err := m.setOrd(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (m *maxValue) Cost() int64 {
	return m.in.Cost()
}

func (m *maxValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *maxValue) DocIDRunEnd() (int, error) {
	return m.in.DocIDRunEnd()
}

func (m *maxValue) OrdValue() (int, error) {
	return m.ord, nil
}

func (m *maxValue) LongValue() (int64, error) {
	return int64(m.ord), nil
}

func (m *maxValue) LookupOrd(ord int) ([]byte, error) {
	return m.in.LookupOrd(ord)
}

func (m *maxValue) GetValueCount() int {
	return m.in.GetValueCount()
}

// LookupTerm mirrors SortedSetSelector.MaxValue#lookupTerm in Apache
// Lucene 10.5.0, which forwards to the wrapped SortedSetDocValues. Lucene
// renders SortedSetDocValues#lookupTerm as the free function
// index.LookupSetTerm, whose body is the binary search of the Java base class.
func (m *maxValue) LookupTerm(key *util.BytesRef) (int, error) {
	return index.LookupSetTerm(m.in, key)
}

func (m *maxValue) setOrd() error {
	if m.DocID() != NO_MORE_DOCS {
		docValueCount := m.in.DocValueCount()
		for i := 0; i < docValueCount-1; i++ {
			if _, err := m.in.NextOrd(); err != nil {
				return err
			}
		}
		ord, err := m.in.NextOrd()
		if err != nil {
			return err
		}
		m.ord = ord
	}
	return nil
}

// middleMinValue wraps a SortedSetDocValues and returns the middle ordinal
// (or the lower of the two middle ordinals for an even number of values).
//
// Mirrors the static nested class SortedSetSelector.MiddleMinValue.
type middleMinValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *middleMinValue) DocID() int {
	return m.in.DocID()
}

func (m *middleMinValue) NextDoc() (int, error) {
	if _, err := m.in.NextDoc(); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *middleMinValue) Advance(target int) (int, error) {
	if _, err := m.in.Advance(target); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *middleMinValue) AdvanceExact(target int) (bool, error) {
	ok, err := m.in.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if ok {
		if err := m.setOrd(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (m *middleMinValue) Cost() int64 {
	return m.in.Cost()
}

func (m *middleMinValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *middleMinValue) DocIDRunEnd() (int, error) {
	return m.in.DocIDRunEnd()
}

func (m *middleMinValue) OrdValue() (int, error) {
	return m.ord, nil
}

func (m *middleMinValue) LongValue() (int64, error) {
	return int64(m.ord), nil
}

func (m *middleMinValue) LookupOrd(ord int) ([]byte, error) {
	return m.in.LookupOrd(ord)
}

func (m *middleMinValue) GetValueCount() int {
	return m.in.GetValueCount()
}

// LookupTerm mirrors SortedSetSelector.MiddleMinValue#lookupTerm in Apache
// Lucene 10.5.0, which forwards to the wrapped SortedSetDocValues. Lucene
// renders SortedSetDocValues#lookupTerm as the free function
// index.LookupSetTerm, whose body is the binary search of the Java base class.
func (m *middleMinValue) LookupTerm(key *util.BytesRef) (int, error) {
	return index.LookupSetTerm(m.in, key)
}

func (m *middleMinValue) setOrd() error {
	if m.DocID() != NO_MORE_DOCS {
		targetIdx := (m.in.DocValueCount() - 1) >> 1
		for i := 0; i < targetIdx; i++ {
			if _, err := m.in.NextOrd(); err != nil {
				return err
			}
		}
		ord, err := m.in.NextOrd()
		if err != nil {
			return err
		}
		m.ord = ord
	}
	return nil
}

// middleMaxValue wraps a SortedSetDocValues and returns the middle ordinal
// (or the higher of the two middle ordinals for an even number of values).
//
// Mirrors the static nested class SortedSetSelector.MiddleMaxValue.
type middleMaxValue struct {
	in  index.SortedSetDocValues
	ord int
}

func (m *middleMaxValue) DocID() int {
	return m.in.DocID()
}

func (m *middleMaxValue) NextDoc() (int, error) {
	if _, err := m.in.NextDoc(); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *middleMaxValue) Advance(target int) (int, error) {
	if _, err := m.in.Advance(target); err != nil {
		return 0, err
	}
	if err := m.setOrd(); err != nil {
		return 0, err
	}
	return m.DocID(), nil
}

func (m *middleMaxValue) AdvanceExact(target int) (bool, error) {
	ok, err := m.in.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if ok {
		if err := m.setOrd(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (m *middleMaxValue) Cost() int64 {
	return m.in.Cost()
}

func (m *middleMaxValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return m.in.IntoBitSet(upTo, bitSet, offset)
}

func (m *middleMaxValue) DocIDRunEnd() (int, error) {
	return m.in.DocIDRunEnd()
}

func (m *middleMaxValue) OrdValue() (int, error) {
	return m.ord, nil
}

func (m *middleMaxValue) LongValue() (int64, error) {
	return int64(m.ord), nil
}

func (m *middleMaxValue) LookupOrd(ord int) ([]byte, error) {
	return m.in.LookupOrd(ord)
}

func (m *middleMaxValue) GetValueCount() int {
	return m.in.GetValueCount()
}

// LookupTerm mirrors SortedSetSelector.MiddleMaxValue#lookupTerm in Apache
// Lucene 10.5.0, which forwards to the wrapped SortedSetDocValues. Lucene
// renders SortedSetDocValues#lookupTerm as the free function
// index.LookupSetTerm, whose body is the binary search of the Java base class.
func (m *middleMaxValue) LookupTerm(key *util.BytesRef) (int, error) {
	return index.LookupSetTerm(m.in, key)
}

func (m *middleMaxValue) setOrd() error {
	if m.DocID() != NO_MORE_DOCS {
		targetIdx := m.in.DocValueCount() >> 1
		for i := 0; i < targetIdx; i++ {
			if _, err := m.in.NextOrd(); err != nil {
				return err
			}
		}
		ord, err := m.in.NextOrd()
		if err != nil {
			return err
		}
		m.ord = ord
	}
	return nil
}
