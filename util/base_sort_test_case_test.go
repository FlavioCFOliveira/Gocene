// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/util/BaseSortTestCase.java
// Purpose: Shared fixtures and helpers used by sorter tests (entry record,
// data-generation strategies, generic per-strategy runner and assertion). The
// Java original is an abstract LuceneTestCase parameterised by a Sorter factory
// and a stability flag. The Go port keeps those primitives as package-private
// helpers reused by sorters_test.go and any future sorter test files.

package util

import (
	"math/rand"
	"slices"
	"testing"
)

// entry represents a sortable entry with value and original ordinal for
// stability testing. Corresponds to BaseSortTestCase.Entry in Java.
type entry struct {
	value int
	ord   int
}

// compareEntry compares two entries by value only.
// Used for non-stable sort verification.
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
// Used for stable sort verification.
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

// assertSorted verifies that the sorted array matches the expected sorted order.
// For stable sorts, it also verifies that original ordinals are preserved for
// equal values. Corresponds to BaseSortTestCase.assertSorted in Java.
func assertSorted(t *testing.T, original, sorted []entry, stable bool) {
	t.Helper()

	if len(original) != len(sorted) {
		t.Errorf("Length mismatch: expected %d, got %d", len(original), len(sorted))
		return
	}

	// Build the expected sorted reference.
	expected := make([]entry, len(original))
	copy(expected, original)

	if stable {
		slices.SortFunc(expected, compareEntryFull)
	} else {
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

// testStrategy defines different data-generation strategies for testing sorts.
// Mirrors BaseSortTestCase.Strategy in Java.
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
// Mirrors the per-strategy set(arr, i, random) helpers in the Java enum.
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
				// 10% chance of breaking the ascending sequence.
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
				// Values from -8 to 10 relative to previous.
				delta := r.Intn(19) - 8
				arr[i] = entry{value: arr[i-1].value + delta, ord: i}
			}
		}
	}

	return arr
}

// runSorterTest exercises a sorter with the given strategy and length.
// Mirrors BaseSortTestCase.test(Strategy, int) in Java, including the random
// offset/tail padding around the sorted region.
func runSorterTest(t *testing.T, r *rand.Rand, strategy testStrategy, length int, newSorter func([]entry) SorterInterface) {
	arr := generateEntries(r, strategy, length)

	// Create a copy with offset padding to mirror the Java test contract.
	offset := r.Intn(1000)
	toSort := make([]entry, offset+len(arr)+r.Intn(3))
	copy(toSort[offset:], arr)

	sorter := newSorter(toSort)
	sorter.Sort(offset, offset+len(arr))

	result := make([]entry, len(arr))
	copy(result, toSort[offset:offset+len(arr)])

	// Stability is not asserted by this generic runner; concrete tests handle it.
	assertSorted(t, arr, result, false)
}
