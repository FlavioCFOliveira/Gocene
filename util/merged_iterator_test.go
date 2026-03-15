// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"testing"
)

// TestMergedIterator_MergeEmpty tests merging empty iterators.
// Source: TestMergedIterator.testMergeEmpty()
// Purpose: Tests that merging empty iterators produces an empty result
func TestMergedIterator_MergeEmpty(t *testing.T) {
	// Test with no iterators
	merged, err := NewMergedIterator()
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}
	if merged.HasNext() {
		t.Error("Expected empty iterator to have no next")
	}

	// Test with single empty iterator
	emptySlice := []int{}
	merged, err = NewMergedIterator(NewIntSliceIterator(emptySlice))
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}
	if merged.HasNext() {
		t.Error("Expected single empty iterator to have no next")
	}

	// Test with multiple empty iterators
	numEmpty := rand.Intn(100)
	iterators := make([]IntIterator, numEmpty)
	for i := 0; i < numEmpty; i++ {
		iterators[i] = NewIntSliceIterator([]int{})
	}
	merged, err = NewMergedIterator(iterators...)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}
	if merged.HasNext() {
		t.Error("Expected multiple empty iterators to have no next")
	}
}

// TestMergedIterator_NoDupsRemoveDups tests merging with no duplicates and removing duplicates.
// Source: TestMergedIterator.testNoDupsRemoveDups()
// Purpose: Tests basic merge with unique values across iterators
func TestMergedIterator_NoDupsRemoveDups(t *testing.T) {
	testCase(t, 1, 1, true)
}

// TestMergedIterator_OffItrDupsRemoveDups tests with duplicates across different iterators.
// Source: TestMergedIterator.testOffItrDupsRemoveDups()
// Purpose: Tests duplicate removal when same value appears in different iterators
func TestMergedIterator_OffItrDupsRemoveDups(t *testing.T) {
	testCase(t, 3, 1, true)
}

// TestMergedIterator_OnItrDupsRemoveDups tests with duplicates within the same iterator.
// Source: TestMergedIterator.testOnItrDupsRemoveDups()
// Purpose: Tests handling of consecutive duplicates in a single iterator
func TestMergedIterator_OnItrDupsRemoveDups(t *testing.T) {
	testCase(t, 1, 3, true)
}

// TestMergedIterator_OnItrRandomDupsRemoveDups tests with random duplicates within iterators.
// Source: TestMergedIterator.testOnItrRandomDupsRemoveDups()
// Purpose: Tests handling of random consecutive duplicates
func TestMergedIterator_OnItrRandomDupsRemoveDups(t *testing.T) {
	testCase(t, 1, -3, true)
}

// TestMergedIterator_BothDupsRemoveDups tests with duplicates both within and across iterators.
// Source: TestMergedIterator.testBothDupsRemoveDups()
// Purpose: Tests comprehensive duplicate removal
func TestMergedIterator_BothDupsRemoveDups(t *testing.T) {
	testCase(t, 3, 3, true)
}

// TestMergedIterator_BothDupsWithRandomDupsRemoveDups tests with random duplicates.
// Source: TestMergedIterator.testBothDupsWithRandomDupsRemoveDups()
// Purpose: Tests duplicate removal with random distribution
func TestMergedIterator_BothDupsWithRandomDupsRemoveDups(t *testing.T) {
	testCase(t, 3, -3, true)
}

// TestMergedIterator_NoDupsKeepDups tests merging with no duplicates while keeping duplicates.
// Source: TestMergedIterator.testNoDupsKeepDups()
// Purpose: Tests basic merge without duplicate removal
func TestMergedIterator_NoDupsKeepDups(t *testing.T) {
	testCase(t, 1, 1, false)
}

// TestMergedIterator_OffItrDupsKeepDups tests with duplicates across iterators while keeping them.
// Source: TestMergedIterator.testOffItrDupsKeepDups()
// Purpose: Tests that duplicates across iterators are preserved
func TestMergedIterator_OffItrDupsKeepDups(t *testing.T) {
	testCase(t, 3, 1, false)
}

// TestMergedIterator_OnItrDupsKeepDups tests with duplicates within iterators while keeping them.
// Source: TestMergedIterator.testOnItrDupsKeepDups()
// Purpose: Tests that consecutive duplicates are preserved
func TestMergedIterator_OnItrDupsKeepDups(t *testing.T) {
	testCase(t, 1, 3, false)
}

// TestMergedIterator_OnItrRandomDupsKeepDups tests with random duplicates while keeping them.
// Source: TestMergedIterator.testOnItrRandomDupsKeepDups()
// Purpose: Tests preservation of random duplicates
func TestMergedIterator_OnItrRandomDupsKeepDups(t *testing.T) {
	testCase(t, 1, -3, false)
}

