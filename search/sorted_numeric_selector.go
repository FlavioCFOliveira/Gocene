// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortedNumericSelectorType picks one value from a multi-valued numeric
// document-values field for sorting.
//
// Mirrors org.apache.lucene.search.SortedNumericSelector.Type.
type SortedNumericSelectorType int

const (
	// SortedNumericSelectorMin picks the smallest value in the set.
	SortedNumericSelectorMin SortedNumericSelectorType = iota
	// SortedNumericSelectorMax picks the largest value in the set.
	SortedNumericSelectorMax
)

// String returns the canonical name of the selector type.
func (t SortedNumericSelectorType) String() string {
	switch t {
	case SortedNumericSelectorMin:
		return "MIN"
	case SortedNumericSelectorMax:
		return "MAX"
	default:
		return "UNKNOWN"
	}
}

// WrapSortedNumeric wraps a multi-valued SortedNumericDocValues as a
// single-valued view, using the specified selector and numericType.
//
// This is the Go port of
// org.apache.lucene.search.SortedNumericSelector#wrap(SortedNumericDocValues,
// Type, SortField.Type).
func WrapSortedNumeric(sortedNumeric index.SortedNumericDocValues, selector SortedNumericSelectorType, numericType SortFieldType) (index.NumericDocValues, error) {
	if numericType != spi.SortFieldTypeInt &&
		numericType != spi.SortFieldTypeLong &&
		numericType != spi.SortFieldTypeFloat &&
		numericType != spi.SortFieldTypeDouble {
		return nil, fmt.Errorf("numericType must be a numeric type")
	}

	var view index.NumericDocValues
	if singleton := index.UnwrapSingletonSortedNumeric(sortedNumeric); singleton != nil {
		// It's actually single-valued in practice, but indexed as multi-valued,
		// so just sort on the underlying single-valued dv directly. Regardless
		// of selector type, this optimization is safe.
		view = singleton
	} else {
		switch selector {
		case SortedNumericSelectorMin:
			view = &sortedNumericMinValue{in: sortedNumeric}
		case SortedNumericSelectorMax:
			view = &sortedNumericMaxValue{in: sortedNumeric}
		default:
			return nil, fmt.Errorf("unknown SortedNumericSelector type: %v", selector)
		}
	}

	// Undo the NumericUtils sortability.
	switch numericType {
	case spi.SortFieldTypeFloat:
		return &sortableFloatBitsDocValues{FilterNumericDocValues: index.NewFilterNumericDocValues(view)}, nil
	case spi.SortFieldTypeDouble:
		return &sortableDoubleBitsDocValues{FilterNumericDocValues: index.NewFilterNumericDocValues(view)}, nil
	default:
		return view, nil
	}
}

// sortableFloatBitsDocValues renders the anonymous FilterNumericDocValues that
// SortedNumericSelector.wrap returns for SortField.Type.FLOAT.
type sortableFloatBitsDocValues struct {
	*index.FilterNumericDocValues
}

func (d *sortableFloatBitsDocValues) LongValue() (int64, error) {
	v, err := d.FilterNumericDocValues.LongValue()
	if err != nil {
		return 0, err
	}
	return int64(util.SortableFloatBits(uint32(int32(v)))), nil
}

// sortableDoubleBitsDocValues renders the anonymous FilterNumericDocValues that
// SortedNumericSelector.wrap returns for SortField.Type.DOUBLE.
type sortableDoubleBitsDocValues struct {
	*index.FilterNumericDocValues
}

func (d *sortableDoubleBitsDocValues) LongValue() (int64, error) {
	v, err := d.FilterNumericDocValues.LongValue()
	if err != nil {
		return 0, err
	}
	return util.SortableDoubleBits(uint64(v)), nil
}

// sortedNumericMinValue wraps a SortedNumericDocValues and returns the first
// value (min). Mirrors the static nested class SortedNumericSelector.MinValue.
type sortedNumericMinValue struct {
	in    index.SortedNumericDocValues
	value int64
}

func (m *sortedNumericMinValue) DocID() int { return m.in.DocID() }

func (m *sortedNumericMinValue) NextDoc() (int, error) {
	docID, err := m.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if docID != NO_MORE_DOCS {
		if m.value, err = m.in.NextValue(); err != nil {
			return 0, err
		}
	}
	return docID, nil
}

func (m *sortedNumericMinValue) Advance(target int) (int, error) {
	docID, err := m.in.Advance(target)
	if err != nil {
		return 0, err
	}
	if docID != NO_MORE_DOCS {
		if m.value, err = m.in.NextValue(); err != nil {
			return 0, err
		}
	}
	return docID, nil
}

func (m *sortedNumericMinValue) AdvanceExact(target int) (bool, error) {
	ok, err := m.in.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if m.value, err = m.in.NextValue(); err != nil {
		return false, err
	}
	return true, nil
}

func (m *sortedNumericMinValue) Cost() int64 { return m.in.Cost() }

func (m *sortedNumericMinValue) LongValue() (int64, error) { return m.value, nil }

func (m *sortedNumericMinValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

func (m *sortedNumericMinValue) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(m) }

// sortedNumericMaxValue wraps a SortedNumericDocValues and returns the last
// value (max). Mirrors the static nested class SortedNumericSelector.MaxValue.
type sortedNumericMaxValue struct {
	in    index.SortedNumericDocValues
	value int64
}

func (m *sortedNumericMaxValue) DocID() int { return m.in.DocID() }

func (m *sortedNumericMaxValue) setValue() error {
	count, err := m.in.DocValueCount()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if m.value, err = m.in.NextValue(); err != nil {
			return err
		}
	}
	return nil
}

func (m *sortedNumericMaxValue) NextDoc() (int, error) {
	docID, err := m.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if docID != NO_MORE_DOCS {
		if err := m.setValue(); err != nil {
			return 0, err
		}
	}
	return docID, nil
}

func (m *sortedNumericMaxValue) Advance(target int) (int, error) {
	docID, err := m.in.Advance(target)
	if err != nil {
		return 0, err
	}
	if docID != NO_MORE_DOCS {
		if err := m.setValue(); err != nil {
			return 0, err
		}
	}
	return docID, nil
}

func (m *sortedNumericMaxValue) AdvanceExact(target int) (bool, error) {
	ok, err := m.in.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if err := m.setValue(); err != nil {
		return false, err
	}
	return true, nil
}

func (m *sortedNumericMaxValue) Cost() int64 { return m.in.Cost() }

func (m *sortedNumericMaxValue) LongValue() (int64, error) { return m.value, nil }

func (m *sortedNumericMaxValue) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

func (m *sortedNumericMaxValue) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(m) }
