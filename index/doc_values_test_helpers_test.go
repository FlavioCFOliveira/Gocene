// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

func TestEmptyDocValues_AllIteratorsAtNoMoreDocs(t *testing.T) {
	bin := EmptyBinary()
	if d, _ := bin.NextDoc(); d != NO_MORE_DOCS {
		t.Errorf("EmptyBinary.NextDoc=%d, want NO_MORE_DOCS", d)
	}
	num := EmptyNumeric()
	if d, _ := num.NextDoc(); d != NO_MORE_DOCS {
		t.Errorf("EmptyNumeric.NextDoc=%d", d)
	}
	sorted := EmptySorted()
	if d, _ := sorted.NextDoc(); d != NO_MORE_DOCS {
		t.Errorf("EmptySorted.NextDoc=%d", d)
	}
	if c := sorted.GetValueCount(); c != 0 {
		t.Errorf("EmptySorted.GetValueCount=%d", c)
	}
	sn := EmptySortedNumeric()
	if d, _ := sn.NextDoc(); d != NO_MORE_DOCS {
		t.Errorf("EmptySortedNumeric.NextDoc=%d", d)
	}
	ss := EmptySortedSet()
	if d, _ := ss.NextDoc(); d != NO_MORE_DOCS {
		t.Errorf("EmptySortedSet.NextDoc=%d", d)
	}
	if c := ss.GetValueCount(); c != 0 {
		t.Errorf("EmptySortedSet.GetValueCount=%d", c)
	}
}

// stubNumericDV is a single-shot NumericDocValues used for Singleton tests.
// Exposes the iterator-shaped surface mandated by rmp #4710 (no legacy
// Get(docID)).
type stubNumericDV struct {
	docID int
	val   int64
}

func (s *stubNumericDV) Advance(target int) (int, error) {
	s.docID = target
	return target, nil
}
func (s *stubNumericDV) AdvanceExact(target int) (bool, error) {
	s.docID = target
	return true, nil
}
func (s *stubNumericDV) LongValue() (int64, error) { return s.val, nil }
func (s *stubNumericDV) NextDoc() (int, error)     { s.docID++; return s.docID, nil }
func (s *stubNumericDV) DocID() int                { return s.docID }
func (s *stubNumericDV) Cost() int64               { return 1 }

func TestSingleton_NumericToSortedNumeric(t *testing.T) {
	src := &stubNumericDV{val: 42}
	wrapped := Singleton(src)
	// Position the iterator on doc 0 and drain its values via the
	// iterator-shaped helper.
	got, err := DrainSortedNumeric(wrapped, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 42 {
		t.Errorf("DrainSortedNumeric=%v, want [42]", got)
	}
	// UnwrapSingletonSortedNumeric returns the underlying NumericDocValues
	// (typed as the interface, not the concrete *stubNumericDV).
	unwrapped := UnwrapSingletonSortedNumeric(wrapped)
	if unwrapped == nil {
		t.Errorf("UnwrapSingletonSortedNumeric returned nil")
	}
	if _, ok := unwrapped.(*stubNumericDV); !ok {
		t.Errorf("UnwrapSingletonSortedNumeric returned %T, want *stubNumericDV", unwrapped)
	}
	// nil wrap returns empty
	empty := Singleton(nil)
	if d, _ := empty.NextDoc(); d != NO_MORE_DOCS {
		t.Errorf("Singleton(nil) should be empty iterator, got %d", d)
	}
}

// stubSortedDV is a single-ord SortedDocValues for SingletonSortedSet tests.
type stubSortedDV struct {
	docID int
	ord   int
	value []byte
}

func (s *stubSortedDV) Advance(t int) (int, error)       { s.docID = t; return t, nil }
func (s *stubSortedDV) AdvanceExact(t int) (bool, error) { s.docID = t; return true, nil }
func (s *stubSortedDV) BinaryValue() ([]byte, error)     { return s.value, nil }
func (s *stubSortedDV) OrdValue() (int, error)           { return s.ord, nil }
func (s *stubSortedDV) LongValue() (int64, error)        { return int64(s.ord), nil }
func (s *stubSortedDV) NextDoc() (int, error)            { s.docID++; return s.docID, nil }
func (s *stubSortedDV) DocID() int                       { return s.docID }
func (s *stubSortedDV) LookupOrd(int) ([]byte, error) {
	return s.value, nil
}
func (s *stubSortedDV) GetValueCount() int { return 1 }
func (s *stubSortedDV) Cost() int64        { return 1 }

func TestSingleton_SortedToSortedSet(t *testing.T) {
	src := &stubSortedDV{ord: 7, value: []byte("foo")}
	wrapped := SingletonSortedSet(src)
	got, err := DrainSortedSet(wrapped, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 7 {
		t.Errorf("DrainSortedSet=%v, want [7]", got)
	}
	val, _ := wrapped.LookupOrd(0)
	if string(val) != "foo" {
		t.Errorf("LookupOrd=%q", val)
	}
	if wrapped.GetValueCount() != 1 {
		t.Errorf("GetValueCount=%d", wrapped.GetValueCount())
	}
	unwrapped := UnwrapSingletonSortedSet(wrapped)
	if unwrapped == nil {
		t.Errorf("UnwrapSingletonSortedSet returned nil")
	}
	if _, ok := unwrapped.(*stubSortedDV); !ok {
		t.Errorf("UnwrapSingletonSortedSet returned %T, want *stubSortedDV", unwrapped)
	}
	// no-singleton check
	if u := UnwrapSingletonSortedSet(EmptySortedSet()); u != nil {
		t.Errorf("UnwrapSingletonSortedSet(empty) should be nil")
	}
}

func TestSingleton_SortedNegativeOrdReturnsNil(t *testing.T) {
	src := &stubSortedDV{ord: -1, value: nil}
	wrapped := SingletonSortedSet(src)
	got, err := DrainSortedSet(wrapped, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("DrainSortedSet returned %v for missing ord, want nil", got)
	}
}
