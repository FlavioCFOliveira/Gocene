// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import "fmt"

// FrequencyTrackingRingBuffer is a fixed-size ring buffer that also
// tracks how many times each value currently in the buffer occurs.
//
// It is the Go port of org.apache.lucene.util.FrequencyTrackingRingBuffer.
// Internally a sentinel-filled int slice models the FIFO buffer and an
// open-addressing IntBag-style hash table maintains per-value counts.
// Each Add(i) evicts the oldest element from the buffer and decrements
// its frequency, then inserts the new value and increments its
// frequency. Frequency(i) returns the current count for i (0 when
// absent). All operations are O(1) amortized.
//
// Used by SearcherManager caches for popularity-aware eviction.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/FrequencyTrackingRingBuffer.java
type FrequencyTrackingRingBuffer struct {
	maxSize   int
	buffer    []int
	position  int
	frequency *intBag
}

// NewFrequencyTrackingRingBuffer creates a buffer that retains at most
// maxSize entries. The buffer is pre-filled with sentinel, whose initial
// frequency is therefore maxSize. maxSize must be > 0 (Lucene enforces
// >= 2 in its constructor argument check; we mirror that).
func NewFrequencyTrackingRingBuffer(maxSize, sentinel int) (*FrequencyTrackingRingBuffer, error) {
	if maxSize < 2 {
		return nil, fmt.Errorf("maxSize must be >= 2, got %d", maxSize)
	}
	buf := make([]int, maxSize)
	for i := range buf {
		buf[i] = sentinel
	}
	// Capacity: ceil((maxSize * 4) / 3) rounded up to the next power of two,
	// matching Lucene's IntBag sizing for ~75% load factor.
	target := maxSize + (maxSize+2)/3
	cap := 1
	for cap < target {
		cap <<= 1
	}
	bag := newIntBag(cap)
	for i := 0; i < maxSize; i++ {
		bag.add(sentinel)
	}
	return &FrequencyTrackingRingBuffer{
		maxSize:   maxSize,
		buffer:    buf,
		frequency: bag,
	}, nil
}

// Add inserts i into the ring buffer, evicting the oldest entry and
// updating per-value frequencies accordingly.
func (r *FrequencyTrackingRingBuffer) Add(i int) {
	evicted := r.buffer[r.position]
	r.buffer[r.position] = i
	r.position++
	if r.position == r.maxSize {
		r.position = 0
	}
	r.frequency.add(i)
	r.frequency.remove(evicted)
}

// Frequency returns the number of occurrences of i currently in the
// buffer, or 0 if i is not present.
func (r *FrequencyTrackingRingBuffer) Frequency(i int) int {
	return r.frequency.frequency(i)
}

// AsFrequencyMap returns a snapshot of the current frequency table as a
// fresh map[int]int. It exists primarily for tests and diagnostics; the
// Lucene reference exposes Map<Integer,Integer> asFrequencyMap() with
// the same semantics.
func (r *FrequencyTrackingRingBuffer) AsFrequencyMap() map[int]int {
	out := make(map[int]int, r.frequency.size)
	for i, k := range r.frequency.keys {
		if r.frequency.freqs[i] > 0 {
			out[k] = r.frequency.freqs[i]
		}
	}
	return out
}

// intBag is an open-addressing hash table from int keys to non-negative
// counts. Capacity is a power of two. Empty slots have freqs[i] == 0.
// Deletion uses Robin-Hood-style backward shift to keep probe chains
// contiguous, matching Lucene's IntBag.relocateAdjacentKeys.
type intBag struct {
	keys  []int
	freqs []int
	mask  int
	size  int
}

func newIntBag(capacity int) *intBag {
	return &intBag{
		keys:  make([]int, capacity),
		freqs: make([]int, capacity),
		mask:  capacity - 1,
	}
}

// slot returns the initial probe position for key.
func (b *intBag) slot(key int) int {
	// 32-bit murmur-hash-like mixer adapted from Lucene's mix32.
	h := uint32(key)
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return int(h) & b.mask
}

// next advances a probe index with wrap-around.
func (b *intBag) next(i int) int {
	return (i + 1) & b.mask
}

func (b *intBag) frequency(key int) int {
	for i := b.slot(key); b.freqs[i] != 0; i = b.next(i) {
		if b.keys[i] == key {
			return b.freqs[i]
		}
	}
	return 0
}

// add inserts a single occurrence of key.
func (b *intBag) add(key int) {
	for i := b.slot(key); ; i = b.next(i) {
		if b.freqs[i] == 0 {
			b.keys[i] = key
			b.freqs[i] = 1
			b.size++
			return
		}
		if b.keys[i] == key {
			b.freqs[i]++
			return
		}
	}
}

// remove decrements a single occurrence of key. If the count reaches
// zero, the slot is cleared and any displaced successors are shifted
// back so probe chains remain valid.
func (b *intBag) remove(key int) {
	for i := b.slot(key); b.freqs[i] != 0; i = b.next(i) {
		if b.keys[i] == key {
			b.freqs[i]--
			if b.freqs[i] == 0 {
				b.relocate(i)
				b.size--
			}
			return
		}
	}
}

// relocate fills the hole at index `hole` by shifting later entries
// whose natural slot is at or before the hole, restoring the
// open-addressing invariant. Implements Knuth's Algorithm R for
// backward-shift deletion from a linear-probe hash table.
//
// The walk proceeds forward from `hole+1` and only stops at the first
// empty slot. Each non-empty slot whose natural index lies in the
// `(hole, j]` arc (clockwise) cannot move (moving it would break its
// own probe sequence); the walk skips it and continues. Otherwise the
// entry is moved into the hole and the hole jumps to its old position.
func (b *intBag) relocate(hole int) {
	j := b.next(hole)
	for b.freqs[j] != 0 {
		natural := b.slot(b.keys[j])
		// Entry must stay iff its natural slot falls in (hole, j].
		mustStay := inOpenClosed(hole, j, natural)
		if mustStay {
			// Skip this entry; advance j and continue probing.
			j = b.next(j)
			continue
		}
		// Move the entry into the hole; the hole jumps to j.
		b.keys[hole] = b.keys[j]
		b.freqs[hole] = b.freqs[j]
		hole = j
		j = b.next(j)
	}
	b.keys[hole] = 0
	b.freqs[hole] = 0
}

// inOpenClosed reports whether x falls in the (lo, hi] arc clockwise.
// Used by intBag.relocate; lo, hi and x are slot indices into the same
// power-of-two-sized table so modular arithmetic falls out automatically
// from the slot bounds — the helper itself only needs to distinguish
// the no-wrap (lo < hi) and wrap (lo > hi) cases.
func inOpenClosed(lo, hi, x int) bool {
	if lo == hi {
		return false
	}
	if lo < hi {
		return x > lo && x <= hi
	}
	// Wrap around: arc goes lo+1..mask, 0..hi.
	return x > lo || x <= hi
}
