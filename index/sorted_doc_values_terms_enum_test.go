// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"errors"
	"testing"
)

// stubSortedDocValues is a minimal in-memory SortedDocValues used by the
// SortedDocValuesTermsEnum tests. The doc-iterator side of the contract
// (Get/Advance/NextDoc/DocID) is irrelevant here, so those methods are
// no-op stubs.
type stubSortedDocValues struct {
	terms [][]byte // sorted, unique
}

func (s *stubSortedDocValues) Get(int) ([]byte, error)         { return nil, nil }
func (s *stubSortedDocValues) Advance(int) (int, error)        { return -1, nil }
func (s *stubSortedDocValues) AdvanceExact(int) (bool, error)  { return false, nil }
func (s *stubSortedDocValues) BinaryValue() ([]byte, error)    { return nil, nil }
func (s *stubSortedDocValues) OrdValue() (int, error)          { return -1, nil }
func (s *stubSortedDocValues) NextDoc() (int, error)           { return -1, nil }
func (s *stubSortedDocValues) DocID() int                      { return -1 }
func (s *stubSortedDocValues) GetOrd(int) (int, error)         { return -1, nil }

func (s *stubSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(s.terms) {
		return nil, errors.New("stubSortedDocValues: ord out of range")
	}
	return s.terms[ord], nil
}

func (s *stubSortedDocValues) GetValueCount() int { return len(s.terms) }

func newStubSDV(terms ...string) *stubSortedDocValues {
	out := make([][]byte, len(terms))
	for i, t := range terms {
		out[i] = []byte(t)
	}
	return &stubSortedDocValues{terms: out}
}

func TestSortedDocValuesTermsEnum_NextEnumeratesInOrder(t *testing.T) {
	t.Parallel()
	want := []string{"apple", "banana", "cherry"}
	te := NewSortedDocValuesTermsEnum("f", newStubSDV(want...))

	for i, w := range want {
		got, err := te.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		if got == nil {
			t.Fatalf("Next #%d: nil term", i)
		}
		if got.Field != "f" {
			t.Errorf("Next #%d: field = %q, want %q", i, got.Field, "f")
		}
		if !bytes.Equal(got.Bytes.ValidBytes(), []byte(w)) {
			t.Errorf("Next #%d: bytes = %q, want %q", i, got.Bytes.ValidBytes(), w)
		}
		if te.Ord() != int64(i) {
			t.Errorf("Next #%d: Ord() = %d, want %d", i, te.Ord(), i)
		}
	}

	end, err := te.Next()
	if err != nil {
		t.Fatalf("Next end: %v", err)
	}
	if end != nil {
		t.Errorf("Next at end: got %v, want nil", end)
	}
}

