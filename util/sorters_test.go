// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"reflect"
	"testing"
)

type testSortable struct {
	data    []int
	indices []int
	pivot   int
}

func (s *testSortable) Compare(i, j int) int {
	a, b := s.data[s.indices[i]], s.data[s.indices[j]]
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func (s *testSortable) Swap(i, j int) {
	s.indices[i], s.indices[j] = s.indices[j], s.indices[i]
}

func (s *testSortable) SetPivot(i int) {
	s.pivot = s.data[s.indices[i]]
}

func (s *testSortable) ComparePivot(j int) int {
	b := s.data[s.indices[j]]
	if s.pivot < b {
		return -1
	}
	if s.pivot > b {
		return 1
	}
	return 0
}

func TestIntroSorter(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{"empty", []int{}, []int{}},
		{"single", []int{1}, []int{1}},
		{"sorted", []int{1, 2, 3, 4, 5}, []int{1, 2, 3, 4, 5}},
		{"reverse", []int{5, 4, 3, 2, 1}, []int{1, 2, 3, 4, 5}},
		{"random", []int{3, 1, 4, 1, 5, 9, 2, 6, 5}, []int{1, 1, 2, 3, 4, 5, 5, 6, 9}},
		{"duplicates", []int{2, 2, 2, 2}, []int{2, 2, 2, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indices := make([]int, len(tt.input))
			for i := range indices {
				indices[i] = i
			}
			s := &testSortable{data: tt.input, indices: indices}
			NewIntroSorter(s).Sort(0, len(indices))

			result := make([]int, len(tt.input))
			for i, idx := range indices {
				result[i] = tt.input[idx]
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("IntroSort() = %v, want %v", result, tt.expected)
			}
		})
	}
}

type testRadixSortable struct {
	data    [][]byte
	indices []int
	pivot   []byte
}

func (s *testRadixSortable) Compare(i, j int) int {
	a, b := s.data[s.indices[i]], s.data[s.indices[j]]
	res := bytesCompare(a, b)
	return res
}

func (s *testRadixSortable) Swap(i, j int) {
	s.indices[i], s.indices[j] = s.indices[j], s.indices[i]
}

func (s *testRadixSortable) ByteAt(i, k int) int {
	ref := s.data[s.indices[i]]
	if k >= len(ref) {
		return -1
	}
	return int(ref[k])
}

func (s *testRadixSortable) SetPivot(i int) {
	s.pivot = s.data[s.indices[i]]
}

func (s *testRadixSortable) ComparePivot(j int) int {
	b := s.data[s.indices[j]]
	return bytesCompare(s.pivot, b)
}

func TestMSBRadixSort(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]byte
		expected [][]byte
	}{
		{"empty", [][]byte{}, [][]byte{}},
		{"single", [][]byte{[]byte("a")}, [][]byte{[]byte("a")}},
		{"sorted", [][]byte{[]byte("a"), []byte("b"), []byte("c")}, [][]byte{[]byte("a"), []byte("b"), []byte("c")}},
		{"reverse", [][]byte{[]byte("c"), []byte("b"), []byte("a")}, [][]byte{[]byte("a"), []byte("b"), []byte("c")}},
		{"random", [][]byte{[]byte("banana"), []byte("apple"), []byte("cherry"), []byte("date")}, [][]byte{[]byte("apple"), []byte("banana"), []byte("cherry"), []byte("date")}},
		{"prefixes", [][]byte{[]byte("apple"), []byte("app"), []byte("apply")}, [][]byte{[]byte("app"), []byte("apple"), []byte("apply")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indices := make([]int, len(tt.input))
			for i := range indices {
				indices[i] = i
			}
			s := &testRadixSortable{data: tt.input, indices: indices}

			maxLength := 0
			for _, ref := range tt.input {
				if len(ref) > maxLength {
					maxLength = len(ref)
				}
			}

			NewMSBRadixSorter(maxLength).Sort(s, 0, len(indices))

			result := make([][]byte, len(tt.input))
			for i, idx := range indices {
				result[i] = tt.input[idx]
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("MSBRadixSort() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStringSort(t *testing.T) {
	data := []BytesRef{
		{Bytes: []byte("banana"), Offset: 0, Length: 6},
		{Bytes: []byte("apple"), Offset: 0, Length: 5},
		{Bytes: []byte("cherry"), Offset: 0, Length: 6},
		{Bytes: []byte("date"), Offset: 0, Length: 4},
	}

	input := make([]BytesRef, len(data))
	copy(input, data)

	SortBytesRefs(input, NaturalComparator)

	expected := []string{"apple", "banana", "cherry", "date"}
	for i, ref := range input {
		if string(ref.ValidBytes()) != expected[i] {
			t.Errorf("SortBytesRefs() at %d = %s, want %s", i, string(ref.ValidBytes()), expected[i])
		}
	}
}
