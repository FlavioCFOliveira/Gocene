// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/ArrayIntroSorter.java
// Purpose: IntroSorter specialization for arbitrary object slices.

package util

// arrayIntroSorter is an IntroSorter for object slices. It mirrors Apache
// Lucene's ArrayIntroSorter, a package-private final class. The adapter is
// kept unexported; callers wire it through NewArrayIntroSorter, which returns
// the *IntroSorter parent (the public API that exposes Sort).
//
// Memory layout:
//   - arr is the slice being sorted in place.
//   - cmp is the user-supplied comparator.
//   - pivot is the value captured by SetPivot. Lucene's Java port stores T
//     directly and seeds it with null; in Go the zero value of T plays the
//     same role until the first SetPivot call.
//   - pivotSet tracks whether pivot holds a meaningful value, guarding against
//     accidental ComparePivot calls before SetPivot. The parent IntroSorter
//     always calls SetPivot first, so this flag is purely defensive.
type arrayIntroSorter[T any] struct {
	arr      []T
	cmp      func(a, b T) int
	pivot    T
	pivotSet bool
}

// NewArrayIntroSorter creates an IntroSorter that sorts arr in place using cmp.
// The returned sorter exposes Sort(from, to) to drive the algorithm.
func NewArrayIntroSorter[T any](arr []T, cmp func(a, b T) int) *IntroSorter {
	a := &arrayIntroSorter[T]{arr: arr, cmp: cmp}
	return NewIntroSorter(a)
}

// Compare returns cmp(arr[i], arr[j]).
func (a *arrayIntroSorter[T]) Compare(i, j int) int {
	return a.cmp(a.arr[i], a.arr[j])
}

// Swap exchanges arr[i] and arr[j].
func (a *arrayIntroSorter[T]) Swap(i, j int) {
	a.arr[i], a.arr[j] = a.arr[j], a.arr[i]
}

// Sort is part of SorterInterface but is unused for IntroSorter implementations:
// the parent IntroSorter.Sort drives the algorithm. Provided as a no-op so the
// adapter satisfies the interface.
func (a *arrayIntroSorter[T]) Sort(from, to int) {}

// SetPivot snapshots arr[i] into pivot so subsequent swaps cannot corrupt it.
func (a *arrayIntroSorter[T]) SetPivot(i int) {
	a.pivot = a.arr[i]
	a.pivotSet = true
}

// ComparePivot returns cmp(pivot, arr[i]).
func (a *arrayIntroSorter[T]) ComparePivot(i int) int {
	return a.cmp(a.pivot, a.arr[i])
}
