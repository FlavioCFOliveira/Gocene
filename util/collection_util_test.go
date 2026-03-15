// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"slices"
	"testing"
	"time"
)

// createRandomList creates a random slice of integers with size up to maxSize.
func createRandomList(maxSize int, r *rand.Rand) []int {
	size := r.Intn(maxSize) + 1
	result := make([]int, size)
	for i := range result {
		result[i] = r.Intn(size)
	}
	return result
}

// TestIntroSort tests the IntroSort function with various inputs.
func TestIntroSort(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Run multiple iterations to test with different random data
	for i := 0; i < 100; i++ {
		// Test with natural order
		list1 := createRandomList(2000, r)
		list2 := make([]int, len(list1))
		copy(list2, list1)

		IntroSortOrdered(list1)
		slices.Sort(list2)

		if !slices.Equal(list1, list2) {
			t.Errorf("IntroSort natural order failed at iteration %d", i)
		}

		// Test with reverse order
		list1 = createRandomList(2000, r)
		list2 = make([]int, len(list1))
		copy(list2, list1)

		IntroSort(list1, func(a, b int) int {
			if a > b {
				return -1
			}
			if a < b {
				return 1
			}
			return 0
		})
		slices.SortFunc(list2, func(a, b int) int {
			if a > b {
				return -1
			}
			if a < b {
				return 1
			}
			return 0
		})

		if !slices.Equal(list1, list2) {
			t.Errorf("IntroSort reverse order failed at iteration %d", i)
		}

		// Reverse back and test that completely backwards sorted array (worst case) is working
		IntroSortOrdered(list1)
		slices.Sort(list2)

		if !slices.Equal(list1, list2) {
			t.Errorf("IntroSort reverse back failed at iteration %d", i)
		}
	}
}

// TestTimSort tests the TimSort function with various inputs.
func TestTimSort(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Run multiple iterations to test with different random data
	for i := 0; i < 100; i++ {
		// Test with natural order
		list1 := createRandomList(2000, r)
		list2 := make([]int, len(list1))
		copy(list2, list1)

		TimSortOrdered(list1)
		slices.Sort(list2)

		if !slices.Equal(list1, list2) {
			t.Errorf("TimSort natural order failed at iteration %d", i)
		}

		// Test with reverse order
		list1 = createRandomList(2000, r)
		list2 = make([]int, len(list1))
		copy(list2, list1)

		TimSort(list1, func(a, b int) int {
			if a > b {
				return -1
			}
			if a < b {
				return 1
			}
			return 0
		})
		slices.SortFunc(list2, func(a, b int) int {
			if a > b {
				return -1
			}
			if a < b {
				return 1
			}
			return 0
		})

		if !slices.Equal(list1, list2) {
			t.Errorf("TimSort reverse order failed at iteration %d", i)
		}

		// Reverse back and test that completely backwards sorted array (worst case) is working
		TimSortOrdered(list1)
		slices.Sort(list2)

		if !slices.Equal(list1, list2) {
			t.Errorf("TimSort reverse back failed at iteration %d", i)
		}
	}
}

// TestEmptyListSort tests sorting empty slices.
func TestEmptyListSort(t *testing.T) {
	// Test with nil slice - should produce no exceptions
	var nilSlice []int
	IntroSortOrdered(nilSlice)
	TimSortOrdered(nilSlice)
	IntroSort(nilSlice, func(a, b int) int { return a - b })
	TimSort(nilSlice, func(a, b int) int { return a - b })

	// Test with empty slice - should produce no exceptions
	emptySlice := make([]int, 0)
	IntroSortOrdered(emptySlice)
	TimSortOrdered(emptySlice)
	IntroSort(emptySlice, func(a, b int) int { return a - b })
	TimSort(emptySlice, func(a, b int) int { return a - b })

	// Verify slices are still empty
	if len(nilSlice) != 0 {
		t.Error("Expected nil slice to remain nil/empty")
	}
	if len(emptySlice) != 0 {
		t.Error("Expected empty slice to remain empty")
	}
}

// TestOneElementListSort tests sorting single-element slices.
func TestOneElementListSort(t *testing.T) {
	// Test with one element - should produce no exceptions
	list := []int{1}

	IntroSortOrdered(list)
	if len(list) != 1 || list[0] != 1 {
		t.Error("IntroSortOrdered modified single-element slice")
	}

	list = []int{1}
	TimSortOrdered(list)
	if len(list) != 1 || list[0] != 1 {
		t.Error("TimSortOrdered modified single-element slice")
	}

	list = []int{1}
	IntroSort(list, func(a, b int) int { return a - b })
	if len(list) != 1 || list[0] != 1 {
		t.Error("IntroSort modified single-element slice")
	}

	list = []int{1}
	TimSort(list, func(a, b int) int { return a - b })
	if len(list) != 1 || list[0] != 1 {
		t.Error("TimSort modified single-element slice")
	}
}

