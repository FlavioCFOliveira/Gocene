// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"testing"
)

// TestFrequencyTrackingRingBuffer mirrors Java
// TestFrequencyTrackingRingBuffer: invalid args, sentinel pre-fill,
// arithmetic invariants over random adds, and frequency-after-remove
// correctness across many cycles.
func TestFrequencyTrackingRingBuffer(t *testing.T) {
	t.Run("rejects small max size", func(t *testing.T) {
		if _, err := NewFrequencyTrackingRingBuffer(1, 0); err == nil {
			t.Fatalf("maxSize=1 must error")
		}
		if _, err := NewFrequencyTrackingRingBuffer(0, 0); err == nil {
			t.Fatalf("maxSize=0 must error")
		}
	})

	t.Run("initial sentinel frequency", func(t *testing.T) {
		r, err := NewFrequencyTrackingRingBuffer(8, -1)
		if err != nil {
			t.Fatalf("constructor: %v", err)
		}
		if got := r.Frequency(-1); got != 8 {
			t.Fatalf("sentinel initial freq got %d want 8", got)
		}
		if got := r.Frequency(0); got != 0 {
			t.Fatalf("unknown freq got %d want 0", got)
		}
	})

	t.Run("single value eviction", func(t *testing.T) {
		r, _ := NewFrequencyTrackingRingBuffer(4, 0)
		r.Add(7)
		r.Add(7)
		r.Add(7)
		r.Add(7)
		if got := r.Frequency(7); got != 4 {
			t.Fatalf("freq(7) got %d want 4", got)
		}
		if got := r.Frequency(0); got != 0 {
			t.Fatalf("sentinel after full fill got %d want 0", got)
		}
		r.Add(8)
		if got := r.Frequency(7); got != 3 {
			t.Fatalf("after add(8) freq(7) got %d want 3", got)
		}
		if got := r.Frequency(8); got != 1 {
			t.Fatalf("after add(8) freq(8) got %d want 1", got)
		}
	})

	t.Run("invariant: frequencies equal buffer counts", func(t *testing.T) {
		const maxSize = 16
		r, _ := NewFrequencyTrackingRingBuffer(maxSize, 0)
		rng := rand.New(rand.NewSource(42))
		for step := 0; step < 500; step++ {
			v := rng.Intn(8)
			r.Add(v)
			// Recompute frequencies from the buffer directly.
			expected := make(map[int]int)
			for _, x := range r.buffer {
				expected[x]++
			}
			for k, want := range expected {
				if got := r.Frequency(k); got != want {
					t.Fatalf("step=%d key=%d freq got %d want %d (buffer=%v)",
						step, k, got, want, r.buffer)
				}
			}
			// Values never present must report 0.
			if got := r.Frequency(9999); got != 0 {
				t.Fatalf("absent key freq got %d want 0", got)
			}
		}
	})

	t.Run("AsFrequencyMap matches recomputed map", func(t *testing.T) {
		r, _ := NewFrequencyTrackingRingBuffer(8, 0)
		for _, v := range []int{1, 1, 2, 3, 3, 3, 4, 4} {
			r.Add(v)
		}
		got := r.AsFrequencyMap()
		want := map[int]int{1: 2, 2: 1, 3: 3, 4: 2}
		if len(got) != len(want) {
			t.Fatalf("len got %d want %d (%v)", len(got), len(want), got)
		}
		for k, v := range want {
			if got[k] != v {
				t.Fatalf("key %d freq got %d want %d", k, got[k], v)
			}
		}
	})

	t.Run("hash collisions stress", func(t *testing.T) {
		// Use a small max size and pack keys that frequently collide.
		r, _ := NewFrequencyTrackingRingBuffer(8, -1)
		// All multiples of 8 share the same lower 3 bits, increasing
		// the chance of probe-chain interaction in the IntBag.
		for i := 0; i < 100; i++ {
			r.Add((i % 10) * 8)
		}
		// Recompute and compare.
		expected := make(map[int]int)
		for _, x := range r.buffer {
			expected[x]++
		}
		for k, want := range expected {
			if got := r.Frequency(k); got != want {
				t.Fatalf("collisions key %d got %d want %d", k, got, want)
			}
		}
	})
}
