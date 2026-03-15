// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"hash/crc32"
	"math/rand"
	"testing"
)

// TestBufferedChecksum_Simple tests basic checksum computation with single byte updates.
// Source: TestBufferedChecksum.testSimple()
// Purpose: Verifies that BufferedChecksum produces the same result as raw CRC32 for simple byte sequences.
func TestBufferedChecksum_Simple(t *testing.T) {
	c := NewBufferedChecksum(crc32.NewIEEE())
	c.Update(1)
	c.Update(2)
	c.Update(3)

	// Expected value computed from Java's CRC32 with bytes 1, 2, 3
	expected := uint32(1438416925)
	if got := c.Sum32(); got != expected {
		t.Errorf("Simple checksum mismatch: got %d, expected %d", got, expected)
	}
}

// TestBufferedChecksum_Random tests randomized checksum operations.
// Source: TestBufferedChecksum.testRandom()
// Purpose: Verifies that BufferedChecksum produces identical results to raw CRC32
// across random sequences of operations (update bytes, update single byte, reset, getValue).
func TestBufferedChecksum_Random(t *testing.T) {
	c1 := crc32.NewIEEE()
	c2 := NewBufferedChecksum(crc32.NewIEEE())

	rng := rand.New(rand.NewSource(42)) // Fixed seed for reproducibility
	iterations := 10000

	for i := 0; i < iterations; i++ {
		switch rng.Intn(4) {
		case 0:
			// update(byte[], int, int)
			length := rng.Intn(1024)
			bytes := make([]byte, length)
			rng.Read(bytes)
			c1.Write(bytes)
			c2.UpdateBytes(bytes)
		case 1:
			// update(int) - single byte
			b := byte(rng.Intn(256))
			c1.Write([]byte{b})
			c2.Update(b)
		case 2:
			// reset()
			c1.Reset()
			c2.Reset()
		case 3:
			// getValue() - compare checksums
			if c1.Sum32() != c2.Sum32() {
				t.Errorf("Checksum mismatch at iteration %d: raw CRC32=%d, BufferedChecksum=%d",
					i, c1.Sum32(), c2.Sum32())
			}
		}
	}

	// Final comparison
	if c1.Sum32() != c2.Sum32() {
		t.Errorf("Final checksum mismatch: raw CRC32=%d, BufferedChecksum=%d",
			c1.Sum32(), c2.Sum32())
	}
}

// TestBufferedChecksum_DifferentInputTypes tests various input update methods.
// Source: TestBufferedChecksum.testDifferentInputTypes()
// Purpose: Verifies that updateShort, updateInt, updateLong, and updateLongs
// methods produce identical checksums to byte-by-byte updates.
func TestBufferedChecksum_DifferentInputTypes(t *testing.T) {
	rng := rand.New(rand.NewSource(12345)) // Fixed seed for reproducibility
	iterations := 1000

	for i := 0; i < iterations; i++ {
		input := make([]byte, 4096)
		rng.Read(input)

		// Compute reference checksum using raw CRC32
		crc := crc32.NewIEEE()
		crc.Write(input)
		expected := crc.Sum32()
		crc.Reset()

		// Test each update method
		testUpdateByShorts(t, expected, input, rng)
		testUpdateByInts(t, expected, input, rng)
		testUpdateByLongs(t, expected, input, rng)
		testUpdateByChunkOfBytes(t, expected, input, rng)
		testUpdateByChunkOfLongs(t, expected, input, rng)
	}
}

// testUpdateByChunkOfBytes tests byte-by-byte and chunk updates.
// Source: TestBufferedChecksum.updateByChunkOfBytes()
func testUpdateByChunkOfBytes(t *testing.T, expected uint32, input []byte, rng *rand.Rand) {
	buffered := NewBufferedChecksum(crc32.NewIEEE())

	// Update byte by byte
	for i := 0; i < len(input); i++ {
		buffered.Update(input[i])
	}
	checkChecksumValueAndReset(t, expected, buffered)

	// Update entire array at once
	buffered.UpdateBytes(input)
	checkChecksumValueAndReset(t, expected, buffered)

	// Random chunk updates
	numIterations := 10
	for ite := 0; ite < numIterations; ite++ {
		len0 := rng.Intn(len(input) / 2)
		buffered.UpdateBytes(input[:len0])
		buffered.UpdateBytes(input[len0:])
		checkChecksumValueAndReset(t, expected, buffered)

		buffered.UpdateBytes(input[:len0])
		len1 := rng.Intn(len(input) / 4)
		for i := 0; i < len1; i++ {
			buffered.Update(input[len0+i])
		}
		buffered.UpdateBytes(input[len0+len1:])
		checkChecksumValueAndReset(t, expected, buffered)
	}
}

