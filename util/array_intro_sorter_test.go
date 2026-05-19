// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/ArrayIntroSorter.java
// Purpose: Tests for ArrayIntroSorter (generic object slice IntroSorter).

package util

import (
	"math/rand"
	"testing"
)

// arrayIntroSorterCmp is a small int comparator shared by the tests.
func arrayIntroSorterCmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// TestArrayIntroSorter_Empty sorts an empty slice.
func TestArrayIntroSorter_Empty(t *testing.T) {
	arr := []int{}
	NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(0, 0)
	if len(arr) != 0 {
		t.Fatal("empty slice mutated")
	}
}

// TestArrayIntroSorter_One sorts a single-element slice.
func TestArrayIntroSorter_One(t *testing.T) {
	arr := []int{42}
	NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(0, 1)
	if arr[0] != 42 {
		t.Fatalf("single element mutated: got %d", arr[0])
	}
}

// TestArrayIntroSorter_Two sorts the two-element edge case.
func TestArrayIntroSorter_Two(t *testing.T) {
	arr := []int{2, 1}
	NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(0, 2)
	if arr[0] != 1 || arr[1] != 2 {
		t.Fatalf("two-element sort failed: %v", arr)
	}
}

// TestArrayIntroSorter_Small sorts a small fixed slice within the insertion
// sort threshold to exercise the InsertionSort fallback.
func TestArrayIntroSorter_Small(t *testing.T) {
	arr := []int{5, 3, 8, 1, 4, 2, 7, 6}
	NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(0, len(arr))
	for i, v := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		if arr[i] != v {
			t.Fatalf("idx %d: want %d, got %d (arr=%v)", i, v, arr[i], arr)
		}
	}
}

// TestArrayIntroSorter_RandomSmall sorts small random slices and checks
// against an insertion-sorted reference.
func TestArrayIntroSorter_RandomSmall(t *testing.T) {
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

		NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(0, length)

		for i := range arr {
			if arr[i] != expected[i] {
				t.Fatalf("trial %d idx %d: want %d, got %d (arr=%v)",
					trial, i, expected[i], arr[i], arr)
			}
		}
	}
}

// TestArrayIntroSorter_DescendingLarge sorts a strictly descending slice
// large enough to exercise both the median-of-medians pivot path
// (size > SingleMedianThreshold) and the heap-sort fallback if recursion
// were ever to escalate.
func TestArrayIntroSorter_DescendingLarge(t *testing.T) {
	const n = 1024
	arr := make([]int, n)
	for i := range arr {
		arr[i] = n - i
	}

	NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(0, n)

	for i := 0; i < n; i++ {
		if arr[i] != i+1 {
			t.Fatalf("idx %d: want %d, got %d", i, i+1, arr[i])
		}
	}
}

// TestArrayIntroSorter_LowCardinality sorts a long slice with very few
// distinct values to stress the Bentley-McIlroy 3-way partition path.
func TestArrayIntroSorter_LowCardinality(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	arr := make([]int, 2048)
	expected := make([]int, len(arr))
	for i := range arr {
		arr[i] = r.Intn(3)
		expected[i] = arr[i]
	}
	sortInts(expected)

	NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(0, len(arr))

	for i := range arr {
		if arr[i] != expected[i] {
			t.Fatalf("idx %d: want %d, got %d", i, expected[i], arr[i])
		}
	}
}

// TestArrayIntroSorter_SubRange sorts only a middle slice and confirms the
// outside is untouched.
func TestArrayIntroSorter_SubRange(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	arr := make([]int, 100)
	for i := range arr {
		arr[i] = r.Intn(1000)
	}
	saved := make([]int, len(arr))
	copy(saved, arr)

	NewArrayIntroSorter(arr, arrayIntroSorterCmp).Sort(25, 75)

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

// TestArrayIntroSorter_GenericStruct exercises the generic parameter with a
// struct payload and a non-trivial comparator.
func TestArrayIntroSorter_GenericStruct(t *testing.T) {
	type kv struct {
		key int
		v   string
	}

	in := []kv{
		{3, "c"}, {1, "a"}, {3, "c2"}, {1, "a2"}, {2, "b"}, {3, "c3"}, {1, "a3"}, {2, "b2"},
	}

	NewArrayIntroSorter(in, func(a, b kv) int {
		return arrayIntroSorterCmp(a.key, b.key)
	}).Sort(0, len(in))

	// IntroSorter is NOT stable, so we only assert the key ordering.
	wantKeys := []int{1, 1, 1, 2, 2, 3, 3, 3}
	for i := range in {
		if in[i].key != wantKeys[i] {
			t.Fatalf("idx %d: want key %d, got %+v", i, wantKeys[i], in[i])
		}
	}
}
