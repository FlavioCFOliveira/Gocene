// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.index.memory.TestSlicedIntBlockPool.
package memory_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/memory"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// ByteTrackingAllocator — port of the inner class in TestSlicedIntBlockPool.
// Implements util.IntAllocator and tracks total bytes allocated/recycled.
// ---------------------------------------------------------------------------

type byteTrackingAllocator struct {
	blockSize int
	bytes     util.CounterAPI
}

func newByteTrackingAllocator(blockSize int, bytes util.CounterAPI) *byteTrackingAllocator {
	return &byteTrackingAllocator{blockSize: blockSize, bytes: bytes}
}

func (a *byteTrackingAllocator) GetIntBlock() []int32 {
	a.bytes.AddAndGet(int64(a.blockSize) * 4) // int32 = 4 bytes
	return make([]int32, a.blockSize)
}

func (a *byteTrackingAllocator) RecycleIntBlocks(blocks [][]int32, start, end int) {
	a.bytes.AddAndGet(-int64(end-start) * int64(a.blockSize) * 4)
}

// ---------------------------------------------------------------------------
// startEndAndValues — port of the inner class in TestSlicedIntBlockPool.
// ---------------------------------------------------------------------------

type startEndAndValues struct {
	valueOffset int
	valueCount  int
	start       int
	end         int
}

func newStartEndAndValues(valueOffset int) *startEndAndValues {
	return &startEndAndValues{valueOffset: valueOffset}
}

func (s *startEndAndValues) nextValue() int32 {
	v := int32(s.valueOffset + s.valueCount)
	s.valueCount++
	return v
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSlicedIntBlockPool_SingleWriterReader is a port of testSingleWriterReader.
// It writes a sequential run of integers into a single slice and reads them back.
func TestSlicedIntBlockPool_SingleWriterReader(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	alloc := newByteTrackingAllocator(util.IntBlockSize, bytesUsed)
	pool := memory.NewSlicedIntBlockPool(alloc)

	rng := rand.New(rand.NewSource(42))

	for j := 0; j < 2; j++ {
		writer := memory.NewSliceWriter(pool)
		start := writer.StartNewSlice()

		num := 100 + rng.Intn(100) // atLeast(100)
		for i := 0; i < num; i++ {
			writer.WriteInt(int32(i))
		}
		upto := writer.CurrentOffset()

		reader := memory.NewSliceReader(pool)
		reader.Reset(start, upto)
		for i := 0; i < num; i++ {
			if got := reader.ReadInt(); got != int32(i) {
				t.Fatalf("j=%d i=%d: expected %d, got %d", j, i, i, got)
			}
		}
		if !reader.EndOfSlice() {
			t.Fatalf("j=%d: expected EndOfSlice after reading all values", j)
		}

		if rng.Intn(2) == 0 {
			pool.Reset(true, false) // reuseFirst=false: all buffers recycled
			if got := bytesUsed.Get(); got != 0 {
				t.Errorf("j=%d reset(false): expected bytesUsed=0, got %d", j, got)
			}
		} else {
			pool.Reset(true, true) // reuseFirst=true: keep one buffer
			want := int64(util.IntBlockSize) * 4
			if got := bytesUsed.Get(); got != want {
				t.Errorf("j=%d reset(true): expected bytesUsed=%d, got %d", j, want, got)
			}
		}
	}
}

// TestSlicedIntBlockPool_MultipleWriterReader is a port of testMultipleWriterReader.
// It interleaves writes to multiple concurrent slices and verifies each can be
// read back independently.
func TestSlicedIntBlockPool_MultipleWriterReader(t *testing.T) {
	bytesUsed := util.NewSerialCounter()
	alloc := newByteTrackingAllocator(util.IntBlockSize, bytesUsed)
	pool := memory.NewSlicedIntBlockPool(alloc)

	rng := rand.New(rand.NewSource(99))

	for j := 0; j < 2; j++ {
		num := 4 + rng.Intn(4) // atLeast(4)
		holders := make([]*startEndAndValues, num)
		for i := range holders {
			holders[i] = newStartEndAndValues(rng.Intn(1000))
		}

		writer := memory.NewSliceWriter(pool)
		reader := memory.NewSliceReader(pool)

		numValues := 10000 + rng.Intn(5000) // atLeast(10000)
		for i := 0; i < numValues; i++ {
			sv := holders[rng.Intn(len(holders))]
			if sv.valueCount == 0 {
				sv.start = writer.StartNewSlice()
			} else {
				writer.Reset(sv.end)
			}
			writer.WriteInt(sv.nextValue())
			sv.end = writer.CurrentOffset()

			if rng.Intn(5) == 0 {
				// spot-check a random holder
				pick := holders[rng.Intn(len(holders))]
				assertSliceReader(t, reader, pick)
			}
		}

		// drain all remaining holders
		remaining := make([]*startEndAndValues, len(holders))
		copy(remaining, holders)
		for len(remaining) > 0 {
			idx := rng.Intn(len(remaining))
			assertSliceReader(t, reader, remaining[idx])
			remaining = append(remaining[:idx], remaining[idx+1:]...)
		}

		if rng.Intn(2) == 0 {
			pool.Reset(true, false)
			if got := bytesUsed.Get(); got != 0 {
				t.Errorf("j=%d reset(false): expected bytesUsed=0, got %d", j, got)
			}
		} else {
			pool.Reset(true, true)
			want := int64(util.IntBlockSize) * 4
			if got := bytesUsed.Get(); got != want {
				t.Errorf("j=%d reset(true): expected bytesUsed=%d, got %d", j, want, got)
			}
		}
	}
}

// assertSliceReader verifies that a SliceReader yields exactly the values
// written by the corresponding startEndAndValues holder, in order.
func assertSliceReader(t *testing.T, reader *memory.SliceReader, sv *startEndAndValues) {
	t.Helper()
	if sv.valueCount == 0 {
		return // nothing written yet
	}
	reader.Reset(sv.start, sv.end)
	for i := 0; i < sv.valueCount; i++ {
		want := int32(sv.valueOffset + i)
		if got := reader.ReadInt(); got != want {
			t.Fatalf("assertSliceReader: index %d expected %d, got %d", i, want, got)
		}
	}
	if !reader.EndOfSlice() {
		t.Fatalf("assertSliceReader: expected EndOfSlice after reading all %d values", sv.valueCount)
	}
}
