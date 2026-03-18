// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// Sorter is the base class for sorting algorithm implementations.
// There are a number of implementations to choose from that vary in performance
// and stability:
//   - IntroSorter: Not a stable sort, fast on most data shapes
//   - TimSorter: Stable sort, especially good at sorting partially-sorted arrays
//
// This is a port of Apache Lucene's Sorter class.
type Sorter struct {
	pivotIndex int
}

// SorterInterface defines the methods that concrete sorter implementations must provide
type SorterInterface interface {
	// Compare entries found in slots i and j.
	// The contract for the returned value is the same as cmp.Compare.
	Compare(i, j int) int

	// Swap values at slots i and j.
	Swap(i, j int)

	// Sort the slice which starts at from (inclusive) and ends at to (exclusive).
	Sort(from, to int)
}

// SetPivot saves the value at slot i so that it can later be used as a pivot.
func (s *Sorter) SetPivot(i int, impl SorterInterface) {
	s.pivotIndex = i
}

// ComparePivot compares the pivot with the slot at j.
func (s *Sorter) ComparePivot(j int, impl SorterInterface) int {
	return impl.Compare(s.pivotIndex, j)
}

// CheckRange validates the range parameters.
func (s *Sorter) CheckRange(from, to int) {
	if to < from {
		panic("'to' must be >= 'from'")
	}
}

// Reverse reverses the elements in the range [from, to).
func (s *Sorter) Reverse(from, to int, impl SorterInterface) {
	to--
	for from < to {
		impl.Swap(from, to)
		from++
		to--
	}
}

// Rotate rotates the elements in the range [lo, hi) so that the element
// at mid becomes the first element.
func (s *Sorter) Rotate(lo, mid, hi int, impl SorterInterface) {
	if lo == mid || mid == hi {
		return
	}
	s.DoRotate(lo, mid, hi, impl)
}

// DoRotate performs the actual rotation. Subclasses may override.
func (s *Sorter) DoRotate(lo, mid, hi int, impl SorterInterface) {
	if mid-lo == hi-mid {
		// happens rarely but saves n/2 swaps
		for mid < hi {
			impl.Swap(lo, mid)
			lo++
			mid++
		}
	} else {
		s.Reverse(lo, mid, impl)
		s.Reverse(mid, hi, impl)
		s.Reverse(lo, hi, impl)
	}
}

// BinarySort performs a binary sort. This performs O(n*log(n)) comparisons
// and O(n^2) swaps. It is typically used as a fall-back when the number of
// items to sort has become less than BinarySortThreshold. This algorithm is stable.
func (s *Sorter) BinarySort(from, to int, impl SorterInterface) {
	s.BinarySortWithStart(from, to, from+1, impl)
}

// BinarySortWithStart performs binary sort starting from index i.
func (s *Sorter) BinarySortWithStart(from, to, i int, impl SorterInterface) {
	for ; i < to; i++ {
		s.SetPivot(i, impl)
		l := from
		h := i - 1
		for l <= h {
			mid := (l + h) >> 1
			cmp := s.ComparePivot(mid, impl)
			if cmp < 0 {
				h = mid - 1
			} else {
				l = mid + 1
			}
		}
		for j := i; j > l; j-- {
			impl.Swap(j-1, j)
		}
	}
}

// InsertionSort sorts between from (inclusive) and to (exclusive) with insertion sort.
// Runs in O(n^2). It is typically used as a fall-back when the number of items
// to sort becomes less than InsertionSortThreshold. This algorithm is stable.
func (s *Sorter) InsertionSort(from, to int, impl SorterInterface) {
	for i := from + 1; i < to; {
		current := i
		i++
		for {
			previous := current - 1
			if impl.Compare(previous, current) <= 0 {
				break
			}
			impl.Swap(previous, current)
			if previous == from {
				break
			}
			current = previous
		}
	}
}

// HeapSort uses heap sort to sort items between from inclusive and to exclusive.
// This runs in O(n*log(n)) and is used as a fall-back by IntroSorter.
// This algorithm is NOT stable.
func (s *Sorter) HeapSort(from, to int, impl SorterInterface) {
	if to-from <= 1 {
		return
	}
	s.Heapify(from, to, impl)
	for end := to - 1; end > from; end-- {
		impl.Swap(from, end)
		s.SiftDown(from, from, end, impl)
	}
}

