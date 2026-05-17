// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubSub is a minimal DocIDMergerSub backed by a finite list of mapped docs.
type stubSub struct {
	docs []int
	idx  int
}

func (s *stubSub) MappedDocID() int {
	if s.idx < 0 || s.idx >= len(s.docs) {
		return NO_MORE_DOCS
	}
	return s.docs[s.idx]
}

func (s *stubSub) NextDoc() (int, error) {
	s.idx++
	return s.MappedDocID(), nil
}

func (s *stubSub) NextMappedDoc() (int, error) {
	return s.NextDoc()
}

func TestDocIDMerger_ConcatStrategy(t *testing.T) {
	a := &stubSub{docs: []int{0, 5}, idx: -1}
	b := &stubSub{docs: []int{1, 3}, idx: -1}
	m, err := NewDocIDMerger([]DocIDMergerSub{a, b}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 5, 1, 3}
	for _, w := range want {
		sub, err := m.Next()
		if err != nil {
			t.Fatal(err)
		}
		if sub == nil {
			t.Fatalf("Next returned nil before exhaustion (expected mappedDoc=%d)", w)
		}
		if sub.MappedDocID() != w {
			t.Errorf("MappedDocID=%d, want %d", sub.MappedDocID(), w)
		}
	}
	if sub, _ := m.Next(); sub != nil {
		t.Errorf("expected exhaustion, got %v", sub)
	}
}

func TestDocIDMerger_SortedStrategy(t *testing.T) {
	a := &stubSub{docs: []int{0, 5, 10}, idx: -1}
	b := &stubSub{docs: []int{1, 6, 9}, idx: -1}
	m, err := NewDocIDMerger([]DocIDMergerSub{a, b}, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1, 5, 6, 9, 10}
	for _, w := range want {
		sub, err := m.Next()
		if err != nil {
			t.Fatal(err)
		}
		if sub == nil {
			t.Fatalf("Next returned nil before exhaustion (expected mappedDoc=%d)", w)
		}
		if sub.MappedDocID() != w {
			t.Errorf("MappedDocID=%d, want %d", sub.MappedDocID(), w)
		}
	}
	if sub, _ := m.Next(); sub != nil {
		t.Errorf("expected exhaustion")
	}
}

// --- KnnVectorValues interface contract ---------------------------------------

// realStubKnnVV is a properly-typed KnnVectorValues to assert satisfiability.
type realStubKnnVV struct{}

func (realStubKnnVV) Dimension() int                       { return 4 }
func (realStubKnnVV) Size() int                            { return 2 }
func (realStubKnnVV) OrdToDoc(o int) int                   { return o }
func (realStubKnnVV) Copy() (KnnVectorValues, error)       { return realStubKnnVV{}, nil }
func (realStubKnnVV) VectorByteLength() int                { return 16 }
func (realStubKnnVV) GetEncoding() VectorEncoding          { return 0 }
func (realStubKnnVV) GetAcceptOrds(_ util.Bits) util.Bits  { return nil }
func (realStubKnnVV) Iterator() DocIndexIterator           { return nil }

func TestKnnVectorValues_InterfaceLooselyTyped(t *testing.T) {
	var v KnnVectorValues = realStubKnnVV{}
	if v.Dimension() != 4 || v.Size() != 2 {
		t.Errorf("interface dispatch mismatch")
	}
}
