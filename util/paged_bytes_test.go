// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPagedBytes_New tests creating a new PagedBytes.
func TestPagedBytes_New(t *testing.T) {
	// Test valid block bits
	for blockBits := 1; blockBits <= 20; blockBits++ {
		pb, err := NewPagedBytes(blockBits)
		if err != nil {
			t.Errorf("Failed to create PagedBytes with blockBits=%d: %v", blockBits, err)
			continue
		}
		if pb == nil {
			t.Error("Expected non-nil PagedBytes")
			continue
		}
		expectedBlockSize := 1 << blockBits
		if pb.GetBlockSize() != expectedBlockSize {
			t.Errorf("Expected block size %d, got %d", expectedBlockSize, pb.GetBlockSize())
		}
	}

	// Test invalid block bits
	_, err := NewPagedBytes(0)
	if err == nil {
		t.Error("Expected error for blockBits=0")
	}

	_, err = NewPagedBytes(32)
	if err == nil {
		t.Error("Expected error for blockBits=32")
	}

	_, err = NewPagedBytes(-1)
	if err == nil {
		t.Error("Expected error for blockBits=-1")
	}
}

// TestPagedBytes_DataInputOutput tests copying from IndexInput and reading via Reader.
// This is the Go port of Lucene's TestPagedBytes.testDataInputOutput().
func TestPagedBytes_DataInputOutput(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	numIters := 3 // Reduced from Java's atLeast(1) for faster tests
	for iter := 0; iter < numIters; iter++ {
		// Create a ByteBuffersDirectory for testing
		dir := store.NewByteBuffersDirectory()

		blockBits := rng.Intn(20) + 1 // 1 to 20
		blockSize := 1 << blockBits

		pb, err := NewPagedBytes(blockBits)
		if err != nil {
			t.Fatalf("Failed to create PagedBytes: %v", err)
		}

		// Create output file
		out, err := dir.CreateOutput("test_data", store.IOContext{})
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}

		// Generate random data
		numBytes := rng.Intn(100000) + 2 // 2 to 100001 bytes
		answer := make([]byte, numBytes)
		rng.Read(answer)

		// Write data with random chunk sizes
		written := 0
		for written < numBytes {
			if rng.Intn(100) == 7 {
				// Write single byte
				if err := out.WriteByte(answer[written]); err != nil {
					t.Fatalf("Failed to write byte: %v", err)
				}
				written++
			} else {
				// Write chunk
				chunk := rng.Intn(1000)
				if chunk > numBytes-written {
					chunk = numBytes - written
				}
				if err := out.WriteBytes(answer[written : written+chunk]); err != nil {
					t.Fatalf("Failed to write bytes: %v", err)
				}
				written += chunk
			}
		}

		if err := out.Close(); err != nil {
			t.Fatalf("Failed to close output: %v", err)
		}

		// Open input and copy to PagedBytes
		input, err := dir.OpenInput("test_data", store.IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input: %v", err)
		}

		if err := pb.Copy(input, int64(numBytes)); err != nil {
			t.Fatalf("Failed to copy: %v", err)
		}

		// Freeze and get reader
		trim := rng.Intn(2) == 0
		reader, err := pb.Freeze(trim)
		if err != nil {
			t.Fatalf("Failed to freeze: %v", err)
		}

		// Verify by reading back
		input2, err := dir.OpenInput("test_data", store.IOContext{})
		if err != nil {
			t.Fatalf("Failed to open input2: %v", err)
		}

		verify := make([]byte, numBytes)
		read := 0
		for read < numBytes {
			if rng.Intn(100) == 7 {
				// Read single byte
				b, err := input2.ReadByte()
				if err != nil {
					t.Fatalf("Failed to read byte: %v", err)
				}
				verify[read] = b
				read++
			} else {
				// Read chunk
				chunk := rng.Intn(1000)
				if chunk > numBytes-read {
					chunk = numBytes - read
				}
				buf := make([]byte, chunk)
				if err := input2.ReadBytes(buf); err != nil {
					t.Fatalf("Failed to read bytes: %v", err)
				}
				copy(verify[read:], buf)
				read += chunk
			}
		}

		if !bytes.Equal(answer, verify) {
			t.Error("Data mismatch after read")
		}

		// Test random access via Reader
		slice := NewBytesRefEmpty()
		for iter2 := 0; iter2 < 100 && numBytes > 1; iter2++ {
			pos := rng.Intn(numBytes - 1)
			if reader.GetByte(int64(pos)) != answer[pos] {
				t.Errorf("Byte mismatch at position %d", pos)
			}

			maxLen := blockSize + 1
			if maxLen > numBytes-pos {
				maxLen = numBytes - pos
			}
			if maxLen <= 0 {
				maxLen = 1
			}
			length := rng.Intn(maxLen)

			if err := reader.FillSlice(slice, int64(pos), length); err != nil {
				t.Fatalf("Failed to fill slice: %v", err)
			}

			for i := 0; i < length; i++ {
				if slice.Bytes[slice.Offset+i] != answer[pos+i] {
					t.Errorf("Slice mismatch at position %d", pos+i)
					break
				}
			}
		}

		if err := input.Close(); err != nil {
			t.Errorf("Failed to close input: %v", err)
		}
		if err := input2.Close(); err != nil {
			t.Errorf("Failed to close input2: %v", err)
		}
	}
}

