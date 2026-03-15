// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/util/TestIntroSorter.java
// Source: lucene/core/src/test/org/apache/lucene/util/TestTimSorter.java
// Source: lucene/core/src/test/org/apache/lucene/util/BaseSortTestCase.java
// Purpose: Tests for IntroSorter and TimSorter implementations

package util

import (
	"math/rand"
	"slices"
	"testing"
	"time"
)

// entry represents a sortable entry with value and original ordinal for stability testing.
// This corresponds to BaseSortTestCase.Entry in Java.
type entry struct {
	value int
	ord   int
}

// compareEntry compares two entries by value only.
// This is used for non-stable sort verification.
func compareEntry(a, b entry) int {
	if a.value < b.value {
		return -1
	}
	if a.value > b.value {
		return 1
	}
	return 0
}

// compareEntryFull compares entries by value, then by ordinal for total ordering.
// This is used for stable sort verification.
func compareEntryFull(a, b entry) int {
	cmp := compareEntry(a, b)
	if cmp != 0 {
		return cmp
	}
	if a.ord < b.ord {
		return -1
	}
	if a.ord > b.ord {
		return 1
	}
	return 0
}

// entryIntroSorter adapts a slice of entries for use with IntroSorter.
type entryIntroSorter struct {
	arr   []entry
	pivot entry
}

func newEntryIntroSorter(arr []entry) *entryIntroSorter {
	return &entryIntroSorter{arr: arr}
}

func (s *entryIntroSorter) Compare(i, j int) int {
	return compareEntry(s.arr[i], s.arr[j])
}

func (s *entryIntroSorter) Swap(i, j int) {
	s.arr[i], s.arr[j] = s.arr[j], s.arr[i]
}

func (s *entryIntroSorter) SetPivot(i int) {
	s.pivot = s.arr[i]
}

func (s *entryIntroSorter) ComparePivot(j int) int {
	return compareEntry(s.pivot, s.arr[j])
}

func (s *entryIntroSorter) Sort(from, to int) {}

// entryTimSorter adapts a slice of entries for use with TimSorter.
type entryTimSorter struct {
	arr []entry
	tmp []entry
}

func newEntryTimSorter(arr []entry, maxTempSlots int) *entryTimSorter {
	tmp := make([]entry, maxTempSlots)
	return &entryTimSorter{arr: arr, tmp: tmp}
}

func (s *entryTimSorter) Compare(i, j int) int {
	return compareEntry(s.arr[i], s.arr[j])
}

func (s *entryTimSorter) Swap(i, j int) {
	s.arr[i], s.arr[j] = s.arr[j], s.arr[i]
}

func (s *entryTimSorter) Copy(src, dest int) {
	s.arr[dest] = s.arr[src]
}

func (s *entryTimSorter) Save(i, length int) {
	for j := 0; j < length && j < len(s.tmp); j++ {
		s.tmp[j] = s.arr[i+j]
	}
}

func (s *entryTimSorter) Restore(i, j int) {
	if i < len(s.tmp) {
		s.arr[j] = s.tmp[i]
	}
}

func (s *entryTimSorter) CompareSaved(i, j int) int {
	if i < len(s.tmp) {
		return compareEntry(s.tmp[i], s.arr[j])
	}
	return 0
}

func (s *entryTimSorter) Sort(from, to int) {}

// assertSorted verifies that the sorted array matches the expected sorted order.
// For stable sorts, it also verifies that original ordinals are preserved for equal values.
func assertSorted(t *testing.T, original, sorted []entry, stable bool) {
	t.Helper()

	if len(original) != len(sorted) {
		t.Errorf("Length mismatch: expected %d, got %d", len(original), len(sorted))
		return
	}

	// Create expected sorted array
	expected := make([]entry, len(original))
	copy(expected, original)

	if stable {
		// For stable sort, sort by value then by ordinal
		slices.SortFunc(expected, compareEntryFull)
	} else {
		// For non-stable sort, only sort by value
		slices.SortFunc(expected, compareEntry)
	}

	for i := range original {
		if sorted[i].value != expected[i].value {
			t.Errorf("Value mismatch at index %d: expected %d, got %d", i, expected[i].value, sorted[i].value)
			return
		}
		if stable && sorted[i].ord != expected[i].ord {
			t.Errorf("Ordinal mismatch at index %d for value %d: expected %d, got %d",
				i, sorted[i].value, expected[i].ord, sorted[i].ord)
			return
		}
	}
}

