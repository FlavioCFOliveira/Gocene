// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ── minimal stubs ─────────────────────────────────────────────────────────────

// stubSortedDV is a minimal SortedDocValues that returns a fixed ordinal per doc.
type stubSortedDV struct {
	ords []int // index == docID
}

func (s *stubSortedDV) DocID() int                            { return -1 }
func (s *stubSortedDV) NextDoc() (int, error)                 { return search.NO_MORE_DOCS, nil }
func (s *stubSortedDV) Advance(target int) (int, error)       { return search.NO_MORE_DOCS, nil }
func (s *stubSortedDV) AdvanceExact(target int) (bool, error) { return false, nil }
func (s *stubSortedDV) BinaryValue() ([]byte, error)          { return nil, nil }
func (s *stubSortedDV) OrdValue() (int, error)                { return -1, nil }
func (s *stubSortedDV) Get(docID int) ([]byte, error)         { return nil, nil }
func (s *stubSortedDV) GetOrd(docID int) (int, error) {
	if docID < len(s.ords) {
		return s.ords[docID], nil
	}
	return -1, nil
}
func (s *stubSortedDV) LookupOrd(ord int) ([]byte, error) { return []byte{byte(ord)}, nil }
func (s *stubSortedDV) GetValueCount() int                { return 10 }

// stubNumericDV returns a fixed long per doc.
type stubNumericDV struct {
	vals []int64
}

func (s *stubNumericDV) DocID() int                            { return -1 }
func (s *stubNumericDV) NextDoc() (int, error)                 { return search.NO_MORE_DOCS, nil }
func (s *stubNumericDV) Advance(target int) (int, error)       { return search.NO_MORE_DOCS, nil }
func (s *stubNumericDV) AdvanceExact(target int) (bool, error) { return false, nil }
func (s *stubNumericDV) LongValue() (int64, error)             { return 0, nil }
func (s *stubNumericDV) Get(docID int) (int64, error) {
	if docID < len(s.vals) {
		return s.vals[docID], nil
	}
	return 0, nil
}

// listDISI iterates over a fixed list of doc IDs.
type listDISI struct {
	docs  []int
	pos   int
	docID int
}

func newListDISI(docs []int) *listDISI { return &listDISI{docs: docs, pos: -1, docID: -1} }

func (l *listDISI) DocID() int { return l.docID }
func (l *listDISI) NextDoc() (int, error) {
	l.pos++
	if l.pos >= len(l.docs) {
		l.docID = search.NO_MORE_DOCS
	} else {
		l.docID = l.docs[l.pos]
	}
	return l.docID, nil
}
func (l *listDISI) Advance(target int) (int, error) {
	for l.docID < target {
		if _, err := l.NextDoc(); err != nil {
			return search.NO_MORE_DOCS, err
		}
	}
	return l.docID, nil
}
func (l *listDISI) Cost() int64      { return int64(len(l.docs)) }
func (l *listDISI) DocIDRunEnd() int { return l.docID + 1 }

// ── helpers ───────────────────────────────────────────────────────────────────