// TestMergedIterator_BothDupsKeepDups tests with all duplicates while keeping them.
// Source: TestMergedIterator.testBothDupsKeepDups()
// Purpose: Tests comprehensive duplicate preservation
func TestMergedIterator_BothDupsKeepDups(t *testing.T) {
	testCase(t, 3, 3, false)
}

// TestMergedIterator_BothDupsWithRandomDupsKeepDups tests with random duplicates preserved.
// Source: TestMergedIterator.testBothDupsWithRandomDupsKeepDups()
// Purpose: Tests preservation of randomly distributed duplicates
func TestMergedIterator_BothDupsWithRandomDupsKeepDups(t *testing.T) {
	testCase(t, 3, -3, false)
}

// testCase is the main test helper that implements the test logic from Java.
// itrsWithVal: number of iterators that will have each value
// specifiedValsOnItr: number of times each value appears on an iterator (negative means random 1 to abs(value))
// removeDups: whether to remove duplicates
func testCase(t *testing.T, itrsWithVal int, specifiedValsOnItr int, removeDups bool) {
	const valsToMerge = 15000

	// Build a random number of lists
	expected := make([]int, 0, valsToMerge)
	r := rand.New(rand.NewSource(rand.Int63()))
	numLists := itrsWithVal + r.Intn(1000-itrsWithVal)
	lists := make([][]int, numLists)
	for i := range lists {
		lists[i] = make([]int, 0)
	}

	start := r.Intn(1000000)
	absValsOnItr := specifiedValsOnItr
	if absValsOnItr < 0 {
		absValsOnItr = -absValsOnItr
	}
	end := start + valsToMerge/itrsWithVal/absValsOnItr

	for i := start; i < end; i++ {
		maxList := len(lists)
		maxValsOnItr := 0
		sumValsOnItr := 0

		for itrWithVal := 0; itrWithVal < itrsWithVal; itrWithVal++ {
			listIdx := r.Intn(maxList)
			valsOnItr := specifiedValsOnItr
			if valsOnItr < 0 {
				valsOnItr = 1 + r.Intn(-valsOnItr)
			}
			if valsOnItr > maxValsOnItr {
				maxValsOnItr = valsOnItr
			}
			sumValsOnItr += valsOnItr

			for valOnItr := 0; valOnItr < valsOnItr; valOnItr++ {
				lists[listIdx] = append(lists[listIdx], i)
			}

			maxList--
			// Swap to avoid reusing the same list
			lists[listIdx], lists[maxList] = lists[maxList], lists[listIdx]
		}

		maxCount := sumValsOnItr
		if removeDups {
			maxCount = maxValsOnItr
		}
		for count := 0; count < maxCount; count++ {
			expected = append(expected, i)
		}
	}

	// Create iterators from lists
	iterators := make([]IntIterator, numLists)
	for i := 0; i < numLists; i++ {
		iterators[i] = NewIntSliceIterator(lists[i])
	}

	// Create merged iterator
	merged, err := NewMergedIteratorWithOptions(removeDups, iterators...)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	// Verify merged output matches expected
	expectedIdx := 0
	for merged.HasNext() {
		if expectedIdx >= len(expected) {
			t.Fatalf("Merged iterator has more elements than expected (expected %d)", len(expected))
		}
		actual := merged.Next()
		if actual != expected[expectedIdx] {
			t.Errorf("Expected %d at position %d, got %d", expected[expectedIdx], expectedIdx, actual)
		}
		expectedIdx++
	}

	if expectedIdx != len(expected) {
		t.Errorf("Expected %d elements, got %d", len(expected), expectedIdx)
	}
}

// TestMergedIterator_BasicMerge tests basic merging of sorted iterators.
// Purpose: Verifies correct merge order with simple test cases
func TestMergedIterator_BasicMerge(t *testing.T) {
	// Test merging two sorted iterators
	iter1 := NewIntSliceIterator([]int{1, 3, 5, 7})
	iter2 := NewIntSliceIterator([]int{2, 4, 6, 8})

	merged, err := NewMergedIterator(iter1, iter2)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	expected := []int{1, 2, 3, 4, 5, 6, 7, 8}
	for i, exp := range expected {
		if !merged.HasNext() {
			t.Fatalf("Expected more elements at position %d", i)
		}
		actual := merged.Next()
		if actual != exp {
			t.Errorf("Position %d: expected %d, got %d", i, exp, actual)
		}
	}

	if merged.HasNext() {
		t.Error("Expected no more elements")
	}
}