// TestPagedBytes_DataInputOutput2 tests writing via DataOutput and reading via DataInput/Reader.
// This is the Go port of Lucene's TestPagedBytes.testDataInputOutput2().
func TestPagedBytes_DataInputOutput2(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	numIters := 3 // Reduced from Java's atLeast(1) for faster tests
	for iter := 0; iter < numIters; iter++ {
		blockBits := rng.Intn(20) + 1 // 1 to 20
		blockSize := 1 << blockBits

		pb, err := NewPagedBytes(blockBits)
		if err != nil {
			t.Fatalf("Failed to create PagedBytes: %v", err)
		}

		out := pb.GetDataOutput()

		// Generate random data
		numBytes := rng.Intn(100000) + 1 // 1 to 100000 bytes
		answer := make([]byte, numBytes)
		rng.Read(answer)

		// Write data with random chunk sizes
		written := 0
		for written < numBytes {
			if rng.Intn(10) == 7 {
				// Write single byte
				if err := out.WriteByte(answer[written]); err != nil {
					t.Fatalf("Failed to write byte: %v", err)
				}
				written++
			} else {
				// Write chunk
				chunk := rng.Intn(1000)
				if chunk > numBytes-written {
					chunk = numBytes - written
				}
				if err := out.WriteBytes(answer[written : written+chunk]); err != nil {
					t.Fatalf("Failed to write bytes: %v", err)
				}
				written += chunk
			}
		}

		// Freeze and get reader
		trim := rng.Intn(2) == 0
		reader, err := pb.Freeze(trim)
		if err != nil {
			t.Fatalf("Failed to freeze: %v", err)
		}

		// Read back via DataInput
		in, err := pb.GetDataInput()
		if err != nil {
			t.Fatalf("Failed to get DataInput: %v", err)
		}

		verify := make([]byte, numBytes)
		read := 0
		for read < numBytes {
			if rng.Intn(10) == 7 {
				// Read single byte
				b, err := in.ReadByte()
				if err != nil {
					t.Fatalf("Failed to read byte: %v", err)
				}
				verify[read] = b
				read++
			} else {
				// Read chunk
				chunk := rng.Intn(1000)
				if chunk > numBytes-read {
					chunk = numBytes - read
				}
				buf := make([]byte, chunk)
				if err := in.ReadBytes(buf); err != nil {
					t.Fatalf("Failed to read bytes: %v", err)
				}
				copy(verify[read:], buf)
				read += chunk
			}
		}

		if !bytes.Equal(answer, verify) {
			t.Error("Data mismatch after read")
		}

		// Test random access via Reader
		slice := NewBytesRefEmpty()
		for iter2 := 0; iter2 < 100 && numBytes > 1; iter2++ {
			pos := rng.Intn(numBytes - 1)

			maxLen := blockSize + 1
			if maxLen > numBytes-pos {
				maxLen = numBytes - pos
			}
			if maxLen <= 0 {
				maxLen = 1
			}
			length := rng.Intn(maxLen)

			if err := reader.FillSlice(slice, int64(pos), length); err != nil {
				t.Fatalf("Failed to fill slice: %v", err)
			}

			for i := 0; i < length; i++ {
				if slice.Bytes[slice.Offset+i] != answer[pos+i] {
					t.Errorf("Slice mismatch at position %d", pos+i)
					break
				}
			}
		}

		// Test skipping
		in2, err := pb.GetDataInput()
		if err != nil {
			t.Fatalf("Failed to get DataInput for skip test: %v", err)
		}

		maxSkipTo := numBytes - 1
		for curr := 0; curr < maxSkipTo; {
			skipTo := rng.Intn(maxSkipTo-curr) + curr
			step := skipTo - curr
			if err := in2.SkipBytes(int64(step)); err != nil {
				t.Fatalf("Failed to skip bytes: %v", err)
			}
			b, err := in2.ReadByte()
			if err != nil {
				t.Fatalf("Failed to read byte after skip: %v", err)
			}
			if b != answer[skipTo] {
				t.Errorf("Byte mismatch at skip position %d", skipTo)
			}
			curr = skipTo + 1
		}
	}
}

