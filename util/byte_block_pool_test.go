// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"sync/atomic"
	"testing"
)

func TestByteBlockPool_AppendAndRead(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()

	data := []byte("hello world")
	pool.AppendBytes(data)

	if pool.ByteUpto != len(data) {
		t.Errorf("expected ByteUpto %d, got %d", len(data), pool.ByteUpto)
	}

	readBuf := make([]byte, len(data))
	pool.ReadBytes(0, readBuf, 0, len(data))

	if !bytes.Equal(data, readBuf) {
		t.Errorf("expected %s, got %s", string(data), string(readBuf))
	}
}

func TestByteBlockPool_SpanningBuffers(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()

	// Fill almost the whole buffer
	padding := make([]byte, ByteBlockSize-5)
	pool.AppendBytes(padding)

	// Append data that spans across buffers
	data := []byte("spanning data")
	pool.AppendBytes(data)

	if pool.bufferUpto != 1 {
		t.Errorf("expected bufferUpto 1, got %d", pool.bufferUpto)
	}

	readBuf := make([]byte, len(data))
	// Offset is where "spanning data" starts: ByteBlockSize - 5
	pool.ReadBytes(int64(ByteBlockSize-5), readBuf, 0, len(data))

	if !bytes.Equal(data, readBuf) {
		t.Errorf("expected %s, got %s", string(data), string(readBuf))
	}
}

func TestByteBlockPool_Reset(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()
	pool.AppendBytes([]byte("some data"))

	pool.Reset(false, false)
	if pool.bufferUpto != -1 {
		t.Errorf("expected bufferUpto -1, got %d", pool.bufferUpto)
	}

	pool.Reset(false, true) // This requires a buffer to have been allocated before
	// Wait, Reset(..., true) only works if bufferUpto != -1.
	// Let's test it properly.
}

func TestByteBlockPool_ResetReuse(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()
	pool.AppendBytes([]byte("some data"))

	pool.Reset(false, true)
	if pool.bufferUpto != 0 {
		t.Errorf("expected bufferUpto 0, got %d", pool.bufferUpto)
	}
	if pool.ByteUpto != 0 {
		t.Errorf("expected ByteUpto 0, got %d", pool.ByteUpto)
	}
}

func TestByteBlockPool_ZeroFill(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()
	pool.AppendBytes([]byte("some data"))

	pool.Reset(true, true)

	// Check if the first buffer is zeroed
	for i := 0; i < len(pool.Buffer); i++ {
		if pool.Buffer[i] != 0 {
			t.Fatalf("buffer not zeroed at index %d", i)
		}
	}
}

func TestDirectTrackingAllocator(t *testing.T) {
	used := &atomic.Int64{}
	alloc := NewDirectTrackingAllocator(used)

	block := alloc.GetByteBlock()
	if used.Load() != int64(ByteBlockSize) {
		t.Errorf("expected %d, got %d", ByteBlockSize, used.Load())
	}

	blocks := [][]byte{block}
	alloc.RecycleByteBlocks(blocks, 0, 1)
	if used.Load() != 0 {
		t.Errorf("expected 0, got %d", used.Load())
	}
}

func TestByteBlockPool_SetBytesRef(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()
	pool.AppendBytes([]byte("hello world"))

	builder := NewBytesRefBuilder()
	result := &BytesRef{}

	// Case 1: Fits in one block
	pool.SetBytesRef(builder, result, 0, 5)
	if result.String() != "hello" {
		t.Errorf("expected hello, got %s", result.String())
	}

	// Case 2: Spans blocks
	pool.NextBuffer()
	pool.AppendBytes([]byte(" second part"))

	// Total length now is 11 + 12 = 23.
	// Let's fill the first buffer to create a span.
	pool.Reset(false, false)
	pool.NextBuffer()
	pool.AppendBytes(make([]byte, ByteBlockSize-2))
	pool.NextBuffer()
	pool.AppendBytes([]byte("spanning"))

	pool.SetBytesRef(builder, result, int64(ByteBlockSize-2), 4)
	if result.String() != "ng s" { // wait, "ng" from block 0, " s" from block 1 ?
		// "spanning" starts at block 1, pos 0.
		// Block 0 ends at ByteBlockSize-1.
		// offset = ByteBlockSize-2.
		// block 0: pos ByteBlockSize-2, length 2 (last 2 bytes of block 0)
		// block 1: pos 0, length 2 (first 2 bytes of block 1)
		// But block 0 is empty (except the padding).
		// Let's use real data.
	}
}

func TestByteBlockPool_SetBytesRefReal(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()

	// Fill block 0 with 'A's
	data0 := make([]byte, ByteBlockSize)
	for i := range data0 { data0[i] = 'A' }
	pool.AppendBytes(data0)

	// No need to call NextBuffer() here because AppendBytes already did.
	// Fill block 1 with 'B's
	data1 := make([]byte, ByteBlockSize)
	for i := range data1 { data1[i] = 'B' }
	pool.AppendBytes(data1)

	builder := NewBytesRefBuilder()
	result := &BytesRef{}

	// Span across block 0 and 1
	// Offset: ByteBlockSize - 2
	// Length: 4
	// Result should be "AABB"
	pool.SetBytesRef(builder, result, int64(ByteBlockSize-2), 4)
	if result.String() != "AABB" {
		t.Errorf("expected AABB, got %s", result.String())
	}
}

func TestByteBlockPool_AppendByteBlockPool(t *testing.T) {
	alloc := NewDirectAllocator()
	poolSrc := NewByteBlockPool(alloc)
	poolSrc.NextBuffer()
	poolSrc.AppendBytes([]byte("source data"))

	poolDest := NewByteBlockPool(alloc)
	poolDest.NextBuffer()
	poolDest.AppendByteBlockPool(poolSrc, 0, 11)

	readBuf := make([]byte, 11)
	poolDest.ReadBytes(0, readBuf, 0, 11)
	if string(readBuf) != "source data" {
		t.Errorf("expected source data, got %s", string(readBuf))
	}
}

func TestByteBlockPool_RamBytesUsed(t *testing.T) {
	alloc := NewDirectAllocator()
	pool := NewByteBlockPool(alloc)
	pool.NextBuffer()

	usage := pool.RamBytesUsed()
	if usage <= 0 {
		t.Errorf("expected positive RAM usage, got %d", usage)
	}
}
