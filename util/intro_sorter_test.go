// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"sort"
	"testing"
)

// intIntroAdapter exposes a []int slice to IntroSorter for the
// sort.IsSorted validation tests required by Sprint 1 batch D.
type intIntroAdapter struct {
	arr   []int
	pivot int
}

func (a *intIntroAdapter) Compare(i, j int) int {
	switch {
	case a.arr[i] < a.arr[j]:
		return -1
	case a.arr[i] > a.arr[j]:
		return 1
	}
	return 0
}
func (a *intIntroAdapter) Swap(i, j int)     { a.arr[i], a.arr[j] = a.arr[j], a.arr[i] }
func (a *intIntroAdapter) Sort(from, to int) {}
func (a *intIntroAdapter) SetPivot(i int)    { a.pivot = a.arr[i] }
func (a *intIntroAdapter) ComparePivot(j int) int {
	switch {
	case a.pivot < a.arr[j]:
		return -1
	case a.pivot > a.arr[j]:
		return 1
	}
	return 0
}

// TestIntroSorter_RandomIsSorted feeds randomised inputs of varying size
// through IntroSorter and asserts the final order via sort.IsSorted. This
// complements the existing entry-based IntroSorter tests with the pure
// []int path required by the Sprint 1 acceptance constraints.
func TestIntroSorter_RandomIsSorted(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1018D2026))
	for _, n := range []int{0, 1, 2, 3, 16, 17, 39, 40, 41, 100, 1024, 4096} {
		arr := make([]int, n)
		for i := range arr {
			arr[i] = rng.Intn(1000)
		}
		original := append([]int(nil), arr...)
		impl := &intIntroAdapter{arr: arr}
		NewIntroSorter(impl).Sort(0, n)
		if !sort.IntsAreSorted(arr) {
			t.Fatalf("n=%d sorted=%v original=%v", n, arr, original)
		}
	}
}

// TestIntroSorter_LowCardinalityIsSorted exercises 3-way partitioning
// with a very low-cardinality input that produces many equal keys.
func TestIntroSorter_LowCardinalityIsSorted(t *testing.T) {
	rng := rand.New(rand.NewSource(0xABCD1234))
	arr := make([]int, 4096)
	for i := range arr {
		arr[i] = rng.Intn(8)
	}
	NewIntroSorter(&intIntroAdapter{arr: arr}).Sort(0, len(arr))
	if !sort.IntsAreSorted(arr) {
		t.Fatalf("low-cardinality result not sorted: %v", arr[:32])
	}
}
