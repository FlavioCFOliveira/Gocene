// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// Sortable defines the interface for a data structure that can be sorted.
// This mirrors the internal contract of Lucene's Sorter class.
type Sortable interface {
	// Compare entries found in slots i and j.
	// The contract for the returned value is the same as a standard comparator.
	Compare(i, j int) int
	// Swap values at slots i and j.
	Swap(i, j int)
}

// Pivotable extends Sortable to allow stable pivot operations.
// This is necessary for algorithms like IntroSort that move elements
// while comparing them against a fixed pivot value.
type Pivotable interface {
	Sortable
	// SetPivot saves the value at slot i so that it can later be used as a pivot.
	SetPivot(i int)
	// ComparePivot compares the saved pivot with the slot at j.
	ComparePivot(j int) int
}

// RadixSortable extends Sortable for radix sorting algorithms.
type RadixSortable interface {
	Sortable
	// ByteAt returns the k-th byte of the entry at index i, or -1 if its length is less than or equal to k.
	ByteAt(i, k int) int
}

// MSBRadixSorterImpl is a wrapper for RadixSortable, used in stable sorters.
type MSBRadixSorterImpl interface {
	RadixSortable
}

// StringSorterImpl defines the interface for a data structure that can be
// sorted using a string-aware sorter (radix or merge sort).
type StringSorterImpl interface {
	// Get resolves the value at slot i into a BytesRef.
	Get(builder *BytesRefBuilder, out *BytesRef, i int)
	// Swap values at slots i and j.
	Swap(i, j int)
	// Save writes the value at slot i into the j-th position in the
	// caller's scratch storage.
	Save(i, j int)
	// Restore copies the scratch values back into slots [i, j) of the
	// caller's primary storage.
	Restore(i, j int)
}

const (
	// BinarySortThreshold is the size threshold below which binary sort is used.
	BinarySortThreshold = 20
	// InsertionSortThreshold is the size threshold below which insertion sort is used.
	InsertionSortThreshold = 16
)

// Reverse reverses the elements in the range [from, to).
func Reverse(s Sortable, from, to int) {
	to--
	for from < to {
		s.Swap(from, to)
		from++
		to--
	}
}

// Rotate rotates the elements in the range [lo, hi) so that the element
// at mid becomes the first element.
func Rotate(s Sortable, lo, mid, hi int) {
	if lo == mid || mid == hi {
		return
	}
	DoRotate(s, lo, mid, hi)
}

// DoRotate performs the actual rotation.
func DoRotate(s Sortable, lo, mid, hi int) {
	if mid-lo == hi-mid {
		for mid < hi {
			s.Swap(lo, mid)
			lo++
			mid++
		}
	} else {
		Reverse(s, lo, mid)
		Reverse(s, mid, hi)
		Reverse(s, lo, hi)
	}
}

// Lower returns the index of the first element in [from, to) that is not less than val.
func Lower(s Sortable, from, to, val int) int {
	len_ := to - from
	for len_ > 0 {
		half := len_ >> 1
		mid := from + half
		if s.Compare(mid, val) < 0 {
			from = mid + 1
			len_ = len_ - half - 1
		} else {
			len_ = half
		}
	}
	return from
}

// Upper returns the index of the first element in [from, to) that is greater than val.
func Upper(s Sortable, from, to, val int) int {
	len_ := to - from
	for len_ > 0 {
		half := len_ >> 1
		mid := from + half
		if s.Compare(val, mid) < 0 {
			len_ = half
		} else {
			from = mid + 1
			len_ = len_ - half - 1
		}
	}
	return from
}

// Lower2 is faster than Lower when val is at the end of [from, to).
func Lower2(s Sortable, from, to, val int) int {
	f, t := to-1, to
	for f > from {
		if s.Compare(f, val) < 0 {
			return Lower(s, f, t, val)
		}
		delta := t - f
		t = f
		f -= delta << 1
	}
	return Lower(s, from, t, val)
}

