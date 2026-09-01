// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "bytes"

// StringSortable implements Sortable and RadixSortable for sorting indices of BytesRefs.
type StringSortable struct {
	data     []BytesRef
	indices  []int
	cmp      func(a, b *BytesRef) int
	scratch1 *BytesRef
	scratch2 *BytesRef
	pivot    *BytesRef
}

func NewStringSortable(data []BytesRef, indices []int, cmp func(a, b *BytesRef) int) *StringSortable {
	return &StringSortable{
		data:     data,
		indices:  indices,
		cmp:      cmp,
		scratch1: NewBytesRefEmpty(),
		scratch2: NewBytesRefEmpty(),
		pivot:    NewBytesRefEmpty(),
	}
}

func (s *StringSortable) Compare(i, j int) int {
	s.get(s.scratch1, i)
	s.get(s.scratch2, j)
	return s.cmp(s.scratch1, s.scratch2)
}

func (s *StringSortable) Swap(i, j int) {
	s.indices[i], s.indices[j] = s.indices[j], s.indices[i]
}

func (s *StringSortable) ByteAt(i, k int) int {
	s.get(s.scratch1, i)
	if k >= s.scratch1.Length {
		return -1
	}
	return int(s.scratch1.Bytes[s.scratch1.Offset+k])
}

func (s *StringSortable) SetPivot(i int) {
	s.get(s.pivot, i)
}

func (s *StringSortable) ComparePivot(j int) int {
	s.get(s.scratch1, j)
	return s.cmp(s.pivot, s.scratch1)
}

func (s *StringSortable) get(dest *BytesRef, i int) {
	ref := s.data[s.indices[i]]
	dest.Bytes = ref.Bytes
	dest.Offset = ref.Offset
	dest.Length = ref.Length
}

// SortBytesRefs sorts a slice of BytesRefs using the most efficient sorter available.
func SortBytesRefs(data []BytesRef, cmp func(a, b *BytesRef) int) {
	indices := make([]int, len(data))
	for i := range indices {
		indices[i] = i
	}

	ss := NewStringSortable(data, indices, cmp)

	maxLength := 0
	for _, ref := range data {
		if ref.Length > maxLength {
			maxLength = ref.Length
		}
	}

	// Try MSBRadixSort
	radix := NewMSBRadixSorter(maxLength)
	radix.Sort(ss, 0, len(indices))

	// Copy sorted indices back to data
	sortedData := make([]BytesRef, len(data))
	for i, idx := range indices {
		sortedData[i] = data[idx]
	}
	copy(data, sortedData)
}

// NaturalComparator provides the natural byte-order comparison for BytesRefs.
func NaturalComparator(a, b *BytesRef) int {
	return bytes.Compare(a.ValidBytes(), b.ValidBytes())
}