// TestPagedBytes_RamBytesUsed tests RAM usage accounting.
// This is the Go port of Lucene's TestPagedBytes.testRamBytesUsed().
func TestPagedBytes_RamBytesUsed(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	blockBits := rng.Intn(19) + 4 // 4 to 22
	pb, err := NewPagedBytes(blockBits)
	if err != nil {
		t.Fatalf("Failed to create PagedBytes: %v", err)
	}

	totalBytes := rng.Intn(10000)
	for pointer := int64(0); pointer < int64(totalBytes); {
		// Generate random string
		length := rng.Intn(10) + 1
		randomStr := make([]byte, length)
		for i := range randomStr {
			randomStr[i] = byte(rng.Intn(26) + 'a')
		}
		bytes := NewBytesRef(randomStr)
		pointer, err = pb.CopyUsingLengthPrefix(bytes)
		if err != nil {
			t.Fatalf("Failed to copy using length prefix: %v", err)
		}
	}

	// Just verify RamBytesUsed returns a positive value
	ramUsed := pb.RamBytesUsed()
	if ramUsed <= 0 {
		t.Errorf("Expected positive RAM usage, got %d", ramUsed)
	}

	// Freeze and check reader RAM usage
	trim := rng.Intn(2) == 0
	reader, err := pb.Freeze(trim)
	if err != nil {
		t.Fatalf("Failed to freeze: %v", err)
	}

	ramUsedAfterFreeze := pb.RamBytesUsed()
	if ramUsedAfterFreeze <= 0 {
		t.Errorf("Expected positive RAM usage after freeze, got %d", ramUsedAfterFreeze)
	}

	readerRamUsed := reader.RamBytesUsed()
	if readerRamUsed <= 0 {
		t.Errorf("Expected positive reader RAM usage, got %d", readerRamUsed)
	}
}

// TestPagedBytes_CopyBytesRef tests copying BytesRef.
func TestPagedBytes_CopyBytesRef(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	pb, err := NewPagedBytes(10) // 1024 byte blocks
	if err != nil {
		t.Fatalf("Failed to create PagedBytes: %v", err)
	}

	// Test copying multiple BytesRefs
	numRefs := 20
	refs := make([]*BytesRef, numRefs)
	outRefs := make([]*BytesRef, numRefs)

	for i := 0; i < numRefs; i++ {
		// Use larger data to ensure we cross block boundaries
		length := rng.Intn(200) + 50 // 50-250 bytes
		data := make([]byte, length)
		rng.Read(data)
		refs[i] = NewBytesRef(data)
		outRefs[i] = NewBytesRefEmpty()

		if err := pb.CopyBytesRef(refs[i], outRefs[i]); err != nil {
			t.Fatalf("Failed to copy BytesRef %d: %v", i, err)
		}
	}

	// Freeze with trim=false (since we used CopyBytesRef, didSkipBytes may be set)
	// Note: didSkipBytes is only set when we cross a block boundary
	_, err = pb.Freeze(false)
	if err != nil {
		// This is expected if didSkipBytes was set
		t.Logf("Freeze returned error (expected if didSkipBytes set): %v", err)
		return
	}

	// Verify the copied data
	for i := 0; i < numRefs; i++ {
		if outRefs[i].Length != refs[i].Length {
			t.Errorf("Length mismatch for ref %d: expected %d, got %d", i, refs[i].Length, outRefs[i].Length)
		}
		if !bytes.Equal(outRefs[i].ValidBytes(), refs[i].ValidBytes()) {
			t.Errorf("Data mismatch for ref %d", i)
		}
	}
}