// Upper2 is faster than Upper when val is at the beginning of [from, to).
func Upper2(s Sortable, from, to, val int) int {
	f, t := from, from+1
	for t < to {
		if s.Compare(t, val) > 0 {
			return Upper(s, f, t, val)
		}
		delta := t - f
		f = t
		t += delta << 1
	}
	return Upper(s, f, to, val)
}

// MergeInPlace merges two sorted ranges [from, mid) and [mid, to) in place.
func MergeInPlace(s Sortable, from, mid, to int) {
	if from == mid || mid == to || s.Compare(mid-1, mid) <= 0 {
		return
	} else if to-from == 2 {
		s.Swap(mid-1, mid)
		return
	}
	for s.Compare(from, mid) <= 0 {
		from++
	}
	for s.Compare(mid-1, to-1) <= 0 {
		to--
	}
	var firstCut, secondCut int
	var len11, len22 int
	if mid-from > to-mid {
		len11 = (mid - from) >> 1
		firstCut = from + len11
		secondCut = Lower(s, mid, to, firstCut)
		len22 = secondCut - mid
	} else {
		len22 = (to - mid) >> 1
		secondCut = mid + len22
		firstCut = Upper(s, from, mid, secondCut)
		len11 = firstCut - from
	}
	Rotate(s, firstCut, mid, secondCut)
	newMid := firstCut + len22
	MergeInPlace(s, from, firstCut, newMid)
	MergeInPlace(s, newMid, secondCut, to)
}

// BinarySort sorts the range [from, to) using binary sort.
func BinarySort(s Sortable, from, to int) {
	binarySort(s, from, to, from+1)
}

// BinarySortWithStart sorts the range [from, to) using binary sort, starting from index i.
func BinarySortWithStart(s Sortable, from, to, i int) {
	binarySort(s, from, to, i)
}

func binarySort(s Sortable, from, to, i int) {
	for ; i < to; i++ {
		pivot := i
		l := from
		h := i - 1
		for l <= h {
			mid := (l + h) >> 1
			if s.Compare(mid, pivot) < 0 {
				h = mid - 1
			} else {
				l = mid + 1
			}
		}
		for j := i; j > l; j-- {
			s.Swap(j-1, j)
		}
	}
}

// InsertionSort sorts the range [from, to) using insertion sort.
func InsertionSort(s Sortable, from, to int) {
	for i := from + 1; i < to; {
		current := i
		i++
		previous := current - 1
		for previous >= from && s.Compare(previous, current) > 0 {
			s.Swap(previous, current)
			current = previous
			previous--
		}
	}
}

// HeapSort sorts the range [from, to) using heap sort.
func HeapSort(s Sortable, from, to int) {
	if to-from <= 1 {
		return
	}
	Heapify(s, from, to)
	for end := to - 1; end > from; end-- {
		s.Swap(from, end)
		SiftDown(s, from, from, end)
	}
}

// Heapify builds a heap from the range [from, to).
func Heapify(s Sortable, from, to int) {
	for i := HeapParent(from, to-1); i >= from; i-- {
		SiftDown(s, i, from, to)
	}
}

// SiftDown restores the heap property for the element at index i.
func SiftDown(s Sortable, i, from, to int) {
	for leftChild := HeapChild(from, i); leftChild < to; leftChild = HeapChild(from, i) {
		rightChild := leftChild + 1
		if s.Compare(i, leftChild) < 0 {
			if rightChild < to && s.Compare(leftChild, rightChild) < 0 {
				s.Swap(i, rightChild)
				i = rightChild
			} else {
				s.Swap(i, leftChild)
				i = leftChild
			}
		} else if rightChild < to && s.Compare(i, rightChild) < 0 {
			s.Swap(i, rightChild)
			i = rightChild
		} else {
			break
		}
	}
}

// HeapParent returns the index of the parent of the element at index i.
func HeapParent(from, i int) int {
	return ((i - 1 - from) >> 1) + from
}

// HeapChild returns the index of the left child of the element at index i.
func HeapChild(from, i int) int {
	return ((i - from) << 1) + 1 + from
}