// TestMergedIterator_DuplicateHandling tests duplicate value handling.
// Purpose: Verifies correct behavior with duplicate values
func TestMergedIterator_DuplicateHandling(t *testing.T) {
	// Test with duplicates across iterators - remove dups
	iter1 := NewIntSliceIterator([]int{1, 2, 3})
	iter2 := NewIntSliceIterator([]int{2, 3, 4})

	merged, err := NewMergedIteratorWithOptions(true, iter1, iter2)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	expected := []int{1, 2, 3, 4}
	for i, exp := range expected {
		if !merged.HasNext() {
			t.Fatalf("Expected more elements at position %d", i)
		}
		actual := merged.Next()
		if actual != exp {
			t.Errorf("Position %d: expected %d, got %d", i, exp, actual)
		}
	}

	// Test with duplicates - keep dups
	iter1 = NewIntSliceIterator([]int{1, 2, 3})
	iter2 = NewIntSliceIterator([]int{2, 3, 4})

	merged, err = NewMergedIteratorWithOptions(false, iter1, iter2)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	expected = []int{1, 2, 2, 3, 3, 4}
	for i, exp := range expected {
		if !merged.HasNext() {
			t.Fatalf("Expected more elements at position %d", i)
		}
		actual := merged.Next()
		if actual != exp {
			t.Errorf("Position %d: expected %d, got %d", i, exp, actual)
		}
	}
}

// TestMergedIterator_SingleIterator tests with a single iterator.
// Purpose: Verifies that single iterator passes through correctly
func TestMergedIterator_SingleIterator(t *testing.T) {
	iter := NewIntSliceIterator([]int{1, 2, 3, 4, 5})

	merged, err := NewMergedIterator(iter)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	expected := []int{1, 2, 3, 4, 5}
	for i, exp := range expected {
		if !merged.HasNext() {
			t.Fatalf("Expected more elements at position %d", i)
		}
		actual := merged.Next()
		if actual != exp {
			t.Errorf("Position %d: expected %d, got %d", i, exp, actual)
		}
	}

	if merged.HasNext() {
		t.Error("Expected no more elements")
	}
}

// TestMergedIterator_MultipleIterators tests with multiple iterators.
// Purpose: Verifies correct merging of many iterators
func TestMergedIterator_MultipleIterators(t *testing.T) {
	// Create 5 iterators with interleaved values
	iterators := make([]IntIterator, 5)
	for i := 0; i < 5; i++ {
		slice := make([]int, 0, 10)
		for j := 0; j < 10; j++ {
			slice = append(slice, i+j*5)
		}
		iterators[i] = NewIntSliceIterator(slice)
	}

	merged, err := NewMergedIterator(iterators...)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	// Should produce 0, 1, 2, 3, 4, 5, 6, ... 49
	for i := 0; i < 50; i++ {
		if !merged.HasNext() {
			t.Fatalf("Expected more elements at position %d", i)
		}
		actual := merged.Next()
		if actual != i {
			t.Errorf("Position %d: expected %d, got %d", i, i, actual)
		}
	}

	if merged.HasNext() {
		t.Error("Expected no more elements")
	}
}

// TestMergedIterator_ConsecutiveDuplicates tests consecutive duplicates in single iterator.
// Purpose: Verifies handling of repeated values within one iterator
func TestMergedIterator_ConsecutiveDuplicates(t *testing.T) {
	// With removeDups=true, consecutive duplicates should be kept (they're from same iterator)
	iter := NewIntSliceIterator([]int{1, 1, 1, 2, 2, 3})

	merged, err := NewMergedIteratorWithOptions(true, iter)
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	expected := []int{1, 1, 1, 2, 2, 3}
	for i, exp := range expected {
		if !merged.HasNext() {
			t.Fatalf("Expected more elements at position %d", i)
		}
		actual := merged.Next()
		if actual != exp {
			t.Errorf("Position %d: expected %d, got %d", i, exp, actual)
		}
	}
}

// TestIntSliceIterator tests the IntSliceIterator helper.
// Purpose: Verifies the test helper works correctly
func TestIntSliceIterator(t *testing.T) {
	slice := []int{1, 2, 3}
	iter := NewIntSliceIterator(slice)

	if !iter.HasNext() {
		t.Error("Expected HasNext to be true initially")
	}

	if iter.Next() != 1 {
		t.Error("Expected 1")
	}
	if iter.Next() != 2 {
		t.Error("Expected 2")
	}
	if iter.Next() != 3 {
		t.Error("Expected 3")
	}

	if iter.HasNext() {
		t.Error("Expected HasNext to be false after consuming all elements")
	}

	// Test empty slice
	emptyIter := NewIntSliceIterator([]int{})
	if emptyIter.HasNext() {
		t.Error("Expected empty iterator to have no next")
	}
}

// TestMergedIterator_PanicOnEmptyNext tests that Next panics when empty.
// Purpose: Verifies error handling for empty iterator
func TestMergedIterator_PanicOnEmptyNext(t *testing.T) {
	merged, err := NewMergedIterator()
	if err != nil {
		t.Fatalf("Failed to create MergedIterator: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on Next() with no elements")
		}
	}()

	merged.Next()
}
