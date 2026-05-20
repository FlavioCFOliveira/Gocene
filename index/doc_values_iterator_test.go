// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Compile-time assertions that DocValuesIterator embeds the DocIdSetIterator
// contract and that the test stub satisfies the interface.
var (
	_ util.DocIdSetIterator = (DocValuesIterator)(nil)
	_ DocValuesIterator     = (*stubDocValuesIterator)(nil)
)

// stubDocValuesIterator is a minimal DocValuesIterator over a fixed value set.
type stubDocValuesIterator struct {
	withValue map[int]bool
	maxDoc    int
	docID     int
}

func (s *stubDocValuesIterator) DocID() int { return s.docID }

func (s *stubDocValuesIterator) NextDoc() (int, error) {
	for d := s.docID + 1; d < s.maxDoc; d++ {
		if s.withValue[d] {
			s.docID = d
			return d, nil
		}
	}
	s.docID = util.NO_MORE_DOCS
	return s.docID, nil
}

func (s *stubDocValuesIterator) Advance(target int) (int, error) {
	for d := target; d < s.maxDoc; d++ {
		if s.withValue[d] {
			s.docID = d
			return d, nil
		}
	}
	s.docID = util.NO_MORE_DOCS
	return s.docID, nil
}

func (s *stubDocValuesIterator) Cost() int64 { return int64(len(s.withValue)) }

func (s *stubDocValuesIterator) DocIDRunEnd() int { return s.docID + 1 }

func (s *stubDocValuesIterator) AdvanceExact(target int) (bool, error) {
	s.docID = target
	return s.withValue[target], nil
}

func TestDocValuesIterator_AdvanceExact(t *testing.T) {
	var it DocValuesIterator = &stubDocValuesIterator{
		withValue: map[int]bool{1: true, 3: true},
		maxDoc:    5,
		docID:     -1,
	}

	tests := []struct {
		name    string
		target  int
		wantHas bool
		wantDoc int
	}{
		{name: "value present", target: 1, wantHas: true, wantDoc: 1},
		{name: "no value at target", target: 2, wantHas: false, wantDoc: 2},
		{name: "value present again", target: 3, wantHas: true, wantDoc: 3},
		{name: "no value at last", target: 4, wantHas: false, wantDoc: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has, err := it.AdvanceExact(tt.target)
			if err != nil {
				t.Fatalf("AdvanceExact(%d) error: %v", tt.target, err)
			}
			if has != tt.wantHas {
				t.Errorf("AdvanceExact(%d) has = %v, want %v", tt.target, has, tt.wantHas)
			}
			if got := it.DocID(); got != tt.wantDoc {
				t.Errorf("after AdvanceExact(%d), DocID() = %d, want %d", tt.target, got, tt.wantDoc)
			}
		})
	}
}

func TestDocValuesIterator_EmbedsDocIdSetIterator(t *testing.T) {
	var it DocValuesIterator = &stubDocValuesIterator{
		withValue: map[int]bool{2: true},
		maxDoc:    4,
		docID:     -1,
	}

	doc, err := it.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != 2 {
		t.Errorf("NextDoc() = %d, want 2", doc)
	}
	if doc, _ := it.NextDoc(); doc != util.NO_MORE_DOCS {
		t.Errorf("exhausted NextDoc() = %d, want NO_MORE_DOCS", doc)
	}
}