// testUpdateByShorts tests updating with 16-bit values.
// Source: TestBufferedChecksum.updateByShorts()
func testUpdateByShorts(t *testing.T, expected uint32, input []byte, rng *rand.Rand) {
	buffered := NewBufferedChecksum(crc32.NewIEEE())
	ix := shiftArray(buffered, input, rng)

	for ix <= len(input)-2 {
		val := int16(input[ix]) | int16(input[ix+1])<<8
		buffered.UpdateShort(val)
		ix += 2
	}
	if ix < len(input) {
		buffered.UpdateBytes(input[ix:])
	}
	checkChecksumValueAndReset(t, expected, buffered)
}

// testUpdateByInts tests updating with 32-bit values.
// Source: TestBufferedChecksum.updateByInts()
func testUpdateByInts(t *testing.T, expected uint32, input []byte, rng *rand.Rand) {
	buffered := NewBufferedChecksum(crc32.NewIEEE())
	ix := shiftArray(buffered, input, rng)

	for ix <= len(input)-4 {
		val := int32(input[ix]) |
			int32(input[ix+1])<<8 |
			int32(input[ix+2])<<16 |
			int32(input[ix+3])<<24
		buffered.UpdateInt(val)
		ix += 4
	}
	if ix < len(input) {
		buffered.UpdateBytes(input[ix:])
	}
	checkChecksumValueAndReset(t, expected, buffered)
}

// testUpdateByLongs tests updating with 64-bit values.
// Source: TestBufferedChecksum.updateByLongs()
func testUpdateByLongs(t *testing.T, expected uint32, input []byte, rng *rand.Rand) {
	buffered := NewBufferedChecksum(crc32.NewIEEE())
	ix := shiftArray(buffered, input, rng)

	for ix <= len(input)-8 {
		val := int64(input[ix]) |
			int64(input[ix+1])<<8 |
			int64(input[ix+2])<<16 |
			int64(input[ix+3])<<24 |
			int64(input[ix+4])<<32 |
			int64(input[ix+5])<<40 |
			int64(input[ix+6])<<48 |
			int64(input[ix+7])<<56
		buffered.UpdateLong(val)
		ix += 8
	}
	if ix < len(input) {
		buffered.UpdateBytes(input[ix:])
	}
	checkChecksumValueAndReset(t, expected, buffered)
}

// testUpdateByChunkOfLongs tests batch long array updates.
// Source: TestBufferedChecksum.updateByChunkOfLongs()
func testUpdateByChunkOfLongs(t *testing.T, expected uint32, input []byte, rng *rand.Rand) {
	buffered := NewBufferedChecksum(crc32.NewIEEE())
	ix := rng.Intn(len(input) / 4)
	remaining := 8 - (ix & 7)
	if remaining == 8 {
		remaining = 0
	}

	// Convert bytes to longs (little-endian)
	numLongs := (len(input) - ix) / 8
	longInput := make([]int64, numLongs)
	for i := 0; i < numLongs; i++ {
		longInput[i] = int64(input[ix+i*8]) |
			int64(input[ix+i*8+1])<<8 |
			int64(input[ix+i*8+2])<<16 |
			int64(input[ix+i*8+3])<<24 |
			int64(input[ix+i*8+4])<<32 |
			int64(input[ix+i*8+5])<<40 |
			int64(input[ix+i*8+6])<<48 |
			int64(input[ix+i*8+7])<<56
	}

	// Test individual long updates
	buffered.UpdateBytes(input[:ix])
	for i := 0; i < len(longInput); i++ {
		buffered.UpdateLong(longInput[i])
	}
	if remaining > 0 {
		buffered.UpdateBytes(input[len(input)-remaining:])
	}
	checkChecksumValueAndReset(t, expected, buffered)

	// Test batch long updates
	buffered.UpdateBytes(input[:ix])
	buffered.UpdateLongs(longInput, 0, len(longInput))
	if remaining > 0 {
		buffered.UpdateBytes(input[len(input)-remaining:])
	}
	checkChecksumValueAndReset(t, expected, buffered)

	// Random chunk updates with longs
	numIterations := 10
	for ite := 0; ite < numIterations; ite++ {
		len0 := rng.Intn(len(longInput) / 2)

		buffered.UpdateBytes(input[:ix])
		buffered.UpdateLongs(longInput, 0, len0)
		buffered.UpdateLongs(longInput, len0, len(longInput)-len0)
		if remaining > 0 {
			buffered.UpdateBytes(input[len(input)-remaining:])
		}
		checkChecksumValueAndReset(t, expected, buffered)

		buffered.UpdateBytes(input[:ix])
		buffered.UpdateLongs(longInput, 0, len0)
		len1 := rng.Intn(len(longInput) / 4)
		for i := 0; i < len1; i++ {
			buffered.UpdateLong(longInput[len0+i])
		}
		buffered.UpdateLongs(longInput, len0+len1, len(longInput)-len1-len0)
		if remaining > 0 {
			buffered.UpdateBytes(input[len(input)-remaining:])
		}
		checkChecksumValueAndReset(t, expected, buffered)

		buffered.UpdateBytes(input[:ix])
		buffered.UpdateLongs(longInput, 0, len0)
		buffered.UpdateBytes(input[ix+len0*8:])
		checkChecksumValueAndReset(t, expected, buffered)
	}
}