func TestSortedDocValuesTermsEnum_SeekCeil(t *testing.T) {
	t.Parallel()
	te := NewSortedDocValuesTermsEnum("f", newStubSDV("apple", "cherry", "mango"))

	cases := []struct {
		name      string
		seek      string
		wantTerm  string // empty == nil
		wantOrd   int64
		expectNil bool
	}{
		{"exact-first", "apple", "apple", 0, false},
		{"exact-middle", "cherry", "cherry", 1, false},
		{"between", "banana", "cherry", 1, false},
		{"before-first", "aardvark", "apple", 0, false},
		{"past-last", "zebra", "", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := te.SeekCeil(NewTerm("f", tc.seek))
			if err != nil {
				t.Fatalf("SeekCeil(%q): %v", tc.seek, err)
			}
			if tc.expectNil {
				if got != nil {
					t.Fatalf("SeekCeil(%q): got %v, want nil", tc.seek, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("SeekCeil(%q): got nil", tc.seek)
			}
			if string(got.Bytes.ValidBytes()) != tc.wantTerm {
				t.Errorf("SeekCeil(%q): term = %q, want %q", tc.seek, got.Bytes.ValidBytes(), tc.wantTerm)
			}
			if te.Ord() != tc.wantOrd {
				t.Errorf("SeekCeil(%q): Ord() = %d, want %d", tc.seek, te.Ord(), tc.wantOrd)
			}
		})
	}
}

func TestSortedDocValuesTermsEnum_SeekExact(t *testing.T) {
	t.Parallel()
	te := NewSortedDocValuesTermsEnum("f", newStubSDV("apple", "cherry", "mango"))

	found, err := te.SeekExact(NewTerm("f", "cherry"))
	if err != nil {
		t.Fatalf("SeekExact hit: %v", err)
	}
	if !found {
		t.Error("SeekExact(cherry): want true")
	}
	if te.Ord() != 1 {
		t.Errorf("SeekExact(cherry): Ord() = %d, want 1", te.Ord())
	}

	found, err = te.SeekExact(NewTerm("f", "banana"))
	if err != nil {
		t.Fatalf("SeekExact miss: %v", err)
	}
	if found {
		t.Error("SeekExact(banana): want false")
	}
}

func TestSortedDocValuesTermsEnum_SeekExactOrd(t *testing.T) {
	t.Parallel()
	te := NewSortedDocValuesTermsEnum("f", newStubSDV("apple", "cherry", "mango"))

	if err := te.SeekExactOrd(2); err != nil {
		t.Fatalf("SeekExactOrd(2): %v", err)
	}
	if te.Ord() != 2 {
		t.Errorf("Ord() = %d, want 2", te.Ord())
	}
	if got := te.Term(); got == nil || string(got.Bytes.ValidBytes()) != "mango" {
		t.Errorf("Term() = %v, want mango", got)
	}

	if err := te.SeekExactOrd(7); err == nil {
		t.Error("SeekExactOrd(7): want range error, got nil")
	}
	if err := te.SeekExactOrd(-1); err == nil {
		t.Error("SeekExactOrd(-1): want range error, got nil")
	}
}

func TestSortedDocValuesTermsEnum_TermStateRoundtrip(t *testing.T) {
	t.Parallel()
	te := NewSortedDocValuesTermsEnum("f", newStubSDV("apple", "cherry", "mango"))

	if err := te.SeekExactOrd(1); err != nil {
		t.Fatalf("SeekExactOrd(1): %v", err)
	}
	state, err := te.TermState()
	if err != nil {
		t.Fatalf("TermState: %v", err)
	}
	ots, ok := state.(*OrdTermState)
	if !ok {
		t.Fatalf("TermState type = %T, want *OrdTermState", state)
	}
	if ots.Ord != 1 {
		t.Errorf("snapshot.Ord = %d, want 1", ots.Ord)
	}

	// Move away, then restore.
	if err := te.SeekExactOrd(0); err != nil {
		t.Fatalf("SeekExactOrd(0): %v", err)
	}
	if err := te.SeekExactWithTermState(NewTerm("f", "cherry"), state); err != nil {
		t.Fatalf("SeekExactWithTermState: %v", err)
	}
	if te.Ord() != 1 {
		t.Errorf("after restore: Ord() = %d, want 1", te.Ord())
	}
	if got := te.Term(); got == nil || string(got.Bytes.ValidBytes()) != "cherry" {
		t.Errorf("after restore: Term() = %v, want cherry", got)
	}
}

func TestSortedDocValuesTermsEnum_UnsupportedOps(t *testing.T) {
	t.Parallel()
	te := NewSortedDocValuesTermsEnum("f", newStubSDV("a"))

	if _, err := te.DocFreq(); !errors.Is(err, ErrUnsupportedSortedDVOp) {
		t.Errorf("DocFreq: err = %v, want ErrUnsupportedSortedDVOp", err)
	}
	if _, err := te.TotalTermFreq(); !errors.Is(err, ErrUnsupportedSortedDVOp) {
		t.Errorf("TotalTermFreq: err = %v, want ErrUnsupportedSortedDVOp", err)
	}
	if _, err := te.Postings(0); !errors.Is(err, ErrUnsupportedSortedDVOp) {
		t.Errorf("Postings: err = %v, want ErrUnsupportedSortedDVOp", err)
	}
	if _, err := te.PostingsWithLiveDocs(nil, 0); !errors.Is(err, ErrUnsupportedSortedDVOp) {
		t.Errorf("PostingsWithLiveDocs: err = %v, want ErrUnsupportedSortedDVOp", err)
	}
}

// Compile-time check: SortedDocValuesTermsEnum satisfies the TermsEnum
// interface defined in terms_enum.go.
var _ TermsEnum = (*SortedDocValuesTermsEnum)(nil)