// Heapify builds a heap from the range [from, to).
func (s *Sorter) Heapify(from, to int, impl SorterInterface) {
	for i := HeapParent(from, to-1); i >= from; i-- {
		s.SiftDown(i, from, to, impl)
	}
}

// SiftDown maintains the heap property by sifting down from position i.
func (s *Sorter) SiftDown(i, from, to int, impl SorterInterface) {
	for leftChild := HeapChild(from, i); leftChild < to; leftChild = HeapChild(from, i) {
		rightChild := leftChild + 1
		if impl.Compare(i, leftChild) < 0 {
			if rightChild < to && impl.Compare(leftChild, rightChild) < 0 {
				impl.Swap(i, rightChild)
				i = rightChild
			} else {
				impl.Swap(i, leftChild)
				i = leftChild
			}
		} else if rightChild < to && impl.Compare(i, rightChild) < 0 {
			impl.Swap(i, rightChild)
			i = rightChild
		} else {
			break
		}
	}
}

// HeapParent returns the parent index in a heap.
func HeapParent(from, i int) int {
	return ((i - 1 - from) >> 1) + from
}

// HeapChild returns the left child index in a heap.
func HeapChild(from, i int) int {
	return ((i - from) << 1) + 1 + from
}

// Lower finds the first position in [from, to) where val could be inserted.
func (s *Sorter) Lower(from, to, val int, impl SorterInterface) int {
	len := to - from
	for len > 0 {
		half := len >> 1
		mid := from + half
		if impl.Compare(mid, val) < 0 {
			from = mid + 1
			len = len - half - 1
		} else {
			len = half
		}
	}
	return from
}

// Upper finds the first position in [from, to) where val is less than the element.
func (s *Sorter) Upper(from, to, val int, impl SorterInterface) int {
	len := to - from
	for len > 0 {
		half := len >> 1
		mid := from + half
		if impl.Compare(val, mid) < 0 {
			len = half
		} else {
			from = mid + 1
			len = len - half - 1
		}
	}
	return from
}

// Lower2 is faster than Lower when val is at the end of [from, to).
func (s *Sorter) Lower2(from, to, val int, impl SorterInterface) int {
	f, t := to-1, to
	for f > from {
		if impl.Compare(f, val) < 0 {
			return s.Lower(f, t, val, impl)
		}
		delta := t - f
		t = f
		f -= delta << 1
	}
	return s.Lower(from, t, val, impl)
}

// Upper2 is faster than Upper when val is at the beginning of [from, to).
func (s *Sorter) Upper2(from, to, val int, impl SorterInterface) int {
	f, t := from, from+1
	for t < to {
		if impl.Compare(t, val) > 0 {
			return s.Upper(f, t, val, impl)
		}
		delta := t - f
		f = t
		t += delta << 1
	}
	return s.Upper(from, to, val, impl)
}

// MergeInPlace merges two sorted ranges [from, mid) and [mid, to) in place.
func (s *Sorter) MergeInPlace(from, mid, to int, impl SorterInterface) {
	if from == mid || mid == to || impl.Compare(mid-1, mid) <= 0 {
		return
	} else if to-from == 2 {
		impl.Swap(mid-1, mid)
		return
	}
	for impl.Compare(from, mid) <= 0 {
		from++
	}
	for impl.Compare(mid-1, to-1) <= 0 {
		to--
	}
	var firstCut, secondCut int
	var len11, len22 int
	if mid-from > to-mid {
		len11 = (mid - from) >> 1
		firstCut = from + len11
		secondCut = s.Lower(mid, to, firstCut, impl)
		len22 = secondCut - mid
	} else {
		len22 = (to - mid) >> 1
		secondCut = mid + len22
		firstCut = s.Upper(from, mid, secondCut, impl)
		len11 = firstCut - from
	}
	s.Rotate(firstCut, mid, secondCut, impl)
	newMid := firstCut + len22
	s.MergeInPlace(from, firstCut, newMid, impl)
	s.MergeInPlace(newMid, secondCut, to, impl)
}

// Constants for sorting algorithms
const (
	// BinarySortThreshold is the size below which binary sort is used.
	BinarySortThreshold = 20

	// InsertionSortThreshold is the size below which insertion sort is used.
	InsertionSortThreshold = 16
)