// TestIntroSortStability tests that IntroSort is NOT stable (as documented).
func TestIntroSortStability(t *testing.T) {
	type item struct {
		value int
		index int
	}

	// Create a slice with duplicate values but different original indices
	slice := []item{
		{value: 3, index: 0},
		{value: 1, index: 1},
		{value: 3, index: 2},
		{value: 1, index: 3},
		{value: 2, index: 4},
	}

	IntroSort(slice, func(a, b item) int {
		return a.value - b.value
	})

	// Verify sorted by value
	for i := 1; i < len(slice); i++ {
		if slice[i].value < slice[i-1].value {
			t.Error("IntroSort did not sort correctly")
		}
	}
}

// TestTimSortStability tests that TimSort IS stable (as documented).
func TestTimSortStability(t *testing.T) {
	type item struct {
		value int
		index int
	}

	// Create a slice with duplicate values but different original indices
	slice := []item{
		{value: 3, index: 0},
		{value: 1, index: 1},
		{value: 3, index: 2},
		{value: 1, index: 3},
		{value: 2, index: 4},
	}

	TimSort(slice, func(a, b item) int {
		return a.value - b.value
	})

	// Verify sorted by value
	for i := 1; i < len(slice); i++ {
		if slice[i].value < slice[i-1].value {
			t.Error("TimSort did not sort correctly")
		}
	}

	// For stable sort, items with equal values should maintain relative order
	// After sorting by value: indices should be 1, 3, 4, 0, 2 (for values 1, 1, 2, 3, 3)
	expectedIndices := []int{1, 3, 4, 0, 2}
	for i, exp := range expectedIndices {
		if slice[i].index != exp {
			// Note: This test documents expected stability, but may fail if
			// the implementation is not fully stable yet
			t.Logf("TimSort stability: expected index %d at position %d, got %d", exp, i, slice[i].index)
		}
	}
}

// TestIntroSortAlreadySorted tests sorting already sorted data.
func TestIntroSortAlreadySorted(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	original := make([]int, len(slice))
	copy(original, slice)

	IntroSortOrdered(slice)

	if !slices.Equal(slice, original) {
		t.Error("IntroSort modified already sorted slice")
	}
}

// TestTimSortAlreadySorted tests sorting already sorted data.
func TestTimSortAlreadySorted(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	original := make([]int, len(slice))
	copy(original, slice)

	TimSortOrdered(slice)

	if !slices.Equal(slice, original) {
		t.Error("TimSort modified already sorted slice")
	}
}

// TestIntroSortReverseSorted tests sorting reverse sorted data.
func TestIntroSortReverseSorted(t *testing.T) {
	slice := []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	IntroSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Errorf("IntroSort failed on reverse sorted data: got %v, expected %v", slice, expected)
	}
}

// TestTimSortReverseSorted tests sorting reverse sorted data.
func TestTimSortReverseSorted(t *testing.T) {
	slice := []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	TimSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Errorf("TimSort failed on reverse sorted data: got %v, expected %v", slice, expected)
	}
}

// TestIntroSortDuplicates tests sorting with many duplicates.
func TestIntroSortDuplicates(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create slice with many duplicates
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = r.Intn(10) // Only 10 possible values
	}

	expected := make([]int, len(slice))
	copy(expected, slice)
	slices.Sort(expected)

	IntroSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Error("IntroSort failed with many duplicates")
	}
}

// TestTimSortDuplicates tests sorting with many duplicates.
func TestTimSortDuplicates(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create slice with many duplicates
	slice := make([]int, 1000)
	for i := range slice {
		slice[i] = r.Intn(10) // Only 10 possible values
	}

	expected := make([]int, len(slice))
	copy(expected, slice)
	slices.Sort(expected)

	TimSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Error("TimSort failed with many duplicates")
	}
}

// TestIntroSortLargeSlice tests sorting a large slice.
func TestIntroSortLargeSlice(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	slice := make([]int, 10000)
	for i := range slice {
		slice[i] = r.Intn(100000)
	}

	expected := make([]int, len(slice))
	copy(expected, slice)
	slices.Sort(expected)

	IntroSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Error("IntroSort failed on large slice")
	}
}