// shiftArray updates the checksum with a random initial portion of the input.
// Source: TestBufferedChecksum.shiftArray()
func shiftArray(buffered *BufferedChecksum, input []byte, rng *rand.Rand) int {
	ix := rng.Intn(len(input) / 4)
	buffered.UpdateBytes(input[:ix])
	return ix
}

// checkChecksumValueAndReset verifies the checksum matches expected and resets.
// Source: TestBufferedChecksum.checkChecksumValueAndReset()
func checkChecksumValueAndReset(t *testing.T, expected uint32, checksum *BufferedChecksum) {
	t.Helper()
	if got := checksum.Sum32(); got != expected {
		t.Errorf("Checksum mismatch: got %d, expected %d", got, expected)
	}
	checksum.Reset()
}

// TestBufferedChecksum_BufferSize tests different buffer sizes.
// Purpose: Verifies that BufferedChecksum works correctly with various buffer sizes.
func TestBufferedChecksum_BufferSize(t *testing.T) {
	sizes := []int{1, 16, 64, 256, 1024, 4096}
	data := make([]byte, 10000)
	rand.Read(data)

	// Compute reference checksum
	ref := crc32.NewIEEE()
	ref.Write(data)
	expected := ref.Sum32()

	for _, size := range sizes {
		buffered := NewBufferedChecksumWithSize(crc32.NewIEEE(), size)
		buffered.UpdateBytes(data)
		if got := buffered.Sum32(); got != expected {
			t.Errorf("Buffer size %d: checksum mismatch: got %d, expected %d", size, got, expected)
		}
	}
}

// TestBufferedChecksum_EmptyInput tests behavior with empty input.
// Purpose: Verifies that BufferedChecksum handles empty updates correctly.
func TestBufferedChecksum_EmptyInput(t *testing.T) {
	c1 := crc32.NewIEEE()
	c2 := NewBufferedChecksum(crc32.NewIEEE())

	// Empty update
	c2.UpdateBytes([]byte{})

	if c1.Sum32() != c2.Sum32() {
		t.Errorf("Empty input checksum mismatch")
	}
}

// TestBufferedChecksum_Reset tests the reset functionality.
// Purpose: Verifies that Reset() properly clears the buffer and underlying checksum.
func TestBufferedChecksum_Reset(t *testing.T) {
	c := NewBufferedChecksum(crc32.NewIEEE())

	// Add some data
	c.UpdateBytes([]byte{1, 2, 3, 4, 5})

	// Reset
	c.Reset()

	// Add same data again
	c.UpdateBytes([]byte{1, 2, 3, 4, 5})

	// Compute expected
	ref := crc32.NewIEEE()
	ref.Write([]byte{1, 2, 3, 4, 5})
	expected := ref.Sum32()

	if got := c.Sum32(); got != expected {
		t.Errorf("After reset checksum mismatch: got %d, expected %d", got, expected)
	}
}

// TestBufferedChecksum_LargeData tests with data larger than buffer.
// Purpose: Verifies that BufferedChecksum correctly handles data larger than internal buffer.
func TestBufferedChecksum_LargeData(t *testing.T) {
	data := make([]byte, 10000)
	rand.Read(data)

	c1 := crc32.NewIEEE()
	c1.Write(data)
	expected := c1.Sum32()

	c2 := NewBufferedChecksumWithSize(crc32.NewIEEE(), 256) // Small buffer
	c2.UpdateBytes(data)

	if got := c2.Sum32(); got != expected {
		t.Errorf("Large data checksum mismatch: got %d, expected %d", got, expected)
	}
}

