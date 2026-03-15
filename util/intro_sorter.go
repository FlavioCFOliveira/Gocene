// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "math"

// IntroSorter implements a variant of the quicksort algorithm called introsort.
// When the recursion level exceeds the log of the length of the array to sort,
// it falls back to heapsort. This prevents quicksort from running into its
// worst-case quadratic runtime. Selects the pivot using Tukey's ninther
// median-of-medians, and partitions using Bentley-McIlroy 3-way partitioning.
// Small ranges are sorted with insertion sort.
//
// This algorithm is NOT stable. It's fast on most data shapes, especially with
// low cardinality. If the data to sort is known to be strictly ascending or
// descending, prefer TimSorter.
//
// This is a port of Apache Lucene's IntroSorter class.
type IntroSorter struct {
	Sorter
	impl SorterInterface
}

// IntroSorterInterface extends SorterInterface with methods specific to IntroSorter.
type IntroSorterInterface interface {
	SorterInterface
	// SetPivot saves the value at slot i as the pivot.
	SetPivot(i int)
	// ComparePivot compares the saved pivot with slot j.
	ComparePivot(j int) int
}

// NewIntroSorter creates a new IntroSorter with the given implementation.
func NewIntroSorter(impl SorterInterface) *IntroSorter {
	return &IntroSorter{impl: impl}
}

// Sort sorts the range [from, to).
func (is *IntroSorter) Sort(from, to int) {
	is.CheckRange(from, to)
	if to-from <= 1 {
		return
	}
	maxDepth := 2 * int(math.Log2(float64(to-from)))
	is.sort(from, to, maxDepth)
}

// sort is the internal recursive sort method.
func (is *IntroSorter) sort(from, to, maxDepth int) {
	// Sort small ranges with insertion sort.
	size := to - from
	for size > InsertionSortThreshold {
		if maxDepth <= 0 {
			// Max recursion depth exceeded: fallback to heap sort.
			is.HeapSort(from, to, is.impl)
			return
		}
		maxDepth--

		// Pivot selection based on medians.
		last := to - 1
		mid := (from + last) >> 1
		var pivot int
		if size <= SingleMedianThreshold {
			// Select the pivot with a single median around the middle element.
			range_ := size >> 2
			pivot = is.median(mid-range_, mid, mid+range_)
		} else {
			// Select the pivot with the Tukey's ninther median of medians.
			range_ := size >> 3
			doubleRange := range_ << 1
			medianFirst := is.median(from, from+range_, from+doubleRange)
			medianMiddle := is.median(mid-range_, mid, mid+range_)
			medianLast := is.median(last-doubleRange, last-range_, last)
			pivot = is.median(medianFirst, medianMiddle, medianLast)
		}

		// Bentley-McIlroy 3-way partitioning.
		is.SetPivot(pivot)
		is.impl.Swap(from, pivot)
		i := from
		j := to
		p := from + 1
		q := last

		for {
			var leftCmp, rightCmp int
			for {
				i++
				if i >= to {
					break
				}
				leftCmp = is.ComparePivot(i)
				if leftCmp <= 0 {
					break
				}
			}
			for {
				j--
				if j < from {
					break
				}
				rightCmp = is.ComparePivot(j)
				if rightCmp >= 0 {
					break
				}
			}
			if i >= j {
				if i == j && rightCmp == 0 {
					is.impl.Swap(i, p)
				}
				break
			}
			is.impl.Swap(i, j)
			if rightCmp == 0 {
				is.impl.Swap(i, p)
				p++
			}
			if leftCmp == 0 {
				is.impl.Swap(j, q)
				q--
			}
		}
		i = j + 1
		for k := from; k < p; {
			is.impl.Swap(k, j)
			k++
			j--
		}
		for k := last; k > q; {
			is.impl.Swap(k, i)
			k--
			i++
		}

		// Recursion on the smallest partition. Replace the tail recursion by a loop.
		if j-from < last-i {
			is.sort(from, j+1, maxDepth)
			from = i
		} else {
			is.sort(i, to, maxDepth)
			to = j + 1
		}
		size = to - from
	}

	is.InsertionSort(from, to, is.impl)
}

// median returns the index of the median element among three elements at provided indices.
func (is *IntroSorter) median(i, j, k int) int {
	if is.impl.Compare(i, j) < 0 {
		if is.impl.Compare(j, k) <= 0 {
			return j
		}
		if is.impl.Compare(i, k) < 0 {
			return k
		}
		return i
	}
	if is.impl.Compare(j, k) >= 0 {
		return j
	}
	if is.impl.Compare(i, k) < 0 {
		return i
	}
	return k
}

// SetPivot saves the value at slot i as the pivot.
func (is *IntroSorter) SetPivot(i int) {
	is.Sorter.SetPivot(i, is.impl)
}

// ComparePivot compares the saved pivot with slot j.
func (is *IntroSorter) ComparePivot(j int) int {
	return is.Sorter.ComparePivot(j, is.impl)
}

// Compare compares elements at slots i and j.
func (is *IntroSorter) Compare(i, j int) int {
	is.SetPivot(i)
	return is.ComparePivot(j)
}

// Swap swaps elements at slots i and j.
func (is *IntroSorter) Swap(i, j int) {
	is.impl.Swap(i, j)
}

// SingleMedianThreshold is the size below which a single median is used for pivot selection.
const SingleMedianThreshold = 40