// testStrategy defines different data generation strategies for testing sorts.
type testStrategy int

const (
	strategyRandom testStrategy = iota
	strategyRandomLowCardinality
	strategyRandomMediumCardinality
	strategyAscending
	strategyDescending
	strategyStrictlyDescending
	strategyAscendingSequences
	strategyMostlyAscending
)

// generateEntries generates a slice of entries using the specified strategy.
func generateEntries(r *rand.Rand, strategy testStrategy, length int) []entry {
	arr := make([]entry, length)

	switch strategy {
	case strategyRandom:
		for i := 0; i < length; i++ {
			arr[i] = entry{value: r.Int(), ord: i}
		}

	case strategyRandomLowCardinality:
		for i := 0; i < length; i++ {
			arr[i] = entry{value: r.Intn(6), ord: i}
		}

	case strategyRandomMediumCardinality:
		for i := 0; i < length; i++ {
			arr[i] = entry{value: r.Intn(length / 2), ord: i}
		}

	case strategyAscending:
		for i := 0; i < length; i++ {
			if i == 0 {
				arr[i] = entry{value: r.Intn(6), ord: i}
			} else {
				arr[i] = entry{value: arr[i-1].value + r.Intn(6), ord: i}
			}
		}

	case strategyDescending:
		for i := 0; i < length; i++ {
			if i == 0 {
				arr[i] = entry{value: r.Intn(6), ord: i}
			} else {
				arr[i] = entry{value: arr[i-1].value - r.Intn(6), ord: i}
			}
		}

	case strategyStrictlyDescending:
		for i := 0; i < length; i++ {
			if i == 0 {
				arr[i] = entry{value: r.Intn(6), ord: i}
			} else {
				arr[i] = entry{value: arr[i-1].value - (1 + r.Intn(5)), ord: i}
			}
		}

	case strategyAscendingSequences:
		for i := 0; i < length; i++ {
			if i == 0 {
				arr[i] = entry{value: r.Intn(6), ord: i}
			} else {
				// 10% chance of breaking the ascending sequence
				if r.Intn(10) == 0 {
					arr[i] = entry{value: r.Intn(1000), ord: i}
				} else {
					arr[i] = entry{value: arr[i-1].value + r.Intn(6), ord: i}
				}
			}
		}

	case strategyMostlyAscending:
		for i := 0; i < length; i++ {
			if i == 0 {
				arr[i] = entry{value: r.Intn(6), ord: i}
			} else {
				// Values from -8 to 10 relative to previous
				delta := r.Intn(19) - 8
				arr[i] = entry{value: arr[i-1].value + delta, ord: i}
			}
		}
	}

	return arr
}

// runSorterTest tests a sorter with the given strategy and length.
func runSorterTest(t *testing.T, r *rand.Rand, strategy testStrategy, length int, newSorter func([]entry) SorterInterface) {
	arr := generateEntries(r, strategy, length)

	// Create a copy with offset padding like in Java tests
	offset := r.Intn(1000)
	toSort := make([]entry, offset+len(arr)+r.Intn(3))
	copy(toSort[offset:], arr)

	sorter := newSorter(toSort)
	sorter.Sort(offset, offset+len(arr))

	result := make([]entry, len(arr))
	copy(result, toSort[offset:offset+len(arr)])

	// Determine if this is a stable sort test
	// For simplicity, we don't check stability in this generic test
	assertSorted(t, arr, result, false)
}

// ==================== IntroSorter Tests ====================

// TestIntroSorter_Empty tests sorting an empty array.
func TestIntroSorter_Empty(t *testing.T) {
	arr := []entry{}
	sorter := newEntryIntroSorter(arr)
	NewIntroSorter(sorter).Sort(0, 0)
	if len(arr) != 0 {
		t.Error("Empty array should remain empty")
	}
}

// TestIntroSorter_One tests sorting a single element.
func TestIntroSorter_One(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	runSorterTest(t, r, strategyRandom, 1, func(arr []entry) SorterInterface {
		return newEntryIntroSorter(arr)
	})
}

