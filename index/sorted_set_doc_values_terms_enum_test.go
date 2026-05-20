// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"errors"
	"testing"
)

// stubSortedSetDocValues is a minimal in-memory SortedSetDocValues used by the
// SortedSetDocValuesTermsEnum tests. The doc-iterator side of the contract
// (Get/Advance/NextDoc/DocID) is irrelevant here, so those methods are
// no-op stubs.
type stubSortedSetDocValues struct {
	terms [][]byte // sorted, unique
}

func (s *stubSortedSetDocValues) Get(int) ([]int, error)   { return nil, nil }
func (s *stubSortedSetDocValues) Advance(int) (int, error) { return -1, nil }
func (s *stubSortedSetDocValues) NextDoc() (int, error)    { return -1, nil }
func (s *stubSortedSetDocValues) DocID() int               { return -1 }

func (s *stubSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(s.terms) {
		return nil, errors.New("stubSortedSetDocValues: ord out of range")
	}
	return s.terms[ord], nil
}

func (s *stubSortedSetDocValues) GetValueCount() int { return len(s.terms) }

func newStubSSDV(terms ...string) *stubSortedSetDocValues {
	out := make([][]byte, len(terms))
	for i, t := range terms {
		out[i] = []byte(t)
	}
	return &stubSortedSetDocValues{terms: out}
}

func TestSortedSetDocValuesTermsEnum_NextEnumeratesInOrder(t *testing.T) {
	t.Parallel()
	want := []string{"apple", "banana", "cherry"}
	te := NewSortedSetDocValuesTermsEnum("f", newStubSSDV(want...))

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

func TestSortedSetDocValuesTermsEnum_SeekCeil(t *testing.T) {
	t.Parallel()
	te := NewSortedSetDocValuesTermsEnum("f", newStubSSDV("apple", "cherry", "mango"))

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

func TestSortedSetDocValuesTermsEnum_SeekExact(t *testing.T) {
	t.Parallel()
	te := NewSortedSetDocValuesTermsEnum("f", newStubSSDV("apple", "cherry", "mango"))

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

func TestSortedSetDocValuesTermsEnum_SeekExactOrd(t *testing.T) {
	t.Parallel()
	te := NewSortedSetDocValuesTermsEnum("f", newStubSSDV("apple", "cherry", "mango"))

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

func TestSortedSetDocValuesTermsEnum_TermStateRoundtrip(t *testing.T) {
	t.Parallel()
	te := NewSortedSetDocValuesTermsEnum("f", newStubSSDV("apple", "cherry", "mango"))

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

func TestSortedSetDocValuesTermsEnum_UnsupportedOps(t *testing.T) {
	t.Parallel()
	te := NewSortedSetDocValuesTermsEnum("f", newStubSSDV("a"))

	if _, err := te.DocFreq(); !errors.Is(err, ErrUnsupportedSortedSetDVOp) {
		t.Errorf("DocFreq: err = %v, want ErrUnsupportedSortedSetDVOp", err)
	}
	if _, err := te.TotalTermFreq(); !errors.Is(err, ErrUnsupportedSortedSetDVOp) {
		t.Errorf("TotalTermFreq: err = %v, want ErrUnsupportedSortedSetDVOp", err)
	}
	if _, err := te.Postings(0); !errors.Is(err, ErrUnsupportedSortedSetDVOp) {
		t.Errorf("Postings: err = %v, want ErrUnsupportedSortedSetDVOp", err)
	}
	if _, err := te.PostingsWithLiveDocs(nil, 0); !errors.Is(err, ErrUnsupportedSortedSetDVOp) {
		t.Errorf("PostingsWithLiveDocs: err = %v, want ErrUnsupportedSortedSetDVOp", err)
	}
}

// Compile-time check: SortedSetDocValuesTermsEnum satisfies the TermsEnum
// interface defined in terms_enum.go.
var _ TermsEnum = (*SortedSetDocValuesTermsEnum)(nil)
