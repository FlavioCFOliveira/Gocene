// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"math/rand"
	"testing"
)

// Helper functions for tests

func generateRandomData(rng *rand.Rand, count int) []byte {
	data := make([]byte, count)
	rng.Read(data)
	return data
}

func computeRamBytesUsed(o *ByteBuffersDataOutput) int64 {
	if o.Size() == 0 {
		return 0
	}
	var total int64
	for i := 0; i < o.BufferCount(); i++ {
		total += int64(o.BlockCapacity(i))
	}
	total += int64(o.BufferCount()) * NumBytesObjectRef
	return total
}

// Tests

// TestByteBuffersDataOutput_Reuse tests buffer recycling functionality.
// Source: TestByteBuffersDataOutput.testReuse()
// Purpose: Verifies that buffers are properly recycled when reset() is called
func TestByteBuffersDataOutput_Reuse(t *testing.T) {
	allocations := 0
	allocator := func(size int) []byte {
		allocations++
		return make([]byte, size)
	}

	recycler := func(buf []byte) {
	}

	o := NewByteBuffersDataOutputWithRecycler(
		DefaultMinBitsPerBlock,
		DefaultMaxBitsPerBlock,
		allocator,
		recycler,
	)

	seed := rand.Int63()
	addCount := rand.Intn(4000) + 1000
	rng := rand.New(rand.NewSource(seed))
	data := generateRandomData(rng, addCount)
	o.WriteBytes(data)
	result := o.ToArrayCopy()

	expectedAllocations := allocations

	o.Reset()
	rng = rand.New(rand.NewSource(seed))
	data = generateRandomData(rng, addCount)
	o.WriteBytes(data)

	if allocations != expectedAllocations {
		t.Errorf("Expected %d allocations, got %d", expectedAllocations, allocations)
	}

	if !bytes.Equal(result, o.ToArrayCopy()) {
		t.Error("Data mismatch after reuse")
	}
}

// TestByteBuffersDataOutput_ConstructorWithExpectedSize tests constructor with expected size hint.
// Source: TestByteBuffersDataOutput.testConstructorWithExpectedSize()
func TestByteBuffersDataOutput_ConstructorWithExpectedSize(t *testing.T) {
	t.Run("zero_size", func(t *testing.T) {
		o := NewByteBuffersDataOutputWithSize(0)
		o.WriteByte(0)
		buffers := o.ToBufferList()
		if len(buffers) == 0 {
			t.Fatal("Expected at least one buffer")
		}
		expectedCapacity := 1 << DefaultMinBitsPerBlock
		if buffers[0].Len() < expectedCapacity {
			t.Errorf("Expected capacity >= %d, got %d", expectedCapacity, buffers[0].Len())
		}
	})

	t.Run("small_size", func(t *testing.T) {
		o := NewByteBuffersDataOutputWithSize(1 << 20) // 1 MB
		o.WriteByte(0)
		buffers := o.ToBufferList()
		if len(buffers) == 0 {
			t.Fatal("Expected at least one buffer")
		}
		expectedCapacity := 1 << 14 // 16384 B
		if buffers[0].Len() < expectedCapacity {
			t.Errorf("Expected capacity >= %d, got %d", expectedCapacity, buffers[0].Len())
		}
	})

	t.Run("large_size", func(t *testing.T) {
		o := NewByteBuffersDataOutputWithSize(1 << 30) // 1 GB
		o.WriteByte(0)
		buffers := o.ToBufferList()
		if len(buffers) == 0 {
			t.Fatal("Expected at least one buffer")
		}
		expectedCapacity := 1 << 24 // 16 MB
		if buffers[0].Len() < expectedCapacity {
			t.Errorf("Expected capacity >= %d, got %d", expectedCapacity, buffers[0].Len())
		}
	})
}

// TestByteBuffersDataOutput_RandomWrites tests random write operations.
// Source: TestByteBuffersDataOutput.testRandomWrites()
func TestByteBuffersDataOutput_RandomWrites(t *testing.T) {
	seed := rand.Int63()
	rng := rand.New(rand.NewSource(seed))

	numWrites := rng.Intn(50000) + 10000
	o := NewByteBuffersDataOutput()

	var expected bytes.Buffer
	for i := 0; i < numWrites; i++ {
		v := rng.Intn(256)
		o.WriteByte(byte(v))
		expected.WriteByte(byte(v))
	}

	result := o.ToArrayCopy()
	if !bytes.Equal(result, expected.Bytes()) {
		t.Errorf("Data mismatch after %d writes", numWrites)
	}
}

