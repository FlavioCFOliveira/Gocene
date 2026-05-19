// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hppc

import (
	"errors"
	"math"
	"testing"
)

func TestConstants(t *testing.T) {
	t.Parallel()
	if DefaultExpectedElements != 4 {
		t.Fatalf("DefaultExpectedElements: want 4, got %d", DefaultExpectedElements)
	}
	if DefaultLoadFactor != 0.75 {
		t.Fatalf("DefaultLoadFactor: want 0.75, got %v", DefaultLoadFactor)
	}
	if MinLoadFactor != 1.0/100.0 {
		t.Fatalf("MinLoadFactor: want 0.01, got %v", MinLoadFactor)
	}
	if MaxLoadFactor != 99.0/100.0 {
		t.Fatalf("MaxLoadFactor: want 0.99, got %v", MaxLoadFactor)
	}
	if MinHashArrayLength != 4 {
		t.Fatalf("MinHashArrayLength: want 4, got %d", MinHashArrayLength)
	}
	if MaxHashArrayLength != 1<<30 {
		t.Fatalf("MaxHashArrayLength: want 0x40000000, got %#x", MaxHashArrayLength)
	}
}

func TestIterationIncrement(t *testing.T) {
	t.Parallel()
	// Lucene formula: 29 + ((seed & 7) << 1). Always small odd integer in [29, 43].
	for seed := int32(0); seed < 32; seed++ {
		got := IterationIncrement(seed)
		want := int32(29 + ((seed & 7) << 1))
		if got != want {
			t.Fatalf("IterationIncrement(%d): want %d, got %d", seed, want, got)
		}
		if got%2 == 0 {
			t.Fatalf("IterationIncrement(%d) = %d must be odd", seed, got)
		}
		if got < 29 || got > 43 {
			t.Fatalf("IterationIncrement(%d) = %d out of [29,43]", seed, got)
		}
	}
}

func TestNextIterationSeedMonotonic(t *testing.T) {
	t.Parallel()
	first := NextIterationSeed()
	second := NextIterationSeed()
	if second-first != 1 {
		t.Fatalf("seed not monotonic: %d -> %d", first, second)
	}
}

func TestNextBufferSize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		arraySize int
		want      int
	}{
		{4, 8},
		{8, 16},
		{1 << 10, 1 << 11},
		{1 << 29, 1 << 30},
	}
	for _, tc := range cases {
		got, err := NextBufferSize(tc.arraySize, 0, DefaultLoadFactor)
		if err != nil {
			t.Fatalf("NextBufferSize(%d): unexpected error: %v", tc.arraySize, err)
		}
		if got != tc.want {
			t.Fatalf("NextBufferSize(%d): want %d, got %d", tc.arraySize, tc.want, got)
		}
	}

	// At the cap, must surface a BufferAllocationException.
	_, err := NextBufferSize(MaxHashArrayLength, 0, DefaultLoadFactor)
	var bae *BufferAllocationException
	if !errors.As(err, &bae) {
		t.Fatalf("NextBufferSize at cap: want *BufferAllocationException, got %T (%v)", err, err)
	}

	// Non-power-of-two must be rejected.
	if _, err := NextBufferSize(6, 0, DefaultLoadFactor); err == nil {
		t.Fatalf("NextBufferSize(6): expected error, got nil")
	}
}

func TestExpandAtCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		arraySize  int
		loadFactor float64
		want       int
	}{
		// ceil(8 * 0.75) = 6, min(8-1=7, 6) = 6
		{8, 0.75, 6},
		// ceil(16 * 0.75) = 12, min(15, 12) = 12
		{16, 0.75, 12},
		// ceil(8 * 0.99) = 8, min(7, 8) = 7 (invariant kicks in)
		{8, 0.99, 7},
		// ceil(1024 * 0.5) = 512, min(1023, 512) = 512
		{1024, 0.5, 512},
	}
	for _, tc := range cases {
		got := ExpandAtCount(tc.arraySize, tc.loadFactor)
		if got != tc.want {
			t.Fatalf("ExpandAtCount(%d, %v): want %d, got %d",
				tc.arraySize, tc.loadFactor, tc.want, got)
		}
	}
}

