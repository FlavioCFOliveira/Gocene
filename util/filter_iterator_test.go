// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// sliceIntIterator is a small Iterator[int] used by FilterIterator tests.
type sliceIntIterator struct {
	values []int
	pos    int
}

func newSliceIntIterator(values []int) *sliceIntIterator {
	return &sliceIntIterator{values: values}
}

func (s *sliceIntIterator) HasNext() bool {
	return s.pos < len(s.values)
}

func (s *sliceIntIterator) Next() int {
	if s.pos >= len(s.values) {
		panic("NoSuchElementException")
	}
	v := s.values[s.pos]
	s.pos++
	return v
}

// TestFilterIterator mirrors the Java TestFilterIterator coverage: empty
// inputs, all-accept, all-reject, alternating predicates, and the contract
// that Remove panics with UnsupportedOperationException.
func TestFilterIterator(t *testing.T) {
	t.Run("empty source", func(t *testing.T) {
		it := NewFilterIterator[int](newSliceIntIterator(nil), func(int) bool { return true })
		if it.HasNext() {
			t.Fatalf("HasNext() on empty source must be false")
		}
		defer func() {
			if recover() == nil {
				t.Fatalf("Next() on empty iterator must panic with NoSuchElementException")
			}
		}()
		it.Next()
	})

	t.Run("accept all", func(t *testing.T) {
		src := newSliceIntIterator([]int{1, 2, 3, 4, 5})
		it := NewFilterIterator[int](src, func(int) bool { return true })
		var got []int
		for it.HasNext() {
			got = append(got, it.Next())
		}
		want := []int{1, 2, 3, 4, 5}
		if !equalIntSlices(got, want) {
			t.Fatalf("accept-all: got %v want %v", got, want)
		}
	})

	t.Run("accept none", func(t *testing.T) {
		src := newSliceIntIterator([]int{1, 2, 3, 4, 5})
		it := NewFilterIterator[int](src, func(int) bool { return false })
		if it.HasNext() {
			t.Fatalf("accept-none: HasNext() must be false")
		}
	})

	t.Run("accept evens", func(t *testing.T) {
		src := newSliceIntIterator([]int{1, 2, 3, 4, 5, 6, 7, 8})
		it := NewFilterIterator[int](src, func(v int) bool { return v%2 == 0 })
		var got []int
		for it.HasNext() {
			got = append(got, it.Next())
		}
		want := []int{2, 4, 6, 8}
		if !equalIntSlices(got, want) {
			t.Fatalf("accept-evens: got %v want %v", got, want)
		}
	})

	t.Run("hasnext is idempotent", func(t *testing.T) {
		src := newSliceIntIterator([]int{10, 20})
		it := NewFilterIterator[int](src, func(int) bool { return true })
		for i := 0; i < 5; i++ {
			if !it.HasNext() {
				t.Fatalf("HasNext() must remain true while elements remain (iteration %d)", i)
			}
		}
		if got := it.Next(); got != 10 {
			t.Fatalf("Next #1 got %d want 10", got)
		}
		for i := 0; i < 5; i++ {
			if !it.HasNext() {
				t.Fatalf("HasNext() must remain true after Next (iteration %d)", i)
			}
		}
		if got := it.Next(); got != 20 {
			t.Fatalf("Next #2 got %d want 20", got)
		}
		if it.HasNext() {
			t.Fatalf("HasNext() after exhaustion must be false")
		}
	})

	t.Run("next without hasnext panics at end", func(t *testing.T) {
		src := newSliceIntIterator([]int{1})
		it := NewFilterIterator[int](src, func(int) bool { return true })
		if got := it.Next(); got != 1 {
			t.Fatalf("first Next got %d want 1", got)
		}
		defer func() {
			if recover() == nil {
				t.Fatalf("Next past end must panic")
			}
		}()
		it.Next()
	})

	t.Run("remove is unsupported", func(t *testing.T) {
		it := NewFilterIterator[int](newSliceIntIterator([]int{1}), func(int) bool { return true })
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Remove must panic with UnsupportedOperationException")
			}
		}()
		it.Remove()
	})

	t.Run("complex predicate skips runs", func(t *testing.T) {
		src := newSliceIntIterator([]int{1, 1, 1, 5, 1, 1, 6, 1, 7, 1, 1})
		it := NewFilterIterator[int](src, func(v int) bool { return v >= 5 })
		var got []int
		for it.HasNext() {
			got = append(got, it.Next())
		}
		want := []int{5, 6, 7}
		if !equalIntSlices(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
