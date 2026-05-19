// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/ArrayInPlaceMergeSorter.java
// Purpose: Tests for ArrayInPlaceMergeSorter (generic object slice
//          InPlaceMergeSorter).

package util

import (
	"math/rand"
	"testing"
)

// arrayInPlaceMergeSorterCmp is a small int comparator shared by the tests.
func arrayInPlaceMergeSorterCmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// TestArrayInPlaceMergeSorter_Empty sorts an empty slice.
func TestArrayInPlaceMergeSorter_Empty(t *testing.T) {
	arr := []int{}
	NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(0, 0)
	if len(arr) != 0 {
		t.Fatal("empty slice mutated")
	}
}

// TestArrayInPlaceMergeSorter_One sorts a single-element slice.
func TestArrayInPlaceMergeSorter_One(t *testing.T) {
	arr := []int{42}
	NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(0, 1)
	if arr[0] != 42 {
		t.Fatalf("single element mutated: got %d", arr[0])
	}
}

// TestArrayInPlaceMergeSorter_Two sorts the two-element edge case.
func TestArrayInPlaceMergeSorter_Two(t *testing.T) {
	arr := []int{2, 1}
	NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(0, 2)
	if arr[0] != 1 || arr[1] != 2 {
		t.Fatalf("two-element sort failed: %v", arr)
	}
}

// TestArrayInPlaceMergeSorter_Small sorts a small fixed slice within the
// insertion sort threshold to exercise the BinarySort fallback.
func TestArrayInPlaceMergeSorter_Small(t *testing.T) {
	arr := []int{5, 3, 8, 1, 4, 2, 7, 6}
	NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(0, len(arr))
	for i, v := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		if arr[i] != v {
			t.Fatalf("idx %d: want %d, got %d (arr=%v)", i, v, arr[i], arr)
		}
	}
}

// TestArrayInPlaceMergeSorter_RandomSmall sorts small random slices and
// checks against an insertion-sorted reference.
func TestArrayInPlaceMergeSorter_RandomSmall(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for trial := 0; trial < 64; trial++ {
		length := 1 + r.Intn(64)
		arr := make([]int, length)
		expected := make([]int, length)
		for i := range arr {
			arr[i] = r.Intn(100)
			expected[i] = arr[i]
		}
		sortInts(expected)

		NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(0, length)

		for i := range arr {
			if arr[i] != expected[i] {
				t.Fatalf("trial %d idx %d: want %d, got %d (arr=%v)",
					trial, i, expected[i], arr[i], arr)
			}
		}
	}
}

// TestArrayInPlaceMergeSorter_DescendingLarge sorts a strictly descending
// slice large enough to exercise multiple in-place merge levels.
func TestArrayInPlaceMergeSorter_DescendingLarge(t *testing.T) {
	const n = 1024
	arr := make([]int, n)
	for i := range arr {
		arr[i] = n - i
	}

	NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(0, n)

	for i := 0; i < n; i++ {
		if arr[i] != i+1 {
			t.Fatalf("idx %d: want %d, got %d", i, i+1, arr[i])
		}
	}
}

// TestArrayInPlaceMergeSorter_LowCardinality sorts a long slice with very
// few distinct values to stress the merge path with many equal keys.
func TestArrayInPlaceMergeSorter_LowCardinality(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	arr := make([]int, 2048)
	expected := make([]int, len(arr))
	for i := range arr {
		arr[i] = r.Intn(3)
		expected[i] = arr[i]
	}
	sortInts(expected)

	NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(0, len(arr))

	for i := range arr {
		if arr[i] != expected[i] {
			t.Fatalf("idx %d: want %d, got %d", i, expected[i], arr[i])
		}
	}
}

// TestArrayInPlaceMergeSorter_SubRange sorts only a middle slice and
// confirms the outside is untouched.
func TestArrayInPlaceMergeSorter_SubRange(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	arr := make([]int, 100)
	for i := range arr {
		arr[i] = r.Intn(1000)
	}
	saved := make([]int, len(arr))
	copy(saved, arr)

	NewArrayInPlaceMergeSorter(arr, arrayInPlaceMergeSorterCmp).Sort(25, 75)

	for i := 26; i < 75; i++ {
		if arr[i] < arr[i-1] {
			t.Fatalf("not sorted at %d: %v", i, arr[25:75])
		}
	}
	for i := 0; i < 25; i++ {
		if arr[i] != saved[i] {
			t.Fatalf("left tail mutated at %d", i)
		}
	}
	for i := 75; i < 100; i++ {
		if arr[i] != saved[i] {
			t.Fatalf("right tail mutated at %d", i)
		}
	}
}

// TestArrayInPlaceMergeSorter_Stable verifies that equal-keyed elements
// preserve their original insertion order, which is the defining property
// of InPlaceMergeSorter over IntroSorter.
func TestArrayInPlaceMergeSorter_Stable(t *testing.T) {
	type kv struct {
		key int
		idx int
	}

	r := rand.New(rand.NewSource(7))
	in := make([]kv, 256)
	for i := range in {
		in[i] = kv{key: r.Intn(8), idx: i}
	}

	NewArrayInPlaceMergeSorter(in, func(a, b kv) int {
		return arrayInPlaceMergeSorterCmp(a.key, b.key)
	}).Sort(0, len(in))

	for i := 1; i < len(in); i++ {
		if in[i].key < in[i-1].key {
			t.Fatalf("not sorted at %d: %+v vs %+v", i, in[i-1], in[i])
		}
		if in[i].key == in[i-1].key && in[i].idx < in[i-1].idx {
			t.Fatalf("not stable at %d: %+v before %+v", i, in[i-1], in[i])
		}
	}
}

// TestArrayInPlaceMergeSorter_GenericStruct exercises the generic parameter
// with a struct payload and a non-trivial comparator.
func TestArrayInPlaceMergeSorter_GenericStruct(t *testing.T) {
	type kv struct {
		key int
		v   string
	}

	in := []kv{
		{3, "c"}, {1, "a"}, {3, "c2"}, {1, "a2"}, {2, "b"}, {3, "c3"}, {1, "a3"}, {2, "b2"},
	}

	NewArrayInPlaceMergeSorter(in, func(a, b kv) int {
		return arrayInPlaceMergeSorterCmp(a.key, b.key)
	}).Sort(0, len(in))

	// Stable sort: equal keys keep their original relative order.
	want := []kv{
		{1, "a"}, {1, "a2"}, {1, "a3"},
		{2, "b"}, {2, "b2"},
		{3, "c"}, {3, "c2"}, {3, "c3"},
	}
	for i := range in {
		if in[i] != want[i] {
			t.Fatalf("idx %d: want %+v, got %+v", i, want[i], in[i])
		}
	}
}
