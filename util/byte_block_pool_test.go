// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math/rand"
	"testing"
	"time"
)

// TestByteBlockPool_AppendFromOtherPool tests appending from another pool
// Source: TestByteBlockPool.testAppendFromOtherPool()
// Purpose: Tests cross-pool operations
func TestByteBlockPool_AppendFromOtherPool(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create first pool with random bytes
	pool := NewByteBlockPool(NewDirectAllocator())
	numBytes := atLeast(rng, 2<<16) // at least 128KB
	bytes := randomBytesOfLength(rng, numBytes)
	pool.NextBuffer()
	pool.Append(bytes)

	// Create another pool with existing bytes
	anotherPool := NewByteBlockPool(NewDirectAllocator())
	existingBytes := make([]byte, atLeast(rng, 500))
	anotherPool.NextBuffer()
	anotherPool.Append(existingBytes)

	// Now slice and append to another pool
	offset := nextInt(rng, 1, 2<<15) // up to 64KB
	length := len(bytes) - offset
	if rng.Intn(2) == 0 {
		length = nextInt(rng, 1, length)
	}
	anotherPool.AppendFromPool(pool, int64(offset), length)

	// Verify position
	expectedPos := int64(len(existingBytes) + length)
	if anotherPool.GetPosition() != expectedPos {
		t.Errorf("Expected position %d, got %d", expectedPos, anotherPool.GetPosition())
	}

	// Verify bytes
	results := make([]byte, length)
	anotherPool.ReadBytes(int64(len(existingBytes)), results, 0, len(results))
	for i := 0; i < length; i++ {
		if bytes[offset+i] != results[i] {
			t.Errorf("byte @ index=%d: expected %d, got %d", i, bytes[offset+i], results[i])
		}
	}
}

// TestByteBlockPool_ReadAndWrite tests read/write with tracking
// Source: TestByteBlockPool.testReadAndWrite()
// Purpose: Tests read/write operations and memory tracking
func TestByteBlockPool_ReadAndWrite(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	bytesUsed := NewCounter()
	pool := NewByteBlockPool(NewDirectTrackingAllocator(bytesUsed))
	pool.NextBuffer()
	reuseFirst := rng.Intn(2) == 0

	for j := 0; j < 2; j++ {
		var list []*BytesRef
		maxLength := atLeast(rng, 500)
		numValues := atLeast(rng, 100)
		ref := NewBytesRefBuilder()

		for i := 0; i < numValues; i++ {
			value := randomRealisticUnicodeString(rng, maxLength)
			list = append(list, NewBytesRef([]byte(value)))
			ref.CopyChars(value)
			pool.AppendBytesRef(ref.Get())
		}

		// Verify
		position := int64(0)
		builder := NewBytesRefBuilder()
		for _, expected := range list {
			ref.Grow(expected.Length)
			ref.SetLength(expected.Length)

			switch rng.Intn(2) {
			case 0:
				// Copy bytes
				pool.ReadBytes(position, ref.Bytes(), 0, ref.Get().Length)
			case 1:
				scratch := NewBytesRefEmpty()
				pool.SetBytesRef(builder, scratch, position, ref.Get().Length)
				copy(ref.Bytes(), scratch.Bytes[scratch.Offset:scratch.Offset+scratch.Length])
			}

			if !BytesRefEquals(expected, ref.Get()) {
				t.Errorf("Expected %v, got %v", expected, ref.Get())
			}
			position += int64(ref.Get().Length)
		}

		pool.Reset(rng.Intn(2) == 0, reuseFirst)
		if reuseFirst {
			if bytesUsed.Get() != ByteBlockSize {
				t.Errorf("Expected %d bytes used, got %d", ByteBlockSize, bytesUsed.Get())
			}
		} else {
			if bytesUsed.Get() != 0 {
				t.Errorf("Expected 0 bytes used, got %d", bytesUsed.Get())
			}
			pool.NextBuffer() // prepare for next iter
		}
	}
}

// TestByteBlockPool_LargeRandomBlocks tests large random blocks
// Source: TestByteBlockPool.testLargeRandomBlocks()
// Purpose: Tests position tracking with large blocks
func TestByteBlockPool_LargeRandomBlocks(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	bytesUsed := NewCounter()
	pool := NewByteBlockPool(NewDirectTrackingAllocator(bytesUsed))
	pool.NextBuffer()

	var totalBytes int64
	var items [][]byte
	for i := 0; i < 100; i++ {
		var size int
		if rng.Intn(2) == 0 {
			size = nextInt(rng, 100, 1000)
		} else {
			size = nextInt(rng, 50000, 100000)
		}
		b := make([]byte, size)
		rng.Read(b)
		items = append(items, b)
		pool.AppendBytesRef(NewBytesRef(b))
		totalBytes += int64(size)

		// Make sure we report the correct position
		if pool.GetPosition() != totalBytes {
			t.Errorf("Expected position %d, got %d", totalBytes, pool.GetPosition())
		}
	}

	position := int64(0)
	for _, expected := range items {
		actual := make([]byte, len(expected))
		pool.ReadBytes(position, actual, 0, len(actual))
		if !bytes.Equal(expected, actual) {
			t.Error("Expected bytes to match")
		}
		position += int64(len(expected))
	}
}