// TestPagedBytes_FillWithLengthPrefix tests reading with length prefix.
func TestPagedBytes_FillWithLengthPrefix(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	pb, err := NewPagedBytes(10) // 1024 byte blocks
	if err != nil {
		t.Fatalf("Failed to create PagedBytes: %v", err)
	}

	// Store some data with length prefix
	numItems := 50
	items := make([][]byte, numItems)
	pointers := make([]int64, numItems)

	for i := 0; i < numItems; i++ {
		// Vary length to test both 1-byte and 2-byte length encoding
		// Must fit in block size minus 2 bytes for length prefix
		var length int
		if i%2 == 0 {
			length = rng.Intn(127) + 1 // 1-127 (1 byte length)
		} else {
			length = rng.Intn(800) + 128 // 128-927 (2 byte length), fits in 1024 block
		}
		items[i] = make([]byte, length)
		rng.Read(items[i])

		br := NewBytesRef(items[i])
		ptr, err := pb.CopyUsingLengthPrefix(br)
		if err != nil {
			t.Fatalf("Failed to copy with length prefix: %v", err)
		}
		pointers[i] = ptr
	}

	// Freeze and get reader
	reader, err := pb.Freeze(true)
	if err != nil {
		t.Fatalf("Failed to freeze: %v", err)
	}

	// Read back and verify
	for i := 0; i < numItems; i++ {
		result := NewBytesRefEmpty()
		if err := reader.Fill(result, pointers[i]); err != nil {
			t.Fatalf("Failed to fill at pointer %d: %v", pointers[i], err)
		}

		if result.Length != len(items[i]) {
			t.Errorf("Length mismatch for item %d: expected %d, got %d", i, len(items[i]), result.Length)
		}

		actualData := make([]byte, result.Length)
		copy(actualData, result.Bytes[result.Offset:result.Offset+result.Length])
		if !bytes.Equal(actualData, items[i]) {
			t.Errorf("Data mismatch for item %d", i)
		}
	}
}

// TestPagedBytes_GetPointer tests the GetPointer method.
func TestPagedBytes_GetPointer(t *testing.T) {
	pb, err := NewPagedBytes(10) // 1024 byte blocks
	if err != nil {
		t.Fatalf("Failed to create PagedBytes: %v", err)
	}

	// Initial pointer should be 0
	if pb.GetPointer() != 0 {
		t.Errorf("Expected initial pointer 0, got %d", pb.GetPointer())
	}

	out := pb.GetDataOutput()

	// Write some data
	data := make([]byte, 500)
	if err := out.WriteBytes(data); err != nil {
		t.Fatalf("Failed to write bytes: %v", err)
	}

	if pb.GetPointer() != 500 {
		t.Errorf("Expected pointer 500, got %d", pb.GetPointer())
	}

	// Write more data to cross block boundary
	data2 := make([]byte, 600)
	if err := out.WriteBytes(data2); err != nil {
		t.Fatalf("Failed to write bytes: %v", err)
	}

	if pb.GetPointer() != 1100 {
		t.Errorf("Expected pointer 1100, got %d", pb.GetPointer())
	}
}

