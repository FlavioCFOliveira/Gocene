// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/ArrayTimSorter.java
// Purpose: TimSorter specialization for arbitrary object slices.

package util

// arrayTimSorter is a TimSorter for object slices. It mirrors Apache Lucene's
// ArrayTimSorter, which is a package-private final class. The adapter itself
// is kept unexported; callers wire it through NewArrayTimSorter, which returns
// the *TimSorter parent (the public API exposed by TimSorter.Sort).
//
// Memory layout:
//   - arr is the slice being sorted in place.
//   - cmp is the user-supplied comparator.
//   - tmp is a scratch buffer sized to maxTempSlots. Allocated lazily by the
//     constructor when maxTempSlots > 0; nil otherwise. The parent TimSorter
//     never calls Save with a length greater than maxTempSlots, so no bounds
//     enforcement is needed here.
type arrayTimSorter[T any] struct {
	arr []T
	cmp func(a, b T) int
	tmp []T
}

// NewArrayTimSorter creates a TimSorter that sorts arr in place using cmp.
// maxTempSlots is the maximum amount of extra memory (in element slots) the
// merge step may use; passing 0 forces fully in-place merges, at the cost of
// runtime.
func NewArrayTimSorter[T any](arr []T, cmp func(a, b T) int, maxTempSlots int) *TimSorter {
	a := &arrayTimSorter[T]{arr: arr, cmp: cmp}
	if maxTempSlots > 0 {
		a.tmp = make([]T, maxTempSlots)
	}
	return NewTimSorter(a, maxTempSlots)
}

// Compare returns cmp(arr[i], arr[j]).
func (a *arrayTimSorter[T]) Compare(i, j int) int {
	return a.cmp(a.arr[i], a.arr[j])
}

// Swap exchanges arr[i] and arr[j].
func (a *arrayTimSorter[T]) Swap(i, j int) {
	a.arr[i], a.arr[j] = a.arr[j], a.arr[i]
}

// Sort is part of SorterInterface but is unused for TimSorter implementations:
// the parent TimSorter.Sort drives the algorithm. Provided as a no-op so the
// adapter satisfies the interface.
func (a *arrayTimSorter[T]) Sort(from, to int) {}

// Copy copies arr[src] into arr[dest].
func (a *arrayTimSorter[T]) Copy(src, dest int) {
	a.arr[dest] = a.arr[src]
}

// Save snapshots arr[start:start+length] into the scratch buffer.
func (a *arrayTimSorter[T]) Save(start, length int) {
	copy(a.tmp[:length], a.arr[start:start+length])
}

// Restore writes tmp[src] back into arr[dest].
func (a *arrayTimSorter[T]) Restore(src, dest int) {
	a.arr[dest] = a.tmp[src]
}

// CompareSaved returns cmp(tmp[i], arr[j]).
func (a *arrayTimSorter[T]) CompareSaved(i, j int) int {
	return a.cmp(a.tmp[i], a.arr[j])
}
