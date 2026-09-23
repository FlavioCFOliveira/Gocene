// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/ArrayInPlaceMergeSorter.java
// Purpose: InPlaceMergeSorter specialization for arbitrary object slices.

package util

// arrayInPlaceMergeSorter is an InPlaceMergeSorter for object slices. It
// mirrors Apache Lucene's ArrayInPlaceMergeSorter, a package-private final
// class. The adapter is kept unexported; callers wire it through
// NewArrayInPlaceMergeSorter, which returns the *InPlaceMergeSorter parent
// (the public API that exposes Sort).
//
// Memory layout:
//   - arr is the slice being sorted in place.
//   - cmp is the user-supplied comparator.
//
// Like the Java class, it overrides only compare and swap; the binary sort
// of short runs therefore uses the default Sorter pivot (the slot index).
type arrayInPlaceMergeSorter[T any] struct {
	arr []T
	cmp func(a, b T) int
}

// NewArrayInPlaceMergeSorter creates an InPlaceMergeSorter that sorts arr in
// place using cmp. The returned sorter exposes Sort(from, to) to drive the
// algorithm and yields a stable ordering.
func NewArrayInPlaceMergeSorter[T any](arr []T, cmp func(a, b T) int) *InPlaceMergeSorter {
	a := &arrayInPlaceMergeSorter[T]{arr: arr, cmp: cmp}
	return NewInPlaceMergeSorter(a)
}

// Compare returns cmp(arr[i], arr[j]).
func (a *arrayInPlaceMergeSorter[T]) Compare(i, j int) int {
	return a.cmp(a.arr[i], a.arr[j])
}

// Swap exchanges arr[i] and arr[j].
func (a *arrayInPlaceMergeSorter[T]) Swap(i, j int) {
	a.arr[i], a.arr[j] = a.arr[j], a.arr[i]
}
