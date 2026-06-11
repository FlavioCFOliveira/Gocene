// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// wrapSortedNumericDocValues converts a SortedNumericDocValues into a
// single-valued NumericDocValues by selecting the minimum or maximum value
// per document.
//
// Mirrors org.apache.lucene.search.SortedNumericSelector.wrap.
func wrapSortedNumericDocValues(
	sorted index.SortedNumericDocValues,
	selector search.SortedNumericSelectorType,
	sortType search.SortFieldType,
) index.NumericDocValues {
	return &sortedNumericWrapper{
		sorted:   sorted,
		selector: selector,
		sortType: sortType,
	}
}

type sortedNumericWrapper struct {
	sorted   index.SortedNumericDocValues
	selector search.SortedNumericSelectorType
	sortType search.SortFieldType
	value    int64
}

func (w *sortedNumericWrapper) DocID() int { return w.sorted.DocID() }

func (w *sortedNumericWrapper) NextDoc() (int, error) {
	doc, err := w.sorted.NextDoc()
	if err != nil || doc == index.NO_MORE_DOCS {
		return doc, err
	}
	w.pickValue()
	return doc, nil
}

func (w *sortedNumericWrapper) Advance(target int) (int, error) {
	doc, err := w.sorted.Advance(target)
	if err != nil || doc == index.NO_MORE_DOCS {
		return doc, err
	}
	w.pickValue()
	return doc, nil
}

func (w *sortedNumericWrapper) AdvanceExact(target int) (bool, error) {
	ok, err := w.sorted.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	w.pickValue()
	return true, nil
}

func (w *sortedNumericWrapper) LongValue() (int64, error) {
	return w.value, nil
}

func (w *sortedNumericWrapper) Cost() int64 { return w.sorted.Cost() }

func (w *sortedNumericWrapper) pickValue() {
	count, err := w.sorted.DocValueCount()
	if err != nil || count == 0 {
		w.value = 0
		return
	}
	first, err := w.sorted.NextValue()
	if err != nil {
		w.value = 0
		return
	}
	if w.selector == search.SortedNumericSelectorMin {
		selected := first
		selectedBits := sortableToComparisonBits(first, w.sortType)
		for i := 1; i < count; i++ {
			v, err := w.sorted.NextValue()
			if err != nil {
				break
			}
			if sortableToComparisonBits(v, w.sortType) < selectedBits {
				selected = v
				selectedBits = sortableToComparisonBits(v, w.sortType)
			}
		}
		w.value = selected
	} else {
		selected := first
		selectedBits := sortableToComparisonBits(first, w.sortType)
		for i := 1; i < count; i++ {
			v, err := w.sorted.NextValue()
			if err != nil {
				break
			}
			if sortableToComparisonBits(v, w.sortType) > selectedBits {
				selected = v
				selectedBits = sortableToComparisonBits(v, w.sortType)
			}
		}
		w.value = selected
	}
}

// sortableToComparisonBits converts an int64 from Lucene's sortable storage
// to a uint64 suitable for unsigned comparison. For INT and LONG, the
// signed bit is flipped so negative values sort before positive ones.
func sortableToComparisonBits(v int64, sortType search.SortFieldType) uint64 {
	switch sortType {
	case search.SortFieldTypeInt, search.SortFieldTypeLong:
		return uint64(v) ^ (uint64(1) << 63)
	default:
		return uint64(v)
	}
}

// floatBitsToFloat interprets v as an IEEE-754 float32 stored as sortable
// int32 bits in the lower 32 bits of v.
func floatBitsToFloat(v int64) float32 {
	return math.Float32frombits(uint32(v))
}

// doubleBitsToDouble interprets v as an IEEE-754 float64 stored as sortable
// int64 bits.
func doubleBitsToDouble(v int64) float64 {
	return math.Float64frombits(uint64(v))
}
