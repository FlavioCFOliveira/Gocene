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
	impl Sortable
}

// NewIntroSorter creates a new IntroSorter with the given implementation.
func NewIntroSorter(impl Sortable) *IntroSorter {
	return &IntroSorter{impl: impl}
}

// Sort sorts the range [from, to).
func (is *IntroSorter) Sort(from, to int) {
	if to-from <= 1 {
		return
	}
	maxDepth := 2 * int(math.Log2(float64(to-from)))
	is.sort(from, to, maxDepth)
}

func (is *IntroSorter) sort(from, to, maxDepth int) {
	size := to - from
	for size > InsertionSortThreshold {
		if maxDepth <= 0 {
			HeapSort(is.impl, from, to)
			return
		}
		maxDepth--

		last := to - 1
		mid := (from + last) >> 1
		var pivot int

		if size <= SingleMedianThreshold {
			range_ := size >> 2
			pivot = is.median(mid-range_, mid, mid+range_)
		} else {
			range_ := size >> 3
			doubleRange := range_ << 1
			medianFirst := is.median(from, from+range_, from+doubleRange)
			medianMiddle := is.median(mid-range_, mid, mid+range_)
			medianLast := is.median(last-doubleRange, last-range_, last)
			pivot = is.median(medianFirst, medianMiddle, medianLast)
		}

		// Pivot handling
		if p, ok := is.impl.(Pivotable); ok {
			p.SetPivot(pivot)
		} else {
			panic("Sortable must implement Pivotable to be used with IntroSorter")
		}

		// Bentley-McIlroy 3-way partitioning.
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
				leftCmp = is.comparePivot(i)
				if leftCmp <= 0 {
					break
				}
			}
			for {
				j--
				if j < from {
					break
				}
				rightCmp = is.comparePivot(j)
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

		if j-from < last-i {
			is.sort(from, j+1, maxDepth)
			from = i
		} else {
			is.sort(i, to, maxDepth)
			to = j + 1
		}
		size = to - from
	}

	InsertionSort(is.impl, from, to)
}

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

func (is *IntroSorter) comparePivot(j int) int {
	if p, ok := is.impl.(Pivotable); ok {
		return p.ComparePivot(j)
	}
	panic("Sortable must implement Pivotable to be used with IntroSorter")
}

const SingleMedianThreshold = 40
