// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package knn

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestHnswConstructorThreshold mirrors the Java guard:
// filteredSearchThreshold must be in [0, 100].
func TestHnswConstructorThreshold(t *testing.T) {
	t.Run("zero ok", func(t *testing.T) {
		if got := NewHnsw(0).FilteredSearchThreshold(); got != 0 {
			t.Errorf("FilteredSearchThreshold() = %d, want 0", got)
		}
	})
	t.Run("hundred ok", func(t *testing.T) {
		if got := NewHnsw(100).FilteredSearchThreshold(); got != 100 {
			t.Errorf("FilteredSearchThreshold() = %d, want 100", got)
		}
	})
	for _, bad := range []int{-1, 101, 1000} {
		bad := bad
		t.Run("panic on bad", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("NewHnsw(%d) did not panic", bad)
				}
			}()
			_ = NewHnsw(bad)
		})
	}
}

// TestHnswUseFilteredSearch covers the threshold inequality and the
// out-of-range panic on the ratio argument.
func TestHnswUseFilteredSearch(t *testing.T) {
	h := NewHnsw(50)
	cases := []struct {
		ratio float32
		want  bool
	}{
		{0.0, true},   // 0*100 = 0 < 50
		{0.49, true},  // 49 < 50
		{0.50, false}, // 50 < 50 → false
		{0.99, false},
		{1.0, false},
	}
	for _, tc := range cases {
		if got := h.UseFilteredSearch(tc.ratio); got != tc.want {
			t.Errorf("UseFilteredSearch(%v) = %v, want %v", tc.ratio, got, tc.want)
		}
	}
	t.Run("never uses filtered at 0", func(t *testing.T) {
		// DefaultHnsw has threshold 0, so even ratio 0 must return false.
		if DefaultHnsw.UseFilteredSearch(0) {
			t.Errorf("DefaultHnsw.UseFilteredSearch(0) = true, want false")
		}
	})
	t.Run("always uses filtered at 100", func(t *testing.T) {
		h := NewHnsw(100)
		if !h.UseFilteredSearch(0.99) {
			t.Errorf("Hnsw(100).UseFilteredSearch(0.99) = false, want true")
		}
		// 1.0 * 100 = 100 is NOT < 100 → false; matches Java.
		if h.UseFilteredSearch(1.0) {
			t.Errorf("Hnsw(100).UseFilteredSearch(1.0) = true, want false")
		}
	})
	t.Run("panic on out-of-range ratio", func(t *testing.T) {
		for _, bad := range []float32{-0.1, 1.1, 2} {
			bad := bad
			func() {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("UseFilteredSearch(%v) did not panic", bad)
					}
				}()
				_ = h.UseFilteredSearch(bad)
			}()
		}
	})
}

// TestHnswEquals checks that equality follows
// filteredSearchThreshold only, both for identical values and across
// different threshold values.
func TestHnswEquals(t *testing.T) {
	a := NewHnsw(10)
	b := NewHnsw(10)
	c := NewHnsw(20)
	if !a.Equals(a) {
		t.Errorf("a.Equals(a) = false, want true (identity)")
	}
	if !a.Equals(b) {
		t.Errorf("a.Equals(b) = false, want true (value equality)")
	}
	if a.Equals(c) {
		t.Errorf("a.Equals(c) = true, want false (different threshold)")
	}
	if a.Equals(nil) {
		t.Errorf("a.Equals(nil) = true, want false")
	}
	if a.Equals("not-a-strategy") {
		t.Errorf("a.Equals(non-Hnsw) = true, want false")
	}
}

// TestHnswHashCode asserts that the hash is stable across equal
// instances and differs across unequal ones (probabilistically).
func TestHnswHashCode(t *testing.T) {
	if NewHnsw(10).HashCode() != NewHnsw(10).HashCode() {
		t.Errorf("equal Hnsws produced different hashes")
	}
	if NewHnsw(10).HashCode() == NewHnsw(11).HashCode() {
		t.Errorf("different Hnsws produced same hash (allowed but unlikely)")
	}
}

// TestSeededConstructorGuards covers the Java IllegalArgumentExceptions:
// numberOfEntryPoints < 0; numberOfEntryPoints > 0 with nil entries.
func TestSeededConstructorGuards(t *testing.T) {
	t.Run("negative count panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("NewSeeded(_, -1, _) did not panic")
			}
		}()
		_ = NewSeeded(nil, -1, nil)
	})
	t.Run("non-zero count with nil iterator panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("NewSeeded(nil, 1, _) did not panic")
			}
		}()
		_ = NewSeeded(nil, 1, nil)
	})
	t.Run("zero count with nil iterator yields empty iter", func(t *testing.T) {
		s := NewSeeded(nil, 0, nil)
		if s.EntryPoints() == nil {
			t.Errorf("EntryPoints() = nil, want empty iterator")
		}
		if got, want := s.NumberOfEntryPoints(), 0; got != want {
			t.Errorf("NumberOfEntryPoints() = %d, want %d", got, want)
		}
	})
}

// TestSeededNextVectorsBlock asserts the delegation to the original
// strategy and the no-op behaviour when no original strategy is set.
func TestSeededNextVectorsBlock(t *testing.T) {
	t.Run("nil original is no-op", func(t *testing.T) {
		s := NewSeeded(nil, 0, nil)
		s.NextVectorsBlock() // must not panic
	})
	t.Run("forwards to original", func(t *testing.T) {
		spy := &countingStrategy{}
		s := NewSeeded(nil, 0, spy)
		s.NextVectorsBlock()
		s.NextVectorsBlock()
		if spy.calls != 2 {
			t.Errorf("delegate called %d times, want 2", spy.calls)
		}
	})
}

// TestSeededEquals checks reference-identity on entryPoints (matching
// Java's Objects.equals on DocIdSetIterator) plus value equality on
// the count and original strategy.
func TestSeededEquals(t *testing.T) {
	it1 := search.NewEmptyDocIdSetIterator()
	it2 := search.NewEmptyDocIdSetIterator()
	a := NewSeeded(it1, 0, nil)
	b := NewSeeded(it1, 0, nil)
	c := NewSeeded(it2, 0, nil) // different iterator instance
	if !a.Equals(b) {
		t.Errorf("a.Equals(b) = false, want true (same iter, same count)")
	}
	if a.Equals(c) {
		t.Errorf("a.Equals(c) = true, want false (different iter)")
	}
	if a.Equals(nil) {
		t.Errorf("a.Equals(nil) = true, want false")
	}
	if a.Equals("not-a-strategy") {
		t.Errorf("a.Equals(non-Seeded) = true, want false")
	}
	// Different counts.
	if a.Equals(NewSeeded(it1, 0, &countingStrategy{})) {
		t.Errorf("a.Equals(other strategy variant) = true, want false")
	}
}

// countingStrategy is a test helper KnnSearchStrategy that counts
// NextVectorsBlock invocations.
type countingStrategy struct{ calls int }

func (s *countingStrategy) NextVectorsBlock() { s.calls++ }
func (s *countingStrategy) Equals(o any) bool {
	_, ok := o.(*countingStrategy)
	return ok && o == any(s)
}
func (s *countingStrategy) HashCode() uint64 { return 0 }
