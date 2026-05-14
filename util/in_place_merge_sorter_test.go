// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"slices"
	"testing"
)

// intSliceSorter is a minimal SorterInterface for ints, used by the
// in-place merge sort tests.
type intSliceSorter struct {
	arr []int
}

func (s *intSliceSorter) Compare(i, j int) int {
	switch {
	case s.arr[i] < s.arr[j]:
		return -1
	case s.arr[i] > s.arr[j]:
		return 1
	default:
		return 0
	}
}

func (s *intSliceSorter) Swap(i, j int) {
	s.arr[i], s.arr[j] = s.arr[j], s.arr[i]
}

func (s *intSliceSorter) Sort(from, to int) {
	NewInPlaceMergeSorter(s).Sort(from, to)
}

func TestInPlaceMergeSorter(t *testing.T) {
	t.Run("empty range", func(t *testing.T) {
		s := &intSliceSorter{arr: []int{}}
		s.Sort(0, 0)
	})

	t.Run("single element", func(t *testing.T) {
		s := &intSliceSorter{arr: []int{42}}
		s.Sort(0, 1)
		if s.arr[0] != 42 {
			t.Fatalf("got %d want 42", s.arr[0])
		}
	})

	t.Run("already sorted", func(t *testing.T) {
		arr := []int{1, 2, 3, 4, 5}
		s := &intSliceSorter{arr: arr}
		s.Sort(0, len(arr))
		for i := range arr {
			if arr[i] != i+1 {
				t.Fatalf("arr[%d]=%d want %d", i, arr[i], i+1)
			}
		}
	})

	t.Run("reverse sorted", func(t *testing.T) {
		arr := []int{5, 4, 3, 2, 1}
		s := &intSliceSorter{arr: arr}
		s.Sort(0, len(arr))
		for i := range arr {
			if arr[i] != i+1 {
				t.Fatalf("arr[%d]=%d want %d", i, arr[i], i+1)
			}
		}
	})

	t.Run("random ints", func(t *testing.T) {
		rng := rand.New(rand.NewSource(123))
		for size := 0; size < 200; size++ {
			orig := make([]int, size)
			for i := range orig {
				orig[i] = rng.Intn(1000)
			}
			arr := slices.Clone(orig)
			s := &intSliceSorter{arr: arr}
			s.Sort(0, len(arr))
			expected := slices.Clone(orig)
			slices.Sort(expected)
			if !slices.Equal(arr, expected) {
				t.Fatalf("size=%d sort mismatch", size)
			}
		}
	})

	t.Run("stability via entries", func(t *testing.T) {
		// Use entry to verify stability: equal values keep relative order.
		orig := []entry{
			{value: 3, ord: 0},
			{value: 1, ord: 1},
			{value: 3, ord: 2},
			{value: 1, ord: 3},
			{value: 2, ord: 4},
			{value: 3, ord: 5},
		}
		// Build the same stable expected output as the existing
		// assertSorted helper, then check pointwise.
		arr := make([]entry, len(orig))
		copy(arr, orig)
		s := newEntryInPlaceMergeSorter(arr)
		s.Sort(0, len(arr))
		expected := []entry{
			{value: 1, ord: 1},
			{value: 1, ord: 3},
			{value: 2, ord: 4},
			{value: 3, ord: 0},
			{value: 3, ord: 2},
			{value: 3, ord: 5},
		}
		for i, e := range expected {
			if arr[i] != e {
				t.Fatalf("stability index %d got %+v want %+v", i, arr[i], e)
			}
		}
	})

	t.Run("sort sub-range only", func(t *testing.T) {
		arr := []int{9, 8, 3, 1, 2, 7, 6}
		s := &intSliceSorter{arr: arr}
		s.Sort(1, 6) // sorts indices [1..5]: {8,3,1,2,7} -> {1,2,3,7,8}
		expected := []int{9, 1, 2, 3, 7, 8, 6}
		if !slices.Equal(arr, expected) {
			t.Fatalf("got %v want %v", arr, expected)
		}
	})
}

// entryInPlaceMergeSorter adapts a slice of entries to the
// InPlaceMergeSorter, used to verify stability of the merge sort.
type entryInPlaceMergeSorter struct {
	arr []entry
}

func newEntryInPlaceMergeSorter(arr []entry) *entryInPlaceMergeSorter {
	return &entryInPlaceMergeSorter{arr: arr}
}

func (s *entryInPlaceMergeSorter) Compare(i, j int) int {
	return compareEntry(s.arr[i], s.arr[j])
}

func (s *entryInPlaceMergeSorter) Swap(i, j int) {
	s.arr[i], s.arr[j] = s.arr[j], s.arr[i]
}

func (s *entryInPlaceMergeSorter) Sort(from, to int) {
	NewInPlaceMergeSorter(s).Sort(from, to)
}