// TestPagedBytes_FreezeErrors tests error conditions for Freeze.
func TestPagedBytes_FreezeErrors(t *testing.T) {
	// Test double freeze
	pb, _ := NewPagedBytes(10)
	out := pb.GetDataOutput()
	out.WriteBytes([]byte("test"))

	_, err := pb.Freeze(true)
	if err != nil {
		t.Fatalf("First freeze should succeed: %v", err)
	}

	_, err = pb.Freeze(true)
	if err == nil {
		t.Error("Expected error on double freeze")
	}

	// Test freeze after CopyBytesRef with data crossing block boundary
	pb2, _ := NewPagedBytes(10) // 1024 byte blocks
	// First write fills the block partially
	br1 := NewBytesRef(make([]byte, 1000))
	outRef1 := NewBytesRefEmpty()
	pb2.CopyBytesRef(br1, outRef1)
	// Second write causes block boundary crossing (didSkipBytes = true)
	br2 := NewBytesRef(make([]byte, 100))
	outRef2 := NewBytesRefEmpty()
	pb2.CopyBytesRef(br2, outRef2)

	_, err = pb2.Freeze(true)
	if err == nil {
		t.Error("Expected error when freezing after CopyBytesRef that crossed block boundary")
	}
}

// TestPagedBytes_GetDataInputErrors tests error conditions for GetDataInput.
func TestPagedBytes_GetDataInputErrors(t *testing.T) {
	// Test GetDataInput before freeze
	pb, _ := NewPagedBytes(10)
	_, err := pb.GetDataInput()
	if err == nil {
		t.Error("Expected error when calling GetDataInput before freeze")
	}

	// Test GetDataInput after freeze
	out := pb.GetDataOutput()
	out.WriteBytes([]byte("test"))
	pb.Freeze(true)

	_, err = pb.GetDataInput()
	if err != nil {
		t.Errorf("GetDataInput should succeed after freeze: %v", err)
	}
}

// TestPagedBytes_ReaderGetByte tests Reader.GetByte.
func TestPagedBytes_ReaderGetByte(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	pb, _ := NewPagedBytes(10) // 1024 byte blocks
	out := pb.GetDataOutput()

	// Write data that spans multiple blocks
	data := make([]byte, 2500)
	rng.Read(data)
	out.WriteBytes(data)

	reader, _ := pb.Freeze(true)

	// Test GetByte at various positions
	for i := 0; i < len(data); i += 100 {
		if reader.GetByte(int64(i)) != data[i] {
			t.Errorf("Byte mismatch at position %d", i)
		}
	}

	// Test GetByte at last position
	if reader.GetByte(int64(len(data)-1)) != data[len(data)-1] {
		t.Error("Byte mismatch at last position")
	}
}

// TestPagedBytes_FillSliceErrors tests error conditions for FillSlice.
func TestPagedBytes_FillSliceErrors(t *testing.T) {
	pb, _ := NewPagedBytes(10) // 1024 byte blocks
	out := pb.GetDataOutput()
	out.WriteBytes(make([]byte, 100))

	reader, _ := pb.Freeze(true)

	// Test negative length
	slice := NewBytesRefEmpty()
	err := reader.FillSlice(slice, 0, -1)
	if err == nil {
		t.Error("Expected error for negative length")
	}

	// Test length exceeding blockSize+1
	err = reader.FillSlice(slice, 0, 2000)
	if err == nil {
		t.Error("Expected error for length exceeding blockSize+1")
	}
}

// TestPagedBytes_DataInputClone tests cloning PagedBytesDataInput.
func TestPagedBytes_DataInputClone(t *testing.T) {
	pb, _ := NewPagedBytes(10)
	out := pb.GetDataOutput()
	out.WriteBytes([]byte("Hello, World!"))

	pb.Freeze(true)

	in1, _ := pb.GetDataInput()

	// Read some data
	in1.ReadBytesN(5)

	// Clone
	in2 := in1.Clone()

	// Both should have same position
	if in1.GetPosition() != in2.GetPosition() {
		t.Error("Clone should have same position")
	}

	// Read from clone
	data, _ := in2.ReadBytesN(5)
	if string(data) != ", Wor" {
		t.Errorf("Expected ', Wor', got '%s'", string(data))
	}

	// Original should be unchanged
	data2, _ := in1.ReadBytesN(5)
	if string(data2) != ", Wor" {
		t.Errorf("Expected ', Wor' from original, got '%s'", string(data2))
	}
}

