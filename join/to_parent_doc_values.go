// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// accumulator accumulates per-child doc values into a single parent value.
// reset is called on the first child; increment is called on each subsequent
// child in the same parent block.
type accumulator interface {
	reset(childDoc int) error
	increment(childDoc int) error
}

// toParentDocValues is a DocIdSetIterator that advances over parent documents
// whose child block contains at least one child with a doc-value entry.
// It drives an accumulator that computes the representative (MIN/MAX) value
// across all children in the block.
//
// Mirrors the inner class ToParentDocValues in
// org.apache.lucene.search.join.ToParentDocValues.
type toParentDocValues struct {
	parents         util.BitSet
	docID           int
	collector       accumulator
	seen            bool
	childWithValues search.DocIdSetIterator
}

func newToParentDocValues(
	children search.DocIdSetIterator,
	parents util.BitSet,
	col accumulator,
) *toParentDocValues {
	return &toParentDocValues{
		parents:         parents,
		docID:           -1,
		collector:       col,
		childWithValues: children,
	}
}

// DocID returns the current document ID.
func (t *toParentDocValues) DocID() int { return t.docID }

// NextDoc advances to the next parent document that has at least one child
// with a value.
func (t *toParentDocValues) NextDoc() (int, error) {
	if t.docID == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS, nil
	}

	// Advance child cursor past the current parent if needed.
	if t.childWithValues.DocID() < t.docID || t.docID == -1 {
		if _, err := t.childWithValues.NextDoc(); err != nil {
			return search.NO_MORE_DOCS, err
		}
	}
	if t.childWithValues.DocID() == search.NO_MORE_DOCS {
		t.docID = search.NO_MORE_DOCS
		return t.docID, nil
	}

	// Find the parent of the current child.
	nextParentDocID := t.parents.NextSetBitBounded(t.childWithValues.DocID())
	if nextParentDocID == search.NO_MORE_DOCS {
		t.docID = search.NO_MORE_DOCS
		return t.docID, nil
	}

	// Accumulate the first child's value.
	if err := t.collector.reset(t.childWithValues.DocID()); err != nil {
		return search.NO_MORE_DOCS, err
	}
	t.seen = true

	// Accumulate remaining children in the same block.
	for {
		childDocID, err := t.childWithValues.NextDoc()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if childDocID > nextParentDocID {
			break
		}
		if err := t.collector.increment(childDocID); err != nil {
			return search.NO_MORE_DOCS, err
		}
	}

	t.docID = nextParentDocID
	return t.docID, nil
}

// Advance advances to the first parent document >= target that has children
// with values.
func (t *toParentDocValues) Advance(target int) (int, error) {
	if target >= t.parents.Length() {
		t.docID = search.NO_MORE_DOCS
		return t.docID, nil
	}
	if target == 0 {
		return t.NextDoc()
	}
	prevParentDocID := t.parents.PrevSetBit(target - 1)
	if prevParentDocID >= 0 && t.childWithValues.DocID() <= prevParentDocID {
		if _, err := t.childWithValues.Advance(prevParentDocID + 1); err != nil {
			return search.NO_MORE_DOCS, err
		}
	}
	return t.NextDoc()
}

