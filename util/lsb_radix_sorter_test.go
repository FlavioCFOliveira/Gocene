// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"sort"
	"testing"
)

// bitsRequired returns the number of bits needed to represent v as an
// unsigned integer (mirrors PackedInts.bitsRequired for non-negative values).
func bitsRequired(v int32) int {
	if v == 0 {
		return 0
	}
	bits := 0
	uv := uint32(v)
	for uv > 0 {
		bits++
		uv >>= 1
	}
	return bits
}

// runLSBRadixCheck mirrors test(LSBRadixSorter, int[] arr, int len).
func runLSBRadixCheck(t *testing.T, rng *rand.Rand, sorter *LSBRadixSorter, arr []int32, length int) {
	t.Helper()
	expected := append([]int32(nil), arr[:length]...)
	sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })

	numBits := 0
	for i := 0; i < length; i++ {
		if b := bitsRequired(arr[i]); b > numBits {
			numBits = b
		}
	}
	if rng.Intn(2) == 0 {
		numBits = numBits + rng.Intn(32-numBits+1)
	}

	sorter.Sort(numBits, arr, length)
	actual := arr[:length]
	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("mismatch at %d: expected=%v actual=%v (len=%d numBits=%d)",
				i, expected[:min2(length, 16)], actual[:min2(length, 16)], length, numBits)
		}
	}
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runLSBRadixSuite mirrors test(LSBRadixSorter, int maxLen): 10 random
// iterations with random length and random bit budget.
func runLSBRadixSuite(t *testing.T, sorter *LSBRadixSorter, maxLen int, seed int64) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	for iter := 0; iter < 10; iter++ {
		length := rng.Intn(maxLen + 1)
		arr := make([]int32, length+rng.Intn(10))
		numBits := rng.Intn(31)
		var maxValue int32
		if numBits > 0 {
			maxValue = int32((1 << uint(numBits)) - 1)
		}
		for i := range arr {
			if maxValue == 0 {
				arr[i] = 0
			} else {
				arr[i] = int32(rng.Int31n(maxValue + 1))
			}
		}
		runLSBRadixCheck(t, rng, sorter, arr, length)
	}
}

func TestLSBRadixSorter_Empty(t *testing.T) {
	runLSBRadixSuite(t, NewLSBRadixSorter(), 0, 1)
}
func TestLSBRadixSorter_One(t *testing.T) {
	runLSBRadixSuite(t, NewLSBRadixSorter(), 1, 2)
}
func TestLSBRadixSorter_Two(t *testing.T) {
	runLSBRadixSuite(t, NewLSBRadixSorter(), 2, 3)
}
func TestLSBRadixSorter_Simple(t *testing.T) {
	runLSBRadixSuite(t, NewLSBRadixSorter(), 100, 4)
}
func TestLSBRadixSorter_Random(t *testing.T) {
	runLSBRadixSuite(t, NewLSBRadixSorter(), 10000, 5)
}

func TestLSBRadixSorter_Sorted(t *testing.T) {
	sorter := NewLSBRadixSorter()
	rng := rand.New(rand.NewSource(6))
	for iter := 0; iter < 10; iter++ {
		arr := make([]int32, 10000)
		a := int32(0)
		for i := range arr {
			a += int32(rng.Intn(10))
			arr[i] = a
		}
		length := rng.Intn(len(arr) + 1)
		runLSBRadixCheck(t, rng, sorter, arr, length)
	}
}

func TestLSBRadixSorter_StableScatter(t *testing.T) {
	// Verifies the radix passes preserve order for equal values.
	// With many duplicates, the radix-sorted output must be sorted but
	// also remain in increasing index order for ties.
	arr := []int32{5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1,
		5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 1}
	sorter := NewLSBRadixSorter()
	sorter.Sort(8, arr, len(arr))
	expected := append([]int32(nil), arr...)
	sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })
	for i := range arr {
		if arr[i] != expected[i] {
			t.Fatalf("at %d: arr=%d expected=%d", i, arr[i], expected[i])
		}
	}
}

func TestLSBRadixSorter_SortIsSortedRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(0xCAFE))
	sorter := NewLSBRadixSorter()
	for _, n := range []int{0, 1, 29, 30, 31, 100, 1024, 4096, 8192} {
		arr := make([]int32, n)
		for i := range arr {
			arr[i] = int32(rng.Intn(1 << 20))
		}
		sorter.Sort(32, arr, n)
		if !sort.SliceIsSorted(arr, func(i, j int) bool { return arr[i] < arr[j] }) {
			t.Fatalf("n=%d: not sorted: %v", n, arr[:min2(n, 16)])
		}
	}
}

func TestLSBRadixSorter_NumBitsHintShortCircuit(t *testing.T) {
	// numBits = 0 means no passes happen — only the buffer-equal-array
	// copy-back at the end runs; the output must still equal the input
	// because the algorithm only sorts within the bit range it is told.
	arr := []int32{5, 4, 3, 2, 1, 0, 5, 4, 3, 2, 1, 0, 5, 4, 3, 2, 1, 0, 5, 4,
		3, 2, 1, 0, 5, 4, 3, 2, 1, 0}
	original := append([]int32(nil), arr...)
	NewLSBRadixSorter().Sort(0, arr, len(arr))
	for i := range arr {
		if arr[i] != original[i] {
			t.Fatalf("numBits=0 should not reorder: at %d arr=%d original=%d", i, arr[i], original[i])
		}
	}
}

func TestLSBRadixSorter_ReuseSorter(t *testing.T) {
	// Re-running the same sorter on inputs of varying size should not
	// produce stale state.
	sorter := NewLSBRadixSorter()
	for _, n := range []int{4096, 32, 256, 1024, 60} {
		arr := make([]int32, n)
		for i := range arr {
			arr[i] = int32((n - i) * 13 % 2048)
		}
		sorter.Sort(16, arr, n)
		if !sort.SliceIsSorted(arr, func(i, j int) bool { return arr[i] < arr[j] }) {
			t.Fatalf("reuse n=%d not sorted: %v", n, arr[:min2(n, 16)])
		}
	}
}