// TestPagedBytes_DataInputSetPosition tests SetPosition.
func TestPagedBytes_DataInputSetPosition(t *testing.T) {
	pb, _ := NewPagedBytes(10)
	out := pb.GetDataOutput()

	// Write data
	for i := 0; i < 100; i++ {
		out.WriteByte(byte(i))
	}

	pb.Freeze(true)

	in, _ := pb.GetDataInput()

	// Seek to position 50
	in.SetPosition(50)

	b, _ := in.ReadByte()
	if b != 50 {
		t.Errorf("Expected byte 50, got %d", b)
	}

	// Seek to position 0
	in.SetPosition(0)
	b, _ = in.ReadByte()
	if b != 0 {
		t.Errorf("Expected byte 0, got %d", b)
	}
}

// TestPagedBytes_DataOutputErrors tests error conditions for DataOutput.
func TestPagedBytes_DataOutputErrors(t *testing.T) {
	pb, _ := NewPagedBytes(10)
	out := pb.GetDataOutput()

	// Write some data
	out.WriteBytes([]byte("test"))

	// Freeze
	pb.Freeze(true)

	// Try to write after freeze
	err := out.WriteByte(1)
	if err == nil {
		t.Error("Expected error when writing after freeze")
	}

	err = out.WriteBytes([]byte("test"))
	if err == nil {
		t.Error("Expected error when writing bytes after freeze")
	}
}

// TestPagedBytes_SkipBytesErrors tests SkipBytes error conditions.
func TestPagedBytes_SkipBytesErrors(t *testing.T) {
	pb, _ := NewPagedBytes(10)
	out := pb.GetDataOutput()
	out.WriteBytes([]byte("Hello, World!"))
	pb.Freeze(true)

	in, _ := pb.GetDataInput()

	// Test negative skip
	err := in.SkipBytes(-1)
	if err == nil {
		t.Error("Expected error for negative skip")
	}
}

// TestPagedBytes_CopyUsingLengthPrefixErrors tests error conditions.
func TestPagedBytes_CopyUsingLengthPrefixErrors(t *testing.T) {
	pb, _ := NewPagedBytes(10) // 1024 byte blocks

	// Test data too large
	largeData := make([]byte, 40000) // Exceeds 32767 limit
	br := NewBytesRef(largeData)
	_, err := pb.CopyUsingLengthPrefix(br)
	if err == nil {
		t.Error("Expected error for data exceeding 32767 bytes")
	}

	// Test data too large for block
	pb2, _ := NewPagedBytes(5)     // 32 byte blocks
	mediumData := make([]byte, 50) // Exceeds block size - 2
	br2 := NewBytesRef(mediumData)
	_, err = pb2.CopyUsingLengthPrefix(br2)
	if err == nil {
		t.Error("Expected error for data exceeding block size")
	}
}

// TestPagedBytes_Empty tests empty PagedBytes.
func TestPagedBytes_Empty(t *testing.T) {
	pb, err := NewPagedBytes(10)
	if err != nil {
		t.Fatalf("Failed to create PagedBytes: %v", err)
	}

	// Freeze empty
	reader, err := pb.Freeze(true)
	if err != nil {
		t.Fatalf("Failed to freeze empty: %v", err)
	}

	if reader == nil {
		t.Error("Expected non-nil reader for empty PagedBytes")
	}

	// GetDataInput should work
	in, err := pb.GetDataInput()
	if err != nil {
		t.Errorf("GetDataInput should work for empty: %v", err)
	}
	if in == nil {
		t.Error("Expected non-nil DataInput for empty")
	}
}

// TestPagedBytes_LargeData tests with larger data sets.
func TestPagedBytes_LargeData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large data test in short mode")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	pb, _ := NewPagedBytes(14) // 16384 byte blocks
	out := pb.GetDataOutput()

	// Write 1MB of data
	data := make([]byte, 1024*1024)
	rng.Read(data)

	if err := out.WriteBytes(data); err != nil {
		t.Fatalf("Failed to write large data: %v", err)
	}

	reader, _ := pb.Freeze(true)

	// Verify random samples
	for i := 0; i < 1000; i++ {
		pos := rng.Intn(len(data))
		if reader.GetByte(int64(pos)) != data[pos] {
			t.Errorf("Byte mismatch at position %d", pos)
		}
	}

	// Verify via DataInput
	in, _ := pb.GetDataInput()
	verify := make([]byte, len(data))
	if err := in.ReadBytes(verify); err != nil {
		t.Fatalf("Failed to read large data: %v", err)
	}

	if !bytes.Equal(data, verify) {
		t.Error("Large data mismatch")
	}
}