// AdvanceExact advances to exactly targetParentDocID and reports whether
// any child in its block has a value.
func (t *toParentDocValues) AdvanceExact(targetParentDocID int) (bool, error) {
	if targetParentDocID < t.docID {
		return false, nil
	}
	previousDocID := t.docID
	t.docID = targetParentDocID
	if targetParentDocID == previousDocID {
		return t.seen, nil
	}
	t.seen = false

	if !t.parents.Get(targetParentDocID) {
		return false, nil
	}

	var prevParentDocID int
	if t.docID == 0 {
		prevParentDocID = -1
	} else {
		prevParentDocID = t.parents.PrevSetBit(t.docID - 1)
	}

	childDoc := t.childWithValues.DocID()
	if childDoc <= prevParentDocID {
		var err error
		childDoc, err = t.childWithValues.Advance(prevParentDocID + 1)
		if err != nil {
			return false, err
		}
	}
	if childDoc >= t.docID {
		return false, nil
	}

	if t.childWithValues.DocID() < t.docID {
		if err := t.collector.reset(t.childWithValues.DocID()); err != nil {
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

	for doc := t.childWithValues.DocID(); doc < t.docID; {
		if err := t.collector.increment(doc); err != nil {
			return false, err
		}
		next, err := t.childWithValues.NextDoc()
		if err != nil {
			return false, err
		}
		doc = next
	}
	return true, nil
}

// Cost implements DocIdSetIterator.
func (t *toParentDocValues) Cost() int64 { return 0 }

// DocIDRunEnd implements DocIdSetIterator (Gocene extension).
func (t *toParentDocValues) DocIDRunEnd() int { return t.docID + 1 }

// ── SortedDVsAccumulator ─────────────────────────────────────────────────────

// sortedDVsAccumulator accumulates MIN or MAX ordinal across children, then
// exposes the result through the SortedDocValues interface.
type sortedDVsAccumulator struct {
	values    index.SortedDocValues
	selection BlockJoinSelectorType
	ord       int
	iter      *toParentDocValues
}

func newSortedDVsAccumulator(
	values index.SortedDocValues,
	selection BlockJoinSelectorType,
	parents util.BitSet,
	children search.DocIdSetIterator,
) *sortedDVsAccumulator {
	a := &sortedDVsAccumulator{values: values, selection: selection, ord: -1}
	a.iter = newToParentDocValues(children, parents, a)
	return a
}

func (a *sortedDVsAccumulator) reset(childDoc int) error {
	ord, err := a.values.GetOrd(childDoc)
	if err != nil {
		return err
	}
	a.ord = ord
	return nil
}

func (a *sortedDVsAccumulator) increment(childDoc int) error {
	ord, err := a.values.GetOrd(childDoc)
	if err != nil {
		return err
	}
	switch a.selection {
	case BlockJoinMin:
		if ord < a.ord {
			a.ord = ord
		}
	case BlockJoinMax:
		if ord > a.ord {
			a.ord = ord
		}
	}
	return nil
}

// ── SortedDocValues facade ────────────────────────────────────────────────────

// sortedDVsWrapper wraps sortedDVsAccumulator and exposes index.SortedDocValues.
type sortedDVsWrapper struct {
	acc *sortedDVsAccumulator
}

func (w *sortedDVsWrapper) DocID() int { return w.acc.iter.DocID() }

func (w *sortedDVsWrapper) NextDoc() (int, error) { return w.acc.iter.NextDoc() }

func (w *sortedDVsWrapper) Advance(target int) (int, error) { return w.acc.iter.Advance(target) }

func (w *sortedDVsWrapper) Get(docID int) ([]byte, error) {
	ord := w.acc.ord
	if ord < 0 {
		return nil, nil
	}
	return w.acc.values.LookupOrd(ord)
}

func (w *sortedDVsWrapper) GetOrd(docID int) (int, error) { return w.acc.ord, nil }

func (w *sortedDVsWrapper) LookupOrd(ord int) ([]byte, error) {
	return w.acc.values.LookupOrd(ord)
}

func (w *sortedDVsWrapper) GetValueCount() int { return w.acc.values.GetValueCount() }

// ── NumericDVAccumulator ─────────────────────────────────────────────────────

// numericDVsAccumulator accumulates MIN or MAX long value across children, then
// exposes the result through the NumericDocValues interface.
type numericDVsAccumulator struct {
	values    index.NumericDocValues
	selection BlockJoinSelectorType
	value     int64
	iter      *toParentDocValues
}

func newNumericDVsAccumulator(
	values index.NumericDocValues,
	selection BlockJoinSelectorType,
	parents util.BitSet,
	children search.DocIdSetIterator,
) *numericDVsAccumulator {
	a := &numericDVsAccumulator{values: values, selection: selection}
	a.iter = newToParentDocValues(children, parents, a)
	return a
}

func (a *numericDVsAccumulator) reset(childDoc int) error {
	v, err := a.values.Get(childDoc)
	if err != nil {
		return err
	}
	a.value = v
	return nil
}

func (a *numericDVsAccumulator) increment(childDoc int) error {
	v, err := a.values.Get(childDoc)
	if err != nil {
		return err
	}
	switch a.selection {
	case BlockJoinMin:
		if v < a.value {
			a.value = v
		}
	case BlockJoinMax:
		if v > a.value {
			a.value = v
		}
	}
	return nil
}

// ── NumericDocValues facade ───────────────────────────────────────────────────

// numericDVsWrapper wraps numericDVsAccumulator and exposes index.NumericDocValues.
type numericDVsWrapper struct {
	acc *numericDVsAccumulator
}

func (w *numericDVsWrapper) DocID() int { return w.acc.iter.DocID() }

func (w *numericDVsWrapper) NextDoc() (int, error) { return w.acc.iter.NextDoc() }

func (w *numericDVsWrapper) Advance(target int) (int, error) { return w.acc.iter.Advance(target) }

func (w *numericDVsWrapper) Get(docID int) (int64, error) { return w.acc.value, nil }

// ── Public factory functions ─────────────────────────────────────────────────

// WrapSortedDocValues creates a SortedDocValues that iterates over parent
// documents and reports the MIN or MAX ordinal across all children in each
// block that have a value.
//
// Mirrors ToParentDocValues.wrap(SortedDocValues, Type, BitSet,
// DocIdSetIterator).
func WrapSortedDocValues(
	values index.SortedDocValues,
	selection BlockJoinSelectorType,
	parents util.BitSet,
	children search.DocIdSetIterator,
) index.SortedDocValues {
	acc := newSortedDVsAccumulator(values, selection, parents, children)
	return &sortedDVsWrapper{acc: acc}
}

// WrapNumericDocValues creates a NumericDocValues that iterates over parent
// documents and reports the MIN or MAX long value across all children in each
// block that have a value.
//
// Mirrors ToParentDocValues.wrap(NumericDocValues, Type, BitSet,
// DocIdSetIterator).
func WrapNumericDocValues(
	values index.NumericDocValues,
	selection BlockJoinSelectorType,
	parents util.BitSet,
	children search.DocIdSetIterator,
) index.NumericDocValues {
	acc := newNumericDVsAccumulator(values, selection, parents, children)
	return &numericDVsWrapper{acc: acc}
}

// interface compliance
var _ index.SortedDocValues = (*sortedDVsWrapper)(nil)
var _ index.NumericDocValues = (*numericDVsWrapper)(nil)