// TestByteBuffersDataOutput_RamBytesUsed tests RAM usage calculation.
// Source: TestByteBuffersDataOutput.testRamBytesUsed()
func TestByteBuffersDataOutput_RamBytesUsed(t *testing.T) {
	seed := rand.Int63()
	rng := rand.New(rand.NewSource(seed))

	numWrites := rng.Intn(50000) + 10000
	o := NewByteBuffersDataOutput()

	var expected bytes.Buffer
	for i := 0; i < numWrites; i++ {
		v := rng.Intn(256)
		o.WriteByte(byte(v))
		expected.WriteByte(byte(v))
	}

	expectedRamBytesUsed := computeRamBytesUsed(o)
	if o.RamBytesUsed() != expectedRamBytesUsed {
		t.Errorf("Expected RamBytesUsed %d, got %d", expectedRamBytesUsed, o.RamBytesUsed())
	}
}

// TestByteBuffersDataOutput_ToBufferList tests buffer list conversion.
// Source: TestByteBuffersDataOutput.testToBufferList()
func TestByteBuffersDataOutput_ToBufferList(t *testing.T) {
	o := NewByteBuffersDataOutput()

	// Write some data
	data := make([]byte, 1000)
	rand.Read(data)
	o.WriteBytes(data)

	buffers := o.ToBufferList()
	if len(buffers) == 0 {
		t.Fatal("Expected at least one buffer")
	}

	// Verify all buffers are read-only
	for i, buf := range buffers {
		if !buf.IsReadOnly() {
			t.Errorf("Buffer %d should be read-only", i)
		}
	}
}

// TestByteBuffersDataOutput_ToWriteableBufferList tests writable buffer list conversion.
// Source: TestByteBuffersDataOutput.testToWriteableBufferList()
func TestByteBuffersDataOutput_ToWriteableBufferList(t *testing.T) {
	o := NewByteBuffersDataOutput()

	// Write some data
	data := make([]byte, 1000)
	rand.Read(data)
	o.WriteBytes(data)

	buffers := o.ToWriteableBufferList()
	if len(buffers) == 0 {
		t.Fatal("Expected at least one buffer")
	}

	// Verify all buffers are writable
	for i, buf := range buffers {
		if buf.IsReadOnly() {
			t.Errorf("Buffer %d should be writable", i)
		}
	}
}

// TestByteBuffersDataOutput_Reset tests reset functionality.
// Source: TestByteBuffersDataOutput.testReset()
func TestByteBuffersDataOutput_Reset(t *testing.T) {
	o := NewByteBuffersDataOutput()

	// Write some data
	data := make([]byte, 1000)
	rand.Read(data)
	o.WriteBytes(data)

	if o.Size() == 0 {
		t.Fatal("Expected non-zero size after writing")
	}

	o.Reset()

	if o.Size() != 0 {
		t.Errorf("Expected size 0 after reset, got %d", o.Size())
	}

	// Should be able to write again after reset
	o.WriteByte(42)
	if o.Size() != 1 {
		t.Errorf("Expected size 1 after writing, got %d", o.Size())
	}
}

// TestByteBuffersDataOutput_WriteVInt tests variable-length integer writing.
// Source: TestByteBuffersDataOutput.testWriteVInt()
func TestByteBuffersDataOutput_WriteVInt(t *testing.T) {
	testCases := []struct {
		value    int32
		expected []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
		{16383, []byte{0xFF, 0x7F}},
		{16384, []byte{0x80, 0x80, 0x01}},
	}

	for _, tc := range testCases {
		o := NewByteBuffersDataOutput()
		o.WriteVInt(tc.value)
		result := o.ToArrayCopy()
		if !bytes.Equal(result, tc.expected) {
			t.Errorf("WriteVInt(%d): expected %v, got %v", tc.value, tc.expected, result)
		}
	}
}

// TestByteBuffersDataOutput_WriteVLong tests variable-length long writing.
// Source: TestByteBuffersDataOutput.testWriteVLong()
func TestByteBuffersDataOutput_WriteVLong(t *testing.T) {
	testCases := []struct {
		value    int64
		expected []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
		{16383, []byte{0xFF, 0x7F}},
		{16384, []byte{0x80, 0x80, 0x01}},
	}

	for _, tc := range testCases {
		o := NewByteBuffersDataOutput()
		o.WriteVLong(tc.value)
		result := o.ToArrayCopy()
		if !bytes.Equal(result, tc.expected) {
			t.Errorf("WriteVLong(%d): expected %v, got %v", tc.value, tc.expected, result)
		}
	}
}

// TestByteBuffersDataOutput_WriteString tests string writing.
// Source: TestByteBuffersDataOutput.testWriteString()
func TestByteBuffersDataOutput_WriteString(t *testing.T) {
	testCases := []string{
		"",
		"hello",
		"world",
		"test string with spaces",
		"special chars: !@#$%^&*()",
	}

	for _, s := range testCases {
		o := NewByteBuffersDataOutput()
		o.WriteString(s)

		// Read back using ByteArrayDataInput
		data := o.ToArrayCopy()
		in := NewByteArrayDataInput(data)

		// Read length
		length, err := in.ReadVInt()
		if err != nil {
			t.Fatalf("Failed to read length for %q: %v", s, err)
		}

		// Read string bytes
		strBytes := make([]byte, length)
		if err := in.ReadBytes(strBytes); err != nil {
			t.Fatalf("Failed to read string bytes for %q: %v", s, err)
		}

		result := string(strBytes)
		if result != s {
			t.Errorf("WriteString(%q): got %q", s, result)
		}
	}
}

