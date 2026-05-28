// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// recordingNumericDV captures call counts to verify pass-through.
// Mirrors the iterator surface mandated by rmp #4710 (no legacy
// Get(docID)).
type recordingNumericDV struct {
	advanceCalls int
	docID        int
}

func (r *recordingNumericDV) Advance(t int) (int, error)       { r.docID = t; return t, nil }
func (r *recordingNumericDV) AdvanceExact(t int) (bool, error) { r.advanceCalls++; r.docID = t; return true, nil }
func (r *recordingNumericDV) LongValue() (int64, error)        { return 99, nil }
func (r *recordingNumericDV) NextDoc() (int, error)            { r.docID++; return r.docID, nil }
func (r *recordingNumericDV) DocID() int                       { return r.docID }
func (r *recordingNumericDV) Cost() int64                      { return 0 }

func TestFilterNumericDocValues_PassThrough(t *testing.T) {
	src := &recordingNumericDV{}
	f := NewFilterNumericDocValues(src)
	ok, err := f.AdvanceExact(7)
	if err != nil || !ok {
		t.Errorf("AdvanceExact=(%v,%v), want (true,nil)", ok, err)
	}
	v, err := f.LongValue()
	if err != nil || v != 99 {
		t.Errorf("LongValue=(%d,%v), want (99,nil)", v, err)
	}
	if src.advanceCalls != 1 {
		t.Errorf("AdvanceExact not delegated: calls=%d", src.advanceCalls)
	}
	if d, _ := f.Advance(11); d != 11 || f.DocID() != 11 {
		t.Errorf("Advance/DocID mismatch")
	}
}

func TestFilterNumericDocValues_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil 'in'")
		}
	}()
	_ = NewFilterNumericDocValues(nil)
}

// recordingSortedSetDV is a minimal SortedSetDocValues stub on the
// iterator surface (no legacy Get(docID)).
type recordingSortedSetDV struct {
	docID   int
	ords    []int
	ordIdx  int
	hasDoc  bool
}

func (s *recordingSortedSetDV) Advance(t int) (int, error) { s.docID = t; s.ordIdx = 0; return t, nil }
func (s *recordingSortedSetDV) AdvanceExact(t int) (bool, error) {
	s.docID = t
	s.ordIdx = 0
	s.hasDoc = true
	return true, nil
}
func (s *recordingSortedSetDV) NextOrd() (int, error) {
	if s.ordIdx >= len(s.ords) {
		return -1, nil
	}
	o := s.ords[s.ordIdx]
	s.ordIdx++
	return o, nil
}
func (s *recordingSortedSetDV) NextDoc() (int, error) { s.docID++; s.ordIdx = 0; return s.docID, nil }
func (s *recordingSortedSetDV) DocID() int            { return s.docID }
func (s *recordingSortedSetDV) LookupOrd(int) ([]byte, error) {
	return []byte("v"), nil
}
func (s *recordingSortedSetDV) GetValueCount() int { return len(s.ords) }
func (s *recordingSortedSetDV) Cost() int64        { return int64(len(s.ords)) }

func TestFilterSortedSetDocValues_PassThrough(t *testing.T) {
	src := &recordingSortedSetDV{ords: []int{1, 2, 3}}
	f := NewFilterSortedSetDocValues(src)
	got, err := DrainSortedSet(f, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("DrainSortedSet returned %v", got)
	}
	if v, _ := f.LookupOrd(0); string(v) != "v" {
		t.Errorf("LookupOrd=%q", v)
	}
	if c := f.GetValueCount(); c != 3 {
		t.Errorf("GetValueCount=%d", c)
	}
}