func TestMinBufferSize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		elements   int
		loadFactor float64
		want       int
	}{
		// ceil(0/0.75)=0, ==elements -> length++ = 1, NHP2(1)=1, max(4,1)=4
		{0, 0.75, 4},
		// ceil(1/0.75)=2, NHP2(2)=2, max(4,2)=4
		{1, 0.75, 4},
		// ceil(3/0.75)=4, ==elements? 4!=3 -> stays, NHP2(4)=4, max(4,4)=4
		{3, 0.75, 4},
		// ceil(4/0.75)=6, NHP2(6)=8, max(4,8)=8
		{4, 0.75, 8},
		// ceil(12/0.75)=16, NHP2(16)=16, max(4,16)=16
		{12, 0.75, 16},
		// ceil(13/0.75)=18, NHP2(18)=32
		{13, 0.75, 32},
	}
	for _, tc := range cases {
		got, err := MinBufferSize(tc.elements, tc.loadFactor)
		if err != nil {
			t.Fatalf("MinBufferSize(%d, %v): unexpected error: %v",
				tc.elements, tc.loadFactor, err)
		}
		if got != tc.want {
			t.Fatalf("MinBufferSize(%d, %v): want %d, got %d",
				tc.elements, tc.loadFactor, tc.want, got)
		}
	}

	// Negative element count must error (not a BufferAllocationException).
	if _, err := MinBufferSize(-1, 0.75); err == nil {
		t.Fatalf("MinBufferSize(-1): expected error, got nil")
	}

	// Demand more than MaxHashArrayLength can hold must surface BAE.
	// elements close to MaxInt with low load factor.
	_, err := MinBufferSize(math.MaxInt32, MinLoadFactor)
	var bae *BufferAllocationException
	if !errors.As(err, &bae) {
		t.Fatalf("MinBufferSize overflow: want *BufferAllocationException, got %T (%v)", err, err)
	}
}

func TestCheckLoadFactor(t *testing.T) {
	t.Parallel()
	if err := CheckLoadFactor(0.5, MinLoadFactor, MaxLoadFactor); err != nil {
		t.Fatalf("CheckLoadFactor(0.5): unexpected error: %v", err)
	}
	if err := CheckLoadFactor(MinLoadFactor, MinLoadFactor, MaxLoadFactor); err != nil {
		t.Fatalf("CheckLoadFactor at min: unexpected error: %v", err)
	}
	if err := CheckLoadFactor(MaxLoadFactor, MinLoadFactor, MaxLoadFactor); err != nil {
		t.Fatalf("CheckLoadFactor at max: unexpected error: %v", err)
	}
	var bae *BufferAllocationException
	if err := CheckLoadFactor(0.0, MinLoadFactor, MaxLoadFactor); !errors.As(err, &bae) {
		t.Fatalf("CheckLoadFactor(0.0): want *BufferAllocationException, got %T (%v)", err, err)
	}
	if err := CheckLoadFactor(1.0, MinLoadFactor, MaxLoadFactor); !errors.As(err, &bae) {
		t.Fatalf("CheckLoadFactor(1.0): want *BufferAllocationException, got %T (%v)", err, err)
	}
}

func TestCheckPowerOfTwoInternal(t *testing.T) {
	t.Parallel()
	for _, n := range []int{2, 4, 8, 16, 1024, 1 << 30} {
		if err := checkPowerOfTwo(n); err != nil {
			t.Fatalf("checkPowerOfTwo(%d): unexpected error: %v", n, err)
		}
	}
	for _, n := range []int{-1, 0, 1, 3, 6, 7, 9, 100} {
		if err := checkPowerOfTwo(n); err == nil {
			t.Fatalf("checkPowerOfTwo(%d): expected error", n)
		}
	}
}
