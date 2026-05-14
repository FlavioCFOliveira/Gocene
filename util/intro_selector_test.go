// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"sort"
	"testing"
)

// introSelectorImpl is the test-only IntroSelectorInterface implementation
// that mirrors the anonymous IntroSelector used in TestIntroSelector.java.
type introSelectorImpl struct {
	arr   []int
	pivot int
}

func (s *introSelectorImpl) Swap(i, j int) { s.arr[i], s.arr[j] = s.arr[j], s.arr[i] }
func (s *introSelectorImpl) Select(from, to, k int) {
	// Required by SelectorInterface but unused in tests; NewIntroSelector(...).Select drives the work.
}
func (s *introSelectorImpl) Compare(i, j int) int {
	switch {
	case s.arr[i] < s.arr[j]:
		return -1
	case s.arr[i] > s.arr[j]:
		return 1
	}
	return 0
}
func (s *introSelectorImpl) SetPivot(i int) { s.pivot = s.arr[i] }
func (s *introSelectorImpl) ComparePivot(j int) int {
	switch {
	case s.pivot < s.arr[j]:
		return -1
	case s.pivot > s.arr[j]:
		return 1
	}
	return 0
}

// runSelectCheck mirrors doTestSelect in TestIntroSelector.java.
func runSelectCheck(t *testing.T, rng *rand.Rand, useMaxDepth bool) {
	t.Helper()
	from := rng.Intn(5)
	to := from + 1 + rng.Intn(10000)
	var maxV int
	if rng.Intn(2) == 0 {
		maxV = rng.Intn(100)
	} else {
		maxV = rng.Intn(100000)
	}
	arr := make([]int, to+rng.Intn(5))
	for i := range arr {
		arr[i] = rng.Intn(maxV + 1)
	}
	k := from + rng.Intn(to-from)

	original := append([]int(nil), arr...)
	expected := append([]int(nil), arr...)
	sort.Ints(expected[from:to])

	actual := append([]int(nil), arr...)
	impl := &introSelectorImpl{arr: actual}
	sel := NewIntroSelector(impl)
	sel.SetRandomSeed(rng.Int63())
	if useMaxDepth {
		sel.SelectWithMaxDepth(from, to, k, rng.Intn(3))
	} else {
		sel.Select(from, to, k)
	}

	if actual[k] != expected[k] {
		t.Fatalf("actual[k]=%d expected[k]=%d (from=%d to=%d k=%d max=%d)",
			actual[k], expected[k], from, to, k, maxV)
	}
	for i := 0; i < len(actual); i++ {
		switch {
		case i < from || i >= to:
			if actual[i] != original[i] {
				t.Fatalf("slot %d outside [from,to) mutated: %d -> %d", i, original[i], actual[i])
			}
		case i <= k:
			if actual[i] > actual[k] {
				t.Fatalf("slot %d=%d > actual[k=%d]=%d", i, actual[i], k, actual[k])
			}
		default:
			if actual[i] < actual[k] {
				t.Fatalf("slot %d=%d < actual[k=%d]=%d", i, actual[i], k, actual[k])
			}
		}
	}
}

func TestIntroSelector_Select(t *testing.T) {
	rng := rand.New(rand.NewSource(0xCAFEBABE))
	for iter := 0; iter < 100; iter++ {
		runSelectCheck(t, rng, false)
	}
}

func TestIntroSelector_SelectWithMaxDepth(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDEADBEEF))
	for iter := 0; iter < 100; iter++ {
		runSelectCheck(t, rng, true)
	}
}

func TestIntroSelector_CheckArgs(t *testing.T) {
	impl := &introSelectorImpl{arr: []int{1, 2, 3}}
	sel := NewIntroSelector(impl)

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on k < from")
		}
	}()
	sel.Select(1, 3, 0)
}

func TestIntroSelector_CheckArgs_KAboveTo(t *testing.T) {
	impl := &introSelectorImpl{arr: []int{1, 2, 3}}
	sel := NewIntroSelector(impl)

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on k >= to")
		}
	}()
	sel.Select(0, 2, 5)
}

func TestIntroSelector_DegenerateTinyRanges(t *testing.T) {
	cases := [][]int{
		{},
		{42},
		{2, 1},
		{3, 1, 2},
	}
	for _, in := range cases {
		if len(in) == 0 {
			continue
		}
		for k := 0; k < len(in); k++ {
			arr := append([]int(nil), in...)
			impl := &introSelectorImpl{arr: arr}
			sel := NewIntroSelector(impl)
			sel.Select(0, len(arr), k)
			expected := append([]int(nil), in...)
			sort.Ints(expected)
			if arr[k] != expected[k] {
				t.Fatalf("input=%v k=%d arr[k]=%d expected[k]=%d", in, k, arr[k], expected[k])
			}
		}
	}
}

func TestIntroSelector_AllEqual(t *testing.T) {
	arr := make([]int, 200)
	for i := range arr {
		arr[i] = 7
	}
	impl := &introSelectorImpl{arr: arr}
	sel := NewIntroSelector(impl)
	sel.Select(0, len(arr), 137)
	for _, v := range arr {
		if v != 7 {
			t.Fatalf("expected all 7, got %d", v)
		}
	}
}

func TestIntroSelector_AlreadySorted(t *testing.T) {
	arr := make([]int, 500)
	for i := range arr {
		arr[i] = i
	}
	impl := &introSelectorImpl{arr: arr}
	sel := NewIntroSelector(impl)
	sel.Select(0, len(arr), 123)
	if arr[123] != 123 {
		t.Fatalf("sorted: arr[123]=%d want 123", arr[123])
	}
}

func TestIntroSelector_DescendingInput(t *testing.T) {
	arr := make([]int, 500)
	for i := range arr {
		arr[i] = len(arr) - i
	}
	impl := &introSelectorImpl{arr: arr}
	sel := NewIntroSelector(impl)
	sel.Select(0, len(arr), 250)
	// Verify k-th element matches the same element from a sorted clone.
	clone := append([]int(nil), arr...)
	sort.Ints(clone)
	if arr[250] != clone[250] {
		t.Fatalf("descending: arr[250]=%d want %d", arr[250], clone[250])
	}
}