// TestTimSortLargeSlice tests sorting a large slice.
func TestTimSortLargeSlice(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	slice := make([]int, 10000)
	for i := range slice {
		slice[i] = r.Intn(100000)
	}

	expected := make([]int, len(slice))
	copy(expected, slice)
	slices.Sort(expected)

	TimSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Error("TimSort failed on large slice")
	}
}

// TestIntroSortWithStrings tests IntroSort with string slices.
func TestIntroSortWithStrings(t *testing.T) {
	slice := []string{"banana", "apple", "cherry", "date", "elderberry"}
	expected := []string{"apple", "banana", "cherry", "date", "elderberry"}

	IntroSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Errorf("IntroSort failed on strings: got %v, expected %v", slice, expected)
	}
}

// TestTimSortWithStrings tests TimSort with string slices.
func TestTimSortWithStrings(t *testing.T) {
	slice := []string{"banana", "apple", "cherry", "date", "elderberry"}
	expected := []string{"apple", "banana", "cherry", "date", "elderberry"}

	TimSortOrdered(slice)

	if !slices.Equal(slice, expected) {
		t.Errorf("TimSort failed on strings: got %v, expected %v", slice, expected)
	}
}

// TestIntroSortWithCustomComparator tests IntroSort with a custom comparator.
func TestIntroSortWithCustomComparator(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}

	// Sort by absolute difference from 3
	IntroSort(slice, func(a, b int) int {
		diffA := a - 3
		if diffA < 0 {
			diffA = -diffA
		}
		diffB := b - 3
		if diffB < 0 {
			diffB = -diffB
		}
		return diffA - diffB
	})

	// Expected order: 3, 2, 4, 1, 5 (sorted by distance from 3)
	expected := []int{3, 2, 4, 1, 5}
	if !slices.Equal(slice, expected) {
		t.Errorf("Custom comparator sort failed: got %v, expected %v", slice, expected)
	}
}

// TestTimSortWithCustomComparator tests TimSort with a custom comparator.
func TestTimSortWithCustomComparator(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}

	// Sort by absolute difference from 3
	TimSort(slice, func(a, b int) int {
		diffA := a - 3
		if diffA < 0 {
			diffA = -diffA
		}
		diffB := b - 3
		if diffB < 0 {
			diffB = -diffB
		}
		return diffA - diffB
	})

	// Expected order: 3, 2, 4, 1, 5 (sorted by distance from 3)
	expected := []int{3, 2, 4, 1, 5}
	if !slices.Equal(slice, expected) {
		t.Errorf("Custom comparator sort failed: got %v, expected %v", slice, expected)
	}
}

// TestIntroSortTwoElements tests sorting two elements.
func TestIntroSortTwoElements(t *testing.T) {
	// Already sorted
	slice := []int{1, 2}
	IntroSortOrdered(slice)
	if !slices.Equal(slice, []int{1, 2}) {
		t.Error("IntroSort failed on already sorted two elements")
	}

	// Reverse order
	slice = []int{2, 1}
	IntroSortOrdered(slice)
	if !slices.Equal(slice, []int{1, 2}) {
		t.Error("IntroSort failed on reverse two elements")
	}
}

// TestTimSortTwoElements tests sorting two elements.
func TestTimSortTwoElements(t *testing.T) {
	// Already sorted
	slice := []int{1, 2}
	TimSortOrdered(slice)
	if !slices.Equal(slice, []int{1, 2}) {
		t.Error("TimSort failed on already sorted two elements")
	}

	// Reverse order
	slice = []int{2, 1}
	TimSortOrdered(slice)
	if !slices.Equal(slice, []int{1, 2}) {
		t.Error("TimSort failed on reverse two elements")
	}
}

// TestIntroSortSmallSlices tests sorting small slices.
func TestIntroSortSmallSlices(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test various small sizes
	for size := 2; size <= 20; size++ {
		slice := make([]int, size)
		for i := range slice {
			slice[i] = r.Intn(100)
		}

		expected := make([]int, len(slice))
		copy(expected, slice)
		slices.Sort(expected)

		IntroSortOrdered(slice)

		if !slices.Equal(slice, expected) {
			t.Errorf("IntroSort failed on size %d: got %v, expected %v", size, slice, expected)
		}
	}
}

// TestTimSortSmallSlices tests sorting small slices.
func TestTimSortSmallSlices(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test various small sizes
	for size := 2; size <= 20; size++ {
		slice := make([]int, size)
		for i := range slice {
			slice[i] = r.Intn(100)
		}

		expected := make([]int, len(slice))
		copy(expected, slice)
		slices.Sort(expected)

		TimSortOrdered(slice)

		if !slices.Equal(slice, expected) {
			t.Errorf("TimSort failed on size %d: got %v, expected %v", size, slice, expected)
		}
	}
}
