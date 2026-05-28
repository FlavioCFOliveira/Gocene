// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"reflect"
	"testing"
)

// fakeSortedDV is an in-memory SortedDocValues used only to exercise the
// singletonSortedSet adapter. It exposes a controllable starting docID so
// we can drive the "iterator already used" error path of
// GetSortedDocValues without depending on a concrete codec implementation.
type fakeSortedDV struct {
	ords    map[int]int    // docID -> ord (negative means "no value")
	terms   map[int][]byte // ord  -> term bytes
	docs    []int
	pos     int
	docID   int
	currOrd int
}

func newFakeSortedDV(ords map[int]int, terms map[int][]byte) *fakeSortedDV {
	docs := make([]int, 0, len(ords))
	for d := range ords {
		docs = append(docs, d)
	}
	for i := 1; i < len(docs); i++ {
		for j := i; j > 0 && docs[j-1] > docs[j]; j-- {
			docs[j-1], docs[j] = docs[j], docs[j-1]
		}
	}
	return &fakeSortedDV{ords: ords, terms: terms, docs: docs, pos: -1, docID: -1, currOrd: -1}
}

func (f *fakeSortedDV) Get(docID int) ([]byte, error) {
	return f.terms[f.ords[docID]], nil
}

func (f *fakeSortedDV) Advance(target int) (int, error) {
	for f.pos+1 < len(f.docs) {
		f.pos++
		if f.docs[f.pos] >= target {
			f.docID = f.docs[f.pos]
			f.currOrd = f.ords[f.docID]
			return f.docID, nil
		}
	}
	f.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

func (f *fakeSortedDV) NextDoc() (int, error) {
	f.pos++
	if f.pos >= len(f.docs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docs[f.pos]
	f.currOrd = f.ords[f.docID]
	return f.docID, nil
}

func (f *fakeSortedDV) DocID() int                { return f.docID }
func (f *fakeSortedDV) GetOrd(_ int) (int, error) { return f.currOrd, nil }
func (f *fakeSortedDV) AdvanceExact(target int) (bool, error) {
	got, err := f.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}
func (f *fakeSortedDV) BinaryValue() ([]byte, error) {
	if f.currOrd < 0 {
		return nil, nil
	}
	return f.terms[f.currOrd], nil
}
func (f *fakeSortedDV) OrdValue() (int, error) { return f.currOrd, nil }
func (f *fakeSortedDV) LookupOrd(o int) ([]byte, error) {
	return f.terms[o], nil
}
func (f *fakeSortedDV) GetValueCount() int { return len(f.terms) }

func TestSingletonSortedSet_DelegatesIterationAndGet(t *testing.T) {
	t.Parallel()
	in := newFakeSortedDV(
		map[int]int{1: 0, 3: 2, 7: 1},
		map[int][]byte{0: []byte("alpha"), 1: []byte("beta"), 2: []byte("gamma")},
	)
	dv := SingletonSortedSet(in)
	if dv == nil {
		t.Fatal("SingletonSortedSet returned nil for non-nil input")
	}
	if got := dv.DocID(); got != -1 {
		t.Fatalf("initial DocID = %d, want -1", got)
	}

	want := []struct {
		docID int
		ords  []int
	}{
		{1, []int{0}},
		{3, []int{2}},
		{7, []int{1}},
	}
	for i, w := range want {
		got, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("step %d NextDoc: %v", i, err)
		}
		if got != w.docID {
			t.Fatalf("step %d NextDoc = %d, want %d", i, got, w.docID)
		}
		ords, err := dv.Get(got)
		if err != nil {
			t.Fatalf("step %d Get: %v", i, err)
		}
		if !reflect.DeepEqual(ords, w.ords) {
			t.Fatalf("step %d Get = %v, want %v", i, ords, w.ords)
		}
	}

	end, err := dv.NextDoc()
	if err != nil {
		t.Fatalf("final NextDoc: %v", err)
	}
	if end != NO_MORE_DOCS {
		t.Fatalf("final NextDoc = %d, want NO_MORE_DOCS", end)
	}
}

func TestSingletonSortedSet_AdvanceDelegates(t *testing.T) {
	t.Parallel()
	in := newFakeSortedDV(
		map[int]int{2: 0, 5: 1, 9: 0},
		map[int][]byte{0: []byte("x"), 1: []byte("y")},
	)
	dv := SingletonSortedSet(in)

	got, err := dv.Advance(4)
	if err != nil {
		t.Fatalf("Advance(4): %v", err)
	}
	if got != 5 {
		t.Fatalf("Advance(4) = %d, want 5", got)
	}
	if dv.DocID() != 5 {
		t.Fatalf("DocID after Advance = %d, want 5", dv.DocID())
	}
	ords, err := dv.Get(5)
	if err != nil {
		t.Fatalf("Get(5): %v", err)
	}
	if !reflect.DeepEqual(ords, []int{1}) {
		t.Fatalf("Get(5) = %v, want [1]", ords)
	}
}

func TestSingletonSortedSet_NegativeOrdReturnsNil(t *testing.T) {
	t.Parallel()
	in := newFakeSortedDV(
		map[int]int{1: -1},
		map[int][]byte{},
	)
	dv := SingletonSortedSet(in)
	if _, err := dv.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	got, err := dv.Get(1)
	if err != nil {
		t.Fatalf("Get(1): %v", err)
	}
	if got != nil {
		t.Fatalf("Get(1) = %v for missing ord, want nil", got)
	}
}

func TestSingletonSortedSet_LookupOrdAndGetValueCount(t *testing.T) {
	t.Parallel()
	in := newFakeSortedDV(
		map[int]int{0: 0},
		map[int][]byte{0: []byte("alpha"), 1: []byte("beta")},
	)
	dv := SingletonSortedSet(in)

	if n := dv.GetValueCount(); n != 2 {
		t.Fatalf("GetValueCount = %d, want 2", n)
	}
	val, err := dv.LookupOrd(1)
	if err != nil {
		t.Fatalf("LookupOrd(1): %v", err)
	}
	if string(val) != "beta" {
		t.Fatalf("LookupOrd(1) = %q, want %q", val, "beta")
	}
}

func TestSingletonSortedSet_NilReturnsEmpty(t *testing.T) {
	t.Parallel()
	dv := SingletonSortedSet(nil)
	got, err := dv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc on empty: %v", err)
	}
	if got != NO_MORE_DOCS {
		t.Fatalf("empty NextDoc = %d, want NO_MORE_DOCS", got)
	}
}