// TestIntroSorter_Two tests sorting two elements.
func TestIntroSorter_Two(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	runSorterTest(t, r, strategyRandomLowCardinality, 2, func(arr []entry) SorterInterface {
		return newEntryIntroSorter(arr)
	})
}

// TestIntroSorter_Random tests sorting random data.
func TestIntroSorter_Random(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		runSorterTest(t, r, strategyRandom, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_RandomLowCardinality tests sorting with low cardinality values.
func TestIntroSorter_RandomLowCardinality(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		runSorterTest(t, r, strategyRandomLowCardinality, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_RandomMediumCardinality tests sorting with medium cardinality values.
func TestIntroSorter_RandomMediumCardinality(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		if length < 2 {
			length = 2
		}
		runSorterTest(t, r, strategyRandomMediumCardinality, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_Ascending tests sorting already ascending data.
func TestIntroSorter_Ascending(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		runSorterTest(t, r, strategyAscending, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_AscendingSequences tests sorting data with ascending sequences.
func TestIntroSorter_AscendingSequences(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		runSorterTest(t, r, strategyAscendingSequences, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_Descending tests sorting descending data.
func TestIntroSorter_Descending(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		runSorterTest(t, r, strategyDescending, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_StrictlyDescending tests sorting strictly descending data.
func TestIntroSorter_StrictlyDescending(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		runSorterTest(t, r, strategyStrictlyDescending, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_MostlyAscending tests sorting mostly ascending data.
func TestIntroSorter_MostlyAscending(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		runSorterTest(t, r, strategyMostlyAscending, length, func(arr []entry) SorterInterface {
			return newEntryIntroSorter(arr)
		})
	}
}

// TestIntroSorter_Stability verifies that IntroSorter is NOT stable.
func TestIntroSorter_Stability(t *testing.T) {
	// Create data with duplicate values
	arr := []entry{
		{value: 3, ord: 0},
		{value: 1, ord: 1},
		{value: 3, ord: 2},
		{value: 1, ord: 3},
		{value: 2, ord: 4},
	}

	sorter := newEntryIntroSorter(arr)
	NewIntroSorter(sorter).Sort(0, len(arr))

	// Verify sorted by value
	for i := 1; i < len(arr); i++ {
		if arr[i].value < arr[i-1].value {
			t.Error("IntroSorter did not sort correctly")
		}
	}

	// Note: We don't check stability since IntroSorter is explicitly NOT stable
}

// TestIntroSorter_WorstCase tests worst-case scenarios for IntroSorter.
func TestIntroSorter_WorstCase(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test with various sizes that might trigger worst-case behavior
	sizes := []int{16, 17, 32, 33, 64, 65, 100, 127, 128, 129, 255, 256, 257, 1000, 10000}

	for _, size := range sizes {
		// Test with strictly descending (worst case for quicksort)
		arr := generateEntries(r, strategyStrictlyDescending, size)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryIntroSorter(arr)
		NewIntroSorter(sorter).Sort(0, len(arr))

		assertSorted(t, original, arr, false)
	}
}

// ==================== TimSorter Tests ====================

// TestTimSorter_Empty tests sorting an empty array.
func TestTimSorter_Empty(t *testing.T) {
	arr := []entry{}
	sorter := newEntryTimSorter(arr, 0)
	NewTimSorter(sorter, 0).Sort(0, 0)
	if len(arr) != 0 {
		t.Error("Empty array should remain empty")
	}
}

// TestTimSorter_One tests sorting a single element.
func TestTimSorter_One(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	arr := generateEntries(r, strategyRandom, 1)
	sorter := newEntryTimSorter(arr, 1)
	NewTimSorter(sorter, 1).Sort(0, 1)
	if len(arr) != 1 {
		t.Error("Single element array should have one element")
	}
}

// TestTimSorter_Two tests sorting two elements.
func TestTimSorter_Two(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	arr := generateEntries(r, strategyRandomLowCardinality, 2)
	original := make([]entry, len(arr))
	copy(original, arr)

	sorter := newEntryTimSorter(arr, 2)
	NewTimSorter(sorter, 2).Sort(0, 2)

	assertSorted(t, original, arr, true)
}

// TestTimSorter_Random tests sorting random data.
func TestTimSorter_Random(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		if length == 0 {
			length = 1
		}
		maxTempSlots := r.Intn(length + 1)
		arr := generateEntries(r, strategyRandom, length)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, maxTempSlots)
		NewTimSorter(sorter, maxTempSlots).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// TestTimSorter_RandomLowCardinality tests sorting with low cardinality values.
func TestTimSorter_RandomLowCardinality(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		if length == 0 {
			length = 1
		}
		maxTempSlots := r.Intn(length + 1)
		arr := generateEntries(r, strategyRandomLowCardinality, length)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, maxTempSlots)
		NewTimSorter(sorter, maxTempSlots).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// TestTimSorter_Ascending tests sorting already ascending data.
func TestTimSorter_Ascending(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		if length == 0 {
			length = 1
		}
		maxTempSlots := r.Intn(length + 1)
		arr := generateEntries(r, strategyAscending, length)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, maxTempSlots)
		NewTimSorter(sorter, maxTempSlots).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// TestTimSorter_AscendingSequences tests sorting data with ascending sequences.
func TestTimSorter_AscendingSequences(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		if length == 0 {
			length = 1
		}
		maxTempSlots := r.Intn(length + 1)
		arr := generateEntries(r, strategyAscendingSequences, length)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, maxTempSlots)
		NewTimSorter(sorter, maxTempSlots).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// TestTimSorter_Descending tests sorting descending data.
func TestTimSorter_Descending(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		if length == 0 {
			length = 1
		}
		maxTempSlots := r.Intn(length + 1)
		arr := generateEntries(r, strategyDescending, length)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, maxTempSlots)
		NewTimSorter(sorter, maxTempSlots).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// TestTimSorter_StrictlyDescending tests sorting strictly descending data.
func TestTimSorter_StrictlyDescending(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		length := r.Intn(20000)
		if length == 0 {
			length = 1
		}
		maxTempSlots := r.Intn(length + 1)
		arr := generateEntries(r, strategyStrictlyDescending, length)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, maxTempSlots)
		NewTimSorter(sorter, maxTempSlots).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// TestTimSorter_Stability verifies that TimSorter IS stable.
func TestTimSorter_Stability(t *testing.T) {
	// Create data with duplicate values
	arr := []entry{
		{value: 3, ord: 0},
		{value: 1, ord: 1},
		{value: 3, ord: 2},
		{value: 1, ord: 3},
		{value: 2, ord: 4},
	}

	sorter := newEntryTimSorter(arr, len(arr))
	NewTimSorter(sorter, len(arr)).Sort(0, len(arr))

	// Verify sorted by value
	for i := 1; i < len(arr); i++ {
		if arr[i].value < arr[i-1].value {
			t.Error("TimSorter did not sort correctly")
		}
	}

	// For stable sort, items with equal values should maintain relative order
	// After sorting by value: indices should be 1, 3, 4, 0, 2 (for values 1, 1, 2, 3, 3)
	expectedIndices := []int{1, 3, 4, 0, 2}
	for i, exp := range expectedIndices {
		if arr[i].ord != exp {
			t.Errorf("TimSort stability: expected index %d at position %d, got %d", exp, i, arr[i].ord)
		}
	}
}

// TestTimSorter_StabilityLarge tests stability with larger datasets.
func TestTimSorter_StabilityLarge(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create data with limited value range to ensure duplicates
	length := 10000
	arr := make([]entry, length)
	for i := 0; i < length; i++ {
		arr[i] = entry{value: r.Intn(100), ord: i}
	}
	original := make([]entry, len(arr))
	copy(original, arr)

	maxTempSlots := length / 64
	sorter := newEntryTimSorter(arr, maxTempSlots)
	NewTimSorter(sorter, maxTempSlots).Sort(0, len(arr))

	assertSorted(t, original, arr, true)
}

// TestTimSorter_VariousTempSlots tests TimSort with various temp slot configurations.
func TestTimSorter_VariousTempSlots(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	length := 1000
	arr := generateEntries(r, strategyRandom, length)

	// Test with different maxTempSlots values
	tempSlots := []int{0, 1, 10, 100, length / 2, length}

	for _, slots := range tempSlots {
		testArr := make([]entry, len(arr))
		copy(testArr, arr)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(testArr, slots)
		NewTimSorter(sorter, slots).Sort(0, len(testArr))

		assertSorted(t, original, testArr, true)
	}
}

// TestTimSorter_WorstCase tests worst-case scenarios for TimSorter.
func TestTimSorter_WorstCase(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test with various sizes
	sizes := []int{32, 33, 64, 65, 100, 127, 128, 129, 255, 256, 257, 1000, 10000}

	for _, size := range sizes {
		// Test with strictly descending
		arr := generateEntries(r, strategyStrictlyDescending, size)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, size/64)
		NewTimSorter(sorter, size/64).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// TestTimSorter_MinRunBoundary tests around min run boundaries.
func TestTimSorter_MinRunBoundary(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test around the MinRun boundary (32)
	sizes := []int{30, 31, 32, 33, 34, 60, 61, 62, 63, 64, 65}

	for _, size := range sizes {
		arr := generateEntries(r, strategyRandom, size)
		original := make([]entry, len(arr))
		copy(original, arr)

		sorter := newEntryTimSorter(arr, size/64)
		NewTimSorter(sorter, size/64).Sort(0, len(arr))

		assertSorted(t, original, arr, true)
	}
}

// ==================== Direct Sorter Tests ====================

// TestIntroSorter_Direct tests the IntroSorter directly with edge cases.
func TestIntroSorter_Direct(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test with sub-range sorting
	arr := make([]entry, 100)
	for i := range arr {
		arr[i] = entry{value: r.Intn(1000), ord: i}
	}

	// Sort only middle 50 elements
	sorter := newEntryIntroSorter(arr)
	NewIntroSorter(sorter).Sort(25, 75)

	// Verify the middle section is sorted
	for i := 26; i < 75; i++ {
		if arr[i].value < arr[i-1].value {
			t.Errorf("Sub-range sort failed at index %d", i)
		}
	}
}

// TestTimSorter_Direct tests the TimSorter directly with edge cases.
func TestTimSorter_Direct(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test with sub-range sorting
	arr := make([]entry, 100)
	for i := range arr {
		arr[i] = entry{value: r.Intn(1000), ord: i}
	}
	original := make([]entry, len(arr))
	copy(original, arr)

	// Sort only middle 50 elements
	sorter := newEntryTimSorter(arr, 50)
	NewTimSorter(sorter, 50).Sort(25, 75)

	// Verify the middle section is sorted
	for i := 26; i < 75; i++ {
		if arr[i].value < arr[i-1].value {
			t.Errorf("Sub-range sort failed at index %d", i)
		}
	}

	// Verify elements outside the range are unchanged
	for i := 0; i < 25; i++ {
		if arr[i].value != original[i].value || arr[i].ord != original[i].ord {
			t.Errorf("Element at index %d was modified", i)
		}
	}
	for i := 75; i < 100; i++ {
		if arr[i].value != original[i].value || arr[i].ord != original[i].ord {
			t.Errorf("Element at index %d was modified", i)
		}
	}
}

// TestSorter_CheckRange tests range validation.
func TestSorter_CheckRange(t *testing.T) {
	arr := []entry{{value: 1, ord: 0}, {value: 2, ord: 1}}
	sorter := newEntryIntroSorter(arr)
	introSorter := NewIntroSorter(sorter)

	// This should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid range")
		}
	}()

	introSorter.Sort(1, 0) // to < from should panic
}

// TestSorter_SingleElement tests sorting a single element range.
func TestSorter_SingleElement(t *testing.T) {
	arr := []entry{{value: 42, ord: 0}}

	// IntroSorter
	sorter1 := newEntryIntroSorter(arr)
	NewIntroSorter(sorter1).Sort(0, 1)
	if arr[0].value != 42 {
		t.Error("Single element was modified by IntroSorter")
	}

	// TimSorter
	arr2 := []entry{{value: 42, ord: 0}}
	sorter2 := newEntryTimSorter(arr2, 1)
	NewTimSorter(sorter2, 1).Sort(0, 1)
	if arr2[0].value != 42 {
		t.Error("Single element was modified by TimSorter")
	}
}