// TestBufferedChecksum_UpdateShortBoundary tests short updates at buffer boundary.
// Purpose: Verifies correct behavior when short updates cross buffer boundaries.
func TestBufferedChecksum_UpdateShortBoundary(t *testing.T) {
	buffered := NewBufferedChecksumWithSize(crc32.NewIEEE(), 10) // Small buffer for testing

	// Fill buffer partially (9 bytes)
	for i := 0; i < 9; i++ {
		buffered.Update(byte(i))
	}

	// Add a short (2 bytes) - should trigger flush
	buffered.UpdateShort(0x1234)

	// Add more data
	buffered.Update(byte(10))

	// Compute expected
	ref := crc32.NewIEEE()
	for i := 0; i < 9; i++ {
		ref.Write([]byte{byte(i)})
	}
	ref.Write([]byte{0x34, 0x12}) // Little-endian
	ref.Write([]byte{10})
	expected := ref.Sum32()

	if got := buffered.Sum32(); got != expected {
		t.Errorf("Boundary short update checksum mismatch: got %d, expected %d", got, expected)
	}
}

// TestBufferedChecksum_UpdateIntBoundary tests int updates at buffer boundary.
// Purpose: Verifies correct behavior when int updates cross buffer boundaries.
func TestBufferedChecksum_UpdateIntBoundary(t *testing.T) {
	buffered := NewBufferedChecksumWithSize(crc32.NewIEEE(), 10) // Small buffer for testing

	// Fill buffer partially (7 bytes)
	for i := 0; i < 7; i++ {
		buffered.Update(byte(i))
	}

	// Add an int (4 bytes) - should trigger flush
	buffered.UpdateInt(0x12345678)

	// Compute expected
	ref := crc32.NewIEEE()
	for i := 0; i < 7; i++ {
		ref.Write([]byte{byte(i)})
	}
	ref.Write([]byte{0x78, 0x56, 0x34, 0x12}) // Little-endian
	expected := ref.Sum32()

	if got := buffered.Sum32(); got != expected {
		t.Errorf("Boundary int update checksum mismatch: got %d, expected %d", got, expected)
	}
}

// TestBufferedChecksum_UpdateLongBoundary tests long updates at buffer boundary.
// Purpose: Verifies correct behavior when long updates cross buffer boundaries.
func TestBufferedChecksum_UpdateLongBoundary(t *testing.T) {
	buffered := NewBufferedChecksumWithSize(crc32.NewIEEE(), 16) // Small buffer for testing

	// Fill buffer partially (10 bytes)
	for i := 0; i < 10; i++ {
		buffered.Update(byte(i))
	}

	// Add a long (8 bytes) - should trigger flush
	buffered.UpdateLong(0x123456789ABCDEF0)

	// Compute expected
	ref := crc32.NewIEEE()
	for i := 0; i < 10; i++ {
		ref.Write([]byte{byte(i)})
	}
	ref.Write([]byte{0xF0, 0xDE, 0xBC, 0x9A, 0x78, 0x56, 0x34, 0x12}) // Little-endian
	expected := ref.Sum32()

	if got := buffered.Sum32(); got != expected {
		t.Errorf("Boundary long update checksum mismatch: got %d, expected %d", got, expected)
	}
}

// TestBufferedChecksum_UpdateLongsBatch tests batch long array updates.
// Purpose: Verifies that UpdateLongs correctly handles batch updates.
func TestBufferedChecksum_UpdateLongsBatch(t *testing.T) {
	longs := []int64{0x123456789ABCDEF0, 0x0FEDCBA987654321, 0x1122334455667788}

	// Compute expected using individual long updates
	ref := crc32.NewIEEE()
	for _, v := range longs {
		b := []byte{
			byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24),
			byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56),
		}
		ref.Write(b)
	}
	expected := ref.Sum32()

	// Test with BufferedChecksum using batch update
	buffered := NewBufferedChecksum(crc32.NewIEEE())
	buffered.UpdateLongs(longs, 0, len(longs))

	if got := buffered.Sum32(); got != expected {
		t.Errorf("Batch longs update checksum mismatch: got %d, expected %d", got, expected)
	}
}

// TestBufferedChecksum_Hash32Interface tests that BufferedChecksum implements hash.Hash32.
// Purpose: Verifies BufferedChecksum satisfies the hash.Hash32 interface.
func TestBufferedChecksum_Hash32Interface(t *testing.T) {
	c := NewBufferedChecksum(crc32.NewIEEE())

	// Test Write method (from hash.Hash interface)
	data := []byte{1, 2, 3, 4, 5}
	n, err := c.Write(data)
	if err != nil {
		t.Errorf("Write returned error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned wrong count: got %d, expected %d", n, len(data))
	}

	// Test Sum method
	sum := c.Sum(nil)
	if len(sum) == 0 {
		t.Error("Sum returned empty slice")
	}

	// Test Size method
	if c.Size() != 4 { // CRC32 produces 4 bytes
		t.Errorf("Size returned wrong value: got %d, expected 4", c.Size())
	}

	// Test BlockSize method
	if c.BlockSize() != 1 { // CRC32 has block size of 1
		t.Errorf("BlockSize returned wrong value: got %d, expected 1", c.BlockSize())
	}
}
