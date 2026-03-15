// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "cmp"

// CollectionUtil provides methods for manipulating (sorting) collections.
// Sort methods work directly on the supplied slices and don't copy to/from
// arrays before/after. For medium size collections as used in the Lucene indexer
// that is much more efficient.
//
// This is a port of Apache Lucene's CollectionUtil class.

// IntroSort sorts the given slice using the provided comparison function.
// This method uses the intro sort algorithm, but falls back to insertion sort for small slices.
//
// See IntroSorter for more details.
func IntroSort[T any](slice []T, comp func(a, b T) int) {
	size := len(slice)
	if size <= 1 {
		return
	}
	sorter := newSliceIntroSorter(slice, comp)
	NewIntroSorter(sorter).Sort(0, size)
}

// IntroSortOrdered sorts the given ordered slice in natural order.
// This method uses the intro sort algorithm, but falls back to insertion sort for small slices.
//
// See IntroSorter for more details.
func IntroSortOrdered[T cmp.Ordered](slice []T) {
	size := len(slice)
	if size <= 1 {
		return
	}
	sorter := newOrderedSliceIntroSorter(slice)
	NewIntroSorter(sorter).Sort(0, size)
}

// TimSort sorts the given slice using the provided comparison function.
// This method uses the Tim sort algorithm, but falls back to binary sort for small slices.
//
// See TimSorter for more details.
func TimSort[T any](slice []T, comp func(a, b T) int) {
	size := len(slice)
	if size <= 1 {
		return
	}
	sorter := newSliceTimSorter(slice, comp)
	NewTimSorter(sorter, size/64).Sort(0, size)
}

// TimSortOrdered sorts the given ordered slice in natural order.
// This method uses the Tim sort algorithm, but falls back to binary sort for small slices.
//
// See TimSorter for more details.
func TimSortOrdered[T cmp.Ordered](slice []T) {
	size := len(slice)
	if size <= 1 {
		return
	}
	sorter := newOrderedSliceTimSorter(slice)
	NewTimSorter(sorter, size/64).Sort(0, size)
}

// sliceIntroSorter adapts a generic slice for use with IntroSorter.
type sliceIntroSorter[T any] struct {
	slice []T
	comp  func(a, b T) int
	pivot T
}

func newSliceIntroSorter[T any](slice []T, comp func(a, b T) int) *sliceIntroSorter[T] {
	return &sliceIntroSorter[T]{slice: slice, comp: comp}
}

func (s *sliceIntroSorter[T]) Compare(i, j int) int {
	return s.comp(s.slice[i], s.slice[j])
}

func (s *sliceIntroSorter[T]) Swap(i, j int) {
	s.slice[i], s.slice[j] = s.slice[j], s.slice[i]
}

func (s *sliceIntroSorter[T]) SetPivot(i int) {
	s.pivot = s.slice[i]
}

func (s *sliceIntroSorter[T]) ComparePivot(j int) int {
	return s.comp(s.pivot, s.slice[j])
}

func (s *sliceIntroSorter[T]) Sort(from, to int) {
	// This is called by the IntroSorter
}

// orderedSliceIntroSorter adapts an ordered slice for use with IntroSorter.
type orderedSliceIntroSorter[T cmp.Ordered] struct {
	slice []T
	pivot T
}

func newOrderedSliceIntroSorter[T cmp.Ordered](slice []T) *orderedSliceIntroSorter[T] {
	return &orderedSliceIntroSorter[T]{slice: slice}
}

func (s *orderedSliceIntroSorter[T]) Compare(i, j int) int {
	if s.slice[i] < s.slice[j] {
		return -1
	}
	if s.slice[i] > s.slice[j] {
		return 1
	}
	return 0
}

func (s *orderedSliceIntroSorter[T]) Swap(i, j int) {
	s.slice[i], s.slice[j] = s.slice[j], s.slice[i]
}

func (s *orderedSliceIntroSorter[T]) SetPivot(i int) {
	s.pivot = s.slice[i]
}

func (s *orderedSliceIntroSorter[T]) ComparePivot(j int) int {
	if s.pivot < s.slice[j] {
		return -1
	}
	if s.pivot > s.slice[j] {
		return 1
	}
	return 0
}

func (s *orderedSliceIntroSorter[T]) Sort(from, to int) {
	// This is called by the IntroSorter
}

// sliceTimSorter adapts a generic slice for use with TimSorter.
type sliceTimSorter[T any] struct {
	slice []T
	comp  func(a, b T) int
	tmp   []T
}

func newSliceTimSorter[T any](slice []T, comp func(a, b T) int) *sliceTimSorter[T] {
	return &sliceTimSorter[T]{
		slice: slice,
		comp:  comp,
		tmp:   make([]T, len(slice)/64),
	}
}

func (s *sliceTimSorter[T]) Compare(i, j int) int {
	return s.comp(s.slice[i], s.slice[j])
}

func (s *sliceTimSorter[T]) Swap(i, j int) {
	s.slice[i], s.slice[j] = s.slice[j], s.slice[i]
}

func (s *sliceTimSorter[T]) Copy(src, dest int) {
	s.slice[dest] = s.slice[src]
}

func (s *sliceTimSorter[T]) Save(i, length int) {
	for j := 0; j < length; j++ {
		if j < len(s.tmp) {
			s.tmp[j] = s.slice[i+j]
		}
	}
}

func (s *sliceTimSorter[T]) Restore(i, j int) {
	if i < len(s.tmp) {
		s.slice[j] = s.tmp[i]
	}
}

func (s *sliceTimSorter[T]) CompareSaved(i, j int) int {
	if i < len(s.tmp) {
		return s.comp(s.tmp[i], s.slice[j])
	}
	return 0
}

func (s *sliceTimSorter[T]) Sort(from, to int) {
	// This is called by the TimSorter
}

// orderedSliceTimSorter adapts an ordered slice for use with TimSorter.
type orderedSliceTimSorter[T cmp.Ordered] struct {
	slice []T
	tmp   []T
}

func newOrderedSliceTimSorter[T cmp.Ordered](slice []T) *orderedSliceTimSorter[T] {
	return &orderedSliceTimSorter[T]{
		slice: slice,
		tmp:   make([]T, len(slice)/64),
	}
}

func (s *orderedSliceTimSorter[T]) Compare(i, j int) int {
	if s.slice[i] < s.slice[j] {
		return -1
	}
	if s.slice[i] > s.slice[j] {
		return 1
	}
	return 0
}

func (s *orderedSliceTimSorter[T]) Swap(i, j int) {
	s.slice[i], s.slice[j] = s.slice[j], s.slice[i]
}

func (s *orderedSliceTimSorter[T]) Copy(src, dest int) {
	s.slice[dest] = s.slice[src]
}

func (s *orderedSliceTimSorter[T]) Save(i, len_ int) {
	for j := 0; j < len_; j++ {
		if j < len(s.tmp) {
			s.tmp[j] = s.slice[i+j]
		}
	}
}

func (s *orderedSliceTimSorter[T]) Restore(i, j int) {
	if i < len(s.tmp) {
		s.slice[j] = s.tmp[i]
	}
}

func (s *orderedSliceTimSorter[T]) CompareSaved(i, j int) int {
	if i < len(s.tmp) {
		if s.tmp[i] < s.slice[j] {
			return -1
		}
		if s.tmp[i] > s.slice[j] {
			return 1
		}
	}
	return 0
}

func (s *orderedSliceTimSorter[T]) Sort(from, to int) {
	// This is called by the TimSorter
}