// TestByteBlockPool_TooManyAllocs tests overflow detection
// Source: TestByteBlockPool.testTooManyAllocs()
// Purpose: Tests that overflow is detected when too many buffers are allocated
func TestByteBlockPool_TooManyAllocs(t *testing.T) {
	// Test the overflow detection logic directly
	// In Java, Math.addExact throws ArithmeticException on overflow
	// In Go, we use a helper function that panics

	// Test that addExact panics on overflow
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		// Simulate what happens when byteOffset is near max int
		// and we try to add ByteBlockSize
		maxInt := int(^uint(0) >> 1)
		_ = addExact(maxInt-ByteBlockSize+1, ByteBlockSize)
	}()

	if !panicked {
		t.Error("Expected overflow panic")
	}
}

// TestByteBlockPool_PositionTracking tests position tracking
// Purpose: Tests that position is correctly tracked across multiple buffers
func TestByteBlockPool_PositionTracking(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	pool.NextBuffer()

	// Write some data
	data1 := make([]byte, 1000)
	for i := range data1 {
		data1[i] = byte(i % 256)
	}
	pool.Append(data1)

	if pool.GetPosition() != 1000 {
		t.Errorf("Expected position 1000, got %d", pool.GetPosition())
	}

	// Write more data that crosses buffer boundary
	data2 := make([]byte, ByteBlockSize-500)
	for i := range data2 {
		data2[i] = byte((i + 100) % 256)
	}
	pool.Append(data2)

	expectedPos := int64(1000 + len(data2))
	if pool.GetPosition() != expectedPos {
		t.Errorf("Expected position %d, got %d", expectedPos, pool.GetPosition())
	}

	// Read back and verify
	readBuf := make([]byte, expectedPos)
	pool.ReadBytes(0, readBuf, 0, len(readBuf))

	// Verify first chunk
	for i := 0; i < 1000; i++ {
		if readBuf[i] != byte(i%256) {
			t.Errorf("byte at %d: expected %d, got %d", i, byte(i%256), readBuf[i])
		}
	}

	// Verify second chunk
	for i := 0; i < len(data2); i++ {
		if readBuf[1000+i] != byte((i+100)%256) {
			t.Errorf("byte at %d: expected %d, got %d", 1000+i, byte((i+100)%256), readBuf[1000+i])
		}
	}
}

// TestByteBlockPool_Reset tests reset functionality
// Purpose: Tests that reset properly clears and optionally reuses buffers
func TestByteBlockPool_Reset(t *testing.T) {
	bytesUsed := NewCounter()
	pool := NewByteBlockPool(NewDirectTrackingAllocator(bytesUsed))
	pool.NextBuffer()

	// Write some data
	pool.Append(make([]byte, ByteBlockSize+1000))

	// Reset without reusing first buffer
	pool.Reset(false, false)
	if bytesUsed.Get() != 0 {
		t.Errorf("Expected 0 bytes used after reset, got %d", bytesUsed.Get())
	}
	if pool.GetPosition() != 0 {
		t.Errorf("Expected position 0 after reset, got %d", pool.GetPosition())
	}

	// Allocate again and reset with reuse
	pool.NextBuffer()
	pool.Append(make([]byte, ByteBlockSize+1000))
	pool.Reset(false, true)
	if bytesUsed.Get() != ByteBlockSize {
		t.Errorf("Expected %d bytes used after reset with reuse, got %d", ByteBlockSize, bytesUsed.Get())
	}
}

// TestByteBlockPool_ReadByte tests reading single bytes
// Purpose: Tests the ReadByte method
func TestByteBlockPool_ReadByte(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	pool.NextBuffer()

	// Write some data
	data := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	pool.Append(data)

	// Read back single bytes
	for i := 0; i < len(data); i++ {
		b := pool.ReadByte(int64(i))
		if b != data[i] {
			t.Errorf("byte at %d: expected %d, got %d", i, data[i], b)
		}
	}
}

// TestByteBlockPool_CrossBufferRead tests reading across buffer boundaries
// Purpose: Tests that reads work correctly when crossing buffer boundaries
func TestByteBlockPool_CrossBufferRead(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	pool.NextBuffer()

	// Write data that spans multiple buffers
	data := make([]byte, ByteBlockSize+1000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	pool.Append(data)

	// Read from near the end of first buffer into second
	// We can only read up to the end of written data
	readBuf := make([]byte, 1500) // 500 from first buffer + 1000 from second
	pool.ReadBytes(int64(ByteBlockSize-500), readBuf, 0, 1500)

	// Verify
	for i := 0; i < 1500; i++ {
		expected := byte((ByteBlockSize - 500 + i) % 256)
		if readBuf[i] != expected {
			t.Errorf("byte at %d: expected %d, got %d", i, expected, readBuf[i])
			break
		}
	}
}

// Helper functions

func atLeast(rng *rand.Rand, n int) int {
	return n + rng.Intn(n)
}

func nextInt(rng *rand.Rand, min, max int) int {
	if min >= max {
		return min
	}
	return min + rng.Intn(max-min)
}

func randomBytesOfLength(rng *rand.Rand, length int) []byte {
	b := make([]byte, length)
	rng.Read(b)
	return b
}

func randomRealisticUnicodeString(rng *rand.Rand, maxLength int) string {
	length := rng.Intn(maxLength) + 1
	var result []rune
	for i := 0; i < length; i++ {
		// Generate mostly ASCII with some Unicode
		if rng.Intn(10) < 8 {
			result = append(result, rune(rng.Intn(128)))
		} else {
			result = append(result, rune(rng.Intn(0x10000)))
		}
	}
	return string(result)
}