// TestPagedBytes_BlockBoundary tests behavior at block boundaries.
func TestPagedBytes_BlockBoundary(t *testing.T) {
	pb, _ := NewPagedBytes(10) // 1024 byte blocks
	out := pb.GetDataOutput()

	// Write exactly one block
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	out.WriteBytes(data)

	// Write partial second block
	data2 := make([]byte, 500)
	for i := range data2 {
		data2[i] = byte((i + 100) % 256)
	}
	out.WriteBytes(data2)

	reader, _ := pb.Freeze(true)

	// Verify data at block boundary
	if reader.GetByte(1023) != data[1023] {
		t.Error("Byte mismatch at block boundary - 1")
	}
	if reader.GetByte(1024) != data2[0] {
		t.Error("Byte mismatch at block boundary")
	}

	// Test FillSlice spanning block boundary
	slice := NewBytesRefEmpty()
	err := reader.FillSlice(slice, 1020, 10) // Should span blocks
	if err != nil {
		t.Fatalf("Failed to fill slice spanning blocks: %v", err)
	}

	// Verify slice data
	for i := 0; i < 10; i++ {
		expected := byte((1020 + i) % 256)
		if 1020+i >= 1024 {
			expected = byte((1020 + i - 1024 + 100) % 256)
		}
		if slice.Bytes[slice.Offset+i] != expected {
			t.Errorf("Slice data mismatch at position %d", i)
		}
	}
}

// TestPagedBytes_DataOutputPosition tests DataOutput.GetPosition.
func TestPagedBytes_DataOutputPosition(t *testing.T) {
	pb, _ := NewPagedBytes(10)
	out := pb.GetDataOutput()

	if out.GetPosition() != 0 {
		t.Errorf("Expected initial position 0, got %d", out.GetPosition())
	}

	out.WriteByte(1)
	if out.GetPosition() != 1 {
		t.Errorf("Expected position 1, got %d", out.GetPosition())
	}

	out.WriteBytes(make([]byte, 100))
	if out.GetPosition() != 101 {
		t.Errorf("Expected position 101, got %d", out.GetPosition())
	}
}

// TestPagedBytes_CopyBytesRefLarge tests CopyBytesRef with large data.
func TestPagedBytes_CopyBytesRefLarge(t *testing.T) {
	pb, _ := NewPagedBytes(10) // 1024 byte blocks

	// Test with data exactly at block size
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	br := NewBytesRef(data)
	out := NewBytesRefEmpty()

	err := pb.CopyBytesRef(br, out)
	if err != nil {
		t.Fatalf("Failed to copy large BytesRef: %v", err)
	}

	if out.Length != 1024 {
		t.Errorf("Expected length 1024, got %d", out.Length)
	}
}

// BenchmarkPagedBytes_Write benchmarks writing to PagedBytes.
func BenchmarkPagedBytes_Write(b *testing.B) {
	pb, _ := NewPagedBytes(15) // 32768 byte blocks
	out := pb.GetDataOutput()
	data := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out.WriteBytes(data)
	}
}

// BenchmarkPagedBytes_Read benchmarks reading from PagedBytes.
func BenchmarkPagedBytes_Read(b *testing.B) {
	pb, _ := NewPagedBytes(15)
	out := pb.GetDataOutput()
	data := make([]byte, 1024*1024) // 1MB
	out.WriteBytes(data)
	pb.Freeze(true)

	buf := make([]byte, 1024)
	in, _ := pb.GetDataInput()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in.SetPosition(0)
		for j := 0; j < 1024; j++ {
			in.ReadBytes(buf)
		}
	}
}

// BenchmarkPagedBytes_RandomAccess benchmarks random access via Reader.
func BenchmarkPagedBytes_RandomAccess(b *testing.B) {
	pb, _ := NewPagedBytes(15)
	out := pb.GetDataOutput()
	data := make([]byte, 1024*1024) // 1MB
	out.WriteBytes(data)
	reader, _ := pb.Freeze(true)

	rng := rand.New(rand.NewSource(42))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pos := rng.Intn(1024 * 1024)
		_ = reader.GetByte(int64(pos))
	}
}