// makeBitSet builds a FixedBitSet with the given bit positions set.
func makeBitSet(t *testing.T, numBits int, setBits ...int) util.BitSet {
	t.Helper()
	bs, err := util.NewFixedBitSet(numBits)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for _, b := range setBits {
		bs.Set(b)
	}
	return bs
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestWrapSortedDocValues_MinMax verifies that WrapSortedDocValues correctly
// selects the MIN or MAX ordinal from a child block.
//
// Layout: docs 0,1 are children of parent 2; doc 3 is a child of parent 4.
// ordinals: doc0=3, doc1=1, doc2=N/A(parent), doc3=5, doc4=N/A(parent).
func TestWrapSortedDocValues_Min(t *testing.T) {
	sdv := &stubSortedDV{ords: []int{3, 1, 0, 5, 0}}
	parents := makeBitSet(t, 8, 2, 4)
	children := newListDISI([]int{0, 1, 3})

	wrapped := WrapSortedDocValues(sdv, BlockJoinMin, parents, children)

	doc, err := wrapped.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 2 {
		t.Fatalf("expected parent 2, got %d", doc)
	}
	ord, err := wrapped.GetOrd(doc)
	if err != nil {
		t.Fatalf("GetOrd: %v", err)
	}
	if ord != 1 {
		t.Errorf("MIN ord = %d, want 1 (min of 3,1)", ord)
	}

	doc, err = wrapped.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 4 {
		t.Fatalf("expected parent 4, got %d", doc)
	}
	ord, _ = wrapped.GetOrd(doc)
	if ord != 5 {
		t.Errorf("single child ord = %d, want 5", ord)
	}

	doc, _ = wrapped.NextDoc()
	if doc != search.NO_MORE_DOCS {
		t.Errorf("expected NO_MORE_DOCS, got %d", doc)
	}
}

func TestWrapSortedDocValues_Max(t *testing.T) {
	sdv := &stubSortedDV{ords: []int{3, 1, 0, 5, 0}}
	parents := makeBitSet(t, 8, 2, 4)
	children := newListDISI([]int{0, 1, 3})

	wrapped := WrapSortedDocValues(sdv, BlockJoinMax, parents, children)

	doc, _ := wrapped.NextDoc()
	if doc != 2 {
		t.Fatalf("expected parent 2, got %d", doc)
	}
	ord, _ := wrapped.GetOrd(doc)
	if ord != 3 {
		t.Errorf("MAX ord = %d, want 3 (max of 3,1)", ord)
	}
}

// TestWrapSortedDocValues_LookupOrd verifies that LookupOrd delegates to the
// underlying SortedDocValues.
func TestWrapSortedDocValues_LookupOrd(t *testing.T) {
	sdv := &stubSortedDV{ords: []int{2, 0, 0}}
	parents := makeBitSet(t, 4, 2)
	children := newListDISI([]int{0, 1})

	wrapped := WrapSortedDocValues(sdv, BlockJoinMin, parents, children)
	wrapped.NextDoc() //nolint:errcheck // test

	val, err := wrapped.LookupOrd(2)
	if err != nil {
		t.Fatalf("LookupOrd: %v", err)
	}
	if len(val) == 0 || val[0] != 2 {
		t.Errorf("LookupOrd(2) = %v, want [2]", val)
	}
}

// TestWrapNumericDocValues_Min verifies MIN aggregation for NumericDocValues.
func TestWrapNumericDocValues_Min(t *testing.T) {
	ndv := &stubNumericDV{vals: []int64{10, 3, 0, 7, 0}}
	parents := makeBitSet(t, 8, 2, 4)
	children := newListDISI([]int{0, 1, 3})

	wrapped := WrapNumericDocValues(ndv, BlockJoinMin, parents, children)

	doc, _ := wrapped.NextDoc()
	if doc != 2 {
		t.Fatalf("expected parent 2, got %d", doc)
	}
	val, err := wrapped.Get(doc)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != 3 {
		t.Errorf("MIN val = %d, want 3 (min of 10,3)", val)
	}

	doc, _ = wrapped.NextDoc()
	if doc != 4 {
		t.Fatalf("expected parent 4, got %d", doc)
	}
	val, _ = wrapped.Get(doc)
	if val != 7 {
		t.Errorf("single child val = %d, want 7", val)
	}
}

// TestWrapNumericDocValues_Max verifies MAX aggregation for NumericDocValues.
func TestWrapNumericDocValues_Max(t *testing.T) {
	ndv := &stubNumericDV{vals: []int64{10, 3, 0, 7, 0}}
	parents := makeBitSet(t, 8, 2, 4)
	children := newListDISI([]int{0, 1, 3})

	wrapped := WrapNumericDocValues(ndv, BlockJoinMax, parents, children)

	doc, _ := wrapped.NextDoc()
	if doc != 2 {
		t.Fatalf("expected parent 2, got %d", doc)
	}
	val, _ := wrapped.Get(doc)
	if val != 10 {
		t.Errorf("MAX val = %d, want 10 (max of 10,3)", val)
	}
}

// TestWrapNumericDocValues_EmptyChildren verifies that a children iterator with
// no docs results in NO_MORE_DOCS immediately.
func TestWrapNumericDocValues_EmptyChildren(t *testing.T) {
	ndv := &stubNumericDV{vals: []int64{1}}
	parents := makeBitSet(t, 4, 2)
	children := newListDISI(nil)

	wrapped := WrapNumericDocValues(ndv, BlockJoinMin, parents, children)
	doc, err := wrapped.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("expected NO_MORE_DOCS for empty children, got %d", doc)
	}
}
