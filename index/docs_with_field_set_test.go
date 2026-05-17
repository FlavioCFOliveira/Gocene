// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

func TestDocsWithFieldSet_DenseFastPath(t *testing.T) {
	s := NewDocsWithFieldSet()
	for i := 0; i < 5; i++ {
		if err := s.Add(i); err != nil {
			t.Fatal(err)
		}
	}
	if s.bits != nil {
		t.Errorf("expected dense (nil bits) after contiguous adds, got %v", s.bits)
	}
	if got := s.Cardinality(); got != 5 {
		t.Errorf("Cardinality=%d, want 5", got)
	}
	for i := 0; i < 5; i++ {
		if !s.Contains(i) {
			t.Errorf("Contains(%d) = false", i)
		}
	}
	if s.Contains(5) {
		t.Errorf("Contains(5) should be false")
	}
}

func TestDocsWithFieldSet_SparseTransition(t *testing.T) {
	s := NewDocsWithFieldSet()
	if err := s.Add(0); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(1); err != nil {
		t.Fatal(err)
	}
	// jump to 10 -> triggers sparse promotion
	if err := s.Add(10); err != nil {
		t.Fatal(err)
	}
	if s.bits == nil {
		t.Errorf("expected sparse representation after non-contiguous add")
	}
	if s.Cardinality() != 3 {
		t.Errorf("Cardinality=%d, want 3", s.Cardinality())
	}
	for _, d := range []int{0, 1, 10} {
		if !s.Contains(d) {
			t.Errorf("Contains(%d) = false", d)
		}
	}
	for _, d := range []int{2, 3, 9, 11} {
		if s.Contains(d) {
			t.Errorf("Contains(%d) = true, want false", d)
		}
	}
}

func TestDocsWithFieldSet_OutOfOrderRejected(t *testing.T) {
	s := NewDocsWithFieldSet()
	if err := s.Add(5); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(5); err == nil {
		t.Errorf("expected error for duplicate doc id")
	}
	if err := s.Add(3); err == nil {
		t.Errorf("expected error for out-of-order doc id")
	}
}

func TestDocsWithFieldSet_LargeSparseGrowsBits(t *testing.T) {
	s := NewDocsWithFieldSet()
	// Force sparse path immediately
	if err := s.Add(127); err != nil {
		t.Fatal(err)
	}
	// And another in a different word
	if err := s.Add(200); err != nil {
		t.Fatal(err)
	}
	if !s.Contains(127) || !s.Contains(200) {
		t.Errorf("missing expected docs")
	}
	if s.Contains(126) || s.Contains(128) || s.Contains(199) || s.Contains(201) {
		t.Errorf("false-positive Contains in sparse mode")
	}
}