func TestUnwrapSingletonSortedSet_RoundTrips(t *testing.T) {
	t.Parallel()
	in := newFakeSortedDV(
		map[int]int{0: 0},
		map[int][]byte{0: []byte("v")},
	)
	dv := SingletonSortedSet(in)
	if got := UnwrapSingletonSortedSet(dv); got != in {
		t.Fatalf("UnwrapSingletonSortedSet returned %v, want wrapped iterator", got)
	}
	if got := UnwrapSingletonSortedSet(EmptySortedSet()); got != nil {
		t.Fatalf("UnwrapSingletonSortedSet on empty = %v, want nil", got)
	}
}

func TestSingletonSortedSet_GetSortedDocValues(t *testing.T) {
	t.Parallel()
	in := newFakeSortedDV(
		map[int]int{1: 0},
		map[int][]byte{0: []byte("v")},
	)
	dv := SingletonSortedSet(in).(*singletonSortedSet)

	got, err := dv.GetSortedDocValues()
	if err != nil {
		t.Fatalf("GetSortedDocValues on pristine: %v", err)
	}
	if got != in {
		t.Fatal("GetSortedDocValues did not return the wrapped iterator")
	}

	// Drive the iterator forward so docID() != -1 and the second call must
	// reject. Mirrors Lucene's IllegalStateException contract.
	if _, err := dv.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	_, err = dv.GetSortedDocValues()
	if err == nil {
		t.Fatal("GetSortedDocValues after use returned nil error, want IllegalStateError")
	}
	var ise *IllegalStateError
	if !errors.As(err, &ise) {
		t.Fatalf("error type = %T, want *IllegalStateError", err)
	}
	if ise.DocID != 1 {
		t.Fatalf("IllegalStateError.DocID = %d, want 1", ise.DocID)
	}
	if ise.Op != "SingletonSortedSetDocValues.GetSortedDocValues" {
		t.Fatalf("IllegalStateError.Op = %q", ise.Op)
	}
}
