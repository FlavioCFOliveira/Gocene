// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// recordingNumericDV captures call counts to verify pass-through.
type recordingNumericDV struct {
	getCalls int
	docID    int
}

func (r *recordingNumericDV) Get(int) (int64, error)   { r.getCalls++; return 99, nil }
func (r *recordingNumericDV) Advance(t int) (int, error) { r.docID = t; return t, nil }
func (r *recordingNumericDV) NextDoc() (int, error)    { r.docID++; return r.docID, nil }
func (r *recordingNumericDV) DocID() int               { return r.docID }

func TestFilterNumericDocValues_PassThrough(t *testing.T) {
	src := &recordingNumericDV{}
	f := NewFilterNumericDocValues(src)
	v, err := f.Get(7)
	if err != nil || v != 99 {
		t.Errorf("Get=(%d,%v), want (99,nil)", v, err)
	}
	if src.getCalls != 1 {
		t.Errorf("Get not delegated: calls=%d", src.getCalls)
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

// recordingSortedSetDV is a minimal SortedSetDocValues stub.
type recordingSortedSetDV struct {
	docID  int
	values []int
}

func (s *recordingSortedSetDV) Get(int) ([]int, error)      { return s.values, nil }
func (s *recordingSortedSetDV) Advance(t int) (int, error)  { s.docID = t; return t, nil }
func (s *recordingSortedSetDV) NextDoc() (int, error)       { s.docID++; return s.docID, nil }
func (s *recordingSortedSetDV) DocID() int                  { return s.docID }
func (s *recordingSortedSetDV) LookupOrd(int) ([]byte, error) { return []byte("v"), nil }
func (s *recordingSortedSetDV) GetValueCount() int           { return len(s.values) }

func TestFilterSortedSetDocValues_PassThrough(t *testing.T) {
	src := &recordingSortedSetDV{values: []int{1, 2, 3}}
	f := NewFilterSortedSetDocValues(src)
	got, _ := f.Get(0)
	if len(got) != 3 {
		t.Errorf("Get returned %v", got)
	}
	if v, _ := f.LookupOrd(0); string(v) != "v" {
		t.Errorf("LookupOrd=%q", v)
	}
	if c := f.GetValueCount(); c != 3 {
		t.Errorf("GetValueCount=%d", c)
	}
}
