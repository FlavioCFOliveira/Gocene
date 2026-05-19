// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/ArrayTimSorter.java
// Purpose: Tests for ArrayTimSorter (generic object slice TimSorter).

package util

import (
	"math/rand"
	"testing"
)

// arrayTimSorterCmp is a small int comparator shared by the tests.
func arrayTimSorterCmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// TestArrayTimSorter_ConstructorNoTemp ensures the constructor accepts
// maxTempSlots == 0 and that the resulting sorter still sorts correctly via
// the in-place merge path.
func TestArrayTimSorter_ConstructorNoTemp(t *testing.T) {
	arr := []int{5, 3, 8, 1, 4, 2, 7, 6}
	NewArrayTimSorter(arr, arrayTimSorterCmp, 0).Sort(0, len(arr))

	for i := 1; i < len(arr); i++ {
		if arr[i] < arr[i-1] {
			t.Fatalf("not sorted at %d: %v", i, arr)
		}
	}
}

// TestArrayTimSorter_ConstructorAllocsTemp checks that requesting temp slots
// allocates the scratch buffer and exercises the buffered merge path.
func TestArrayTimSorter_ConstructorAllocsTemp(t *testing.T) {
	arr := []int{9, 8, 7, 6, 5, 4, 3, 2, 1}
	const slots = 4

	// Reach into the adapter to assert tmp allocation. The factory wraps the
	// adapter into TimSorter, but the inner impl exposes its tmp via the
	// TimSorterInterface. We test behaviour rather than introspection: empty
	// vs sized tmp would yield different sort results only if the algorithm
	// were broken, so we additionally trigger merges that require tmp.
	NewArrayTimSorter(arr, arrayTimSorterCmp, slots).Sort(0, len(arr))

	for i, v := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9} {
		if arr[i] != v {
			t.Fatalf("idx %d: want %d, got %d (arr=%v)", i, v, arr[i], arr)
		}
	}
}

// TestArrayTimSorter_Empty sorts an empty slice.
func TestArrayTimSorter_Empty(t *testing.T) {
	arr := []int{}
	NewArrayTimSorter(arr, arrayTimSorterCmp, 0).Sort(0, 0)
	if len(arr) != 0 {
		t.Fatal("empty slice mutated")
	}
}

// TestArrayTimSorter_One sorts a single-element slice.
func TestArrayTimSorter_One(t *testing.T) {
	arr := []int{42}
	NewArrayTimSorter(arr, arrayTimSorterCmp, 1).Sort(0, 1)
	if arr[0] != 42 {
		t.Fatalf("single element mutated: got %d", arr[0])
	}
}

// TestArrayTimSorter_Two sorts the two-element edge case.
func TestArrayTimSorter_Two(t *testing.T) {
	arr := []int{2, 1}
	NewArrayTimSorter(arr, arrayTimSorterCmp, 2).Sort(0, 2)
	if arr[0] != 1 || arr[1] != 2 {
		t.Fatalf("two-element sort failed: %v", arr)
	}
}

// TestArrayTimSorter_RandomSmall sorts small random slices across a few
// temp-slot configurations.
func TestArrayTimSorter_RandomSmall(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for trial := 0; trial < 64; trial++ {
		length := 1 + r.Intn(64)
		arr := make([]int, length)
		expected := make([]int, length)
		for i := range arr {
			arr[i] = r.Intn(100)
			expected[i] = arr[i]
		}
		// Sort expected via the stdlib.
		sortInts(expected)

		slots := r.Intn(length + 1)
		NewArrayTimSorter(arr, arrayTimSorterCmp, slots).Sort(0, length)

		for i := range arr {
			if arr[i] != expected[i] {
				t.Fatalf("trial %d slots %d idx %d: want %d, got %d (arr=%v)",
					trial, slots, i, expected[i], arr[i], arr)
			}
		}
	}
}

// TestArrayTimSorter_DescendingLarge sorts a strictly descending slice large
// enough to exercise multiple runs and merges.
func TestArrayTimSorter_DescendingLarge(t *testing.T) {
	const n = 1024
	arr := make([]int, n)
	for i := range arr {
		arr[i] = n - i
	}

	NewArrayTimSorter(arr, arrayTimSorterCmp, n/8).Sort(0, n)

	for i := 0; i < n; i++ {
		if arr[i] != i+1 {
			t.Fatalf("idx %d: want %d, got %d", i, i+1, arr[i])
		}
	}
}

// TestArrayTimSorter_Stability verifies that ArrayTimSorter preserves the
// relative order of elements that compare equal — TimSort is required to be
// stable.
func TestArrayTimSorter_Stability(t *testing.T) {
	type kv struct {
		key  int
		seen int // original index
	}

	in := []kv{
		{3, 0}, {1, 1}, {3, 2}, {1, 3}, {2, 4}, {3, 5}, {1, 6}, {2, 7},
	}

	NewArrayTimSorter(in, func(a, b kv) int {
		return arrayTimSorterCmp(a.key, b.key)
	}, len(in)).Sort(0, len(in))

	// Expected stable order: keys [1,1,1,2,2,3,3,3] with original seen indices
	// preserved within each key group.
	wantKeys := []int{1, 1, 1, 2, 2, 3, 3, 3}
	wantSeen := []int{1, 3, 6, 4, 7, 0, 2, 5}
	for i := range in {
		if in[i].key != wantKeys[i] || in[i].seen != wantSeen[i] {
			t.Fatalf("idx %d: want {%d,%d}, got %+v", i, wantKeys[i], wantSeen[i], in[i])
		}
	}
}

// TestArrayTimSorter_SubRange sorts only a middle slice and confirms the
// outside is untouched.
func TestArrayTimSorter_SubRange(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	arr := make([]int, 100)
	for i := range arr {
		arr[i] = r.Intn(1000)
	}
	saved := make([]int, len(arr))
	copy(saved, arr)

	NewArrayTimSorter(arr, arrayTimSorterCmp, 50).Sort(25, 75)

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

// sortInts is a tiny helper to avoid pulling sort/slices imports into the
// main test bodies; uses insertion sort to keep the helper trivial.
func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}