// TestByteBuffersDataOutput_CopyBytes tests copying bytes from DataInput.
// Source: TestByteBuffersDataOutput.testCopyBytes()
func TestByteBuffersDataOutput_CopyBytes(t *testing.T) {
	// Create source data
	sourceData := make([]byte, 1000)
	rand.Read(sourceData)
	source := NewByteArrayDataInput(sourceData)

	o := NewByteBuffersDataOutput()
	if err := o.CopyBytes(source, 500); err != nil {
		t.Fatalf("CopyBytes failed: %v", err)
	}

	result := o.ToArrayCopy()
	if !bytes.Equal(result, sourceData[:500]) {
		t.Error("Data mismatch after CopyBytes")
	}
}

// TestByteBuffersDataOutput_CopyTo tests copying to another DataOutput.
// Source: TestByteBuffersDataOutput.testCopyTo()
func TestByteBuffersDataOutput_CopyTo(t *testing.T) {
	// Create source data
	sourceData := make([]byte, 1000)
	rand.Read(sourceData)

	source := NewByteBuffersDataOutput()
	source.WriteBytes(sourceData)

	target := NewByteBuffersDataOutput()
	if err := source.CopyTo(target); err != nil {
		t.Fatalf("CopyTo failed: %v", err)
	}

	result := target.ToArrayCopy()
	if !bytes.Equal(result, sourceData) {
		t.Error("Data mismatch after CopyTo")
	}
}

// TestByteBuffersDataOutput_BlockExpansion tests block expansion when many blocks are created.
// Source: TestByteBuffersDataOutput.testBlockExpansion()
func TestByteBuffersDataOutput_BlockExpansion(t *testing.T) {
	// Write enough data to trigger block expansion
	data := make([]byte, (1<<DefaultMinBitsPerBlock)*MaxBlocksBeforeBlockExpansion+1000)
	rand.Read(data)

	o := NewByteBuffersDataOutput()
	o.WriteBytes(data)

	result := o.ToArrayCopy()
	if !bytes.Equal(result, data) {
		t.Error("Data mismatch after block expansion")
	}
}

// TestByteBuffersDataOutput_Empty tests empty output behavior.
// Source: TestByteBuffersDataOutput.testEmpty()
func TestByteBuffersDataOutput_Empty(t *testing.T) {
	o := NewByteBuffersDataOutput()

	if o.Size() != 0 {
		t.Errorf("Expected size 0, got %d", o.Size())
	}

	result := o.ToArrayCopy()
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d bytes", len(result))
	}

	buffers := o.ToBufferList()
	if len(buffers) != 1 || buffers[0].Len() != 0 {
		t.Error("Expected one empty buffer")
	}
}

// TestByteBuffersDataOutput_SingleByte tests writing a single byte.
// Source: TestByteBuffersDataOutput.testSingleByte()
func TestByteBuffersDataOutput_SingleByte(t *testing.T) {
	o := NewByteBuffersDataOutput()
	o.WriteByte(42)

	result := o.ToArrayCopy()
	if len(result) != 1 || result[0] != 42 {
		t.Errorf("Expected [42], got %v", result)
	}
}

// TestByteBuffersDataOutput_BufferCount tests buffer count calculation.
// Source: TestByteBuffersDataOutput.testBufferCount()
func TestByteBuffersDataOutput_BufferCount(t *testing.T) {
	o := NewByteBuffersDataOutput()

	if o.BufferCount() != 0 {
		t.Errorf("Expected 0 buffers initially, got %d", o.BufferCount())
	}

	// Write data to create multiple blocks
	blockSize := 1 << DefaultMinBitsPerBlock
	data := make([]byte, blockSize*3+100)
	rand.Read(data)
	o.WriteBytes(data)

	// Should have multiple buffers
	if o.BufferCount() < 3 {
		t.Errorf("Expected at least 3 buffers, got %d", o.BufferCount())
	}
}

// TestByteBuffersDataOutput_BlockCapacity tests block capacity reporting.
// Source: TestByteBuffersDataOutput.testBlockCapacity()
func TestByteBuffersDataOutput_BlockCapacity(t *testing.T) {
	o := NewByteBuffersDataOutput()

	// Write some data
	data := make([]byte, 100)
	rand.Read(data)
	o.WriteBytes(data)

	capacity := o.BlockCapacity(0)
	if capacity < 100 {
		t.Errorf("Expected capacity >= 100, got %d", capacity)
	}

	// Invalid index should return 0
	capacity = o.BlockCapacity(1000)
	if capacity != 0 {
		t.Errorf("Expected capacity 0 for invalid index, got %d", capacity)
	}
}
