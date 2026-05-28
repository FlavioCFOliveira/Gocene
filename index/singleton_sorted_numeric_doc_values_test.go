// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"reflect"
	"testing"
)

// fakeNumericDV is a tiny in-memory NumericDocValues used only to exercise
// the singletonSortedNumeric adapter. It exposes a controllable starting
// docID so we can drive the "iterator already used" error path of
// GetNumericDocValues without depending on a concrete codec implementation.
type fakeNumericDV struct {
	values map[int]int64
	docs   []int
	pos    int
	docID  int
}

func newFakeNumericDV(values map[int]int64) *fakeNumericDV {
	docs := make([]int, 0, len(values))
	for d := range values {
		docs = append(docs, d)
	}
	// Insertion sort keeps the dependency surface to "stdlib testing only".
	for i := 1; i < len(docs); i++ {
		for j := i; j > 0 && docs[j-1] > docs[j]; j-- {
			docs[j-1], docs[j] = docs[j], docs[j-1]
		}
	}
	return &fakeNumericDV{values: values, docs: docs, pos: -1, docID: -1}
}

func (f *fakeNumericDV) Cost() int64 { return int64(len(f.docs)) }

func (f *fakeNumericDV) Advance(target int) (int, error) {
	for f.pos+1 < len(f.docs) {
		f.pos++
		if f.docs[f.pos] >= target {
			f.docID = f.docs[f.pos]
			return f.docID, nil
		}
	}
	f.docID = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

func (f *fakeNumericDV) NextDoc() (int, error) {
	f.pos++
	if f.pos >= len(f.docs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docs[f.pos]
	return f.docID, nil
}

func (f *fakeNumericDV) AdvanceExact(target int) (bool, error) {
	got, err := f.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (f *fakeNumericDV) LongValue() (int64, error) {
	return f.values[f.docID], nil
}

func (f *fakeNumericDV) DocID() int { return f.docID }

func TestSingletonSortedNumeric_DelegatesIterationAndGet(t *testing.T) {
	t.Parallel()
	in := newFakeNumericDV(map[int]int64{1: 10, 3: 30, 7: 70})
	dv := Singleton(in)
	if dv == nil {
		t.Fatal("Singleton returned nil for non-nil input")
	}

	if got := dv.DocID(); got != -1 {
		t.Fatalf("initial DocID = %d, want -1", got)
	}

	want := []struct {
		docID int
		val   []int64
	}{
		{1, []int64{10}},
		{3, []int64{30}},
		{7, []int64{70}},
	}
	for i, w := range want {
		got, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("step %d NextDoc: %v", i, err)
		}
		if got != w.docID {
			t.Fatalf("step %d NextDoc = %d, want %d", i, got, w.docID)
		}
		vals, err := CollectSortedNumericValues(dv)
		if err != nil {
			t.Fatalf("step %d CollectSortedNumericValues: %v", i, err)
		}
		_ = got
		if !reflect.DeepEqual(vals, w.val) {
			t.Fatalf("step %d values = %v, want %v", i, vals, w.val)
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

func TestSingletonSortedNumeric_AdvanceDelegates(t *testing.T) {
	t.Parallel()
	in := newFakeNumericDV(map[int]int64{2: 20, 5: 50, 9: 90})
	dv := Singleton(in)

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
}

func TestSingleton_NilReturnsEmpty(t *testing.T) {
	t.Parallel()
	dv := Singleton(nil)
	got, err := dv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc on empty: %v", err)
	}
	if got != NO_MORE_DOCS {
		t.Fatalf("empty NextDoc = %d, want NO_MORE_DOCS", got)
	}
}

func TestUnwrapSingletonSortedNumeric(t *testing.T) {
	t.Parallel()
	in := newFakeNumericDV(map[int]int64{0: 1})
	dv := Singleton(in)

	// Compare via underlying concrete type since UnwrapSingletonSortedNumeric
	// returns the interface NumericDocValues, not *fakeNumericDV.
	if got := UnwrapSingletonSortedNumeric(dv); got != NumericDocValues(in) {
		t.Fatalf("UnwrapSingletonSortedNumeric returned %v, want wrapped iterator", got)
	}
	if got := UnwrapSingletonSortedNumeric(EmptySortedNumeric()); got != nil {
		t.Fatalf("UnwrapSingletonSortedNumeric on empty = %v, want nil", got)
	}
}

func TestSingletonSortedNumeric_GetNumericDocValues(t *testing.T) {
	t.Parallel()
	in := newFakeNumericDV(map[int]int64{1: 11})
	dv := Singleton(in).(*singletonSortedNumeric)

	got, err := dv.GetNumericDocValues()
	if err != nil {
		t.Fatalf("GetNumericDocValues on pristine: %v", err)
	}
	if got != NumericDocValues(in) {
		t.Fatal("GetNumericDocValues did not return the wrapped iterator")
	}

	// Drive the iterator forward so docID() != -1 and the second call must
	// reject. Mirrors Lucene's IllegalStateException contract.
	if _, err := dv.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	_, err = dv.GetNumericDocValues()
	if err == nil {
		t.Fatal("GetNumericDocValues after use returned nil error, want IllegalStateError")
	}
	var ise *IllegalStateError
	if !errors.As(err, &ise) {
		t.Fatalf("error type = %T, want *IllegalStateError", err)
	}
	if ise.DocID != 1 {
		t.Fatalf("IllegalStateError.DocID = %d, want 1", ise.DocID)
	}
}

func TestIllegalStateError_Message(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  *IllegalStateError
		want string
	}{
		{&IllegalStateError{Op: "X", DocID: 0}, "X: iterator has already been used: docID=0"},
		{&IllegalStateError{Op: "X", DocID: 42}, "X: iterator has already been used: docID=42"},
		{&IllegalStateError{Op: "X", DocID: -7}, "X: iterator has already been used: docID=-7"},
	}
	for _, c := range cases {
		if got := c.err.Error(); got != c.want {
			t.Errorf("Error() = %q, want %q", got, c.want)
		}
	}
}
